package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	"github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// RevalidateResource rechecks existing text; ownership is checked before either
// the old classification is cleared or the background job is inserted.
func (s *Service) RevalidateResource(ctx context.Context, userID, resourceID uuid.UUID) error {
	res, err := s.repo.GetResourceByIDAndUser(ctx, resourceID, userID)
	if err != nil {
		return err
	}
	ext, err := s.repo.GetExtractionByResourceID(ctx, resourceID)
	if err != nil {
		return err
	}
	if res.Status != domain.StatusValidated || !readableExtraction(ext) {
		return apperr.New(apperr.Validation, "RESOURCE_PRACTICE_NO_TEXT",
			"Wait until readable text has been extracted before checking this file again.")
	}
	if s.enqueuer == nil {
		return apperr.New(apperr.Internal, "RESOURCE_REVALIDATION_UNAVAILABLE", "Content validation is not configured.")
	}
	return dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		if err := s.repo.DeleteClassificationTx(txCtx, tx, resourceID); err != nil {
			return fmt.Errorf("clear previous classification: %w", err)
		}
		_, err := s.enqueuer.EnqueueTx(txCtx, tx, job.ClassifyResourceArgs{ResourceID: resourceID}, nil)
		return err
	})
}
