package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/resource/sqlc"
	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
)

// Repository manages database operations for the resource module.
type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// New constructs a new resource repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// ConfirmFileResourceTx is ConfirmFileResource inside the caller's transaction.
//
// The service writes the row and enqueues its validation job together. The row
// write used to go through the pool while only the enqueue joined the
// transaction: an enqueue that failed rolled back the job and left the row at
// 'uploaded' with nothing ever coming to validate it.
func (r *Repository) ConfirmFileResourceTx(
	ctx context.Context, tx pgx.Tx, id, userID uuid.UUID,
) (*contract.Resource, error) {
	return (&Repository{pool: r.pool, queries: r.queries.WithTx(tx)}).ConfirmFileResource(ctx, id, userID)
}

// CreateURLResourceTx is CreateURLResource inside the caller's transaction, for
// the same reason as ConfirmFileResourceTx.
func (r *Repository) CreateURLResourceTx(
	ctx context.Context, tx pgx.Tx, id, userID uuid.UUID, title, sourceURL string,
) (*contract.Resource, error) {
	return (&Repository{pool: r.pool, queries: r.queries.WithTx(tx)}).CreateURLResource(ctx, id, userID, title, sourceURL)
}

// stateChanged maps "no row matched the status guard" to ErrResourceStateChanged.
// The validation updates are conditional on the status the caller read; zero
// rows means the resource moved on — deleted, or validated by another run —
// and that is an outcome, not a fault.
func stateChanged(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrResourceStateChanged
	}
	return fmt.Errorf("%s: %w", op, err)
}

func toContract(r sqlc.ResourceResource) contract.Resource {
	return contract.Resource{
		ID:               r.ID,
		UserID:           r.UserID,
		Kind:             r.Kind,
		Title:            r.Title,
		ObjectKey:        r.ObjectKey,
		OriginalFilename: r.OriginalFilename,
		DeclaredMIME:     r.DeclaredMime,
		DetectedMIME:     r.DetectedMime,
		ByteSize:         r.ByteSize,
		Checksum:         r.Checksum,
		SourceURL:        r.SourceUrl,
		Status:           r.Status,
		FailureReason:    r.FailureReason,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		ValidatedAt:      r.ValidatedAt,
	}
}

// CreateFileResourceIntent persists a new file resource intent in pending status.
func (r *Repository) CreateFileResourceIntent(
	ctx context.Context, id, userID uuid.UUID, title string, objectKey *string, originalFilename, declaredMime string,
) (*contract.Resource, error) {
	row, err := r.queries.CreateFileResourceIntent(ctx, sqlc.CreateFileResourceIntentParams{
		ID:               id,
		UserID:           userID,
		Title:            title,
		ObjectKey:        objectKey,
		OriginalFilename: originalFilename,
		DeclaredMime:     declaredMime,
	})
	if err != nil {
		return nil, fmt.Errorf("create file resource intent: %w", err)
	}
	res := toContract(row)
	return &res, nil
}

// CreateURLResource persists a new URL resource in uploaded status.
func (r *Repository) CreateURLResource(
	ctx context.Context, id, userID uuid.UUID, title, sourceURL string,
) (*contract.Resource, error) {
	row, err := r.queries.CreateURLResource(ctx, sqlc.CreateURLResourceParams{
		ID:        id,
		UserID:    userID,
		Title:     title,
		SourceUrl: &sourceURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create url resource: %w", err)
	}
	res := toContract(row)
	return &res, nil
}

// GetResourceByID fetches a resource by its unique identifier.
func (r *Repository) GetResourceByID(ctx context.Context, id uuid.UUID) (*contract.Resource, error) {
	row, err := r.queries.GetResourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrResourceNotFound
		}
		return nil, fmt.Errorf("get resource by id: %w", err)
	}
	res := toContract(row)
	return &res, nil
}

// GetResourceByIDAndUser fetches a resource belonging to the specified user.
// Returns ErrResourceNotFound if it does not exist or belongs to another user (BR-RESOURCE-01).
func (r *Repository) GetResourceByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	row, err := r.queries.GetResourceByIDAndUser(ctx, sqlc.GetResourceByIDAndUserParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrResourceNotFound
		}
		return nil, fmt.Errorf("get resource by id and user: %w", err)
	}
	res := toContract(row)
	return &res, nil
}

// ListResourcesByUser returns a paginated list of resources for a user.
func (r *Repository) ListResourcesByUser(
	ctx context.Context, userID uuid.UUID, status, kind *string, limit, offset int32,
) ([]contract.Resource, int, error) {
	total, err := r.queries.CountResourcesByUser(ctx, sqlc.CountResourcesByUserParams{
		UserID: userID,
		Status: status,
		Kind:   kind,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count resources by user: %w", err)
	}

	rows, err := r.queries.ListResourcesByUser(ctx, sqlc.ListResourcesByUserParams{
		UserID: userID,
		Status: status,
		Kind:   kind,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list resources by user: %w", err)
	}

	items := make([]contract.Resource, len(rows))
	for i, row := range rows {
		items[i] = toContract(row)
	}

	return items, int(total), nil
}

// GetUserResourceUsage returns total active count and byte size for quota enforcement.
func (r *Repository) GetUserResourceUsage(ctx context.Context, userID uuid.UUID) (int64, int64, error) {
	usage, err := r.queries.GetUserResourceUsage(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("get user resource usage: %w", err)
	}
	return usage.ResourceCount, usage.TotalBytes, nil
}

// ConfirmFileResource moves a pending file resource to uploaded.
func (r *Repository) ConfirmFileResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	row, err := r.queries.ConfirmFileResource(ctx, sqlc.ConfirmFileResourceParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrResourceNotFound
		}
		return nil, fmt.Errorf("confirm file resource: %w", err)
	}
	res := toContract(row)
	return &res, nil
}

// UpdateValidationSuccess marks a resource as validated with metadata.
func (r *Repository) UpdateValidationSuccess(
	ctx context.Context, id uuid.UUID, title, detectedMime string, byteSize int64, checksum string,
) (*contract.Resource, error) {
	row, err := r.queries.UpdateResourceValidationSuccess(ctx, sqlc.UpdateResourceValidationSuccessParams{
		ID:           id,
		Column2:      title,
		DetectedMime: detectedMime,
		ByteSize:     &byteSize,
		Checksum:     &checksum,
	})
	if err != nil {
		return nil, stateChanged(err, "update resource validation success")
	}
	res := toContract(row)
	return &res, nil
}

// UpdateValidationRejected marks a resource as rejected with a learner-facing reason.
func (r *Repository) UpdateValidationRejected(
	ctx context.Context, id uuid.UUID, failureReason string, detectedMime *string,
) (*contract.Resource, error) {
	row, err := r.queries.UpdateResourceValidationRejected(ctx, sqlc.UpdateResourceValidationRejectedParams{
		ID:            id,
		FailureReason: failureReason,
		DetectedMime:  detectedMime,
	})
	if err != nil {
		return nil, stateChanged(err, "update resource validation rejected")
	}
	res := toContract(row)
	return &res, nil
}

// UpdateValidationFailed marks a resource failed, but only if it is still at
// fromStatus. It returns ErrResourceStateChanged when the row has moved on.
func (r *Repository) UpdateValidationFailed(
	ctx context.Context, id uuid.UUID, failureReason, fromStatus string,
) (*contract.Resource, error) {
	row, err := r.queries.UpdateResourceValidationFailed(ctx, sqlc.UpdateResourceValidationFailedParams{
		ID:            id,
		FailureReason: failureReason,
		FromStatus:    fromStatus,
	})
	if err != nil {
		return nil, stateChanged(err, "update resource validation failed")
	}
	res := toContract(row)
	return &res, nil
}

// DeleteResource deletes the resource row belonging to userID.
func (r *Repository) DeleteResource(ctx context.Context, id, userID uuid.UUID) (*string, string, error) {
	row, err := r.queries.DeleteResource(ctx, sqlc.DeleteResourceParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", domain.ErrResourceNotFound
		}
		return nil, "", fmt.Errorf("delete resource: %w", err)
	}
	return row.ObjectKey, row.Kind, nil
}

// ListExpiredPendingResources fetches pending resources created before a threshold.
func (r *Repository) ListExpiredPendingResources(
	ctx context.Context, before time.Time, limit int32,
) ([]contract.Resource, error) {
	rows, err := r.queries.ListExpiredPendingResources(ctx, sqlc.ListExpiredPendingResourcesParams{
		CreatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list expired pending resources: %w", err)
	}
	items := make([]contract.Resource, len(rows))
	for i, row := range rows {
		items[i] = toContract(row)
	}
	return items, nil
}

// ListStuckUploadedResources fetches confirmed resources whose validation has
// not finished since before the threshold.
func (r *Repository) ListStuckUploadedResources(
	ctx context.Context, before time.Time, limit int32,
) ([]contract.Resource, error) {
	rows, err := r.queries.ListStuckUploadedResources(ctx, sqlc.ListStuckUploadedResourcesParams{
		UpdatedAt: before,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list stuck uploaded resources: %w", err)
	}
	items := make([]contract.Resource, len(rows))
	for i, row := range rows {
		items[i] = toContract(row)
	}
	return items, nil
}
