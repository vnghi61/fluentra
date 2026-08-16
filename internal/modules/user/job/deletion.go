package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/repository"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// EventWriter is what the deletion executor uses to write outbox events.
type EventWriter interface {
	Write(ctx context.Context, tx outbox.DBTx, aggregate, event string, payload any) (uuid.UUID, error)
}

const (
	// deletionExecutorLockID is the Postgres advisory lock ID for executing due account deletions.
	deletionExecutorLockID int64 = 1_700_000_050

	// deletionExecutorInterval is how often due account deletions are processed (daily).
	deletionExecutorInterval = 24 * time.Hour
)

// DeletionRepository defines what DeletionExecutor needs from persistence.
type DeletionRepository interface {
	GetDueDeletions(ctx context.Context, cutoff time.Time, limit int32) ([]domain.DeletionRequest, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	UpdateDeletionStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.DeletionStatus,
		startedAt, completedAt *time.Time,
		errorMessage *string,
	) error
	AnonymiseUser(ctx context.Context, userID uuid.UUID, anonymisedEmail string) error
	AnonymiseProfile(ctx context.Context, userID uuid.UUID) error
	DeletePreferences(ctx context.Context, userID uuid.UUID) error
	DeleteLearningProfile(ctx context.Context, userID uuid.UUID) error
	WithTx(tx pgx.Tx) DeletionRepository
}

type repoAdapter struct {
	*repository.Repository
}

func (a repoAdapter) WithTx(tx pgx.Tx) DeletionRepository {
	return repoAdapter{Repository: a.Repository.WithTx(tx)}
}

// DeletionExecutor runs scheduled daily jobs to execute account deletions past their 30-day grace period.
type DeletionExecutor struct {
	pool    dbx.Beginner
	repo    DeletionRepository
	storage StorageStore
	events  EventWriter
	bucket  string
}

// NewDeletionExecutor creates a new DeletionExecutor using the concrete repository.
func NewDeletionExecutor(
	pool dbx.Beginner,
	repo *repository.Repository,
	store StorageStore,
	events EventWriter,
	bucket string,
) *DeletionExecutor {
	return NewDeletionExecutorWithRepo(pool, repoAdapter{Repository: repo}, store, events, bucket)
}

// NewDeletionExecutorWithRepo creates a new DeletionExecutor using the repository interface.
func NewDeletionExecutorWithRepo(
	pool dbx.Beginner,
	repo DeletionRepository,
	store StorageStore,
	events EventWriter,
	bucket string,
) *DeletionExecutor {
	if bucket == "" {
		bucket = storage.BucketAvatars
	}
	return &DeletionExecutor{
		pool:    pool,
		repo:    repo,
		storage: store,
		events:  events,
		bucket:  bucket,
	}
}

// CronJob returns the scheduled job definition.
func (e *DeletionExecutor) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "user.account_deletion_executor",
		LockID:   deletionExecutorLockID,
		Interval: deletionExecutorInterval,
		Task:     e.Run,
	}
}

// Run processes all due account deletions.
func (e *DeletionExecutor) Run(ctx context.Context) error {
	const batchSize = 100
	now := time.Now().UTC()

	for {
		due, err := e.repo.GetDueDeletions(ctx, now, batchSize)
		if err != nil {
			return fmt.Errorf("fetch due deletions: %w", err)
		}
		if len(due) == 0 {
			break
		}

		for _, item := range due {
			if err := e.processDeletion(ctx, item); err != nil {
				slog.ErrorContext(ctx, "failed to process due account deletion",
					"deletion_id", item.ID, "user_id", item.UserID, "error", err)
			}
		}

		if len(due) < batchSize {
			break
		}
	}
	return nil
}

func (e *DeletionExecutor) processDeletion(ctx context.Context, item domain.DeletionRequest) error {
	startedAt := time.Now().UTC()
	err := dbx.InTx(ctx, e.pool, func(_ context.Context, tx pgx.Tx) error {
		txRepo := e.repo.WithTx(tx)

		if err := txRepo.UpdateDeletionStatus(
			ctx, item.ID, domain.DeletionStatusProcessing, &startedAt, nil, nil,
		); err != nil {
			return fmt.Errorf("mark deletion processing: %w", err)
		}

		// 1. Fetch profile to find avatar if any before anonymisation
		profile, err := txRepo.GetProfile(ctx, item.UserID)
		if err == nil && profile.AvatarAssetID != nil && e.storage != nil {
			// Best-effort avatar deletion from storage
			for _, size := range []string{"sm", "md", "lg"} {
				key := fmt.Sprintf("avatars/%s_%s.jpg", item.UserID, size)
				_ = e.storage.Delete(ctx, e.bucket, key)
			}
		}

		// 2. Anonymise user record
		anonymisedEmail := fmt.Sprintf("deleted-%s@anonymised.invalid", item.UserID)
		if err := txRepo.AnonymiseUser(ctx, item.UserID, anonymisedEmail); err != nil {
			return fmt.Errorf("anonymise user: %w", err)
		}

		// 3. Anonymise profile record
		if err := txRepo.AnonymiseProfile(ctx, item.UserID); err != nil {
			return fmt.Errorf("anonymise profile: %w", err)
		}

		// 4. Hard-delete user preferences and learning profiles
		if err := txRepo.DeletePreferences(ctx, item.UserID); err != nil {
			return fmt.Errorf("delete user preferences: %w", err)
		}
		if err := txRepo.DeleteLearningProfile(ctx, item.UserID); err != nil {
			return fmt.Errorf("delete learning profile: %w", err)
		}

		// 5. Publish user.deleted outbox event
		completedAt := time.Now().UTC()
		if e.events != nil {
			_, err := e.events.Write(
				ctx,
				tx,
				contract.Aggregate,
				contract.EventDeleted,
				contract.UserDeleted{
					UserID:     item.UserID,
					OccurredAt: completedAt,
				},
			)
			if err != nil {
				return fmt.Errorf("write user.deleted outbox event: %w", err)
			}
		}

		// 6. Mark deletion completed
		if err := txRepo.UpdateDeletionStatus(
			ctx, item.ID, domain.DeletionStatusCompleted, nil, &completedAt, nil,
		); err != nil {
			return fmt.Errorf("mark deletion completed: %w", err)
		}

		return nil
	})

	if err != nil {
		errMsg := err.Error()
		_ = e.repo.UpdateDeletionStatus(ctx, item.ID, domain.DeletionStatusFailed, nil, nil, &errMsg)
		return err
	}

	return nil
}
