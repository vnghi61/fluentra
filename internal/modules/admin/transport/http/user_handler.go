package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// fieldStatus names the one key every account-state response carries. It is a
// constant because the three handlers that write it must not drift apart: the
// web reads this field to decide what the row may do next.
const fieldStatus = "status"

type userReasonRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) searchUsers(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "user.list")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	q := r.URL.Query()
	var filter usercontract.UserFilter

	if emailPrefix := q.Get("email_prefix"); emailPrefix != "" {
		filter.EmailPrefix = &emailPrefix
	}
	if displayName := q.Get("display_name"); displayName != "" {
		filter.DisplayName = &displayName
	}
	if status := q.Get("status"); status != "" {
		filter.Status = &status
	}
	if afterStr := q.Get("created_after"); afterStr != "" {
		if t, err := time.Parse(time.RFC3339, afterStr); err == nil {
			filter.CreatedAfter = &t
		}
	}
	if beforeStr := q.Get("created_before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			filter.CreatedBefore = &t
		}
	}

	cursor := q.Get("cursor")
	limit := 20
	if limitStr := q.Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	page, err := h.service.SearchUsers(r.Context(), filter, cursor, limit)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// `total` is always present, including when it is zero. AdminUserPage
	// declares it required, and a footer that has to tell "no matches" apart
	// from "the server did not say" is a footer that shows neither.
	resp := map[string]any{
		keyItems: page.Items,
		"total":  page.Total,
	}
	if page.NextCursor != "" {
		resp["next_cursor"] = page.NextCursor
	}

	httpx.WriteJSON(w, r, http.StatusOK, resp)
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, "user.read")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_USER_ID", "user id must be a valid UUID"))
		return
	}

	detail, err := h.service.GetUserByID(r.Context(), actor.UserID, targetID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, detail)
}

func (h *Handler) suspendUser(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, "user.suspend")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_USER_ID", "user id must be a valid UUID"))
		return
	}

	var req userReasonRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := h.service.SuspendUser(r.Context(), actor.UserID, targetID, req.Reason); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"id":        targetID,
		fieldStatus: "suspended",
	})
}

func (h *Handler) softDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, "user.delete")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_USER_ID", "user id must be a valid UUID"))
		return
	}

	var req userReasonRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := h.service.SoftDeleteUser(r.Context(), actor.UserID, targetID, req.Reason); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"id":        targetID,
		fieldStatus: "pending_deletion",
	})
}

func (h *Handler) reinstateUser(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, "user.reinstate")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_USER_ID", "user id must be a valid UUID"))
		return
	}

	var req userReasonRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := h.service.ReinstateUser(r.Context(), actor.UserID, targetID, req.Reason); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"id":        targetID,
		fieldStatus: "active",
	})
}

func (h *Handler) revokeSessions(w http.ResponseWriter, r *http.Request) {
	actor, err := h.authorise(r, "user.manage_sessions")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_USER_ID", "user id must be a valid UUID"))
		return
	}

	var req userReasonRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := h.service.RevokeUserSessions(r.Context(), actor.UserID, targetID, req.Reason); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"id":      targetID,
		"revoked": true,
	})
}
