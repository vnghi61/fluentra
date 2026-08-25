// Package http provides HTTP handlers for vocabulary dictionary and deck endpoints.
package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// PermContentCreate is the permission the authoring endpoint requires. It is a
// named constant for the same reason lesson's is: the route table test and the
// handler must be able to refer to the same string.
const PermContentCreate = "content.create"

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// paging reads the limit and offset query parameters, bounded so a caller
// cannot ask for an unbounded page or overflow the int32 the query takes.
func paging(r *http.Request) (limit, offset int32) {
	limit = defaultPageLimit
	// ParseInt with a 32-bit size, not Atoi: the width is enforced by the parse
	// rather than by a conversion afterwards, so an oversized query string is
	// rejected instead of wrapping.
	if val, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32); err == nil && val > 0 {
		if val > maxPageLimit {
			val = maxPageLimit
		}
		limit = int32(val)
	}
	if val, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 32); err == nil && val > 0 {
		offset = int32(val)
	}
	return limit, offset
}

// Guard defines authorization checks.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// VocabularyService defines the operations exposed by HTTP handlers.
type VocabularyService interface {
	LookupWord(ctx context.Context, lemma string) ([]domain.Word, error)
	SearchWords(ctx context.Context, query string, limit, offset int32) ([]domain.Word, int, error)
	CreateWord(ctx context.Context, word domain.Word) (domain.Word, error)
	CreateDeck(
		ctx context.Context, ownerID *uuid.UUID, slug, name string, description *string, isPublic bool) (domain.Deck, error,
	)
	GetDeck(ctx context.Context, deckID uuid.UUID) (domain.Deck, error)
	ListDecks(ctx context.Context, userID *uuid.UUID) ([]domain.Deck, error)
	AddWordToDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error
	RemoveWordFromDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error
	ListDeckWords(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]domain.DeckItem, error)
	SetWordState(ctx context.Context, userID, wordSenseID uuid.UUID, status domain.WordStatus) error
	GetWordState(ctx context.Context, userID, wordSenseID uuid.UUID) (domain.UserWordState, error)
}

// Handler serves HTTP endpoints for vocabulary.
type Handler struct {
	service VocabularyService
	guard   Guard
}

// NewHandler constructs a Vocabulary HTTP Handler.
func NewHandler(service VocabularyService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED", "authorization guard is required for vocabulary handlers")
	}
	return &Handler{
		service: service,
		guard:   guard,
	}, nil
}

// Routes mounts the learner vocabulary endpoints on the router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/vocabulary/words/{lemma}", h.lookupWord)
	router.Get("/vocabulary/search", h.searchWords)
	router.Get("/vocabulary/decks", h.listDecks)
	router.Post("/vocabulary/decks", h.createDeck)
	router.Get("/vocabulary/decks/{id}/words", h.listDeckWords)
	router.Post("/vocabulary/decks/{id}/words", h.addWordToDeck)
	router.Delete("/vocabulary/decks/{id}/words/{sense_id}", h.removeWordFromDeck)
	router.Post("/vocabulary/words/{sense_id}/state", h.setWordState)
}

// AdminRoutes mounts the staff-facing vocabulary authoring endpoints on the router.
func (h *Handler) AdminRoutes(router chi.Router) {
	router.Post("/admin/vocabulary/words", h.adminCreateWord)
}

func (h *Handler) lookupWord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lemma := chi.URLParam(r, "lemma")
	if lemma == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_LEMMA", "Lemma is required."))
		return
	}

	words, err := h.service.LookupWord(ctx, lemma)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	wordDTOs := make([]WordDetailDTO, 0, len(words))
	for _, word := range words {
		wordDTOs = append(wordDTOs, mapWordDetail(word))
	}

	httpx.WriteJSON(w, r, http.StatusOK, DictionaryLookupResponse{
		Words: wordDTOs,
	})
}

func (h *Handler) searchWords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	if query == "" {
		httpx.WriteJSON(w, r, http.StatusOK, SearchWordsResponse{
			Results: make([]WordSummaryDTO, 0),
		})
		return
	}

	limit, offset := paging(r)

	words, total, err := h.service.SearchWords(ctx, query, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	results := make([]WordSummaryDTO, 0, len(words))
	for _, word := range words {
		results = append(results, mapWordSummary(word))
	}

	httpx.WriteJSON(w, r, http.StatusOK, SearchWordsResponse{
		Results: results,
		Total:   total,
	})
}

func (h *Handler) listDecks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)

	var ownerID *uuid.UUID
	if ok && actor.UserID != uuid.Nil {
		ownerID = &actor.UserID
	}

	decks, err := h.service.ListDecks(ctx, ownerID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	deckDTOs := make([]DeckDTO, 0, len(decks))
	for _, deck := range decks {
		deckDTOs = append(deckDTOs, mapDeckDTO(deck))
	}

	httpx.WriteJSON(w, r, http.StatusOK, ListDecksResponse{
		Decks: deckDTOs,
	})
}

func (h *Handler) createDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	var req CreateDeckRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	deck, err := h.service.CreateDeck(ctx, &actor.UserID, req.Slug, req.Name, req.Description, req.IsPublic)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, mapDeckDTO(deck))
}

func (h *Handler) listDeckWords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_DECK_ID", "Deck ID must be a valid UUID."))
		return
	}

	limit, offset := paging(r)

	items, err := h.service.ListDeckWords(ctx, deckID, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	itemDTOs := make([]DeckItemDTO, 0, len(items))
	for _, it := range items {
		examples := make([]ExampleSentenceDTO, 0)
		if it.Sense != nil {
			for _, e := range it.Sense.Examples {
				examples = append(examples, ExampleSentenceDTO{
					Sentence:   e.Sentence,
					SentenceVi: e.SentenceVi,
					AudioURL:   e.AudioURL,
				})
			}
		}

		lemma := ""
		pos := ""
		cefr := "A1"
		var ipa *string
		var audioURL *string
		if it.Word != nil {
			lemma = it.Word.Lemma
			pos = string(it.Word.POS)
			cefr = string(it.Word.CEFRLevel)
			ipa = it.Word.IPA
			audioURL = it.Word.AudioURL
		}

		def := ""
		var defVi *string
		var wordID uuid.UUID
		if it.Sense != nil {
			def = it.Sense.Definition
			defVi = it.Sense.DefinitionVi
			wordID = it.Sense.WordID
		}

		itemDTOs = append(itemDTOs, DeckItemDTO{
			SenseID:      it.WordSenseID,
			WordID:       wordID,
			Lemma:        lemma,
			POS:          pos,
			CEFRLevel:    cefr,
			IPA:          ipa,
			AudioURL:     audioURL,
			Definition:   def,
			DefinitionVi: defVi,
			Examples:     examples,
			AddedAt:      it.AddedAt,
		})
	}

	httpx.WriteJSON(w, r, http.StatusOK, ListDeckWordsResponse{
		Items: itemDTOs,
	})
}

func (h *Handler) addWordToDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_DECK_ID", "Deck ID must be a valid UUID."))
		return
	}

	var req AddWordToDeckRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	if err := h.service.AddWordToDeck(ctx, deckID, req.WordSenseID); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeWordFromDeck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	deckID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_DECK_ID", "Deck ID must be a valid UUID."))
		return
	}

	senseID, err := uuid.Parse(chi.URLParam(r, "sense_id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_SENSE_ID", "Word sense ID must be a valid UUID."))
		return
	}

	if err := h.service.RemoveWordFromDeck(ctx, deckID, senseID); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setWordState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	senseID, err := uuid.Parse(chi.URLParam(r, "sense_id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_SENSE_ID", "Word sense ID must be a valid UUID."))
		return
	}

	var req SetWordStateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	status := domain.WordStatus(req.Status)
	if status != domain.StatusNew && status != domain.StatusLearning &&
		status != domain.StatusKnown && status != domain.StatusIgnored {
		httpx.WriteProblem(
			w, r, apperr.New(apperr.Validation, "INVALID_STATUS", "Status must be one of: new, learning, known, ignored."),
		)
		return
	}

	if err := h.service.SetWordState(ctx, actor.UserID, senseID, status); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// Read the state back rather than echoing the request. first_seen_at is part
	// of the published response and only the stored row knows it — the learner
	// may have met this sense weeks ago.
	state, err := h.service.GetWordState(ctx, actor.UserID, senseID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	response := SetWordStateResponse{
		UserID:      actor.UserID,
		WordSenseID: senseID,
		Status:      string(state.Status),
		FirstSeenAt: state.FirstSeenAt,
	}
	if !state.UpdatedAt.IsZero() {
		response.UpdatedAt = &state.UpdatedAt
	}
	httpx.WriteJSON(w, r, http.StatusOK, response)
}

func (h *Handler) adminCreateWord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentCreate); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var req CreateWordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	domWord := domain.Word{
		Lemma:         req.Lemma,
		POS:           domain.PartOfSpeech(req.POS),
		CEFRLevel:     domain.CEFRLevel(req.CEFRLevel),
		FrequencyRank: req.FrequencyRank,
		IPA:           req.IPA,
		AudioAssetID:  req.AudioAssetID,
	}

	created, err := h.service.CreateWord(ctx, domWord)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, mapWordDetail(created))
}
