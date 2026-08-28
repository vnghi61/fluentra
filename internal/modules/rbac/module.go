package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/repository"
	"github.com/fluentra/fluentra/internal/modules/rbac/service"
	rbachttp "github.com/fluentra/fluentra/internal/modules/rbac/transport/http"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// PermissionCache is the cached permission resolution the module needs.
// platform/cache's RedisCache[[]string] satisfies it.
type PermissionCache interface {
	Get(ctx context.Context, key string) ([]string, error)
	Set(ctx context.Context, key string, value []string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Deps are what the composition root supplies.
//
// Cache may be nil. The module then resolves every check from the database,
// which is slower and completely correct — the cache is an optimisation, not
// where the answer lives, and a deployment without Redis should still refuse
// the right requests.
type Deps struct {
	Pool  *pgxpool.Pool
	Cache PermissionCache
	Clock clock.Clock
	Env   string
}

// Module is the rbac module, assembled.
type Module struct {
	service *service.Service
	handler *rbachttp.Handler
}

// New wires the module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	environment := deps.Env
	if environment == "" {
		// An unnamed environment would build cache keys a second deployment
		// could collide with. Naming it "unknown" keeps the collision to
		// deployments that both failed to configure themselves.
		environment = "unknown"
	}

	repo := repositoryAdapter{Repository: repository.New(deps.Pool)}
	roles := service.New(service.Deps{
		Pool:   deps.Pool,
		Repo:   repo,
		Cache:  deps.Cache,
		Events: outboxWriter{Writer: outbox.NewWriter()},
		Clock:  timekeeper,
		Env:    environment,
	})

	return &Module{service: roles, handler: rbachttp.NewHandler(roles)}
}

// Routes mounts the module's HTTP operations.
func (m *Module) Routes(router chi.Router) { m.handler.Routes(router) }

// Authorizer is the guard every other module's service calls.
func (m *Module) Authorizer() contract.Authorizer { return m.service }

// RoleReader is the role and permission read surface. `auth` uses it when
// minting a token.
func (m *Module) RoleReader() contract.RoleReader { return m.service }

// AdminOnly is the route-group guard, exported so the composition root can
// wrap other modules' `/admin` routes in the same check.
func (m *Module) AdminOnly() func(http.Handler) http.Handler {
	return rbachttp.AdminOnly(m.service)
}

// AssignRole grants a role. It is exported so the composition root and the
// admin module can drive it without going through HTTP.
func (m *Module) AssignRole(
	ctx context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	return m.service.AssignRole(ctx, actorID, targetID, role)
}

// GrantBaselineRole grants `user` to a newly created account, with no actor.
//
// It is the contract the composition root hands to `user`, so that creating an
// account and holding the role every account holds stop being two things that
// can drift apart. See the service method for why they had.
func (m *Module) GrantBaselineRole(ctx context.Context, userID uuid.UUID) error {
	return m.service.GrantBaselineRole(ctx, userID)
}

// RevokeRole removes a role, subject to the self-demotion and last-admin
// protections.
func (m *Module) RevokeRole(
	ctx context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	return m.service.RevokeRole(ctx, actorID, targetID, role)
}

// ForgetUser drops every role a deleted account held. The `user.deleted`
// consumer calls it.
func (m *Module) ForgetUser(ctx context.Context, userID uuid.UUID) error {
	return m.service.ForgetUser(ctx, userID)
}

// Subscribe registers the event consumers this module runs in the worker.
func (m *Module) Subscribe(bus eventbus.EventBus) error {
	if err := bus.Subscribe("user.deleted", m.handleUserDeleted); err != nil {
		return fmt.Errorf("subscribe rbac consumer to user.deleted: %w", err)
	}
	return nil
}

type userDeletedPayload struct {
	UserID uuid.UUID `json:"user_id"`
}

func (m *Module) handleUserDeleted(ctx context.Context, msg eventbus.Message) error {
	var payload userDeletedPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode user.deleted payload: %w", err)
	}
	if payload.UserID == uuid.Nil {
		return nil
	}
	return m.ForgetUser(ctx, payload.UserID)
}

// ExportUserData implements contract.Exportable for GDPR data export.
func (m *Module) ExportUserData(ctx context.Context, userIDStr string) (map[string]interface{}, error) {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse user id for rbac export: %w", err)
	}

	roles, err := m.service.RolesOf(ctx, userID)
	if err != nil {
		roles = nil
	}
	roleStrings := make([]string, 0, len(roles))
	for _, r := range roles {
		roleStrings = append(roleStrings, string(r))
	}

	perms, err := m.service.PermissionsOf(ctx, userID)
	if err != nil {
		perms = nil
	}
	permStrings := make([]string, 0, len(perms))
	for _, p := range perms {
		permStrings = append(permStrings, string(p))
	}

	return map[string]interface{}{
		"roles":       roleStrings,
		"permissions": permStrings,
	}, nil
}

// repositoryAdapter narrows *repository.Repository to the service's interface.
// Go has no covariant return types, so WithTx returning *Repository cannot
// satisfy a method returning service.Repository.
type repositoryAdapter struct {
	*repository.Repository
}

func (a repositoryAdapter) WithTx(tx pgx.Tx) service.Repository {
	return repositoryAdapter{Repository: a.Repository.WithTx(tx)}
}

// outboxWriter adapts shared/outbox to the service's EventWriter.
type outboxWriter struct {
	*outbox.Writer
}

func (w outboxWriter) Write(
	ctx context.Context, tx service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	return w.Writer.Write(ctx, outboxTx{tx}, aggregate, event, payload)
}

type outboxTx struct{ inner service.OutboxTx }

func (t outboxTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.inner.Exec(ctx, sql, arguments...)
}
