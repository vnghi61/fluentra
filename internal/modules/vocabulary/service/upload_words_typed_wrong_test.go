package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

type routingAI struct {
	responses map[string]string // keyed by Term
}

func (r *routingAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	term, _ := req.Vars["Term"].(string)
	if resp, ok := r.responses[term]; ok {
		return ai.Response{Text: resp, Model: "routing-ai"}, nil
	}
	return ai.Response{Text: `{"valid":false, "reason":"Not found"}`, Model: "routing-ai"}, nil
}

type spyEvents struct {
	verified []contract.WordsVerified
}

func (s *spyEvents) Write(_ context.Context, _ service.OutboxTx, _, event string, payload any) (uuid.UUID, error) {
	if event == contract.EventWordsVerified {
		if ev, ok := payload.(contract.WordsVerified); ok {
			s.verified = append(s.verified, ev)
		}
	}
	return uuid.New(), nil
}

func newTypedWrongPipeline(
	t *testing.T,
	repo *uploadRepo,
	dict *stubDictionary,
	model ai.Client,
	events *spyEvents,
) *service.Uploads {
	t.Helper()
	author := &stubContentAuthor{}
	svc := service.New(service.Deps{Repo: repo, Events: events})
	return service.NewUploads(svc, repo, service.UploadDeps{
		Dictionary: dict,
		AI:         model,
		Content:    author,
		AuthorID:   uuid.New(),
		Pool:       stubPool{},
		Beginner:   stubPool{},
	})
}

func TestUploads_WordsTypedWrong(t *testing.T) {
	// -------------------------------------------------------------------------
	// 1. schol: trường học -> Verified as school, spelling_corrected, one deck item, one XP
	// -------------------------------------------------------------------------
	t.Run("schol_truong_hoc_corrected_to_school", func(t *testing.T) {
		userID := uuid.New()
		uploadID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uploadID,
			UserID:          userID,
			Term:            "schol",
			ProvidedMeaning: "trường học",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"school": {
					Lemma:        "school",
					PartOfSpeech: "noun",
					Definition:   "An institution for educating children.",
				},
			},
			candidates: map[string][]string{
				"schol": {"school"},
			},
		}

		modelReplies := map[string]string{
			"schol": `{"valid": false, "reason": "misspelling", "intended_term": "school"}`,
			"school": `{"valid": true, "lemma": "school", "part_of_speech": "noun", "cefr_level": "A1", "meaning_matches": true, "definition": "An institution for educating children.", "definition_vi": "trường học", "examples": [{"sentence": "I go to school.", "sentence_vi": "Tôi đi học."}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Verified as school
		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, "school", repo.correctedTerms[uploadItem.ID])
		assert.Equal(t, "spelling_corrected", repo.noteCodes[uploadItem.ID])

		// Exactly one XP awarded
		require.Len(t, events.verified, 1)
		assert.Equal(t, 1, events.verified[0].Count)
		assert.Equal(t, userID, events.verified[0].UserID)

		// One deck item in user's deck
		decks, err := repo.ListDecksByUser(context.Background(), &userID)
		require.NoError(t, err)
		require.Len(t, decks, 1)
		words, err := repo.ListDeckWords(context.Background(), decks[0].ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, words, 1)
		assert.Equal(t, "school", words[0].Lemma)
	})

	// -------------------------------------------------------------------------
	// 2. form: từ -> Verified as from, meaning_corrected
	// -------------------------------------------------------------------------
	t.Run("form_tu_corrected_to_from", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "form",
			ProvidedMeaning: "từ",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"form": {Lemma: "form", PartOfSpeech: "noun", Definition: "The visible shape of something."},
				"from": {Lemma: "from", PartOfSpeech: "preposition", Definition: "Indicating origin or source."},
			},
		}

		modelReplies := map[string]string{
			"form": `{"valid": true, "lemma": "form", "meaning_matches": false, "intended_term": "from"}`,
			"from": `{"valid": true, "lemma": "from", "part_of_speech": "preposition", "cefr_level": "A1", "meaning_matches": true, "definition": "Indicating origin or source.", "definition_vi": "từ", "examples": [{"sentence": "Where are you from?", "sentence_vi": "Bạn đến từ đâu?"}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Verified as from, meaning_corrected
		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, "from", repo.correctedTerms[uploadItem.ID])
		assert.Equal(t, "meaning_corrected", repo.noteCodes[uploadItem.ID])
	})

	// -------------------------------------------------------------------------
	// 3. schol (no meaning) -> Rejected, spelling_suggestion, suggested school, nothing added
	// -------------------------------------------------------------------------
	t.Run("schol_no_meaning_suggestion_school", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "schol",
			ProvidedMeaning: "",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"school": {Lemma: "school", PartOfSpeech: "noun", Definition: "A school."},
			},
			candidates: map[string][]string{
				"schol": {"school"},
			},
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &stubAI{}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		assert.NotEmpty(t, repo.rejected[uploadItem.ID])
		assert.Equal(t, "school", repo.suggestedTerms[uploadItem.ID])
		assert.Equal(t, "spelling_suggestion", repo.noteCodes[uploadItem.ID])
		assert.Empty(t, repo.verified)
		assert.Empty(t, events.verified)
	})

	// -------------------------------------------------------------------------
	// 4. cat: con chó -> Verified as cat, meaning_mismatch (dog is not close spelling)
	// -------------------------------------------------------------------------
	t.Run("cat_con_cho_meaning_mismatch", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "cat",
			ProvidedMeaning: "con chó",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"cat": {Lemma: "cat", PartOfSpeech: "noun", Definition: "A small domesticated carnivorous mammal."},
				"dog": {Lemma: "dog", PartOfSpeech: "noun", Definition: "A domesticated carnivorous mammal."},
			},
		}

		// Model suggests dog because meaning is con chó, but dog is distance 3 from cat (outside bound)
		modelReplies := map[string]string{
			"cat": `{"valid": true, "lemma": "cat", "part_of_speech": "noun", "cefr_level": "A1", "meaning_matches": false, "intended_term": "dog", "definition": "A feline.", "definition_vi": "con mèo", "examples": [{"sentence": "The cat slept.", "sentence_vi": "Con mèo đang ngủ."}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, "meaning_mismatch", repo.noteCodes[uploadItem.ID])
		assert.Empty(t, repo.correctedTerms[uploadItem.ID])
	})

	// -------------------------------------------------------------------------
	// 5. schol: trường học and school in one paste -> One deck item, one XP
	// -------------------------------------------------------------------------
	t.Run("schol_and_school_in_one_paste_single_xp", func(t *testing.T) {
		userID := uuid.New()
		uploadID := uuid.New()
		item1 := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uploadID,
			UserID:          userID,
			Term:            "schol",
			ProvidedMeaning: "trường học",
			Status:          "pending",
		}
		item2 := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uploadID,
			UserID:          userID,
			Term:            "school",
			ProvidedMeaning: "trường học",
			Status:          "pending",
		}

		repo := newUploadRepo(item1, item2)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"school": {Lemma: "school", PartOfSpeech: "noun", Definition: "An educational institution."},
			},
			candidates: map[string][]string{
				"schol": {"school"},
			},
		}

		schoolReply := `{"valid": true, "lemma": "school", "part_of_speech": "noun", "cefr_level": "A1", "meaning_matches": true, "definition": "An educational institution.", "definition_vi": "trường học", "examples": [{"sentence": "Go to school.", "sentence_vi": "Đi học."}]}`
		modelReplies := map[string]string{
			"schol":  `{"valid": false, "intended_term": "school"}`,
			"school": schoolReply,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Both items are marked verified
		assert.Contains(t, repo.verified, item1.ID)
		assert.Contains(t, repo.verified, item2.ID)

		// Item 1 is spelling_corrected
		assert.Equal(t, "spelling_corrected", repo.noteCodes[item1.ID])
		assert.Equal(t, "school", repo.correctedTerms[item1.ID])

		// Item 2 is already_in_your_words
		assert.Equal(t, "already_in_your_words", repo.noteCodes[item2.ID])

		// Exactly ONE XP awarded total for the upload!
		require.Len(t, events.verified, 1)
		assert.Equal(t, 1, events.verified[0].Count)

		// Exactly one deck item in the deck
		decks, err := repo.ListDecksByUser(context.Background(), &userID)
		require.NoError(t, err)
		require.Len(t, decks, 1)
		words, err := repo.ListDeckWords(context.Background(), decks[0].ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, words, 1)
	})

	// -------------------------------------------------------------------------
	// 6. Model proposes a word the dictionary does not know -> Not added
	// -------------------------------------------------------------------------
	t.Run("model_proposes_unknown_word_not_added", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "schol",
			ProvidedMeaning: "trường học",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{}, // empty dictionary
		}

		modelReplies := map[string]string{
			"schol": `{"valid": false, "intended_term": "school"}`, // dictionary doesn't have school
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Not added, rejected as not_a_word
		assert.Empty(t, repo.verified)
		assert.NotEmpty(t, repo.rejected[uploadItem.ID])
		assert.Equal(t, "not_a_word", repo.noteCodes[uploadItem.ID])
	})

	// -------------------------------------------------------------------------
	// 7. Model proposes a word outside the spelling bound -> Ignored
	// -------------------------------------------------------------------------
	t.Run("model_proposes_word_outside_spelling_bound_ignored", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "banana",
			ProvidedMeaning: "quả táo",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"banana": {Lemma: "banana", PartOfSpeech: "noun", Definition: "A long curved fruit."},
				"apple":  {Lemma: "apple", PartOfSpeech: "noun", Definition: "A round fruit."},
			},
		}

		// Model suggests apple for banana because meaning is quả táo. But distance > 2
		modelReplies := map[string]string{
			"banana": `{"valid": true, "lemma": "banana", "part_of_speech": "noun", "meaning_matches": false, "intended_term": "apple", "definition": "A long curved fruit.", "definition_vi": "quả chuối", "examples": [{"sentence": "Monkeys like bananas.", "sentence_vi": "Khỉ thích chuối."}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Candidate apple ignored because outside bound. Verified as banana with meaning_mismatch!
		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, "meaning_mismatch", repo.noteCodes[uploadItem.ID])
		assert.Empty(t, repo.correctedTerms[uploadItem.ID])
	})

	// -------------------------------------------------------------------------
	// 8. look aftr: chăm sóc -> Today's behaviour, phrases are not corrected
	// -------------------------------------------------------------------------
	t.Run("phrase_not_corrected", func(t *testing.T) {
		userID := uuid.New()
		uploadItem := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uuid.New(),
			UserID:          userID,
			Term:            "look aftr",
			ProvidedMeaning: "chăm sóc",
			Status:          "pending",
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				"look after": {Lemma: "look after", PartOfSpeech: "verb", Definition: "To take care of."},
			},
			candidates: map[string][]string{
				"look aftr": {"look after"},
			},
		}

		// Model is asked for phrase; phrase does not run spelling correction
		modelReplies := map[string]string{
			"look aftr": `{"valid": false, "reason": "We could not find look aftr as an English phrase."}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		assert.Empty(t, repo.verified)
		assert.NotEmpty(t, repo.rejected[uploadItem.ID])
		assert.Equal(t, "not_a_word", repo.noteCodes[uploadItem.ID])
		assert.Empty(t, repo.correctedTerms[uploadItem.ID])
	})
}
