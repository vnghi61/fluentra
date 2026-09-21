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

// ListResourcesByUserID returns all resources belonging to a user.
func (r *Repository) ListResourcesByUserID(ctx context.Context, userID uuid.UUID) ([]contract.Resource, error) {
	rows, err := r.queries.ListResourcesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list resources by user: %w", err)
	}
	items := make([]contract.Resource, len(rows))
	for i, row := range rows {
		items[i] = toContract(row)
	}
	return items, nil
}

// DeleteAllResourcesByUser removes all resources belonging to a user.
func (r *Repository) DeleteAllResourcesByUser(
	ctx context.Context, userID uuid.UUID,
) error {
	_, err := r.queries.DeleteAllResourcesByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete all resources by user: %w", err)
	}
	return nil
}

func renditionToContract(r sqlc.ResourceRendition) contract.Rendition {
	var w, h, d *int
	if r.Width != nil {
		val := int(*r.Width)
		w = &val
	}
	if r.Height != nil {
		val := int(*r.Height)
		h = &val
	}
	if r.DurationMs != nil {
		val := int(*r.DurationMs)
		d = &val
	}
	return contract.Rendition{
		ID:            r.ID,
		ResourceID:    r.ResourceID,
		Kind:          r.Kind,
		Status:        r.Status,
		ObjectKey:     r.ObjectKey,
		MIMEType:      r.MimeType,
		Width:         w,
		Height:        h,
		DurationMS:    d,
		ByteSize:      r.ByteSize,
		ToolVersion:   r.ToolVersion,
		Attempts:      int(r.Attempts),
		FailureReason: r.FailureReason,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

// InsertRenditionPending creates a pending rendition for a resource if not present.
func (r *Repository) InsertRenditionPending(
	ctx context.Context, resourceID uuid.UUID, kind string,
) (*contract.Rendition, error) {
	row, err := r.queries.InsertRenditionPending(ctx, sqlc.InsertRenditionPendingParams{
		ResourceID: resourceID,
		Kind:       kind,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // ON CONFLICT DO NOTHING
		}
		return nil, fmt.Errorf("insert rendition pending: %w", err)
	}
	res := renditionToContract(row)
	return &res, nil
}

// ClaimPendingRenditions claims up to limit pending renditions with FOR UPDATE SKIP LOCKED.
func (r *Repository) ClaimPendingRenditions(
	ctx context.Context, limit int32,
) ([]contract.Rendition, error) {
	rows, err := r.queries.ClaimPendingRenditions(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending renditions: %w", err)
	}
	items := make([]contract.Rendition, len(rows))
	for i, row := range rows {
		items[i] = renditionToContract(row)
	}
	return items, nil
}

// UpdateRenditionReady marks a rendition as ready with metadata and object key.
func (r *Repository) UpdateRenditionReady(
	ctx context.Context, id uuid.UUID, objectKey, mimeType string,
	width, height, durationMS *int, byteSize *int64, toolVersion string,
) (*contract.Rendition, error) {
	var w, h, d *int32
	if width != nil {
		v := int32(*width)
		w = &v
	}
	if height != nil {
		v := int32(*height)
		h = &v
	}
	if durationMS != nil {
		v := int32(*durationMS)
		d = &v
	}
	row, err := r.queries.UpdateRenditionReady(ctx, sqlc.UpdateRenditionReadyParams{
		ID:          id,
		ObjectKey:   &objectKey,
		MimeType:    mimeType,
		Width:       w,
		Height:      h,
		DurationMs:  d,
		ByteSize:    byteSize,
		ToolVersion: toolVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("update rendition ready: %w", err)
	}
	res := renditionToContract(row)
	return &res, nil
}

// UpdateRenditionSkipped marks a rendition as skipped with a reason.
func (r *Repository) UpdateRenditionSkipped(
	ctx context.Context, id uuid.UUID, reason string,
) (*contract.Rendition, error) {
	row, err := r.queries.UpdateRenditionSkipped(ctx, sqlc.UpdateRenditionSkippedParams{
		ID:            id,
		FailureReason: reason,
	})
	if err != nil {
		return nil, fmt.Errorf("update rendition skipped: %w", err)
	}
	res := renditionToContract(row)
	return &res, nil
}

// UpdateRenditionFailed records a rendition failure, incrementing towards terminal failure.
func (r *Repository) UpdateRenditionFailed(
	ctx context.Context, id uuid.UUID, reason string,
) (*contract.Rendition, error) {
	row, err := r.queries.UpdateRenditionFailed(ctx, sqlc.UpdateRenditionFailedParams{
		ID:            id,
		FailureReason: reason,
	})
	if err != nil {
		return nil, fmt.Errorf("update rendition failed: %w", err)
	}
	res := renditionToContract(row)
	return &res, nil
}

// ListReadyRenditionsByResourceID returns ready renditions for a resource.
func (r *Repository) ListReadyRenditionsByResourceID(
	ctx context.Context, resourceID uuid.UUID,
) ([]contract.Rendition, error) {
	rows, err := r.queries.ListReadyRenditionsByResourceID(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("list ready renditions: %w", err)
	}
	items := make([]contract.Rendition, len(rows))
	for i, row := range rows {
		items[i] = renditionToContract(row)
	}
	return items, nil
}

// ListRenditionsByResourceID returns all renditions for a resource.
func (r *Repository) ListRenditionsByResourceID(
	ctx context.Context, resourceID uuid.UUID,
) ([]contract.Rendition, error) {
	rows, err := r.queries.ListRenditionsByResourceID(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("list renditions: %w", err)
	}
	items := make([]contract.Rendition, len(rows))
	for i, row := range rows {
		items[i] = renditionToContract(row)
	}
	return items, nil
}

// ListRenditionKeysByUserID returns all rendition object keys for a user's resources.
func (r *Repository) ListRenditionKeysByUserID(
	ctx context.Context, userID uuid.UUID,
) ([]string, error) {
	keys, err := r.queries.ListRenditionKeysByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list rendition keys by user: %w", err)
	}
	res := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != nil && *k != "" {
			res = append(res, *k)
		}
	}
	return res, nil
}

// ListValidatedFileResourcesForRenditions fetches validated file resources to plan renditions.
func (r *Repository) ListValidatedFileResourcesForRenditions(
	ctx context.Context, limit int32,
) ([]contract.Resource, error) {
	rows, err := r.queries.ListValidatedFileResourcesForRenditions(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list validated files for renditions: %w", err)
	}
	items := make([]contract.Resource, len(rows))
	for i, row := range rows {
		items[i] = toContract(row)
	}
	return items, nil
}

// UpsertExtraction inserts or updates an extraction row for a resource.
func (r *Repository) UpsertExtraction(
	ctx context.Context,
	resourceID uuid.UUID,
	source, text string,
	charCount int32,
	truncated bool,
	language, toolVersion string,
) (*contract.Extraction, error) {
	row, err := r.queries.UpsertExtraction(ctx, sqlc.UpsertExtractionParams{
		ResourceID:  resourceID,
		Source:      source,
		Text:        text,
		CharCount:   charCount,
		Truncated:   truncated,
		Language:    language,
		ToolVersion: toolVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert extraction: %w", err)
	}
	return extractionToContract(row), nil
}

// GetExtractionByResourceID retrieves the extraction row for a resource.
func (r *Repository) GetExtractionByResourceID(
	ctx context.Context, resourceID uuid.UUID,
) (*contract.Extraction, error) {
	row, err := r.queries.GetExtractionByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get extraction: %w", err)
	}
	return extractionToContract(row), nil
}

// UpsertClassification inserts or updates a classification row for a resource.
func (r *Repository) UpsertClassification(
	ctx context.Context,
	resourceID uuid.UUID,
	cefrEstimate, skill *string,
	nodeCodes []string,
	promptVersion, model string,
	aiRequestID *uuid.UUID,
) (*contract.Classification, error) {
	row, err := r.queries.UpsertClassification(ctx, sqlc.UpsertClassificationParams{
		ResourceID:    resourceID,
		CefrEstimate:  cefrEstimate,
		Skill:         skill,
		NodeCodes:     nodeCodes,
		PromptVersion: promptVersion,
		Model:         model,
		AiRequestID:   aiRequestID,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert classification: %w", err)
	}
	return classificationToContract(row), nil
}

// GetClassificationByResourceID retrieves the classification row for a resource.
func (r *Repository) GetClassificationByResourceID(
	ctx context.Context, resourceID uuid.UUID,
) (*contract.Classification, error) {
	row, err := r.queries.GetClassificationByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get classification: %w", err)
	}
	return classificationToContract(row), nil
}

func extractionToContract(row sqlc.ResourceExtraction) *contract.Extraction {
	excerpt := row.Text
	if len(excerpt) > 2000 {
		excerpt = excerpt[:2000]
	}
	return &contract.Extraction{
		ResourceID:  row.ResourceID,
		Source:      row.Source,
		Text:        row.Text,
		CharCount:   int(row.CharCount),
		Truncated:   row.Truncated,
		Language:    row.Language,
		ToolVersion: row.ToolVersion,
		Excerpt:     excerpt,
		CreatedAt:   row.CreatedAt,
	}
}

func classificationToContract(row sqlc.ResourceClassification) *contract.Classification {
	return &contract.Classification{
		ResourceID:    row.ResourceID,
		CEFR_Estimate: row.CefrEstimate,
		Skill:         row.Skill,
		NodeCodes:     row.NodeCodes,
		PromptVersion: row.PromptVersion,
		Model:         row.Model,
		AIRequestID:   row.AiRequestID,
		CreatedAt:     row.CreatedAt,
	}
}
