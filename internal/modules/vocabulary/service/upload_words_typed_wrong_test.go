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

// The words these cases type and the words they meant.
const (
	wordSchol       = "schol"
	wordSchool      = "school"
	wordForm        = "form"
	wordFrom        = "from"
	wordCat         = "cat"
	wordDog         = "dog"
	wordBanana      = "banana"
	phraseLookAftr  = "look aftr"
	phraseLookAfter = "look after"
	meaningSchool   = "trường học"
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
			Term:            wordSchol,
			ProvidedMeaning: meaningSchool,
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordSchool: {
					Lemma:        wordSchool,
					PartOfSpeech: posNoun,
					Definition:   "An institution for educating children.",
				},
			},
			candidates: map[string][]string{
				wordSchol: {wordSchool},
			},
		}

		modelReplies := map[string]string{
			wordSchol: `{"valid": false, "reason": "misspelling", "intended_term": "school"}`,
			wordSchool: `{"valid": true, "lemma": "school", "part_of_speech": "noun", "cefr_level": "A1",
				"meaning_matches": true, "definition": "An institution for educating children.",
				"definition_vi": "trường học", "examples": [{"sentence": "I go to school.",
				"sentence_vi": "Tôi đi học."}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Verified as school
		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, wordSchool, repo.correctedTerms[uploadItem.ID])
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
		assert.Equal(t, wordSchool, words[0].Lemma)
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
			Term:            wordForm,
			ProvidedMeaning: "từ",
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordForm: {Lemma: wordForm, PartOfSpeech: posNoun, Definition: "The visible shape of something."},
				wordFrom: {Lemma: wordFrom, PartOfSpeech: "preposition", Definition: "Indicating origin or source."},
			},
		}

		modelReplies := map[string]string{
			wordForm: `{"valid": true, "lemma": "form", "meaning_matches": false, "intended_term": "from"}`,
			wordFrom: `{"valid": true, "lemma": "from", "part_of_speech": "preposition", "cefr_level": "A1",
				"meaning_matches": true, "definition": "Indicating origin or source.", "definition_vi": "từ",
				"examples": [{"sentence": "Where are you from?", "sentence_vi": "Bạn đến từ đâu?"}]}`,
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &routingAI{responses: modelReplies}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		// Verified as from, meaning_corrected
		assert.Equal(t, "ai", repo.verified[uploadItem.ID])
		assert.Equal(t, wordFrom, repo.correctedTerms[uploadItem.ID])
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
			Term:            wordSchol,
			ProvidedMeaning: "",
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordSchool: {Lemma: wordSchool, PartOfSpeech: posNoun, Definition: "A school."},
			},
			candidates: map[string][]string{
				wordSchol: {wordSchool},
			},
		}

		events := &spyEvents{}
		uploads := newTypedWrongPipeline(t, repo, dict, &stubAI{}, events)

		err := uploads.VerifyPending(context.Background())
		require.NoError(t, err)

		assert.NotEmpty(t, repo.rejected[uploadItem.ID])
		assert.Equal(t, wordSchool, repo.suggestedTerms[uploadItem.ID])
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
			Term:            wordCat,
			ProvidedMeaning: "con chó",
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordCat: {Lemma: wordCat, PartOfSpeech: posNoun, Definition: "A small domesticated carnivorous mammal."},
				wordDog: {Lemma: wordDog, PartOfSpeech: posNoun, Definition: "A domesticated carnivorous mammal."},
			},
		}

		// Model suggests dog because meaning is con chó, but dog is distance 3 from cat (outside bound)
		modelReplies := map[string]string{
			wordCat: `{"valid": true, "lemma": "cat", "part_of_speech": "noun", "cefr_level": "A1",
				"meaning_matches": false, "intended_term": "dog", "definition": "A feline.", "definition_vi": "con mèo",
				"examples": [{"sentence": "The cat slept.", "sentence_vi": "Con mèo đang ngủ."}]}`,
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
			Term:            wordSchol,
			ProvidedMeaning: meaningSchool,
			Status:          statusPending,
		}
		item2 := sqlc.SkillVocabUploadItem{
			ID:              uuid.New(),
			UploadID:        uploadID,
			UserID:          userID,
			Term:            wordSchool,
			ProvidedMeaning: meaningSchool,
			Status:          statusPending,
		}

		repo := newUploadRepo(item1, item2)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordSchool: {Lemma: wordSchool, PartOfSpeech: posNoun, Definition: "An educational institution."},
			},
			candidates: map[string][]string{
				wordSchol: {wordSchool},
			},
		}

		schoolReply := `{"valid": true, "lemma": "school", "part_of_speech": "noun", "cefr_level": "A1",
			"meaning_matches": true, "definition": "An educational institution.", "definition_vi": "trường học",
			"examples": [{"sentence": "Go to school.", "sentence_vi": "Đi học."}]}`
		modelReplies := map[string]string{
			wordSchol:  `{"valid": false, "intended_term": "school"}`,
			wordSchool: schoolReply,
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
		assert.Equal(t, wordSchool, repo.correctedTerms[item1.ID])

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
			Term:            wordSchol,
			ProvidedMeaning: meaningSchool,
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{}, // empty dictionary
		}

		modelReplies := map[string]string{
			wordSchol: `{"valid": false, "intended_term": "school"}`, // dictionary doesn't have school
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
			Term:            wordBanana,
			ProvidedMeaning: "quả táo",
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				wordBanana: {Lemma: wordBanana, PartOfSpeech: posNoun, Definition: "A long curved fruit."},
				wordApple:  {Lemma: wordApple, PartOfSpeech: posNoun, Definition: defRoundFruit},
			},
		}

		// Model suggests apple for banana because meaning is quả táo. But distance > 2
		modelReplies := map[string]string{
			wordBanana: `{"valid": true, "lemma": "banana", "part_of_speech": "noun", "meaning_matches": false,
				"intended_term": "apple", "definition": "A long curved fruit.", "definition_vi": "quả chuối",
				"examples": [{"sentence": "Monkeys like bananas.", "sentence_vi": "Khỉ thích chuối."}]}`,
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
			Term:            phraseLookAftr,
			ProvidedMeaning: "chăm sóc",
			Status:          statusPending,
		}

		repo := newUploadRepo(uploadItem)
		dict := &stubDictionary{
			entries: map[string]repository.DictionaryEntry{
				phraseLookAfter: {Lemma: phraseLookAfter, PartOfSpeech: "verb", Definition: "To take care of."},
			},
			candidates: map[string][]string{
				phraseLookAftr: {phraseLookAfter},
			},
		}

		// Model is asked for phrase; phrase does not run spelling correction
		modelReplies := map[string]string{
			phraseLookAftr: `{"valid": false, "reason": "We could not find look aftr as an English phrase."}`,
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
