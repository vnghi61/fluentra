package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// A learner's own vocabulary: pasted in, checked on a schedule, and turned into
// something they can review.
//
// Submitting is synchronous and does almost nothing: parse, store, return. The
// work — a dictionary lookup and a model call per word — happens in a job,
// because it is slow, it fails in ways worth retrying, and a learner should not
// be watching a spinner while it runs.

const (
	// verifyBatch is how many words one run checks.
	//
	// Bounded because each costs a dictionary call and a model call: a run that
	// took every pending item would hold the worker for as long as the longest
	// backlog, and what it does not reach it reaches next hour.
	verifyBatch = 50

	// maxVerifyAttempts retires an item nothing can resolve. Without it a word
	// the dictionary and the model both choke on is retried every hour for ever
	// and the log fills with one failure.
	maxVerifyAttempts = 3

	// uploadDeckSlug is the deck verified words land in. One per learner, not
	// one per upload: a learner reviewing their own words wants them together,
	// not split across however many times they happened to paste.
	uploadDeckSlug = "my-words"
	uploadDeckName = "My words"
)

// verdict is what the model is asked for, and mirrors the vocab_verify template.
type verdict struct {
	Valid          bool     `json:"valid"`
	Reason         string   `json:"reason"`
	Lemma          string   `json:"lemma"`
	PartOfSpeech   string   `json:"part_of_speech"`
	CEFRLevel      string   `json:"cefr_level"`
	Definition     string   `json:"definition"`
	MeaningMatches bool     `json:"meaning_matches"`
	Examples       []string `json:"examples"`
}

// JobEnqueuer schedules background River jobs within database transactions.
type JobEnqueuer interface {
	EnqueueVerifyUploadTx(ctx context.Context, tx pgx.Tx, uploadID uuid.UUID) error
}

// UploadDeps are the collaborators the upload pipeline needs beyond the service.
type UploadDeps struct {
	// Dictionary is authoritative on whether a word exists and on its IPA and
	// pronunciation. Free, keyless, and more accurate than a model for exactly
	// this question.
	Dictionary repository.DictionaryLookup
	// AI judges whether the learner's own wording of the meaning is right, and
	// writes example sentences — the parts a dictionary cannot do.
	AI ai.Client
	// Content stores each verified word as a published version, so a review
	// card has a dictionary entry to point at.
	Content ContentAuthor
	// AuthorID owns that content.
	AuthorID uuid.UUID
	// Pool writes the outbox row that tells gamification to pay XP.
	Pool OutboxTx
	// Beginner starts transactions for atomic upload storage and River job enqueueing.
	Beginner dbx.Beginner
	// Enqueuer schedules the immediate verification job.
	Enqueuer JobEnqueuer
}

// Uploads runs the learner-upload pipeline.
type Uploads struct {
	repo       repository.Repository
	service    *Service
	dictionary repository.DictionaryLookup
	ai         ai.Client
	content    ContentAuthor
	author     uuid.UUID
	events     EventWriter
	// pool is the outbox's transaction. The award for a verified word is
	// published on its own row rather than joined to the verification, because
	// the verification is several statements across three modules and there is
	// no single transaction to join.
	pool     OutboxTx
	beginner dbx.Beginner
	enqueuer JobEnqueuer
}

// NewUploads constructs the pipeline.
func NewUploads(svc *Service, repo repository.Repository, deps UploadDeps) *Uploads {
	return &Uploads{
		repo:       repo,
		service:    svc,
		dictionary: deps.Dictionary,
		ai:         deps.AI,
		content:    deps.Content,
		author:     deps.AuthorID,
		events:     svc.events,
		pool:       deps.Pool,
		beginner:   deps.Beginner,
		enqueuer:   deps.Enqueuer,
	}
}

// Upload is what a learner sees back after submitting.
type Upload struct {
	ID          uuid.UUID    `json:"id"`
	Status      string       `json:"status"`
	ItemCount   int          `json:"item_count"`
	Verified    int          `json:"verified_count"`
	Rejected    int          `json:"rejected_count"`
	Pending     int          `json:"pending_count"`
	Queued      int          `json:"queued_count"`
	DeckID      *uuid.UUID   `json:"deck_id,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Items       []UploadItem `json:"items,omitempty"`
}

// UploadItem is one word and what became of it.
type UploadItem struct {
	Term            string     `json:"term"`
	ProvidedMeaning string     `json:"provided_meaning,omitempty"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason,omitempty"`
	WordSenseID     *uuid.UUID `json:"word_sense_id,omitempty"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
}

// Submit stores a learner's pasted vocabulary within a transaction and enqueues verification.
//
// Deliberately fast: it parses, writes, enqueues the River job in the same transaction, and returns.
// No dictionary, no model, no deck. Everything that can fail slowly happens in the
// job, which is why a learner pasting three hundred words gets an answer in
// milliseconds rather than a request that times out half way through.
func (u *Uploads) Submit(ctx context.Context, userID uuid.UUID, rawText string) (Upload, error) {
	entries := domain.ParseUpload(rawText)
	if len(entries) == 0 {
		return Upload{}, apperr.New(apperr.Validation, "UPLOAD_NO_WORDS",
			"We could not find any words in that. One word per line, "+
				"optionally followed by its meaning.")
	}

	var (
		upload sqlc.SkillVocabUpload
		stored int
	)

	if u.beginner != nil {
		err := dbx.InTx(ctx, u.beginner, func(txCtx context.Context, tx pgx.Tx) error {
			txRepo := u.repo.WithTx(tx)
			var createErr error
			upload, createErr = txRepo.InsertUpload(txCtx, sqlc.InsertUploadParams{
				UserID:    userID,
				RawText:   rawText,
				ItemCount: int32(len(entries)), //nolint:gosec // bounded by MaxUploadEntries
			})
			if createErr != nil {
				return fmt.Errorf("store upload: %w", createErr)
			}

			stored = 0
			for _, entry := range entries {
				if _, itemErr := txRepo.InsertUploadItem(txCtx, sqlc.InsertUploadItemParams{
					UploadID:        upload.ID,
					UserID:          userID,
					Term:            entry.Term,
					ProvidedMeaning: entry.Meaning,
				}); itemErr != nil {
					if errors.Is(itemErr, pgx.ErrNoRows) {
						// The unique constraint caught a duplicate the parser did not.
						continue
					}
					return fmt.Errorf("store upload item %q: %w", entry.Term, itemErr)
				}
				stored++
			}

			if u.enqueuer != nil {
				if enqueueErr := u.enqueuer.EnqueueVerifyUploadTx(txCtx, tx, upload.ID); enqueueErr != nil {
					return fmt.Errorf("enqueue verify upload job: %w", enqueueErr)
				}
			}
			return nil
		})
		if err != nil {
			return Upload{}, fmt.Errorf("submit upload: %w", err)
		}
	} else {
		var err error
		upload, err = u.repo.InsertUpload(ctx, sqlc.InsertUploadParams{
			UserID:    userID,
			RawText:   rawText,
			ItemCount: int32(len(entries)), //nolint:gosec // bounded by MaxUploadEntries
		})
		if err != nil {
			return Upload{}, fmt.Errorf("store upload: %w", err)
		}

		stored = 0
		for _, entry := range entries {
			if _, itemErr := u.repo.InsertUploadItem(ctx, sqlc.InsertUploadItemParams{
				UploadID:        upload.ID,
				UserID:          userID,
				Term:            entry.Term,
				ProvidedMeaning: entry.Meaning,
			}); itemErr != nil {
				if errors.Is(itemErr, pgx.ErrNoRows) {
					// The unique constraint caught a duplicate the parser did not.
					continue
				}
				return Upload{}, fmt.Errorf("store upload item %q: %w", entry.Term, itemErr)
			}
			stored++
		}
	}

	return Upload{
		ID:        upload.ID,
		Status:    upload.Status,
		ItemCount: stored,
		Pending:   stored,
		CreatedAt: upload.CreatedAt,
	}, nil
}

// List returns a learner's uploads, newest first.
func (u *Uploads) List(ctx context.Context, userID uuid.UUID, limit int32) ([]Upload, error) {
	rows, err := u.repo.ListUploadsByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	uploads := make([]Upload, 0, len(rows))
	for _, row := range rows {
		uploads = append(uploads, Upload{
			ID:          row.ID,
			Status:      row.Status,
			ItemCount:   int(row.ItemCount),
			Verified:    int(row.VerifiedCount),
			Rejected:    int(row.RejectedCount),
			Pending:     int(row.PendingCount),
			Queued:      int(row.QueuedCount),
			DeckID:      row.DeckID,
			CreatedAt:   row.CreatedAt,
			CompletedAt: row.CompletedAt,
		})
	}
	return uploads, nil
}

// Get returns one upload with every word in it.
func (u *Uploads) Get(ctx context.Context, userID, uploadID uuid.UUID) (Upload, error) {
	row, err := u.repo.GetUpload(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Upload{}, apperr.New(apperr.NotFound, "UPLOAD_NOT_FOUND",
				"That upload does not exist.")
		}
		return Upload{}, err
	}

	items, err := u.repo.ListUploadItems(ctx, uploadID, userID)
	if err != nil {
		return Upload{}, err
	}

	upload := Upload{
		ID:          row.ID,
		Status:      row.Status,
		ItemCount:   int(row.ItemCount),
		DeckID:      row.DeckID,
		CreatedAt:   row.CreatedAt,
		CompletedAt: row.CompletedAt,
		Items:       make([]UploadItem, 0, len(items)),
	}
	for _, item := range items {
		switch item.Status {
		case statusVerified:
			upload.Verified++
		case statusRejected:
			upload.Rejected++
		case statusPending:
			upload.Pending++
		case statusQueued:
			upload.Queued++
		}
		upload.Items = append(upload.Items, UploadItem{
			Term:            item.Term,
			ProvidedMeaning: item.ProvidedMeaning,
			Status:          item.Status,
			Reason:          item.Reason,
			WordSenseID:     item.WordSenseID,
			VerifiedAt:      item.VerifiedAt,
		})
	}
	return upload, nil
}

// The item statuses, named because they are compared in multiple places.
const (
	statusPending  = "pending"
	statusVerified = "verified"
	statusRejected = "rejected"
	statusQueued   = "queued"
	statusFailed   = "failed"
)

// VerifyPending is the scheduled entry point.
//
// One item at a time, and every failure is local to its item: a word the
// dictionary cannot reach leaves its item pending for the next run, a word the
// model rejects is marked rejected with a reason the learner can read, and
// neither costs the rest of the batch.
func (u *Uploads) VerifyPending(ctx context.Context) error {
	if u.dictionary == nil || u.content == nil || u.author == uuid.Nil {
		slog.DebugContext(ctx, "upload verification is not configured; skipping")
		return nil
	}

	items, err := u.repo.ClaimPendingUploadItems(ctx, maxVerifyAttempts, verifyBatch)
	if err != nil {
		return fmt.Errorf("claim pending upload items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	// Verified counts per learner, so one event carries a whole run's worth of
	// words rather than one event per word. gamification pays per word either
	// way; the difference is the number of outbox rows.
	verified := map[uuid.UUID]int{}
	for _, item := range items {
		ok, err := u.verifyItem(ctx, item)
		if err != nil {
			// Transient: the item stays pending and the attempt is recorded, so
			// a word nothing can resolve eventually retires instead of being
			// retried for ever.
			slog.WarnContext(ctx, "upload item verification failed",
				"term", item.Term, "error", err)
			if recErr := u.repo.RecordUploadItemAttempt(ctx, item.ID, truncateReason(err.Error())); recErr != nil {
				slog.WarnContext(ctx, "could not record verification attempt",
					"item_id", item.ID, "error", recErr)
			}
			continue
		}
		if ok {
			verified[item.UserID]++
		}
	}

	if _, err := u.repo.CompleteFinishedUploads(ctx); err != nil {
		slog.WarnContext(ctx, "could not close finished uploads", "error", err)
	}

	for userID, count := range verified {
		u.publishVerified(ctx, userID, count)
	}
	return nil
}

// VerifyUpload processes verification for a specific upload immediately.
func (u *Uploads) VerifyUpload(ctx context.Context, uploadID uuid.UUID) error {
	if u.dictionary == nil || u.content == nil || u.author == uuid.Nil {
		slog.DebugContext(ctx, "upload verification is not configured; skipping", "upload_id", uploadID)
		return nil
	}

	items, err := u.repo.ClaimPendingUploadItemsByUploadID(ctx, uploadID, maxVerifyAttempts, verifyBatch)
	if err != nil {
		return fmt.Errorf("claim pending upload items for %s: %w", uploadID, err)
	}
	if len(items) == 0 {
		return nil
	}

	verified := map[uuid.UUID]int{}
	for _, item := range items {
		ok, err := u.verifyItem(ctx, item)
		if err != nil {
			slog.WarnContext(ctx, "upload item verification failed",
				"term", item.Term, "upload_id", uploadID, "error", err)
			if recErr := u.repo.RecordUploadItemAttempt(ctx, item.ID, truncateReason(err.Error())); recErr != nil {
				slog.WarnContext(ctx, "could not record verification attempt",
					"item_id", item.ID, "error", recErr)
			}
			continue
		}
		if ok {
			verified[item.UserID]++
		}
	}

	if _, err := u.repo.CompleteFinishedUploads(ctx); err != nil {
		slog.WarnContext(ctx, "could not close finished uploads", "error", err)
	}

	for userID, count := range verified {
		u.publishVerified(ctx, userID, count)
	}
	return nil
}

// verifyItem checks one word and, when it holds up, turns it into something the
// learner can review. The bool reports whether it was accepted.
func (u *Uploads) verifyItem(ctx context.Context, item sqlc.SkillVocabUploadItem) (bool, error) {
	term := strings.TrimSpace(item.Term)

	// The dictionary first, and it is authoritative on existence. A model
	// asked "is this a word" will confidently invent an entry for a typo.
	entry, err := u.dictionary.Lookup(ctx, term)
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrWordNotFound):
		// Not a transport failure — a verdict. The model still gets a say,
		// because the dictionary has no entry for a valid fixed phrase.
		entry = repository.DictionaryEntry{}
	default:
		return false, fmt.Errorf("dictionary lookup: %w", err)
	}

	answer, model, err := u.judge(ctx, item, entry)
	if err != nil {
		return false, err
	}

	if model == "queued" {
		senseID, err := u.materialise(ctx, item, entry, answer)
		if err != nil {
			return false, err
		}
		note := "Queued for background enrichment. Your flashcard is ready to review."
		if _, err := u.repo.MarkUploadItemQueued(ctx, item.ID, &senseID, note); err != nil {
			return false, fmt.Errorf("mark queued: %w", err)
		}
		return false, nil
	}

	if !answer.Valid {
		reason := answer.Reason
		if reason == "" {
			reason = fmt.Sprintf("We could not find %q as an English word.", term)
		}
		if _, err := u.repo.MarkUploadItemRejected(ctx, item.ID, truncateReason(reason)); err != nil {
			return false, fmt.Errorf("mark rejected: %w", err)
		}
		return false, nil
	}

	senseID, err := u.materialise(ctx, item, entry, answer)
	if err != nil {
		return false, err
	}

	// A note, not a rejection. The word is real and worth learning; the
	// learner's own gloss was off, and telling them that is more useful than
	// refusing the word.
	note := ""
	if !answer.MeaningMatches && item.ProvidedMeaning != "" {
		note = "Added. Your note did not quite match the usual meaning — " +
			"the definition here is the dictionary's."
	}
	if _, err := u.repo.MarkUploadItemVerified(ctx, item.ID, &senseID, model, note); err != nil {
		return false, fmt.Errorf("mark verified: %w", err)
	}
	return true, nil
}

// judge asks the model the two questions a dictionary cannot answer: whether
// the learner's own wording of the meaning is right, and what the word looks
// like in a sentence.
func (u *Uploads) judge(
	ctx context.Context, item sqlc.SkillVocabUploadItem, entry repository.DictionaryEntry,
) (verdict, string, error) {
	if u.ai == nil {
		// No model configured. The dictionary alone is still a real verdict on
		// existence, and refusing to proceed would mean uploads never complete
		// on a deployment that has not set up AI.
		if entry.Lemma == "" {
			return verdict{Valid: false,
				Reason: "We could not find that word in the dictionary."}, "dictionary", nil
		}
		return verdict{
			Valid: true, Lemma: entry.Lemma, PartOfSpeech: entry.PartOfSpeech,
			CEFRLevel: "", Definition: entry.Definition,
			MeaningMatches: true, Examples: entry.Examples,
		}, "dictionary", nil
	}

	var answer verdict
	request := ai.Request{
		Task: ai.TaskVerifyVocabulary,
		Vars: map[string]any{
			"Term":                 item.Term,
			"ProvidedMeaning":      item.ProvidedMeaning,
			"DictionaryDefinition": entry.Definition,
			"PartOfSpeech":         entry.PartOfSpeech,
			"ExampleCount":         5,
		},
	}
	if err := ai.CompleteJSON(ctx, u.ai, request, &answer); err != nil {
		if errors.Is(err, ai.ErrQuotaExhausted) {
			slog.WarnContext(ctx, "ai: quota exhausted across all providers; degrading word to queued flashcard",
				"term", item.Term)
			def := firstNonEmpty(entry.Definition, item.ProvidedMeaning, item.Term)
			lemma := firstNonEmpty(entry.Lemma, strings.ToLower(strings.TrimSpace(item.Term)))
			return verdict{
				Valid:          false,
				Lemma:          lemma,
				PartOfSpeech:   entry.PartOfSpeech,
				CEFRLevel:      "",
				Definition:     def,
				MeaningMatches: false,
				Examples:       entry.Examples,
			}, "queued", nil
		}
		return verdict{}, "", fmt.Errorf("verify %q: %w", item.Term, err)
	}

	// The dictionary overrules the model on existence in both directions: it
	// found the word, so the word exists whatever the model says.
	if entry.Lemma != "" {
		answer.Valid = true
		if answer.Definition == "" {
			answer.Definition = entry.Definition
		}
		if answer.PartOfSpeech == "" {
			answer.PartOfSpeech = entry.PartOfSpeech
		}
	}
	return answer, "ai", nil
}

// materialise turns an accepted word into a dictionary entry, a deck item and a
// review card.
func (u *Uploads) materialise(
	ctx context.Context,
	item sqlc.SkillVocabUploadItem,
	entry repository.DictionaryEntry,
	answer verdict,
) (uuid.UUID, error) {
	lemma := firstNonEmpty(answer.Lemma, entry.Lemma, strings.ToLower(item.Term))
	pos := firstNonEmpty(answer.PartOfSpeech, entry.PartOfSpeech)
	cefr := normaliseCEFR(answer.CEFRLevel)
	definition := firstNonEmpty(answer.Definition, entry.Definition, item.ProvidedMeaning, item.Term)
	if definition == "" {
		return uuid.Nil, fmt.Errorf("no definition for %q", item.Term)
	}

	examples := answer.Examples
	if len(examples) == 0 {
		examples = entry.Examples
	}

	// The word itself. Shared across learners: two people uploading "leisure"
	// are learning the same word, and giving them a copy each would mean two
	// dictionary entries, two sets of exercises and a review card that points
	// at whichever happened to be created first.
	word, err := u.service.CreateWord(ctx, domain.Word{
		Lemma: lemma, POS: domain.PartOfSpeech(pos),
		CEFRLevel: domain.CEFRLevel(cefr), IPA: nilIfEmpty(entry.IPA),
	})
	if err != nil {
		// Almost always the (lemma, pos) unique constraint: somebody has
		// already uploaded this word. Reuse theirs.
		existing, lookupErr := u.repo.GetWordByLemmaAndPOS(ctx, lemma, pos)
		if lookupErr != nil {
			return uuid.Nil, fmt.Errorf("resolve word %q: %w", lemma, err)
		}
		word = domain.Word{ID: existing.ID, Lemma: existing.Lemma}
	}

	// The sense's own content version, which is what a review card points at.
	// Without it the card has nothing to render and the learner meets "this
	// card has no content yet".
	body, err := json.Marshal(senseBody(lemma, pos, cefr, definition, item.ProvidedMeaning, entry, examples))
	if err != nil {
		return uuid.Nil, err
	}
	versionID, err := u.content.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      "user-vocab-" + slugPart(lemma) + "-" + slugPart(pos),
		Kind:      "vocabulary_quiz",
		CEFRLevel: cefr,
		Body:      body,
		AuthorID:  u.author,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("publish sense content: %w", err)
	}

	sense, err := u.service.CreateSense(ctx, domain.WordSense{
		WordID: word.ID, ContentVersionID: &versionID,
		Definition:   definition,
		DefinitionVi: nilIfEmpty(item.ProvidedMeaning),
		Examples:     toDomainExamples(examples),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create sense for %q: %w", lemma, err)
	}

	// Their own deck, and their own review card. Both best-effort: the word is
	// verified either way, and losing a deck link is recoverable where losing
	// the verification is not.
	if deckID, err := u.ensureDeck(ctx, item.UserID, item.UploadID); err == nil {
		if err := u.service.AddWordToDeck(ctx, deckID, sense.ID); err != nil {
			slog.WarnContext(ctx, "could not add verified word to deck",
				"term", item.Term, "error", err)
		}
	}

	if u.service.reviews != nil {
		if err := u.service.reviews.UpsertCards(ctx, item.UserID, []learningcontract.ReviewItem{{
			ContentVersionID: versionID,
			Skill:            "vocabulary",
			InitialGrade:     "again",
		}}); err != nil {
			slog.WarnContext(ctx, "could not schedule review card for uploaded word",
				"term", item.Term, "error", err)
		}
	}
	return sense.ID, nil
}

// ensureDeck finds or creates the learner's own deck and links it to the upload.
func (u *Uploads) ensureDeck(ctx context.Context, userID, uploadID uuid.UUID) (uuid.UUID, error) {
	decks, err := u.repo.ListDecksByUser(ctx, &userID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, deck := range decks {
		if deck.Slug == uploadDeckSlug {
			_ = u.repo.SetUploadDeck(ctx, uploadID, deck.ID)
			return deck.ID, nil
		}
	}

	description := "Words you added yourself, checked and ready to review."
	deck, err := u.service.CreateDeck(ctx, &userID, uploadDeckSlug, uploadDeckName, &description, false)
	if err != nil {
		return uuid.Nil, err
	}
	_ = u.repo.SetUploadDeck(ctx, uploadID, deck.ID)
	return deck.ID, nil
}

// publishVerified tells gamification how many words this learner earned.
func (u *Uploads) publishVerified(ctx context.Context, userID uuid.UUID, count int) {
	if u.events == nil || u.pool == nil || count <= 0 {
		return
	}
	if _, err := u.events.Write(ctx, u.pool, contract.Aggregate,
		contract.EventWordsVerified, contract.WordsVerified{
			UserID:     userID,
			Count:      count,
			OccurredAt: time.Now(),
		}); err != nil {
		slog.WarnContext(ctx, "could not publish words_verified", "error", err)
	}
}

// senseBody is what the flashcard and the review card render.
func senseBody(
	lemma, pos, cefr, definition, learnerNote string,
	entry repository.DictionaryEntry,
	examples []string,
) map[string]any {
	sentences := make([]map[string]any, 0, len(examples))
	for _, example := range examples {
		sentences = append(sentences, map[string]any{"sentence": example})
	}

	body := map[string]any{
		"word":           lemma,
		"pos":            pos,
		"definition":     definition,
		"cefr_level":     cefr,
		"prompt":         "What does the word '" + lemma + "' mean?",
		"correct_answer": lemma,
		"acceptable":     []string{lemma},
		"word_lemmas":    []string{lemma},
	}
	if entry.IPA != "" {
		body["ipa"] = entry.IPA
	}
	if entry.AudioURL != "" {
		body["audio_url"] = entry.AudioURL
	}
	// The learner's own note, shown as the gloss. It is theirs, so it is what
	// they will recognise — and it is labelled as their own rather than
	// presented as the dictionary's.
	if learnerNote != "" {
		body["definition_vi"] = learnerNote
	}
	if len(sentences) > 0 {
		body["example_sentences"] = sentences
		body["example_sentence"] = examples[0]
	}
	return body
}

func toDomainExamples(examples []string) []domain.ExampleSentence {
	out := make([]domain.ExampleSentence, 0, len(examples))
	for _, example := range examples {
		out = append(out, domain.ExampleSentence{Sentence: example})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nilIfEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

// normaliseCEFR keeps the level inside the check constraint. A model asked for
// a level will occasionally answer "intermediate". When unassigned (e.g. queued words),
// it returns empty string rather than fabricating "B1".
func normaliseCEFR(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "A1", "A2", "B1", "B2", "C1", "C2":
		return strings.ToUpper(strings.TrimSpace(level))
	default:
		return ""
	}
}

func slugPart(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == ' ', r == '-':
			out.WriteByte('-')
		}
	}
	return out.String()
}

// truncateReason keeps a stored message readable. An error chain can be long,
// and the whole of it in a column the learner reads is noise.
func truncateReason(reason string) string {
	const limit = 300
	reason = strings.TrimSpace(reason)
	if len(reason) <= limit {
		return reason
	}
	return reason[:limit] + "…"
}

// maxEnrichBatch is the maximum number of queued words to re-verify in one sweep.
// Bounded to avoid exhausting daily quota and starving live traffic.
const maxEnrichBatch = 20

// maxEnrichAttempts retires an item that fails re-verification repeatedly.
const maxEnrichAttempts = 3

func (u *Uploads) quotaPermitsEnrichment(ctx context.Context) bool {
	qc, ok := u.ai.(ai.QuotaChecker)
	if !ok {
		return true
	}
	hasQuota, err := qc.HasQuota(ctx, ai.TaskVerifyVocabulary)
	if err != nil {
		slog.WarnContext(ctx, "could not check AI quota; yielding enrichment sweep to live traffic",
			"error", err)
		return false
	}
	if !hasQuota {
		slog.InfoContext(ctx, "ai: daily quota exhausted; enrichment sweep yielding to live traffic")
		return false
	}
	return true
}

func (u *Uploads) handleEnrichFailure(ctx context.Context, item sqlc.SkillVocabUploadItem, enrichErr error) {
	slog.WarnContext(ctx, "queued upload item enrichment failed",
		"term", item.Term, "attempts", item.Attempts+1, "error", enrichErr)

	if item.Attempts+1 >= maxEnrichAttempts {
		reason := fmt.Sprintf(
			"Verification could not be completed after %d attempts; retry stopped to avoid unbounded spend.",
			maxEnrichAttempts,
		)
		if _, failErr := u.repo.MarkQueuedUploadItemFailed(ctx, item.ID, truncateReason(reason)); failErr != nil {
			slog.WarnContext(ctx, "could not mark queued upload item failed", "item_id", item.ID, "error", failErr)
		}
	}
}

// EnrichQueued sweeps queued words and runs verification against the model
// when quota is available.
func (u *Uploads) EnrichQueued(ctx context.Context) error {
	if u.ai == nil || u.dictionary == nil || u.content == nil || u.author == uuid.Nil {
		slog.DebugContext(ctx, "upload enrichment is not configured; skipping")
		return nil
	}

	if !u.quotaPermitsEnrichment(ctx) {
		return nil
	}

	items, err := u.repo.ClaimQueuedUploadItems(ctx, maxEnrichAttempts, maxEnrichBatch)
	if err != nil {
		return fmt.Errorf("claim queued upload items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	verified := map[uuid.UUID]int{}
	for _, item := range items {
		ok, enrichErr := u.enrichItem(ctx, item)
		if enrichErr != nil {
			if errors.Is(enrichErr, ai.ErrQuotaExhausted) {
				slog.WarnContext(ctx, "ai: quota exhausted during enrichment sweep; yielding to live traffic")
				break
			}
			u.handleEnrichFailure(ctx, item, enrichErr)
			continue
		}
		if ok {
			verified[item.UserID]++
		}
	}

	if _, err := u.repo.CompleteFinishedUploads(ctx); err != nil {
		slog.WarnContext(ctx, "could not close finished uploads after enrichment", "error", err)
	}

	for userID, count := range verified {
		u.publishVerified(ctx, userID, count)
	}
	return nil
}

func (u *Uploads) enrichItem(ctx context.Context, item sqlc.SkillVocabUploadItem) (bool, error) {
	term := strings.TrimSpace(item.Term)
	entry, err := u.dictionary.Lookup(ctx, term)
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrWordNotFound):
		entry = repository.DictionaryEntry{}
	default:
		return false, fmt.Errorf("dictionary lookup: %w", err)
	}

	var answer verdict
	request := ai.Request{
		Task: ai.TaskVerifyVocabulary,
		Vars: map[string]any{
			"Term":                 item.Term,
			"ProvidedMeaning":      item.ProvidedMeaning,
			"DictionaryDefinition": entry.Definition,
			"PartOfSpeech":         entry.PartOfSpeech,
			"ExampleCount":         5,
		},
	}
	if err := ai.CompleteJSON(ctx, u.ai, request, &answer); err != nil {
		return false, err
	}

	if entry.Lemma != "" {
		answer.Valid = true
		if answer.Definition == "" {
			answer.Definition = entry.Definition
		}
		if answer.PartOfSpeech == "" {
			answer.PartOfSpeech = entry.PartOfSpeech
		}
	}

	if !answer.Valid {
		reason := answer.Reason
		if reason == "" {
			reason = fmt.Sprintf("We could not find %q as an English word.", term)
		}
		if _, err := u.repo.MarkQueuedUploadItemRejected(ctx, item.ID, truncateReason(reason)); err != nil {
			return false, fmt.Errorf("mark queued item rejected: %w", err)
		}
		return false, nil
	}

	if item.WordSenseID != nil {
		if err := u.enrichExistingSense(ctx, item, entry, answer); err != nil {
			slog.WarnContext(ctx, "could not enrich existing sense", "term", term, "error", err)
		}
	}

	note := ""
	if !answer.MeaningMatches && item.ProvidedMeaning != "" {
		note = "Added. Your note did not quite match the usual meaning — the definition here is the dictionary's."
	}
	if _, err := u.repo.MarkQueuedUploadItemVerified(ctx, item.ID, "ai", note); err != nil {
		return false, fmt.Errorf("mark queued item verified: %w", err)
	}
	return true, nil
}

func (u *Uploads) enrichExistingSense(
	ctx context.Context,
	item sqlc.SkillVocabUploadItem,
	entry repository.DictionaryEntry,
	answer verdict,
) error {
	lemma := firstNonEmpty(answer.Lemma, entry.Lemma, strings.ToLower(item.Term))
	pos := firstNonEmpty(answer.PartOfSpeech, entry.PartOfSpeech)
	cefr := normaliseCEFR(answer.CEFRLevel)
	definition := firstNonEmpty(answer.Definition, entry.Definition, item.ProvidedMeaning, item.Term)

	_, err := u.repo.InsertWord(ctx, sqlc.InsertWordParams{
		Lemma:     lemma,
		Pos:       pos,
		CefrLevel: cefr,
		Ipa:       nilIfEmpty(entry.IPA),
	})
	if err != nil {
		return fmt.Errorf("update word: %w", err)
	}

	examples := answer.Examples
	if len(examples) == 0 {
		examples = entry.Examples
	}
	body, err := json.Marshal(senseBody(lemma, pos, cefr, definition, item.ProvidedMeaning, entry, examples))
	if err != nil {
		return fmt.Errorf("marshal sense body: %w", err)
	}
	versionID, err := u.content.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      "user-vocab-" + slugPart(lemma) + "-" + slugPart(pos),
		Kind:      "vocabulary_quiz",
		CEFRLevel: cefr,
		Body:      body,
		AuthorID:  u.author,
	})
	if err != nil {
		return fmt.Errorf("publish enriched content: %w", err)
	}

	examplesJSON, err := json.Marshal(toDomainExamples(examples))
	if err != nil {
		return fmt.Errorf("marshal examples: %w", err)
	}

	_, err = u.repo.UpdateWordSenseEnrichment(ctx, sqlc.UpdateWordSenseEnrichmentParams{
		ID:               *item.WordSenseID,
		ContentVersionID: &versionID,
		Definition:       definition,
		Examples:         examplesJSON,
	})
	if err != nil {
		return fmt.Errorf("update sense enrichment: %w", err)
	}
	return nil
}
