package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// errNoCache means the cache is not configured. It never reaches a caller: the
// resolver treats it as a miss.
var errNoCache = errors.New("rbac: no cache configured")

// AssignRole grants role to targetID on behalf of actorID.
//
// The whole operation is one transaction, and the self-elevation check is
// evaluated inside it. Reading the actor's roles first and writing afterwards,
// outside a transaction, would be correct only until somebody changed those
// roles in between.
func (s *Service) AssignRole(
	ctx context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	if _, ok := contract.ParseRole(string(role)); !ok {
		return nil, domain.ErrUnknownRole
	}

	var roles []contract.Role
	var changed bool
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		exists, err := repo.RoleExists(ctx, role)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrUnknownRole
		}

		change, err := s.describeChange(ctx, repo, actorID, targetID, role)
		if err != nil {
			return err
		}
		if err := change.ValidateGrant(); err != nil {
			return err
		}

		granter := actorID
		if changed, err = repo.AssignRole(ctx, targetID, role, &granter); err != nil {
			return err
		}
		if changed {
			_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventRoleAssigned,
				contract.RoleAssigned{
					UserID: targetID, Role: role, ActorID: actorID, OccurredAt: s.clock.Now(),
				})
			if err != nil {
				return err
			}
		}

		roles, err = repo.RolesOf(ctx, targetID)
		return err
	})
	if err != nil {
		return nil, err
	}

	// After the commit, never before: busting a key for a change that then
	// rolls back would evict a correct entry and reload the same value.
	if changed {
		s.invalidate(ctx, targetID)
	}
	return roles, nil
}

// GrantBaselineRole grants `user` to a newly created account.
//
// It exists because nothing did. AssignRole is an administrative operation: it
// needs an actor holding rbac.assign, and its only caller is the admin handler.
// So no account created by registration or by `make seed` ever received a role,
// while the access token minted for it claimed `user` anyway — HighestRole of
// an empty set is `user`. The token said one thing and the guard read another,
// and the guard reads core.user_roles.
//
// That was invisible until Phase 2. `user` held no permissions at all, so an
// account with no roles and an account with `user` could do exactly the same
// things. P7.1 gave `user` its first permission, content.read.published, and
// from that commit every learner was refused the published catalogue. The
// module's integration tests did not catch it because their fixtures grant the
// role themselves, which is the shape of a test agreeing with itself.
//
// No actor, because there is none: the system made this grant, the way
// db/seeds/rbac.sql makes its own with granted_by NULL. That also keeps
// BR-RBAC-04 intact — this is not somebody granting themselves anything, and
// `user` is not a role anybody could escalate to.
//
// Idempotent: the underlying insert is ON CONFLICT DO NOTHING, so re-running it
// for an account that already holds the role writes nothing, publishes nothing
// and busts nothing.
func (s *Service) GrantBaselineRole(ctx context.Context, userID uuid.UUID) error {
	var changed bool
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		var err error
		if changed, err = repo.AssignRole(ctx, userID, contract.RoleUser, nil); err != nil {
			return err
		}
		if !changed {
			return nil
		}
		_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventRoleAssigned,
			contract.RoleAssigned{
				UserID: userID, Role: contract.RoleUser,
				ActorID: uuid.Nil, OccurredAt: s.clock.Now(),
			})
		return err
	})
	if err != nil {
		return err
	}

	// After the commit, for the reason AssignRole records.
	if changed {
		s.invalidate(ctx, userID)
	}
	return nil
}

// RevokeRole removes role from targetID on behalf of actorID.
//
// The admin count is read inside the transaction, which is what makes the
// last-administrator guard hold under two revocations racing: dbx.InTx runs at
// SERIALIZABLE, so the loser is aborted and retried, and the retry sees the
// count the winner committed. An integration test drives both directions at
// once and asserts an administrator survives.
func (s *Service) RevokeRole(
	ctx context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	if _, ok := contract.ParseRole(string(role)); !ok {
		return nil, domain.ErrUnknownRole
	}

	var roles []contract.Role
	var changed bool
	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.repo.WithTx(tx)

		adminCount, err := repo.LockAndCountRole(ctx, contract.RoleAdmin)
		if err != nil {
			return err
		}

		change, err := s.describeChange(ctx, repo, actorID, targetID, role)
		if err != nil {
			return err
		}
		if err := change.ValidateRevoke(adminCount); err != nil {
			return err
		}

		if changed, err = repo.RevokeRole(ctx, targetID, role); err != nil {
			return err
		}
		if changed {
			_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventRoleRevoked,
				contract.RoleRevoked{
					UserID: targetID, Role: role, ActorID: actorID, OccurredAt: s.clock.Now(),
				})
			if err != nil {
				return err
			}
		}

		roles, err = repo.RolesOf(ctx, targetID)
		return err
	})
	if err != nil {
		return nil, err
	}

	if changed {
		s.invalidate(ctx, targetID)
	}
	return roles, nil
}

// ForgetUser removes every role assignment for a deleted account, and clears
// the cached set so a recycled id cannot inherit them.
func (s *Service) ForgetUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.repo.DeleteAssignmentsForUser(ctx, userID); err != nil {
		return err
	}
	s.invalidate(ctx, userID)
	return nil
}

// describeChange assembles what the rules need to judge a change. Reading both
// sides here, once, is what lets domain.RoleChange be a pure function.
func (s *Service) describeChange(
	ctx context.Context, repo Repository, actorID, targetID uuid.UUID, role contract.Role,
) (domain.RoleChange, error) {
	actorRoles, err := repo.RolesOf(ctx, actorID)
	if err != nil {
		return domain.RoleChange{}, err
	}

	targetRoles := actorRoles
	if actorID != targetID {
		if targetRoles, err = repo.RolesOf(ctx, targetID); err != nil {
			return domain.RoleChange{}, err
		}
	}

	return domain.RoleChange{
		ActorID: actorID, TargetID: targetID, Role: role,
		ActorRoles: actorRoles, TargetRoles: targetRoles,
	}, nil
}
