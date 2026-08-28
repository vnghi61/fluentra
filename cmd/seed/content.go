package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

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

	// 2. Seed 200 Word Senses and Public Deck
	count, err := seedVocabularyWords(ctx, pool, adminID, wordSenseSeedData)
	if err != nil {
		return fmt.Errorf("seed vocabulary words: %w", err)
	}
	_, _ = fmt.Fprintf(out, "  ✓ Vocabulary: %d word senses seeded & linked into curated deck\n", count)

	return nil
}

// The keys an authored content body uses. They are the contract between the seed
// and vocabulary's grader, which reads correct_answer and acceptable.
const (
	bodyKeyPrompt        = "prompt"
	bodyKeyCorrectAnswer = "correct_answer"
	bodyKeyAcceptable    = "acceptable"
	bodyKeyDefinition    = "definition"
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
			const upsertLesson = `
				INSERT INTO learn.lessons (unit_id, position, title, skill_focus, estimated_minutes, status, updated_at)
				VALUES ($1, $2, $3, $4, $5, 'published', now())
				ON CONFLICT (unit_id, position) DO UPDATE
				SET title = EXCLUDED.title,
				    skill_focus = EXCLUDED.skill_focus,
				    estimated_minutes = EXCLUDED.estimated_minutes,
				    status = 'published',
				    updated_at = now()
				RETURNING id`
			err := pool.QueryRow(ctx, upsertLesson,
				unitID, lesson.Position, lesson.Title, lesson.SkillFocus, lesson.EstimatedMinutes,
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

				configJSON, err := json.Marshal(act.Config)
				if err != nil {
					return fmt.Errorf("marshal act config: %w", err)
				}

				const upsertActivity = `
					INSERT INTO learn.activities (lesson_id, position, kind, content_version_id, config, weight, updated_at)
					VALUES ($1, $2, $3, $4, $5, 1, now())
					ON CONFLICT (lesson_id, position) DO UPDATE
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

func seedVocabularyWords(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, senses []seedWordSense,
) (int, error) {
	// The curated deck has no owner.
	//
	// ListDecksByUser shows a learner `owner_id = $1 OR (owner_id IS NULL AND
	// is_public)`, so a "curated" deck owned by the admin account is visible to
	// exactly one person — the admin. NULL is what makes it everyone's. The
	// unique constraint is NULLS NOT DISTINCT, so the upsert still matches on
	// re-run.
	var deckID uuid.UUID
	const upsertDeck = `
		INSERT INTO skill.decks (owner_id, slug, name, description, is_public, updated_at)
		VALUES (NULL, 'a2-b1-essentials', 'A2–B1 Essential Vocabulary',
		        'Curated core 200 vocabulary words for intermediate fluency.', true, now())
		ON CONFLICT (owner_id, slug) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    is_public = true,
		    updated_at = now()
		RETURNING id`
	if err := pool.QueryRow(ctx, upsertDeck).Scan(&deckID); err != nil {
		return 0, fmt.Errorf("upsert curated deck: %w", err)
	}

	seededCount := 0
	for i, s := range senses {
		// 1. Ensure content version for sense
		slug := fmt.Sprintf("vocab-%s-%s", s.Lemma, s.POS)
		body := map[string]any{
			bodyKeyPrompt:        fmt.Sprintf("What does the word '%s' mean?", s.Lemma),
			bodyKeyCorrectAnswer: s.Lemma,
			bodyKeyAcceptable:    []string{s.Lemma},
			bodyKeyDefinition:    s.Definition,
		}
		versionID, err := ensureContentItemAndVersion(ctx, pool, adminID, slug, "vocabulary_quiz", s.CEFRLevel, body)
		if err != nil {
			return seededCount, fmt.Errorf("ensure vocab content for %s: %w", s.Lemma, err)
		}

		// 2. Upsert word
		var wordID uuid.UUID
		rank := i + 1
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

		// 4. Add to public deck
		const insertDeckItem = `
			INSERT INTO skill.deck_items (deck_id, word_sense_id)
			VALUES ($1, $2)
			ON CONFLICT (deck_id, word_sense_id) DO NOTHING`
		if _, err := pool.Exec(ctx, insertDeckItem, deckID, senseID); err != nil {
			return seededCount, fmt.Errorf("link deck item for %s: %w", s.Lemma, err)
		}

		seededCount++
	}

	return seededCount, nil
}

func pgxIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
