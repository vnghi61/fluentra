// Package service holds the rbac use cases: resolving an actor's permissions,
// the guard that decides an operation, and the two administrative changes that
// need protecting from themselves.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// PermissionTTL is how long a resolved permission set is cached.
//
// Five minutes is the revocation window the module's AGENT.md commits to: a
// role taken away is gone within five minutes even if nothing busts the key,
// and immediately when something does. Longer would make a compromised admin
// account harder to shut off; shorter would put this query on the hot path of
// every request.
const PermissionTTL = 5 * time.Minute

// cacheVersion is the `v1` in the cache key. Bump it when the cached shape
// changes, so a deploy cannot read an old value as a new one.
const cacheVersion = 1

// Repository is the persistence surface this service needs, declared by the
// consumer so the rules can be tested without a database.
type Repository interface {
	RolesOf(ctx context.Context, userID uuid.UUID) ([]contract.Role, error)
	PermissionsOf(ctx context.Context, userID uuid.UUID) ([]contract.Permission, error)
	ListRoles(ctx context.Context) ([]domain.RoleWithPermissions, error)
	RoleExists(ctx context.Context, role contract.Role) (bool, error)
	AssignRole(ctx context.Context, userID uuid.UUID, role contract.Role, grantedBy *uuid.UUID) (bool, error)
	RevokeRole(ctx context.Context, userID uuid.UUID, role contract.Role) (bool, error)
	LockAndCountRole(ctx context.Context, role contract.Role) (int, error)
	DeleteAssignmentsForUser(ctx context.Context, userID uuid.UUID) (int, error)
	FirstHolderOf(ctx context.Context, role contract.Role) (uuid.UUID, error)

	WithTx(tx pgx.Tx) Repository
}

// PermissionCache is the cached resolution. It is a narrow interface rather
// than cache.Cache[[]string] so the service can be exercised without Redis,
// and so a cache that is down degrades to a database read rather than a 500.
type PermissionCache interface {
	Get(ctx context.Context, key string) ([]string, error)
	Set(ctx context.Context, key string, value []string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// OutboxTx is the transaction surface the outbox writer needs.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter records an event in the same transaction as the change.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// Service implements the rbac use cases.
type Service struct {
	pool   dbx.Beginner
	repo   Repository
	cache  PermissionCache
	events EventWriter
	clock  clock.Clock
	env    string
}

// Deps are the service's collaborators.
type Deps struct {
	Pool   dbx.Beginner
	Repo   Repository
	Cache  PermissionCache
	Events EventWriter
	Clock  clock.Clock
	// Env namespaces the cache keys, so a staging deploy pointed at a shared
	// Redis cannot answer a production permission check.
	Env string
}

// New creates the rbac service.
func New(deps Deps) *Service {
	return &Service{
		pool: deps.Pool, repo: deps.Repo, cache: deps.Cache,
		events: deps.Events, clock: deps.Clock, env: deps.Env,
	}
}

// The service is what other modules get when they ask for the rbac contract.
var (
	_ contract.Authorizer = (*Service)(nil)
	_ contract.RoleReader = (*Service)(nil)
)

// Require is the guard. It returns nil only when there is a caller in ctx and
// that caller holds permission.
//
// Every other outcome is the same 403: no actor, a malformed permission, an
// unknown one, one not granted, and — deliberately — a failure to resolve the
// set at all. That last one is the important case. A resolver that returns
// "allow" when the database is unreachable turns an outage into an open door,
// so this fails closed and says so in the log.
func (s *Service) Require(ctx context.Context, permission contract.Permission) error {
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		return domain.ErrPermissionDenied
	}

	granted, err := s.permissionSet(ctx, actor.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "permission resolution failed, denying",
			"module", "rbac", "op", "Require", "permission", permission.String(), "error", err)
		return domain.ErrPermissionDenied
	}
	if granted.Has(permission) {
		return nil
	}

	// BR-RBAC-07: a refusal is a security signal, not routine noise. The
	// permission name is safe to log; nothing about the request body is.
	slog.WarnContext(ctx, "permission denied",
		"module", "rbac", "op", "Require", "permission", permission.String())
	return domain.ErrPermissionDenied
}

// Can answers the same question without an error, for deciding what to render.
func (s *Service) Can(ctx context.Context, permission contract.Permission) bool {
	return s.Require(ctx, permission) == nil
}

// RolesOf returns the roles assigned to userID.
func (s *Service) RolesOf(ctx context.Context, userID uuid.UUID) ([]contract.Role, error) {
	return s.repo.RolesOf(ctx, userID)
}

// PermissionsOf returns the flat permission set userID's roles grant.
func (s *Service) PermissionsOf(ctx context.Context, userID uuid.UUID) ([]contract.Permission, error) {
	granted, err := s.permissionSet(ctx, userID)
	if err != nil {
		return nil, err
	}
	return granted.Names(), nil
}

// HasRole reports whether userID holds role. The `/admin/*` middleware uses it.
func (s *Service) HasRole(ctx context.Context, userID uuid.UUID, role contract.Role) (bool, error) {
	roles, err := s.repo.RolesOf(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, held := range roles {
		if held == role {
			return true, nil
		}
	}
	return false, nil
}

// ListRoles returns the role catalogue.
func (s *Service) ListRoles(ctx context.Context) ([]domain.RoleWithPermissions, error) {
	return s.repo.ListRoles(ctx)
}

// permissionSet resolves a user's permissions, through the cache.
//
// A cache miss or a cache error both fall through to the database — the cache
// makes this fast, it is not where the answer lives. Only a database failure
// is an error, and Require turns that into a denial.
func (s *Service) permissionSet(ctx context.Context, userID uuid.UUID) (domain.PermissionSet, error) {
	key := s.permissionsKey(userID)

	if cached, err := s.cacheGet(ctx, key); err == nil {
		return domain.NewPermissionSet(toPermissions(cached)), nil
	}

	permissions, err := s.repo.PermissionsOf(ctx, userID)
	if err != nil {
		return domain.PermissionSet{}, err
	}

	s.cacheSet(ctx, key, toStrings(permissions))
	return domain.NewPermissionSet(permissions), nil
}

func (s *Service) cacheGet(ctx context.Context, key string) ([]string, error) {
	if s.cache == nil {
		return nil, errNoCache
	}
	return s.cache.Get(ctx, key)
}

// cacheSet is best effort. A cache that cannot be written is a slow system,
// not a wrong one, so the failure is logged and the answer still returned.
func (s *Service) cacheSet(ctx context.Context, key string, value []string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Set(ctx, key, value, PermissionTTL); err != nil {
		slog.WarnContext(ctx, "could not cache permission set",
			"module", "rbac", "op", "cacheSet", "error", err)
	}
}

// invalidate busts a user's cached permissions. It is called after a role
// change commits, which is what turns the five-minute TTL into "immediately"
// for the case that matters.
//
// It takes a context deliberately detached from the request: the bust happens
// after the response is on its way, and a cancelled context would leave a
// revoked admin holding their permissions for the rest of the TTL.
func (s *Service) invalidate(ctx context.Context, userID uuid.UUID) {
	if s.cache == nil {
		return
	}
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.cache.Delete(detached, s.permissionsKey(userID)); err != nil {
		slog.WarnContext(ctx, "could not invalidate cached permissions; the TTL still bounds it",
			"module", "rbac", "op", "invalidate", "ttl_seconds", int(PermissionTTL.Seconds()), "error", err)
	}
}

func (s *Service) permissionsKey(userID uuid.UUID) string {
	return "fluentra:" + s.env + ":rbac:perms:" + userID.String() + ":v" + itoa(cacheVersion)
}

func toStrings(permissions []contract.Permission) []string {
	values := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		values = append(values, string(permission))
	}
	return values
}

func toPermissions(values []string) []contract.Permission {
	permissions := make([]contract.Permission, 0, len(values))
	for _, value := range values {
		permissions = append(permissions, contract.Permission(value))
	}
	return permissions
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// FirstHolderOf implements contract.RoleMembers.
func (s *Service) FirstHolderOf(ctx context.Context, role contract.Role) (uuid.UUID, error) {
	return s.repo.FirstHolderOf(ctx, role)
}

var _ contract.RoleMembers = (*Service)(nil)
