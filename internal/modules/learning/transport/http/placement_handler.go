package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// The placement test, the starting path and the weekly plan — work order 13.

// maxPlacementAnswerBytes bounds an answer body. A writing answer is the longest.
const maxPlacementAnswerBytes = 1 << 20

type placementAnswerRequest struct {
	ActivityID uuid.UUID       `json:"activity_id"`
	Response   json.RawMessage `json:"response"`
}

type placementProductiveRequest struct {
	Skip bool `json:"skip"`
}

func (h *Handler) getPlacement(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	overview, err := h.service.GetPlacementOverview(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, overview)
}

func (h *Handler) startPlacement(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	session, err := h.service.StartPlacement(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusCreated, session)
}

func (h *Handler) getPlacementSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	sessionID, ok := sessionIDParam(w, r)
	if !ok {
		return
	}
	session, err := h.service.GetPlacementSession(r.Context(), userID, sessionID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, session)
}

func (h *Handler) answerPlacement(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	sessionID, ok := sessionIDParam(w, r)
	if !ok {
		return
	}
	key, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.WriteProblem(w, r, domain.ErrInvalidIdempotencyKey)
		return
	}

	var req placementAnswerRequest
	body := http.MaxBytesReader(w, r.Body, maxPlacementAnswerBytes)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "ANSWER_TOO_LARGE", "The answer is too large."))
			return
		}
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
		return
	}
	if req.ActivityID == uuid.Nil || len(req.Response) == 0 {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY",
			"activity_id and response are required"))
		return
	}

	session, err := h.service.SubmitPlacementAnswer(r.Context(), userID, sessionID, req.ActivityID, key, req.Response)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, session)
}

func (h *Handler) placementProductive(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	sessionID, ok := sessionIDParam(w, r)
	if !ok {
		return
	}
	var req placementProductiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
		return
	}
	session, err := h.service.StartPlacementProductive(r.Context(), userID, sessionID, req.Skip)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, session)
}

func (h *Handler) getStartingPath(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	path, err := h.service.GetStartingPath(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, path)
}

func (h *Handler) getWeeklyPlan(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	plan, err := h.service.GetWeeklyPlan(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, plan)
}

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return uuid.Nil, false
	}
	return actor.UserID, true
}

func sessionIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid session ID format"))
		return uuid.Nil, false
	}
	return id, true
}
