// Package http is the rbac module's HTTP surface: the `/admin/*` route guard,
// and the operations that read and change roles.
package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// RoleChecker answers whether an actor holds a role. It is narrower than the
// service on purpose: the middleware needs one question answered and should
// not be able to change anything.
type RoleChecker interface {
	HasRole(ctx context.Context, userID uuid.UUID, role contract.Role) (bool, error)
}

// AdminOnly refuses any request from an actor who does not hold `admin`.
//
// It is a route-group guard, not the authorization. Services still call
// `Require` with the specific permission the operation needs: this middleware
// says "you are staff", the guard says "you may do this particular thing", and
// an operation reached by a job or an event has no middleware at all.
//
// Three things deny here, with the same 403: no actor in the context, a role
// lookup that failed, and an actor without the role. The middle one is the one
// worth stating — a guard that lets requests through when the database is
// unreachable turns an outage into an open back office.
func AdminOnly(roles RoleChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			actor, ok := httpx.ActorFrom(request.Context())
			if !ok {
				httpx.WriteProblem(writer, request, apperr.New(
					apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token."))
				return
			}

			isAdmin, err := roles.HasRole(request.Context(), actor.UserID, contract.RoleAdmin)
			if err != nil {
				slog.ErrorContext(request.Context(), "admin role lookup failed, denying",
					"module", "rbac", "op", "AdminOnly", "error", err)
				httpx.WriteProblem(writer, request, domain.ErrPermissionDenied)
				return
			}
			if !isAdmin {
				slog.WarnContext(request.Context(), "non-admin request to an admin route",
					"module", "rbac", "op", "AdminOnly", "route", request.URL.Path)
				httpx.WriteProblem(writer, request, domain.ErrPermissionDenied)
				return
			}

			next.ServeHTTP(writer, request)
		})
	}
}
