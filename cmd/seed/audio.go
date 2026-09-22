package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/shared/config"
)

// The audio backfill: recorded pronunciation for every flashcard that has none.
//
// A card with no `audio_url` falls back to the browser's speech synthesis, and
// on some Android phones synthesis is unreachable — the OS stops Chrome binding
// its text-to-speech engine, so the speaker pulses in silence. A human
// recording plays through a plain <audio> element and needs none of that, and
// the dictionary this repository already reads for IPA and definitions returns
// one, with the page that credits it.
//
// It is a separate mode (`go run ./cmd/seed -audio`) because it is the only
// part of the seed that depends on somebody else's network and their courtesy
// rate limit. `make seed` runs it after the offline seed; a fresh database then
// has recordings without anyone remembering a second command.

// audioPace is how long to wait between two dictionary lookups.
//
// The API is free, keyless and somebody else's. A few hundred words at four
// lookups a second is under a minute and still a well-behaved client.
const audioPace = 250 * time.Millisecond

// audioBackfill is what one run did, for the line it prints at the end.
type audioBackfill struct {
	checked  int
	updated  int
	noAudio  int
	failures int
}

// runAudioBackfill is the `-audio` entry point.
func runAudioBackfill(ctx context.Context, out io.Writer) error {
	var cfg seedConfig
	if err := config.Load(ctx, config.Options{
		Defaults: map[string]any{"app.env": "local"},
		Required: []config.RequiredKey{{
			Name:       "db.dsn",
			DocSection: "docs/deployment/configuration.md#database",
		}},
	}, &cfg); err != nil {
		return fmt.Errorf("load seed configuration: %w", err)
	}
	if cfg.App.Environment == "production" {
		return errors.New("refusing to seed: APP_ENV is production")
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	lookup := newLemmaAudio(
		repository.NewFreeDictionaryAPI("").WithTimeout(3 * time.Second),
	)

	senses, err := backfillSenseAudio(ctx, pool, lookup, out)
	if err != nil {
		return err
	}
	activities, err := backfillFlashcardAudio(ctx, pool, lookup, out)
	if err != nil {
		return err
	}

	total := audioBackfill{
		checked:  senses.checked + activities.checked,
		updated:  senses.updated + activities.updated,
		noAudio:  senses.noAudio + activities.noAudio,
		failures: senses.failures + activities.failures,
	}
	_, _ = fmt.Fprintf(out,
		"  ✓ Pronunciation: %d checked, %d given a recording, %d without one, %d lookups failed\n",
		total.checked, total.updated, total.noAudio, total.failures)
	return nil
}

// seedUserAgent is who the backfill says it is.
//
// Wikimedia refuses the Go client's default User-Agent with a 403 — its policy
// requires a descriptive one. A browser sends one, which is why the browser
// fallback works and this backfill returned nothing until it introduced itself.
const seedUserAgent = "FluentraSeed/0.1 (+https://github.com/fluentra/fluentra)"

// commonsBaseURL is where Wikimedia Commons lives. A field on lemmaAudio so a
// test can point it at its own server.
const commonsBaseURL = "https://commons.wikimedia.org/wiki/"

// lemmaAudio looks a lemma up once per run.
//
// Several senses share a lemma ("bank" the noun and the verb), and so do the
// lesson flashcards and the word sense they were built from. One lookup per
// lemma is the difference between 220 requests and about 200, and it is the
// only way the two paths can be guaranteed to choose the same recording.
type lemmaAudio struct {
	dictionary  repository.DictionaryLookup
	client      *http.Client
	commonsBase string
	pace        time.Duration
	entries     map[string]repository.DictionaryEntry
	errs        map[string]error
}

func newLemmaAudio(dictionary repository.DictionaryLookup) *lemmaAudio {
	return &lemmaAudio{
		dictionary:  dictionary,
		client:      &http.Client{Timeout: 5 * time.Second},
		commonsBase: commonsBaseURL,
		pace:        audioPace,
		entries:     make(map[string]repository.DictionaryEntry),
		errs:        make(map[string]error),
	}
}

// lookup returns the dictionary entry for a lemma, remembering both answers and
// failures. found is false when neither source has anything.
func (l *lemmaAudio) lookup(ctx context.Context, lemma string) (repository.DictionaryEntry, bool, error) {
	key := strings.ToLower(strings.TrimSpace(lemma))
	if entry, ok := l.entries[key]; ok {
		return entry, true, nil
	}
	if err, ok := l.errs[key]; ok {
		return repository.DictionaryEntry{}, false, err
	}

	entry, err := l.dictionary.Lookup(ctx, key)
	switch {
	case err == nil:
		if entry.AudioURL == "" {
			entry = l.withCommonsRecording(ctx, key, entry)
		}
	case errors.Is(err, repository.ErrWordNotFound):
		// The dictionary does not know the word; Commons may still hold a
		// recording of it, and the file page is still a credit.
		commons, found := l.commonsRecording(ctx, key)
		if !found {
			l.entries[key] = repository.DictionaryEntry{}
			return repository.DictionaryEntry{}, false, nil
		}
		entry = commons
	default:
		// A transport failure is the dictionary being unavailable, not a
		// verdict on the word. Commons is the source of last resort, exactly
		// as it is in the browser.
		commons, found := l.commonsRecording(ctx, key)
		if !found {
			l.errs[key] = err
			return repository.DictionaryEntry{}, false, err
		}
		entry = commons
	}

	l.entries[key] = entry
	// The dictionary's courtesy pace, paid once per real request.
	time.Sleep(l.pace)
	return entry, true, nil
}

// withCommonsRecording fills in the audio the dictionary did not return.
func (l *lemmaAudio) withCommonsRecording(
	ctx context.Context, lemma string, entry repository.DictionaryEntry,
) repository.DictionaryEntry {
	commons, found := l.commonsRecording(ctx, lemma)
	if !found {
		return entry
	}
	entry.AudioURL = commons.AudioURL
	entry.AudioAttribution = commons.AudioAttribution
	entry.AudioLicence = ""
	return entry
}

// commonsRecording looks the word up in Wikimedia Commons by its file-naming
// convention: `En-us-<word>.ogg`, then `En-uk-<word>.ogg`.
//
// The same two names the browser tries, and for the same reason: the dictionary
// is a free service that is sometimes slow enough to time out (measured at 20 s
// per request on 2026-09-22), and the recording is still there. The file page
// is the credit CC BY-SA requires; the licence's short name is not on it, so
// none is claimed.
func (l *lemmaAudio) commonsRecording(ctx context.Context, lemma string) (repository.DictionaryEntry, bool) {
	commons := l.commonsBase
	for _, prefix := range []string{"En-us", "En-uk"} {
		file := fmt.Sprintf("%s-%s.ogg", prefix, lemma)
		fileURL := commons + "Special:FilePath/" + url.PathEscape(file)
		request, err := http.NewRequestWithContext(ctx, http.MethodHead, fileURL, nil)
		if err != nil {
			continue
		}
		request.Header.Set("User-Agent", seedUserAgent)
		response, err := l.client.Do(request)
		if err != nil {
			continue
		}
		_ = response.Body.Close()
		// A missing file is a 404; Special:FilePath answers 302 to the file
		// itself, and the client follows it.
		if response.StatusCode < 200 || response.StatusCode >= 400 {
			continue
		}
		return repository.DictionaryEntry{
			Lemma:            lemma,
			AudioURL:         fileURL,
			AudioAttribution: commons + "File:" + url.PathEscape(file),
		}, true
	}
	return repository.DictionaryEntry{}, false
}

// withRecording adds the three fields a card needs, or reports false when the
// dictionary has no recording that can be credited.
func withRecording(body map[string]any, entry repository.DictionaryEntry) bool {
	if entry.AudioURL == "" || entry.AudioAttribution == "" {
		return false
	}
	body["audio_url"] = entry.AudioURL
	body["audio_attribution"] = entry.AudioAttribution
	if entry.AudioLicence != "" {
		body["audio_licence"] = entry.AudioLicence
	}
	return true
}

// backfillSenseAudio republishes every word sense whose current body has no
// recording, and moves its review cards onto the new version.
//
// A new version rather than an edit: a published version is immutable
// (BR-CONTENT-01), and the API itself cannot change one either — a seed that
// could would produce a state the product cannot.
func backfillSenseAudio(
	ctx context.Context, pool *pgxpool.Pool, lookup *lemmaAudio, out io.Writer,
) (audioBackfill, error) {
	const query = `
		SELECT ws.id, ws.content_version_id, w.lemma, COALESCE(v.body, '{}'::jsonb)
		FROM skill.word_senses ws
		JOIN skill.words w ON w.id = ws.word_id
		LEFT JOIN content.content_versions v ON v.id = ws.content_version_id
		WHERE ws.content_version_id IS NOT NULL`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return audioBackfill{}, fmt.Errorf("list word senses: %w", err)
	}
	defer rows.Close()

	var result audioBackfill
	for rows.Next() {
		var (
			senseID   uuid.UUID
			versionID uuid.UUID
			lemma     string
			rawBody   []byte
		)
		if err := rows.Scan(&senseID, &versionID, &lemma, &rawBody); err != nil {
			return result, fmt.Errorf("scan word sense: %w", err)
		}
		result.checked++

		var body map[string]any
		if err := json.Unmarshal(rawBody, &body); err != nil {
			return result, fmt.Errorf("decode sense body for %s: %w", lemma, err)
		}
		if body["audio_url"] != nil && body["audio_url"] != "" {
			continue
		}

		entry, found, err := lookup.lookup(ctx, lemma)
		if err != nil {
			result.failures++
			_, _ = fmt.Fprintf(out, "  ! %s: dictionary lookup failed: %v\n", lemma, err)
			continue
		}
		if !found || !withRecording(body, entry) {
			result.noAudio++
			continue
		}

		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return result, fmt.Errorf("marshal sense body for %s: %w", lemma, err)
		}
		newVersionID, err := publishNewVersion(ctx, pool, versionID, bodyJSON)
		if err != nil {
			return result, fmt.Errorf("publish audio version for %s: %w", lemma, err)
		}
		if _, err := pool.Exec(ctx,
			`UPDATE skill.word_senses SET content_version_id = $1, updated_at = now() WHERE id = $2`,
			newVersionID, senseID); err != nil {
			return result, fmt.Errorf("repoint sense %s: %w", lemma, err)
		}
		if _, err := pool.Exec(ctx,
			`UPDATE learn.review_cards SET content_version_id = $1 WHERE content_version_id = $2`,
			newVersionID, versionID); err != nil {
			return result, fmt.Errorf("repoint review cards for %s: %w", lemma, err)
		}
		result.updated++
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("read word senses: %w", err)
	}
	return result, nil
}

// backfillFlashcardAudio adds a recording to every lesson flashcard whose
// activity config has none.
//
// The config, not a content version: the lesson runner reads the word, the IPA
// and the audio off `learn.activities.config`, and the activity body carries
// only the answer key the grader reads. Publishing a new body version would
// change nothing a learner sees.
func backfillFlashcardAudio(
	ctx context.Context, pool *pgxpool.Pool, lookup *lemmaAudio, out io.Writer,
) (audioBackfill, error) {
	const query = `
		SELECT a.id, COALESCE(a.config, '{}'::jsonb)
		FROM learn.activities a
		WHERE a.kind = 'vocab_flashcard' AND a.retired_at IS NULL`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return audioBackfill{}, fmt.Errorf("list flashcard activities: %w", err)
	}
	defer rows.Close()

	var result audioBackfill
	for rows.Next() {
		var (
			activityID uuid.UUID
			rawConfig  []byte
		)
		if err := rows.Scan(&activityID, &rawConfig); err != nil {
			return result, fmt.Errorf("scan flashcard activity: %w", err)
		}

		var config map[string]any
		if err := json.Unmarshal(rawConfig, &config); err != nil {
			return result, fmt.Errorf("decode flashcard config: %w", err)
		}
		if config["audio_url"] != nil && config["audio_url"] != "" {
			continue
		}
		lemma, _ := config[cfgTargetWord].(string)
		if strings.TrimSpace(lemma) == "" {
			continue
		}
		result.checked++

		entry, found, err := lookup.lookup(ctx, lemma)
		if err != nil {
			result.failures++
			_, _ = fmt.Fprintf(out, "  ! %s: dictionary lookup failed: %v\n", lemma, err)
			continue
		}
		if !found || !withRecording(config, entry) {
			result.noAudio++
			continue
		}

		configJSON, err := json.Marshal(config)
		if err != nil {
			return result, fmt.Errorf("marshal flashcard config: %w", err)
		}
		if _, err := pool.Exec(ctx,
			`UPDATE learn.activities SET config = $1, updated_at = now() WHERE id = $2`,
			configJSON, activityID); err != nil {
			return result, fmt.Errorf("update flashcard activity %s: %w", activityID, err)
		}
		result.updated++
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("read flashcard activities: %w", err)
	}
	return result, nil
}

// publishNewVersion writes the body as the item's next published version and
// returns its id.
//
// The approval row is written for the same reason the seed writes one: a
// version only reaches `published` through draft → review → approved, and
// inserting it straight into `published` without the approval would produce a
// state the API has never emitted.
func publishNewVersion(
	ctx context.Context, pool *pgxpool.Pool, oldVersionID uuid.UUID, body []byte,
) (uuid.UUID, error) {
	var (
		itemID uuid.UUID
		kind   string
		cefr   string
	)
	err := pool.QueryRow(ctx,
		`SELECT item_id, kind, cefr_level FROM content.content_versions WHERE id = $1`,
		oldVersionID).Scan(&itemID, &kind, &cefr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("version %s does not exist", oldVersionID)
		}
		return uuid.Nil, fmt.Errorf("read version %s: %w", oldVersionID, err)
	}

	var versionID uuid.UUID
	const insertVersion = `
		INSERT INTO content.content_versions (item_id, version, kind, body, cefr_level, status, published_at)
		SELECT $1, COALESCE(MAX(version), 0) + 1, $2, $3, $4, 'published', now()
		FROM content.content_versions
		WHERE item_id = $1
		RETURNING id`
	if err := pool.QueryRow(ctx, insertVersion, itemID, kind, body, cefr).Scan(&versionID); err != nil {
		return uuid.Nil, fmt.Errorf("insert version for item %s: %w", itemID, err)
	}

	const insertReview = `
		INSERT INTO content.content_reviews (version_id, reviewer_id, decision, comments)
		SELECT $1, ci.owner_id, 'approved', 'Approved by the pronunciation backfill.'
		FROM content.content_items ci
		WHERE ci.id = $2`
	if _, err := pool.Exec(ctx, insertReview, versionID, itemID); err != nil {
		return uuid.Nil, fmt.Errorf("record approval for item %s: %w", itemID, err)
	}

	const linkVersion = `
		UPDATE content.content_items
		SET current_version_id = $1, status = 'published', updated_at = now()
		WHERE id = $2`
	if _, err := pool.Exec(ctx, linkVersion, versionID, itemID); err != nil {
		return uuid.Nil, fmt.Errorf("link item %s: %w", itemID, err)
	}
	return versionID, nil
}
