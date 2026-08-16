package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// RequestExport starts a data export for the signed-in caller (BR-USER-07).
// If an export is already pending or processing for this account, it returns ErrExportAlreadyPending (409 Conflict).
func (s *Service) RequestExport(ctx context.Context, actorID uuid.UUID) (domain.ExportRequest, error) {
	_, err := s.requireUsableAccount(ctx, actorID)
	if err != nil {
		return domain.ExportRequest{}, err
	}

	existing, found, err := s.repo.GetPendingExportForUser(ctx, actorID)
	if err != nil {
		return domain.ExportRequest{}, fmt.Errorf("check pending export: %w", err)
	}
	if found {
		return existing, domain.ErrExportAlreadyPending
	}

	exportID, err := s.ids(ctx)
	if err != nil {
		return domain.ExportRequest{}, fmt.Errorf("generate export id: %w", err)
	}

	var created domain.ExportRequest
	err = dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		var createErr error
		created, createErr = txRepo.CreateExportRequest(txCtx, exportID, actorID)
		if createErr != nil {
			return createErr
		}

		if s.enqueuer != nil {
			if enqueueErr := s.enqueuer.EnqueueExportTx(txCtx, tx, exportID, actorID); enqueueErr != nil {
				return fmt.Errorf("enqueue export job: %w", enqueueErr)
			}
		}
		return nil
	})
	if err != nil {
		return domain.ExportRequest{}, fmt.Errorf("create and enqueue export request: %w", err)
	}

	return created, nil
}

// GetExportByID reads an export by ID for the authorized owner.
func (s *Service) GetExportByID(ctx context.Context, actorID, exportID uuid.UUID) (domain.ExportRequest, error) {
	export, err := s.repo.GetExportByID(ctx, exportID)
	if err != nil {
		return domain.ExportRequest{}, err
	}
	if export.UserID != actorID {
		return domain.ExportRequest{}, domain.ErrUserNotFound
	}
	return export, nil
}
