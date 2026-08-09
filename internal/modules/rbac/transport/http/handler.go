package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// maxBodyBytes is ample for the one request body this module accepts.
const maxBodyBytes = 4 << 10

// fieldRole is the only member of that body.
const fieldRole = "role"

// Roles is the service surface these handlers need.
type Roles interface {
	RoleChecker
	Require(ctx context.Context, permission contract.Permission) error
	RolesOf(ctx context.Context, userID uuid.UUID) ([]contract.Role, error)
	PermissionsOf(ctx context.Context, userID uuid.UUID) ([]contract.Permission, error)
	ListRoles(ctx context.Context) ([]domain.RoleWithPermissions, error)
	AssignRole(ctx context.Context, actorID, targetID uuid.UUID, role contract.Role) ([]contract.Role, error)
	RevokeRole(ctx context.Context, actorID, targetID uuid.UUID, role contract.Role) ([]contract.Role, error)
}

// Handler serves the rbac operations.
type Handler struct {
	roles Roles
}

// NewHandler creates the handler.
func NewHandler(roles Roles) *Handler { return &Handler{roles: roles} }

// Routes mounts this module's operations.
//
// `/me/permissions` needs only a caller. The `/admin` group is wrapped in
// AdminOnly *and* every handler under it calls Require with its own
// permission — the middleware is a fence around the route, the guard is the
// lock on the operation, and neither is a substitute for the other.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/me/permissions", h.getMyPermissions)

	router.Route("/admin", func(admin chi.Router) {
		admin.Use(AdminOnly(h.roles))
		admin.Get("/roles", h.listRoles)
		admin.Post("/users/{id}/roles", h.assignRole)
		admin.Delete("/users/{id}/roles/{role}", h.revokeRole)
	})
}

func (h *Handler) getMyPermissions(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	roles, err := h.roles.RolesOf(request.Context(), actor.UserID)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	permissions, err := h.roles.PermissionsOf(request.Context(), actor.UserID)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	httpx.WriteJSON(writer, request, http.StatusOK, myPermissionsResponse{
		Roles: roleNames(roles), Permissions: permissionNames(permissions),
	})
}

func (h *Handler) listRoles(writer http.ResponseWriter, request *http.Request) {
	if err := h.roles.Require(request.Context(), contract.PermRBACRead); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	catalogue, err := h.roles.ListRoles(request.Context())
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	items := make([]roleResponse, 0, len(catalogue))
	for _, role := range catalogue {
		items = append(items, roleResponse{
			Name:        role.Name.String(),
			Description: role.Description,
			Permissions: permissionNames(role.Permissions),
		})
	}
	httpx.WriteJSON(writer, request, http.StatusOK, roleListResponse{Items: items})
}

func (h *Handler) assignRole(writer http.ResponseWriter, request *http.Request) {
	actor, targetID, err := h.actorAndTarget(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	role, err := decodeRole(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	roles, err := h.roles.AssignRole(request.Context(), actor.UserID, targetID, role)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, userRolesResponse{
		UserID: targetID, Roles: roleNames(roles),
	})
}

func (h *Handler) revokeRole(writer http.ResponseWriter, request *http.Request) {
	actor, targetID, err := h.actorAndTarget(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	role, ok := contract.ParseRole(chi.URLParam(request, "role"))
	if !ok {
		httpx.WriteProblem(writer, request, domain.ErrUnknownRole)
		return
	}

	roles, err := h.roles.RevokeRole(request.Context(), actor.UserID, targetID, role)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, userRolesResponse{
		UserID: targetID, Roles: roleNames(roles),
	})
}

// actorAndTarget resolves the caller and the user being changed, and applies
// the permission guard. Both write operations need exactly this, and doing it
// in one place is what stops one of them being written without the Require.
func (h *Handler) actorAndTarget(request *http.Request) (httpx.Actor, uuid.UUID, error) {
	actor, err := requireActor(request)
	if err != nil {
		return httpx.Actor{}, uuid.Nil, err
	}
	if err := h.roles.Require(request.Context(), contract.PermRBACAssign); err != nil {
		return httpx.Actor{}, uuid.Nil, err
	}

	targetID, err := uuid.Parse(chi.URLParam(request, "id"))
	if err != nil {
		// A malformed id is a 404 rather than a 400: the resource it names
		// does not exist, and saying "that is not a uuid" tells an unauthorised
		// caller something about the shape of what they are probing.
		return httpx.Actor{}, uuid.Nil, apperr.New(
			apperr.NotFound, "NOT_FOUND", "The requested resource was not found.")
	}
	return actor, targetID, nil
}

func requireActor(request *http.Request) (httpx.Actor, error) {
	actor, ok := httpx.ActorFrom(request.Context())
	if !ok {
		return httpx.Actor{}, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token.")
	}
	return actor, nil
}

// decodeRole reads the one member the assign request carries. Unknown members
// are a 422 naming the field, for the same reason they are on PATCH /me: a
// misspelled field that returns 200 is indistinguishable from success.
func decodeRole(request *http.Request) (contract.Role, error) {
	limited := http.MaxBytesReader(nil, request.Body, maxBodyBytes)
	decoder := json.NewDecoder(limited)

	var decoded map[string]json.RawMessage
	if err := decoder.Decode(&decoded); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return "", apperr.New(apperr.TooLarge, "PAYLOAD_TOO_LARGE", "Request body exceeds the maximum size.")
		}
		return "", apperr.Wrap(err, apperr.BadRequest, "MALFORMED_REQUEST", "Request body must be a JSON object.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", apperr.New(apperr.BadRequest, "MALFORMED_REQUEST", "Request body must contain one JSON value.")
	}

	for field := range decoded {
		if field != fieldRole {
			return "", apperr.New(apperr.Validation, "VALIDATION_FAILED",
				"One or more request fields are invalid.").
				WithFields(apperr.FieldViolation{
					Field: field, Code: "UNKNOWN_FIELD", Message: field + " is not a field of this resource.",
				})
		}
	}

	raw, present := decoded[fieldRole]
	if !present {
		return "", apperr.New(apperr.Validation, "VALIDATION_FAILED",
			"One or more request fields are invalid.").
			WithFields(apperr.FieldViolation{Field: fieldRole, Code: "REQUIRED", Message: "role is required."})
	}

	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", apperr.New(apperr.Validation, "VALIDATION_FAILED",
			"One or more request fields are invalid.").
			WithFields(apperr.FieldViolation{Field: fieldRole, Code: "TYPE", Message: "role has the wrong type."})
	}

	role, ok := contract.ParseRole(name)
	if !ok {
		return "", domain.ErrUnknownRole
	}
	return role, nil
}

func roleNames(roles []contract.Role) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.String())
	}
	return names
}

func permissionNames(permissions []contract.Permission) []string {
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.String())
	}
	return names
}

// The response shapes, mirroring api/openapi/components/rbac.yaml. The
// contract test in this package validates real responses against the spec.
type myPermissionsResponse struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type roleResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type roleListResponse struct {
	Items []roleResponse `json:"items"`
}

type userRolesResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Roles  []string  `json:"roles"`
}
