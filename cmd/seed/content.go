package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedContentAndCurriculum creates and publishes the seed course, 8 lessons,
// activity content items and versions, and 200 vocabulary words and senses.
//
// Re-running is idempotent: courses, units, lessons, activities, words and senses
// are refreshed in place. Content *versions* are not, because a published version
// is immutable — the API cannot edit one either, and a seed that could would be
// producing a state the product cannot. Changing an activity's body means a new
// version, which is authoring work, not seeding.
//
// This writes SQL directly rather than driving the HTTP API. P11 §3 permits that
// for a developer seed on one condition: the rows must be a state the API could
// also have produced, and something must check the claim rather than assert it.
// ensureContentItemAndVersion is where that condition is met, and
// TestSeededContentReachesAPIProducibleState is the check.
func seedContentAndCurriculum(ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "Seeding curriculum (course, lessons, activities) and vocabulary...")

	// 1. Seed Course, Units, Lessons, and Activities
	if err := seedCourseData(ctx, pool, adminID, courseSeedData); err != nil {
		return fmt.Errorf("seed course data: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Course: %s (8 lessons, %d units)\n", courseSeedData.Title, len(courseSeedData.Units))

	if err := seedCourseData(ctx, pool, adminID, readingCourseSeedData); err != nil {
		return fmt.Errorf("seed reading course data: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Course: %s (6 lessons, %d units)\n",
		readingCourseSeedData.Title, len(readingCourseSeedData.Units))

	if err := seedCourseData(ctx, pool, adminID, writingCourseSeedData); err != nil {
		return fmt.Errorf("seed writing course data: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Course: %s (6 lessons, %d units)\n",
		writingCourseSeedData.Title, len(writingCourseSeedData.Units))

	if err := seedCourseData(ctx, pool, adminID, speakingCourseSeedData); err != nil {
		return fmt.Errorf("seed speaking course data: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Course: %s (6 lessons, %d units)\n",
		speakingCourseSeedData.Title, len(speakingCourseSeedData.Units))

	// The clips these lessons need are rendered afterwards by `make tts`, which
	// walks every published listening_comprehension script. Until it has run,
	// the items exist and the play route answers AUDIO_NOT_READY.
	if err := seedCourseData(ctx, pool, adminID, listeningCourseSeedData); err != nil {
		return fmt.Errorf("seed listening course data: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Course: %s (6 lessons, %d units) — run `make tts` to render the audio\n",
		listeningCourseSeedData.Title, len(listeningCourseSeedData.Units))

	// 2. Seed vocabulary: the frozen 10,000-word fixture when it is there, the
	// curated 200 when it is not.
	count, fromFixture, err := seedVocabularyFromFixture(
		ctx, pool, adminID, defaultVocabularyFixtureDir, out)
	if err != nil {
		return fmt.Errorf("seed vocabulary fixtures: %w", err)
	}
	if fromFixture {
		_, _ = fmt.Fprintf(out, "  ✓ Vocabulary: %d words loaded from the frozen fixture\n", count)
	} else {
		count, err = seedVocabularyWords(ctx, pool, adminID, wordSenseSeedData, curatedDeckFor)
		if err != nil {
			return fmt.Errorf("seed vocabulary words: %w", err)
		}
		_, _ = fmt.Fprintf(out, "  ✓ Vocabulary: %d word senses seeded & linked into curated deck\n", count)
	}

	return nil
}

// The keys an authored content body uses. They are the contract between the seed
// and vocabulary's grader, which reads correct_answer and acceptable.
const (
	bodyKeyPrompt        = "prompt"
	bodyKeyCorrectAnswer = "correct_answer"
	bodyKeyAcceptable    = "acceptable"
	bodyKeyDefinition    = "definition"

	// The fields the review screen renders a flashcard from. They are not
	// decoration: web/src/features/review/model/flashcard.ts returns null unless
	// `word` and `definition` are both present, and a null there is the
	// "This card has no content yet" state. Every sense body was authored
	// without `word`, so every card a learner earned rendered as unavailable.
	bodyKeyWord            = "word"
	bodyKeyPOS             = "pos"
	bodyKeyIPA             = "ipa"
	bodyKeyDefinitionVI    = "definition_vi"
	bodyKeyExampleSentence = "example_sentence"

	// The full list, as `{sentence, sentence_vi}` objects — the same shape
	// domain.ExampleSentence and the ExampleSentence schema already define, so
	// one reader serves the dictionary API and the flashcard alike.
	//
	// `example_sentence` stays beside it carrying the first sentence as a bare
	// string, because it is the field the published OpenAPI examples show and
	// the field any content authored before this key existed still uses.
	bodyKeyExampleSentences = "example_sentences"
)

func seedCourseData(ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, c seedCourse) error {
	// Upsert course
	var courseID uuid.UUID
	const upsertCourse = `
		INSERT INTO learn.courses (slug, title, description, cefr_from, cefr_to, status, estimated_hours, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'published', $6, now())
		ON CONFLICT (slug) DO UPDATE
		SET title = EXCLUDED.title,
		    description = EXCLUDED.description,
		    cefr_from = EXCLUDED.cefr_from,
		    cefr_to = EXCLUDED.cefr_to,
		    status = 'published',
		    estimated_hours = EXCLUDED.estimated_hours,
		    updated_at = now()
		RETURNING id`
	if err := pool.QueryRow(ctx, upsertCourse, c.Slug, c.Title, c.Description, c.CEFRFrom, c.CEFRTo, c.EstimatedHours).
		Scan(&courseID); err != nil {
		return fmt.Errorf("upsert course %s: %w", c.Slug, err)
	}

	for _, unit := range c.Units {
		var unitID uuid.UUID
		const upsertUnit = `
			INSERT INTO learn.course_units (course_id, position, title, description, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (course_id, position) DO UPDATE
			SET title = EXCLUDED.title,
			    description = EXCLUDED.description,
			    updated_at = now()
			RETURNING id`
		if err := pool.QueryRow(ctx, upsertUnit, courseID, unit.Position, unit.Title, unit.Description).
			Scan(&unitID); err != nil {
			return fmt.Errorf("upsert unit %s pos %d: %w", c.Slug, unit.Position, err)
		}

		for _, lesson := range unit.Lessons {
			var lessonID uuid.UUID
			lessonCEFR := determineLessonCEFR(c, unit, lesson)
			const upsertLesson = `
				INSERT INTO learn.lessons (unit_id, position, title, skill_focus, estimated_minutes, status, cefr_level, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'published', $6, now())
				ON CONFLICT (unit_id, position) DO UPDATE
				SET title = EXCLUDED.title,
				    skill_focus = EXCLUDED.skill_focus,
				    estimated_minutes = EXCLUDED.estimated_minutes,
				    status = 'published',
				    cefr_level = EXCLUDED.cefr_level,
				    updated_at = now()
				RETURNING id`
			err := pool.QueryRow(ctx, upsertLesson,
				unitID, lesson.Position, lesson.Title, lesson.SkillFocus, lesson.EstimatedMinutes, lessonCEFR,
			).Scan(&lessonID)
			if err != nil {
				return fmt.Errorf("upsert lesson %s pos %d: %w", lesson.Title, lesson.Position, err)
			}

			for _, act := range lesson.Activities {
				actSlug := fmt.Sprintf("c-%s-u%d-l%d-act%d", c.Slug, unit.Position, lesson.Position, act.Position)
				versionID, err := ensureContentItemAndVersion(ctx, pool, adminID, actSlug, act.Kind, "A2", act.Body)
				if err != nil {
					return fmt.Errorf("ensure content version for %s: %w", actSlug, err)
				}

				configJSON, err := json.Marshal(withLearnerGloss(act.Config))
				if err != nil {
					return fmt.Errorf("marshal act config: %w", err)
				}

				// The predicate is not decoration. uq_activities_lesson_position is a
				// partial index — WHERE retired_at IS NULL, so a retired activity can
				// keep its row and its attempts while a new one takes the position —
				// and Postgres matches ON CONFLICT to a partial index only when the
				// statement repeats its predicate. Without it the seed fails with
				// "no unique or exclusion constraint matching the ON CONFLICT".
				const upsertActivity = `
					INSERT INTO learn.activities (lesson_id, position, kind, content_version_id, config, weight, updated_at)
					VALUES ($1, $2, $3, $4, $5, 1, now())
					ON CONFLICT (lesson_id, position) WHERE retired_at IS NULL DO UPDATE
					SET kind = EXCLUDED.kind,
					    content_version_id = EXCLUDED.content_version_id,
					    config = EXCLUDED.config,
					    weight = EXCLUDED.weight,
					    updated_at = now()`
				if _, err := pool.Exec(ctx, upsertActivity, lessonID, act.Position, act.Kind, versionID, configJSON); err != nil {
					return fmt.Errorf("upsert activity %s pos %d: %w", lesson.Title, act.Position, err)
				}
			}
		}
	}

	return nil
}

func ensureContentItemAndVersion(
	ctx context.Context, pool *pgxpool.Pool, ownerID uuid.UUID, slug, kind, cefr string, body map[string]any,
) (uuid.UUID, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal content body: %w", err)
	}

	// 1. Ensure content_item
	var itemID uuid.UUID
	var curVersionID *uuid.UUID
	err = pool.QueryRow(ctx, "SELECT id, current_version_id FROM content.content_items WHERE slug = $1", slug).
		Scan(&itemID, &curVersionID)
	if err != nil && !pgxIsNoRows(err) {
		return uuid.Nil, fmt.Errorf("query content item %s: %w", slug, err)
	}

	if itemID == uuid.Nil {
		const insertItem = `
			INSERT INTO content.content_items (kind, slug, status, owner_id)
			VALUES ($1, $2, 'published', $3)
			RETURNING id`
		if err := pool.QueryRow(ctx, insertItem, kind, slug, ownerID).Scan(&itemID); err != nil {
			return uuid.Nil, fmt.Errorf("insert content item %s: %w", slug, err)
		}
	}

	// 2. Ensure content_version
	var versionID uuid.UUID
	if curVersionID != nil && *curVersionID != uuid.Nil {
		versionID = *curVersionID
	} else {
		err = pool.QueryRow(ctx, "SELECT id FROM content.content_versions WHERE item_id = $1 AND version = 1", itemID).
			Scan(&versionID)
		if err != nil && !pgxIsNoRows(err) {
			return uuid.Nil, fmt.Errorf("query content version %s: %w", slug, err)
		}
	}

	if versionID == uuid.Nil {
		const insertVersion = `
			INSERT INTO content.content_versions (item_id, version, kind, body, cefr_level, status, published_at)
			VALUES ($1, 1, $2, $3, $4, 'published', now())
			RETURNING id`
		if err := pool.QueryRow(ctx, insertVersion, itemID, kind, bodyJSON, cefr).Scan(&versionID); err != nil {
			return uuid.Nil, fmt.Errorf("insert content version %s: %w", slug, err)
		}

		// The approval the authoring workflow would have left behind.
		//
		// A version only reaches `published` through draft → in_review → approved,
		// and approval writes this row. Inserting the version straight into
		// `published` without it produces a state the API has never emitted:
		// content nobody reviewed, which the first real author would then meet as
		// a bug in the workflow rather than in the seed. P11 §3 names this exactly.
		const insertReview = `
			INSERT INTO content.content_reviews (version_id, reviewer_id, decision, comments)
			VALUES ($1, $2, 'approved', 'Approved by the development seed.')`
		if _, err := pool.Exec(ctx, insertReview, versionID, ownerID); err != nil {
			return uuid.Nil, fmt.Errorf("record seed approval for %s: %w", slug, err)
		}

		// Update item current_version_id
		const linkVersion = `
			UPDATE content.content_items
			SET current_version_id = $1, status = 'published'
			WHERE id = $2`
		if _, err := pool.Exec(ctx, linkVersion, versionID, itemID); err != nil {
			return uuid.Nil, fmt.Errorf("link item version %s: %w", slug, err)
		}
	}

	return versionID, nil
}

// withLearnerGloss adds the Vietnamese meaning to an activity that names a word.
//
// The flashcard in the runner showed an English definition and nothing else, so
// a learner reading the interface in Vietnamese was asked to define an unknown
// word with more unknown words. The gloss exists — every one of these words is
// in the 200-sense vocabulary seed — it just never reached the activity.
//
// Looked up rather than authored beside each activity, because the same text
// written twice is the same text drifting: the dictionary is the one place a
// word's meaning belongs, and eight hand-copied translations would be eight
// chances to disagree with it.
//
// Returns the config unchanged when the activity names no word, when the word is
// not in the dictionary, or when the dictionary has no Vietnamese for it.
func withLearnerGloss(config map[string]any) map[string]any {
	target, ok := config[cfgTargetWord].(string)
	if !ok || strings.TrimSpace(target) == "" {
		return config
	}
	if _, already := config[bodyKeyDefinitionVI]; already {
		return config
	}

	lemma := strings.ToLower(strings.TrimSpace(target))
	for _, sense := range wordSenseSeedData {
		if strings.ToLower(sense.Lemma) != lemma || sense.DefinitionVI == "" {
			continue
		}
		enriched := make(map[string]any, len(config)+1)
		for key, value := range config {
			enriched[key] = value
		}
		enriched[bodyKeyDefinitionVI] = sense.DefinitionVI
		return enriched
	}
	return config
}

// vocabSlug is the content slug for a word sense: lower-case, with every
// character a slug cannot hold folded to a hyphen. A lemma the model wrote with
// a capital ("November") or a space would otherwise fail the slug check and
// stop the whole seed.
func vocabSlug(lemma, pos string) string {
	return "vocab-" + slugPart(lemma) + "-" + slugPart(pos)
}

func slugPart(s string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// seedDeck is a public deck a seeded word is linked into.
type seedDeck struct {
	Slug        string
	Name        string
	Description string
}

// curatedDeck is the hand-written 200's deck.
var curatedDeck = seedDeck{
	Slug:        "a2-b1-essentials",
	Name:        "A2–B1 Essential Vocabulary",
	Description: "Curated core 200 vocabulary words for intermediate fluency.",
}

// curatedDeckFor puts every hand-written word in the curated deck.
func curatedDeckFor(seedWordSense) []seedDeck { return []seedDeck{curatedDeck} }

func seedVocabularyWords(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, senses []seedWordSense,
	decksFor func(seedWordSense) []seedDeck,
) (int, error) {
	decks := &deckCache{pool: pool, ids: map[string]uuid.UUID{}, linked: map[uuid.UUID][]uuid.UUID{}}

	seededCount := 0
	for i, s := range senses {
		// 1. Ensure content version for sense
		slug := vocabSlug(s.Lemma, s.POS)
		// The dictionary entry, not just the answer key. All of this is already
		// on seedWordSense; it simply never reached the body, so the review
		// screen had a word to schedule and nothing to show for it.
		body := map[string]any{
			bodyKeyPrompt:        fmt.Sprintf("What does the word '%s' mean?", s.Lemma),
			bodyKeyCorrectAnswer: s.Lemma,
			bodyKeyAcceptable:    []string{s.Lemma},
			bodyKeyDefinition:    s.Definition,
			bodyKeyWord:          s.Lemma,
			bodyKeyPOS:           s.POS,
			bodyKeyIPA:           s.IPA,
		}
		if s.DefinitionVI != "" {
			body[bodyKeyDefinitionVI] = s.DefinitionVI
		}
		if len(s.Examples) > 0 {
			// `example_sentence` stays a bare string: it is the key the
			// published OpenAPI examples show, and the one a client written
			// before the list existed reads.
			body[bodyKeyExampleSentence] = s.Examples[0].Sentence
			body[bodyKeyExampleSentences] = s.Examples
		}
		if s.AudioURL != "" {
			body["audio_url"] = s.AudioURL
			body["audio_attribution"] = s.AudioAttribution
			body["audio_licence"] = s.AudioLicence
		}
		versionID, err := ensureContentItemAndVersion(ctx, pool, adminID, slug, "vocabulary_quiz", s.CEFRLevel, body)
		if err != nil {
			return seededCount, fmt.Errorf("ensure vocab content for %s: %w", s.Lemma, err)
		}

		// 2. Upsert word
		var wordID uuid.UUID
		rank := seedRank(s, i)
		const upsertWord = `
			INSERT INTO skill.words (lemma, pos, cefr_level, frequency_rank, ipa, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (lemma, pos) DO UPDATE
			SET cefr_level = EXCLUDED.cefr_level,
			    frequency_rank = EXCLUDED.frequency_rank,
			    ipa = EXCLUDED.ipa,
			    updated_at = now()
			RETURNING id`
		if err := pool.QueryRow(ctx, upsertWord, s.Lemma, s.POS, s.CEFRLevel, rank, s.IPA).
			Scan(&wordID); err != nil {
			return seededCount, fmt.Errorf("upsert word %s: %w", s.Lemma, err)
		}

		// 3. Upsert word_sense
		examplesJSON, err := json.Marshal(s.Examples)
		if err != nil {
			examplesJSON = []byte("[]")
		}

		var senseID uuid.UUID
		err = pool.QueryRow(ctx, "SELECT id FROM skill.word_senses WHERE word_id = $1 LIMIT 1", wordID).Scan(&senseID)
		if err != nil && !pgxIsNoRows(err) {
			return seededCount, fmt.Errorf("query word sense %s: %w", s.Lemma, err)
		}

		if senseID == uuid.Nil {
			const insertSense = `
				INSERT INTO skill.word_senses (word_id, content_version_id, definition, definition_vi, examples)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id`
			err := pool.QueryRow(ctx, insertSense,
				wordID, versionID, s.Definition, s.DefinitionVI, examplesJSON,
			).Scan(&senseID)
			if err != nil {
				return seededCount, fmt.Errorf("insert sense for %s: %w", s.Lemma, err)
			}
		} else {
			const updateSense = `
				UPDATE skill.word_senses
				SET definition = $1, definition_vi = $2, examples = $3, content_version_id = $4, updated_at = now()
				WHERE id = $5`
			_, err := pool.Exec(ctx, updateSense,
				s.Definition, s.DefinitionVI, examplesJSON, versionID, senseID,
			)
			if err != nil {
				return seededCount, fmt.Errorf("update sense for %s: %w", s.Lemma, err)
			}
		}

		// 4. Add to its public decks
		if err := linkDecks(ctx, pool, decksFor(s), decks, senseID); err != nil {
			return seededCount, fmt.Errorf("link decks for %s: %w", s.Lemma, err)
		}

		seededCount++
	}

	if err := decks.prune(ctx); err != nil {
		return seededCount, err
	}
	return seededCount, nil
}

// seedRank is a word's frequency rank: the fixture's when it carries one,
// otherwise its position in the list.
func seedRank(sense seedWordSense, index int) int {
	if sense.Rank > 0 {
		return sense.Rank
	}
	return index + 1
}

// deckCache upserts each public deck once per seed run.
type deckCache struct {
	pool *pgxpool.Pool
	ids  map[string]uuid.UUID
	// linked is every sense this run put in each deck, by deck id.
	linked map[uuid.UUID][]uuid.UUID
}

// prune removes from each deck this run seeded the senses it did not link, so
// a deck holds exactly the list it was built from: a word the fixture dropped
// ("went", "london") leaves the deck on the next seed, not only on a fresh
// database. A learner's own progress on the word is untouched.
func (c *deckCache) prune(ctx context.Context) error {
	for deckID, senses := range c.linked {
		if _, err := c.pool.Exec(ctx,
			`DELETE FROM skill.deck_items WHERE deck_id = $1 AND NOT (word_sense_id = ANY($2))`,
			deckID, senses); err != nil {
			return fmt.Errorf("prune deck %s: %w", deckID, err)
		}
	}
	return nil
}

// id returns the deck's id, creating or refreshing it on first use.
//
// A seeded deck has no owner. ListDecksByUser shows a learner `owner_id = $1 OR
// (owner_id IS NULL AND is_public)`, so a "curated" deck owned by the admin
// account is visible to exactly one person — the admin. NULL is what makes it
// everyone's. The unique constraint is NULLS NOT DISTINCT, so the upsert still
// matches on re-run.
func (c *deckCache) id(ctx context.Context, deck seedDeck) (uuid.UUID, error) {
	if id, ok := c.ids[deck.Slug]; ok {
		return id, nil
	}
	const upsertDeck = `
		INSERT INTO skill.decks (owner_id, slug, name, description, is_public, updated_at)
		VALUES (NULL, $1, $2, $3, true, now())
		ON CONFLICT (owner_id, slug) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    is_public = true,
		    updated_at = now()
		RETURNING id`
	var id uuid.UUID
	if err := c.pool.QueryRow(ctx, upsertDeck, deck.Slug, deck.Name, deck.Description).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("upsert deck %s: %w", deck.Slug, err)
	}
	c.ids[deck.Slug] = id
	return id, nil
}

// linkDecks adds a sense to each of its decks, idempotently.
func linkDecks(
	ctx context.Context, pool *pgxpool.Pool, decks []seedDeck, cache *deckCache, senseID uuid.UUID,
) error {
	const insertDeckItem = `
		INSERT INTO skill.deck_items (deck_id, word_sense_id)
		VALUES ($1, $2)
		ON CONFLICT (deck_id, word_sense_id) DO NOTHING`
	for _, deck := range decks {
		id, err := cache.id(ctx, deck)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, insertDeckItem, id, senseID); err != nil {
			return err
		}
		cache.linked[id] = append(cache.linked[id], senseID)
	}
	return nil
}

func pgxIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func determineLessonCEFR(c seedCourse, unit seedUnit, lesson seedLesson) string {
	if lesson.CEFRLevel != "" {
		return strings.ToLower(lesson.CEFRLevel)
	}
	// Extract from unit title if present (e.g. "(A2)", "(B1)", "(B2)")
	for _, lvl := range []string{"A1", "A2", "B1", "B2", "C1", "C2"} {
		if strings.Contains(unit.Title, "("+lvl) {
			return strings.ToLower(lvl)
		}
	}
	// Otherwise linear interpolation across units from c.CEFRFrom to c.CEFRTo
	levels := []string{"a1", "a2", "b1", "b2", "c1", "c2"}
	fromIdx := -1
	toIdx := -1
	for i, l := range levels {
		if strings.EqualFold(l, c.CEFRFrom) {
			fromIdx = i
		}
		if strings.EqualFold(l, c.CEFRTo) {
			toIdx = i
		}
	}
	if fromIdx != -1 && toIdx != -1 && len(c.Units) > 1 {
		uPos := unit.Position - 1
		if uPos < 0 {
			uPos = 0
		}
		if uPos >= len(c.Units) {
			uPos = len(c.Units) - 1
		}
		idx := fromIdx + (toIdx-fromIdx)*uPos/(len(c.Units)-1)
		return levels[idx]
	}
	if fromIdx != -1 {
		return levels[fromIdx]
	}
	return "a2"
}
