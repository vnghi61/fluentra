package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type placementSessionResponse struct {
	Session         any                           `json:"session"`
	CurrentActivity *lessoncontract.ActivityHierarchy `json:"current_activity,omitempty"`
}

type submitPlacementAnswerRequest struct {
	Response json.RawMessage `json:"response"`
}

type submitPlacementAnswerResponse struct {
	Session      any                           `json:"session"`
	NextActivity *lessoncontract.ActivityHierarchy `json:"next_activity,omitempty"`
	Finished     bool                          `json:"finished"`
}

func (h *Handler) getPlacementInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	dto, err := h.service.GetPlacementInvitation(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, dto)
}

func (h *Handler) startPlacementSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	session, act, err := h.service.StartPlacementSession(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, placementSessionResponse{
		Session:         session,
		CurrentActivity: act,
	})
}

func (h *Handler) getPlacementSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid session ID"))
		return
	}

	session, act, err := h.service.GetPlacementSession(ctx, actor.UserID, sessionID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, placementSessionResponse{
		Session:         session,
		CurrentActivity: act,
	})
}

func (h *Handler) submitPlacementAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid session ID"))
		return
	}

	var req submitPlacementAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST", "Invalid request body"))
		return
	}
	if len(req.Response) == 0 {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "RESPONSE_REQUIRED", "Response is required"))
		return
	}

	session, nextAct, finished, err := h.service.SubmitPlacementAnswer(ctx, actor.UserID, sessionID, req.Response)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, submitPlacementAnswerResponse{
		Session:      session,
		NextActivity: nextAct,
		Finished:     finished,
	})
}

func (h *Handler) getStartingPath(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	path, err := h.service.GetStartingPath(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, path)
}

func (h *Handler) getWeeklyPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	plan, err := h.service.GetWeeklyPlan(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, plan)
}
