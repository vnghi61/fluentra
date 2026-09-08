package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// RequestDeletion initiates account deletion for the caller with a 30-day grace period.
// It revokes all active sessions immediately by publishing user.deletion_requested.
func (s *Service) RequestDeletion(ctx context.Context, actorID uuid.UUID) (domain.DeletionRequest, error) {
	return s.requestDeletionFor(ctx, actorID)
}

// SoftDeleteUser starts the same deletion an owner would start for themselves,
// on someone else's behalf. `reason` is not stored here: the admin module owns
// the justification trail (core.admin_actions and the audit log), and this
// module owns the account lifecycle. Duplicating it would put the same sentence
// in two places that can disagree.
//
// The grace period is deliberately the one the owner gets. An administrator
// removing an account is not more certain than the owner was, and the whole
// point of a soft delete is that the thirty days are recoverable — by the owner,
// through the cancel path they already have.
func (s *Service) SoftDeleteUser(ctx context.Context, id, actorID uuid.UUID, _ string) error {
	if id == actorID {
		// The self-service path exists and takes no reason. An administrator
		// deleting their own account through the admin endpoint would skip the
		// confirmations that path has, and admin.SoftDeleteUser refuses this
		// before it reaches here; this is the second lock on the same door.
		return domain.ErrDeletionNotCancellable
	}
	_, err := s.requestDeletionFor(ctx, id)
	return err
}

// requestDeletionFor is the body both paths share. `subjectID` is whose account
// is being deleted — the caller's own in RequestDeletion, someone else's in
// SoftDeleteUser. It was written inline in RequestDeletion using `actorID` for
// both roles, which is exactly the conflation the admin path had to undo.
func (s *Service) requestDeletionFor(ctx context.Context, subjectID uuid.UUID) (domain.DeletionRequest, error) {
	var request domain.DeletionRequest

	err := dbx.InTx(ctx, s.pool, func(_ context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		user, err := txRepo.GetUser(ctx, subjectID)
		if err != nil {
			return err
		}
		if user.Status == domain.StatusPendingDeletion || user.Status == domain.StatusDeleted {
			return domain.ErrDeletionAlreadyPending
		}

		if pending, found, err := txRepo.GetPendingDeletionForUser(ctx, subjectID); err != nil {
			return err
		} else if found && (pending.Status == domain.DeletionStatusPending ||
			pending.Status == domain.DeletionStatusProcessing) {
			return domain.ErrDeletionAlreadyPending
		}

		deletionID, err := s.ids(ctx)
		if err != nil {
			return fmt.Errorf("generate deletion id: %w", err)
		}

		now := s.clock.Now().UTC()
		executeAt := now.Add(domain.DeletionGracePeriod)

		req, err := txRepo.CreateDeletionRequest(ctx, deletionID, subjectID, executeAt)
		if err != nil {
			return fmt.Errorf("create deletion request: %w", err)
		}
		request = req

		if err := txRepo.UpdateUserStatus(ctx, subjectID, domain.StatusPendingDeletion); err != nil {
			return fmt.Errorf("update user status to pending_deletion: %w", err)
		}

		if s.events != nil {
			_, err = s.events.Write(
				ctx,
				tx,
				contract.Aggregate,
				contract.EventDeletionRequested,
				contract.DeletionRequested{
					UserID:       subjectID,
					ExecuteAfter: executeAt,
					OccurredAt:   now,
				},
			)
			if err != nil {
				return fmt.Errorf("write deletion requested event: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return domain.DeletionRequest{}, err
	}

	return request, nil
}

// CancelDeletion cancels a pending deletion request during the 30-day grace period.
func (s *Service) CancelDeletion(ctx context.Context, actorID uuid.UUID) (domain.DeletionRequest, error) {
	var cancelledReq domain.DeletionRequest

	err := dbx.InTx(ctx, s.pool, func(_ context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		user, err := txRepo.GetUser(ctx, actorID)
		if err != nil {
			return err
		}
		if user.Status != domain.StatusPendingDeletion {
			return domain.ErrDeletionNotCancellable
		}

		pending, found, err := txRepo.GetPendingDeletionForUser(ctx, actorID)
		if err != nil {
			return err
		}
		if !found || pending.Status != domain.DeletionStatusPending {
			return domain.ErrDeletionNotCancellable
		}

		now := s.clock.Now().UTC()
		if err := txRepo.CancelDeletion(ctx, pending.ID, now); err != nil {
			return err
		}

		if err := txRepo.UpdateUserStatus(ctx, actorID, domain.StatusActive); err != nil {
			return fmt.Errorf("restore user status to active: %w", err)
		}

		cancelledReq = pending
		cancelledReq.Status = domain.DeletionStatusCancelled
		cancelledReq.CancelledAt = &now

		return nil
	})
	if err != nil {
		return domain.DeletionRequest{}, err
	}

	return cancelledReq, nil
}

// GetDeletion reads a deletion request by ID, enforcing that learners can only read their own request.
func (s *Service) GetDeletion(ctx context.Context, actorID, deletionID uuid.UUID) (domain.DeletionRequest, error) {
	req, err := s.repo.GetDeletionByID(ctx, deletionID)
	if err != nil {
		return domain.DeletionRequest{}, err
	}
	if req.UserID != actorID {
		return domain.DeletionRequest{}, domain.ErrDeletionNotFound
	}
	return req, nil
}
