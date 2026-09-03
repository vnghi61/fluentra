package http

import (
	"net/http"

	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// AdminAIUsageItemDTO models one provider and task budget item.
type AdminAIUsageItemDTO struct {
	Provider          string `json:"provider"`
	Task              string `json:"task"`
	RequestsToday     int64  `json:"requests_today"`
	TokensToday       int64  `json:"tokens_today"`
	DailyRequestLimit *int   `json:"daily_request_limit"`
	DailyTokenLimit   *int64 `json:"daily_token_limit"`
	IsExhausted       bool   `json:"is_exhausted"`
}

func (h *Handler) getAIUsage(w http.ResponseWriter, r *http.Request) {
	_, err := h.authorise(r, "admin.dashboard")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	usage, err := h.service.GetAIUsage(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]AdminAIUsageItemDTO, 0, len(usage))
	for _, u := range usage {
		items = append(items, AdminAIUsageItemDTO{
			Provider:          u.Provider,
			Task:              u.Task,
			RequestsToday:     u.RequestsToday,
			TokensToday:       u.TokensToday,
			DailyRequestLimit: u.DailyRequestLimit,
			DailyTokenLimit:   u.DailyTokenLimit,
			IsExhausted:       u.IsExhausted,
		})
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		keyItems: items,
	})
}
