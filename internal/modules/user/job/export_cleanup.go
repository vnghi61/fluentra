package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

const (
	// exportCleanupLockID is the Postgres advisory lock ID for data export retention cleanup.
	exportCleanupLockID int64 = 1_700_000_010

	// exportCleanupInterval is how often the expired export cleaner runs.
	exportCleanupInterval = 24 * time.Hour
)

// ExportCleanerRepository is what the cleanup job needs.
type ExportCleanerRepository interface {
	GetExpiredExports(ctx context.Context, limit int32) ([]domain.ExportRequest, error)
	DeleteExport(ctx context.Context, id uuid.UUID) error
}

// ExportCleaner handles purging expired export artefacts and records (BR-USER-07).
type ExportCleaner struct {
	repo    ExportCleanerRepository
	storage StorageStore
	bucket  string
}

// NewExportCleaner creates an ExportCleaner instance.
func NewExportCleaner(repo ExportCleanerRepository, store StorageStore, bucket string) *ExportCleaner {
	if bucket == "" {
		bucket = storage.BucketExports
	}
	return &ExportCleaner{
		repo:    repo,
		storage: store,
		bucket:  bucket,
	}
}

// CronJob returns the scheduled job definition.
func (c *ExportCleaner) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "user.export_retention_cleanup",
		LockID:   exportCleanupLockID,
		Interval: exportCleanupInterval,
		Task:     c.Run,
	}
}

// Run executes one sweep of expired exports cleanup.
func (c *ExportCleaner) Run(ctx context.Context) error {
	const batchSize = 100
	for {
		expired, err := c.repo.GetExpiredExports(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("fetch expired exports: %w", err)
		}
		if len(expired) == 0 {
			break
		}

		for _, item := range expired {
			if item.ObjectKey != nil && *item.ObjectKey != "" && c.storage != nil {
				if err := c.storage.Delete(ctx, c.bucket, *item.ObjectKey); err != nil {
					slog.WarnContext(ctx, "failed to delete expired export object from storage",
						"export_id", item.ID, "object_key", *item.ObjectKey, "error", err)
				}
			}
			if err := c.repo.DeleteExport(ctx, item.ID); err != nil {
				slog.ErrorContext(ctx, "failed to delete expired export database record",
					"export_id", item.ID, "error", err)
			}
		}

		if len(expired) < batchSize {
			break
		}
	}
	return nil
}
