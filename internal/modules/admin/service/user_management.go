package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
)

// Audit entry keys shared by the write operations. They are constants rather
// than literals so the trail's field names cannot drift apart between methods.
const (
	targetTypeUser = "user"
	metaActorID    = "actor_id"
	metaReason     = "reason"
)

// SearchUsers delegates user search to usercontract.AdminReader.
func (s *Service) SearchUsers(
	ctx context.Context,
	filter usercontract.UserFilter,
	cursor string,
	limit int,
) ([]usercontract.UserSummary, string, error) {
	return s.userReader.SearchUsers(ctx, filter, cursor, limit)
}

// GetUserByID gets user details and audits the READ action (BR-ADMIN-02).
func (s *Service) GetUserByID(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
) (*usercontract.UserDetail, error) {
	userDetail, err := s.userReader.GetUserByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.Record(ctx, auditcontract.Entry{
			Action:     "admin.user_viewed",
			TargetType: targetTypeUser,
			TargetID:   targetID.String(),
			Meta: map[string]any{
				metaActorID: actorID.String(),
			},
		})
	}

	return userDetail, nil
}

// SuspendUser suspends a target user, revokes active sessions, logs admin action, and audits.
func (s *Service) SuspendUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
) error {
	if actorID == targetID {
		return admindomain.ErrSelfAdminActionForbidden
	}

	trimmedReason := strings.TrimSpace(reason)
	if len(trimmedReason) < 10 {
		return admindomain.ErrReasonRequired
	}

	// Rule L4: Separate transactions across modules.
	// 1. Log admin action in admin DB
	if _, err := s.repo.LogAdminAction(ctx, actorID, targetID, "suspend", trimmedReason); err != nil {
		return err
	}

	// 2. Execute user status update in user module
	if err := s.userManager.SuspendUser(ctx, targetID, actorID, trimmedReason); err != nil {
		return err
	}

	// 3. Revoke sessions via auth contract
	if s.sessionRevoker != nil {
		if _, err := s.sessionRevoker.RevokeAll(ctx, targetID); err != nil {
			return err
		}
	}

	// 4. Audit entry
	if s.audit != nil {
		s.audit.Record(ctx, auditcontract.Entry{
			Action:     "admin.user_suspended",
			TargetType: targetTypeUser,
			TargetID:   targetID.String(),
			Meta: map[string]any{
				metaActorID: actorID.String(),
				metaReason:  trimmedReason,
			},
		})
	}

	return nil
}

// ReinstateUser restores a suspended user to active status, logs admin action, and audits.
func (s *Service) ReinstateUser(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
) error {
	if actorID == targetID {
		return admindomain.ErrSelfAdminActionForbidden
	}

	trimmedReason := strings.TrimSpace(reason)
	if len(trimmedReason) < 10 {
		return admindomain.ErrReasonRequired
	}

	if _, err := s.repo.LogAdminAction(ctx, actorID, targetID, "reinstate", trimmedReason); err != nil {
		return err
	}

	if err := s.userManager.ReinstateUser(ctx, targetID, actorID, trimmedReason); err != nil {
		return err
	}

	if s.audit != nil {
		s.audit.Record(ctx, auditcontract.Entry{
			Action:     "admin.user_reinstated",
			TargetType: targetTypeUser,
			TargetID:   targetID.String(),
			Meta: map[string]any{
				metaActorID: actorID.String(),
				metaReason:  trimmedReason,
			},
		})
	}

	return nil
}

// RevokeUserSessions revokes all sessions for a user, logs admin action, and audits.
func (s *Service) RevokeUserSessions(
	ctx context.Context,
	actorID uuid.UUID,
	targetID uuid.UUID,
	reason string,
) error {
	trimmedReason := strings.TrimSpace(reason)
	if len(trimmedReason) < 10 {
		return admindomain.ErrReasonRequired
	}

	if _, err := s.repo.LogAdminAction(ctx, actorID, targetID, "revoke_sessions", trimmedReason); err != nil {
		return err
	}

	var count int
	if s.sessionRevoker != nil {
		var err error
		count, err = s.sessionRevoker.RevokeAll(ctx, targetID)
		if err != nil {
			return err
		}
	}

	if s.audit != nil {
		s.audit.Record(ctx, auditcontract.Entry{
			Action:     "admin.sessions_revoked",
			TargetType: targetTypeUser,
			TargetID:   targetID.String(),
			Meta: map[string]any{
				metaActorID:     actorID.String(),
				metaReason:      trimmedReason,
				"revoked_count": count,
			},
		})
	}

	return nil
}
