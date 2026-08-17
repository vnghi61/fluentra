// Package http is the admin module's HTTP surface: the `/admin/users` and
// `/admin/flags` handlers. Every handler authorises the caller with a named
// permission before it does any work.
package http

import (
	"context"
	"net/http"

	"github.com/fluentra/fluentra/internal/modules/admin/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/go-chi/chi/v5"
)

// Guard is the authorization surface these handlers need.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// Handler serves HTTP endpoints for the admin module.
type Handler struct {
	service *service.Service
	guard   Guard
}

// NewHandler creates a new Handler.
func NewHandler(service *service.Service, guard Guard) *Handler {
	return &Handler{service: service, guard: guard}
}

// Routes registers admin routes on chi.Router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/admin/users", h.searchUsers)
	router.Get("/admin/users/{id}", h.getUser)
	router.Post("/admin/users/{id}/suspend", h.suspendUser)
	router.Post("/admin/users/{id}/reinstate", h.reinstateUser)
	router.Post("/admin/users/{id}/sessions/revoke", h.revokeSessions)

	router.Get("/admin/flags", h.listFlags)
	router.Post("/admin/flags", h.createFlag)
	router.Put("/admin/flags/{key}", h.updateFlag)
	router.Delete("/admin/flags/{key}", h.deleteFlag)
}

func (h *Handler) authorise(r *http.Request, permission string) (httpx.Actor, error) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok {
		return httpx.Actor{}, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token.")
	}
	if h.guard != nil {
		if err := h.guard.Require(r.Context(), permission); err != nil {
			return httpx.Actor{}, err
		}
	}
	return actor, nil
}
