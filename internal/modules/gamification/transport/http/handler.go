// Package http serves the learner-facing gamification endpoints.
package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/gamification/contract"
	"github.com/fluentra/fluentra/internal/modules/gamification/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Guard defines the authorization check handlers run.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// GamificationService is what the handler needs from the service.
type GamificationService interface {
	SummaryOf(ctx context.Context, userID uuid.UUID) (contract.Summary, error)
	UseFreeze(ctx context.Context, userID uuid.UUID) (contract.Streak, error)
	SetDailyGoal(ctx context.Context, userID uuid.UUID, goal int) error
	SetLeaderboardOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error
	Leaderboard(ctx context.Context, userID uuid.UUID) ([]service.LeaderboardEntry, error)
}

// Handler serves the gamification endpoints.
type Handler struct {
	service GamificationService
	guard   Guard
}

// NewHandler constructs the handler, refusing to build one without a guard.
func NewHandler(svc GamificationService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED",
			"authorization guard is required for gamification handlers")
	}
	return &Handler{service: svc, guard: guard}, nil
}

// Routes mounts the gamification endpoints.
//
// Every route is under /me except the leaderboard, and every one of them acts
// on the authenticated learner rather than on an id in the path: there is no
// route here through which one learner can read or change another's state.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/me/gamification", h.getSummary)
	router.Get("/me/streak", h.getStreak)
	router.Post("/me/streak/freeze", h.useFreeze)
	router.Put("/me/daily-goal", h.setDailyGoal)
	router.Put("/me/leaderboard-opt-in", h.setLeaderboardOptIn)
	router.Get("/leaderboard", h.getLeaderboard)
}

// actor resolves the authenticated learner, or writes the 401 itself.
func actor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	who, ok := httpx.ActorFrom(r.Context())
	if !ok || who.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return uuid.Nil, false
	}
	return who.UserID, true
}

func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	summary, err := h.service.SummaryOf(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, mapSummary(summary))
}

func (h *Handler) getStreak(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	// Read through the summary rather than a second service method: the streak
	// needs the same timezone resolution and the same day arithmetic, and a
	// parallel path is a parallel path to get wrong.
	summary, err := h.service.SummaryOf(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, mapStreak(summary.Streak))
}

func (h *Handler) useFreeze(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	streak, err := h.service.UseFreeze(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, mapStreak(streak))
}

func (h *Handler) setDailyGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	var req SetDailyGoalRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if err := h.service.SetDailyGoal(r.Context(), userID, req.DailyGoalXP); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	summary, err := h.service.SummaryOf(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, mapSummary(summary))
}

func (h *Handler) setLeaderboardOptIn(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	var req SetLeaderboardOptInRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if err := h.service.SetLeaderboardOptIn(r.Context(), userID, req.OptIn); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := actor(w, r)
	if !ok {
		return
	}
	entries, err := h.service.Leaderboard(r.Context(), userID)
	if err != nil {
		// LEADERBOARD_NOT_OPTED_IN reaches the learner as a 403 with its own
		// code, so the screen can offer the opt-in rather than showing an error.
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, mapLeaderboard(entries))
}
