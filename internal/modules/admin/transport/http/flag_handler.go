package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "system.flags")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	flags, err := h.service.ListFlags(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		keyItems: flags,
	})
}

func (h *Handler) createFlag(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "system.flags")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var payload struct {
		Key            string `json:"key"`
		Enabled        bool   `json:"enabled"`
		RolloutPercent int    `json:"rollout_percent"`
		Owner          string `json:"owner"`
		ExpiresOn      string `json:"expires_on"`
		Description    string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_BODY", "failed to decode request body"))
		return
	}

	expiresTime, err := time.Parse("2006-01-02", payload.ExpiresOn)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_EXPIRY_FORMAT", "expires_on must be YYYY-MM-DD"))
		return
	}

	flag, err := h.service.CreateFlag(r.Context(), adminsvc.CreateFlagRequest{
		Key:            payload.Key,
		Enabled:        payload.Enabled,
		RolloutPercent: payload.RolloutPercent,
		Owner:          payload.Owner,
		ExpiresOn:      expiresTime,
		Description:    payload.Description,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, flag)
}

func (h *Handler) updateFlag(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "system.flags")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "KEY_REQUIRED", "flag key is required"))
		return
	}

	var payload struct {
		Enabled        bool   `json:"enabled"`
		RolloutPercent int    `json:"rollout_percent"`
		ExpiresOn      string `json:"expires_on"`
		Description    string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_BODY", "failed to decode request body"))
		return
	}

	expiresTime, err := time.Parse("2006-01-02", payload.ExpiresOn)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_EXPIRY_FORMAT", "expires_on must be YYYY-MM-DD"))
		return
	}

	flag, err := h.service.UpdateFlag(r.Context(), key, adminsvc.UpdateFlagRequest{
		Enabled:        payload.Enabled,
		RolloutPercent: payload.RolloutPercent,
		ExpiresOn:      expiresTime,
		Description:    payload.Description,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, flag)
}

func (h *Handler) deleteFlag(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "system.flags")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "KEY_REQUIRED", "flag key is required"))
		return
	}

	if err := h.service.DeleteFlag(r.Context(), key); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
