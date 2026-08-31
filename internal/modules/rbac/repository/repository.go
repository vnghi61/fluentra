// Package repository maps rbac rows to domain values. It holds no rules:
// every decision about whether a change is allowed is made in the service,
// against data this package returns.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlcrbac "github.com/fluentra/fluentra/internal/generated/rbac/sqlc"
	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository reads and writes the rbac tables.
type Repository struct {
	queries *sqlcrbac.Queries
}

// New creates a repository over db.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlcrbac.New(db)}
}

// WithTx returns a repository that runs inside tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: sqlcrbac.New(tx)}
}

// RolesOf returns the roles assigned to userID.
//
// A name the database holds that this build does not know is dropped rather
// than surfaced. Roles come from a seeded catalogue, so this should never
// happen; if it does, treating the unknown role as granting nothing keeps the
// failure on the deny side.
func (r *Repository) RolesOf(ctx context.Context, userID uuid.UUID) ([]contract.Role, error) {
	names, err := r.queries.ListRoleNamesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list roles for user: %w", err)
	}
	roles := make([]contract.Role, 0, len(names))
	for _, name := range names {
		if role, ok := contract.ParseRole(name); ok {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

// PermissionsOf returns the flat set of permissions userID's roles grant.
//
// Malformed names are dropped for the same reason, and with the same
// direction of failure: a permission that cannot be parsed is one nobody
// holds.
func (r *Repository) PermissionsOf(ctx context.Context, userID uuid.UUID) ([]contract.Permission, error) {
	names, err := r.queries.ListPermissionNamesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list permissions for user: %w", err)
	}
	permissions := make([]contract.Permission, 0, len(names))
	for _, name := range names {
		permission := contract.Permission(name)
		if permission.Valid() {
			permissions = append(permissions, permission)
		}
	}
	return permissions, nil
}

// ListRoles returns the catalogue, each role with the permissions it grants.
func (r *Repository) ListRoles(ctx context.Context) ([]domain.RoleWithPermissions, error) {
	rows, err := r.queries.ListRolesWithPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	catalogue := make([]domain.RoleWithPermissions, 0, len(rows))
	for _, row := range rows {
		role, ok := contract.ParseRole(row.Name)
		if !ok {
			continue
		}
		permissions := make([]contract.Permission, 0, len(row.Permissions))
		for _, name := range row.Permissions {
			permissions = append(permissions, contract.Permission(name))
		}
		catalogue = append(catalogue, domain.RoleWithPermissions{
			Name: role, Description: row.Description, Permissions: permissions,
		})
	}
	return catalogue, nil
}

// RoleExists reports whether the named role is in the catalogue.
func (r *Repository) RoleExists(ctx context.Context, role contract.Role) (bool, error) {
	_, err := r.queries.GetRoleByName(ctx, string(role))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("get role: %w", err)
	}
	return true, nil
}

// AssignRole grants role to userID and reports whether anything changed.
// Granting a role the user already holds writes nothing and returns false, so
// the caller can skip the event and the cache bust.
func (r *Repository) AssignRole(
	ctx context.Context, userID uuid.UUID, role contract.Role, grantedBy *uuid.UUID,
) (bool, error) {
	affected, err := r.queries.AssignRole(ctx, sqlcrbac.AssignRoleParams{
		UserID: userID, Name: string(role), GrantedBy: grantedBy,
	})
	if err != nil {
		return false, fmt.Errorf("assign role: %w", err)
	}
	return affected > 0, nil
}

// RevokeRole removes role from userID and reports whether anything changed.
func (r *Repository) RevokeRole(ctx context.Context, userID uuid.UUID, role contract.Role) (bool, error) {
	affected, err := r.queries.RevokeRole(ctx, sqlcrbac.RevokeRoleParams{
		UserID: userID, Name: string(role),
	})
	if err != nil {
		return false, fmt.Errorf("revoke role: %w", err)
	}
	return affected > 0, nil
}

// LockAndCountRole counts the accounts holding role, having first taken a row
// lock on each.
//
// The correctness of the last-administrator guard does not rest on that lock.
// dbx.InTx runs at SERIALIZABLE, so two concurrent revocations produce a 40001
// on one of them and dbx retries it, and the retry sees the state the winner
// committed. Removing the lock and running the concurrency test fifteen times
// still passes, which is how that was established rather than assumed.
//
// The lock is kept because it turns a serialization abort into a wait. Under
// contention an abort costs the whole transaction and a retry; a wait costs
// the time the other transaction takes. That is a performance choice, and
// saying so here is the point — a comment claiming the lock is what makes this
// safe would send the next reader looking in the wrong place when it breaks.
func (r *Repository) LockAndCountRole(ctx context.Context, role contract.Role) (int, error) {
	if _, err := r.queries.LockRoleAssignments(ctx, string(role)); err != nil {
		return 0, fmt.Errorf("lock role assignments: %w", err)
	}
	count, err := r.queries.CountUsersWithRole(ctx, string(role))
	if err != nil {
		return 0, fmt.Errorf("count users with role: %w", err)
	}
	return int(count), nil
}

// DeleteAssignmentsForUser removes every role a user holds. It is the reaction
// to `user.deleted`.
func (r *Repository) DeleteAssignmentsForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	affected, err := r.queries.DeleteRoleAssignmentsForUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("delete role assignments: %w", err)
	}
	return int(affected), nil
}

// FirstHolderOf returns the longest-standing holder of a role, or uuid.Nil when
// nobody holds it.
//
// Nil rather than an error for "nobody": a fresh database has no administrator
// yet, and the one caller — the practice generator — should stand down quietly
// rather than log a failure every twelve hours until somebody signs up.
func (r *Repository) FirstHolderOf(ctx context.Context, role contract.Role) (uuid.UUID, error) {
	id, err := r.queries.FirstHolderOfRole(ctx, string(role))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("first holder of role %s: %w", role, err)
	}
	return id, nil
}
