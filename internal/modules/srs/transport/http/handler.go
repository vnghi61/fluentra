// Package http provides HTTP endpoints for review sessions and cards in SRS.
package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	defaultSessionLimit = 20

	// forecastDays is the 30-day horizon srs/AGENT.md §6 describes.
	forecastDays = 30
)

// Guard defines authorization checks for srs handlers.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// SRSService defines the business methods required by the HTTP handler.
type SRSService interface {
	DueCount(ctx context.Context, userID uuid.UUID) (int, error)
	DueCards(ctx context.Context, userID uuid.UUID, limit int32) ([]contract.ReviewCardSummary, error)
	AnswerCard(ctx context.Context, userID, cardID uuid.UUID, grade string, elapsedMs int) (service.AnswerResult, error)
	SuspendCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error)
	ResetCard(ctx context.Context, userID, cardID uuid.UUID) (contract.ReviewCardSummary, error)
	CompleteSession(ctx context.Context, userID uuid.UUID, reviewed, correct int) (service.SessionResult, error)
	Forecast(ctx context.Context, userID uuid.UUID, days int) ([]service.ForecastDay, error)
}

// Handler serves HTTP endpoints for spaced repetition.
type Handler struct {
	service SRSService
	guard   Guard
}

// NewHandler constructs an SRS HTTP Handler.
func NewHandler(service SRSService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED", "authorization guard is required for srs handlers")
	}
	return &Handler{
		service: service,
		guard:   guard,
	}, nil
}

// Routes mounts the review endpoints on the router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/reviews/session", h.getReviewSession)
	router.Get("/reviews/due-count", h.getReviewDueCount)
	router.Post("/reviews/{id}/answer", h.answerReviewCard)
	router.Post("/reviews/session/complete", h.completeReviewSession)
	router.Post("/reviews/{id}/suspend", h.suspendReviewCard)
	router.Post("/reviews/{id}/reset", h.resetReviewCard)
	router.Get("/reviews/forecast", h.getReviewForecast)
}

func (h *Handler) getReviewSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	// ParseInt with a 32-bit size rather than Atoi plus a conversion: the width
	// is enforced by the parse, so an oversized query string is rejected instead
	// of wrapping. The service clamps the value to its own maximum.
	limit := int32(defaultSessionLimit)
	if val, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32); err == nil && val > 0 {
		limit = int32(val)
	}

	cards, err := h.service.DueCards(ctx, actor.UserID, limit)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	cardResponses := make([]ReviewCardResponse, 0, len(cards))
	for _, c := range cards {
		cardResponses = append(cardResponses, mapCardResponse(c))
	}

	httpx.WriteJSON(w, r, http.StatusOK, ReviewSessionResponse{
		Cards: cardResponses,
	})
}

func (h *Handler) getReviewDueCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	count, err := h.service.DueCount(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, DueCountResponse{
		DueCount: count,
	})
}

func (h *Handler) answerReviewCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_CARD_ID", "Card ID must be a valid UUID."))
		return
	}

	var req AnswerReviewRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	result, err := h.service.AnswerCard(ctx, actor.UserID, cardID, req.Grade, req.ElapsedMs)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, AnswerReviewResponse{
		Card:         mapCardResponse(result.Card),
		NextDueAt:    result.NextDueAt,
		IntervalDays: result.IntervalDays,
	})
}

func (h *Handler) completeReviewSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	var req CompleteReviewSessionRequest
	_ = httpx.DecodeJSON(r, &req)

	result, err := h.service.CompleteSession(ctx, actor.UserID, req.Reviewed, req.Correct)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, CompleteReviewSessionResponse{
		Reviewed:    result.Reviewed,
		Correct:     result.Correct,
		Minutes:     result.Minutes,
		CompletedAt: result.CompletedAt,
	})
}

func (h *Handler) suspendReviewCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_CARD_ID", "Card ID must be a valid UUID."))
		return
	}

	card, err := h.service.SuspendCard(ctx, actor.UserID, cardID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, mapCardResponse(card))
}

func (h *Handler) resetReviewCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_CARD_ID", "Card ID must be a valid UUID."))
		return
	}

	card, err := h.service.ResetCard(ctx, actor.UserID, cardID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, mapCardResponse(card))
}

func (h *Handler) getReviewForecast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	forecast, err := h.service.Forecast(ctx, actor.UserID, forecastDays)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	days := make([]ForecastItem, 0, len(forecast))
	for _, day := range forecast {
		days = append(days, ForecastItem{Date: day.Date, DueCount: day.DueCount})
	}

	httpx.WriteJSON(w, r, http.StatusOK, ForecastResponse{Days: days})
}
