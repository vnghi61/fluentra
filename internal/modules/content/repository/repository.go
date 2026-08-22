package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sqlccontent "github.com/fluentra/fluentra/internal/generated/content/sqlc"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository provides typed database access for the content module.
type Repository struct {
	queries *sqlccontent.Queries
}

// New creates a repository over db (either *pgxpool.Pool or pgx.Tx).
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlccontent.New(db)}
}

// WithTx derives a transactional repository.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: sqlccontent.New(tx)}
}

// CreateItem creates a content item identity record.
func (r *Repository) CreateItem(
	ctx context.Context,
	id uuid.UUID,
	kind, slug string,
	status domain.AuthoringStatus,
	ownerID uuid.UUID,
) (domain.Item, error) {
	row, err := r.queries.CreateContentItem(ctx, sqlccontent.CreateContentItemParams{
		ID:      id,
		Kind:    kind,
		Slug:    slug,
		Status:  sqlccontent.ContentAuthoringStatus(status),
		OwnerID: ownerID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_content_items_slug" {
			return domain.Item{}, domain.ErrSlugAlreadyExists
		}
		return domain.Item{}, fmt.Errorf("create content item: %w", err)
	}
	return toDomainItem(row), nil
}

// GetItemByID retrieves a content item by ID.
func (r *Repository) GetItemByID(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	row, err := r.queries.GetContentItemByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, domain.ErrItemNotFound
		}
		return domain.Item{}, fmt.Errorf("get content item by id: %w", err)
	}
	return toDomainItem(row), nil
}

// GetItemBySlug retrieves a content item by its unique slug.
func (r *Repository) GetItemBySlug(ctx context.Context, slug string) (domain.Item, error) {
	row, err := r.queries.GetContentItemBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, domain.ErrItemNotFound
		}
		return domain.Item{}, fmt.Errorf("get content item by slug: %w", err)
	}
	return toDomainItem(row), nil
}

// UpdateItemStatus updates the authoring status of an item.
func (r *Repository) UpdateItemStatus(
	ctx context.Context, id uuid.UUID, status domain.AuthoringStatus,
) (domain.Item, error) {
	row, err := r.queries.UpdateContentItemStatus(ctx, sqlccontent.UpdateContentItemStatusParams{
		ID:     id,
		Status: sqlccontent.ContentAuthoringStatus(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, domain.ErrItemNotFound
		}
		return domain.Item{}, fmt.Errorf("update content item status: %w", err)
	}
	return toDomainItem(row), nil
}

// UpdateItemCurrentVersion updates the current_version_id pointer of an item.
func (r *Repository) UpdateItemCurrentVersion(
	ctx context.Context, id uuid.UUID, currentVersionID *uuid.UUID,
) (domain.Item, error) {
	row, err := r.queries.UpdateContentItemCurrentVersion(ctx, sqlccontent.UpdateContentItemCurrentVersionParams{
		ID:               id,
		CurrentVersionID: currentVersionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, domain.ErrItemNotFound
		}
		return domain.Item{}, fmt.Errorf("update content item current version: %w", err)
	}
	return toDomainItem(row), nil
}

// ListItemsByOwner retrieves items created by a specific author.
func (r *Repository) ListItemsByOwner(ctx context.Context, ownerID uuid.UUID, limit int32) ([]domain.Item, error) {
	rows, err := r.queries.ListContentItemsByOwner(ctx, sqlccontent.ListContentItemsByOwnerParams{
		OwnerID:     ownerID,
		ResultLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list content items by owner: %w", err)
	}
	items := make([]domain.Item, len(rows))
	for i, row := range rows {
		items[i] = toDomainItem(row)
	}
	return items, nil
}

// DeleteItem deletes an item.
func (r *Repository) DeleteItem(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteContentItem(ctx, id)
}

// CreateVersion inserts a new version snapshot.
func (r *Repository) CreateVersion(
	ctx context.Context,
	id, itemID uuid.UUID,
	version int,
	kind string,
	body []byte,
	cefrLevel string,
	status domain.AuthoringStatus,
	mediaRefs []string,
	publishedAt *time.Time,
) (domain.Version, error) {
	if mediaRefs == nil {
		mediaRefs = []string{}
	}
	// #nosec G115 -- version numbers in the content module are positive integers well within int32 bounds
	vInt32 := int32(version)
	row, err := r.queries.CreateContentVersion(ctx, sqlccontent.CreateContentVersionParams{
		ID:          id,
		ItemID:      itemID,
		Version:     vInt32,
		Kind:        kind,
		Body:        body,
		CefrLevel:   cefrLevel,
		Status:      sqlccontent.ContentAuthoringStatus(status),
		MediaRefs:   mediaRefs,
		PublishedAt: publishedAt,
	})
	if err != nil {
		return domain.Version{}, fmt.Errorf("create content version: %w", err)
	}
	return toDomainVersion(row), nil
}

// GetVersionByID retrieves a version snapshot by ID.
func (r *Repository) GetVersionByID(ctx context.Context, id uuid.UUID) (domain.Version, error) {
	row, err := r.queries.GetContentVersionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrVersionNotFound
		}
		return domain.Version{}, fmt.Errorf("get content version by id: %w", err)
	}
	return toDomainVersion(row), nil
}

// GetVersionByItemAndVersion retrieves a version snapshot by item ID and version number.
func (r *Repository) GetVersionByItemAndVersion(
	ctx context.Context,
	itemID uuid.UUID,
	version int,
) (domain.Version, error) {
	// #nosec G115 -- version numbers in the content module are positive integers within int32 bounds
	row, err := r.queries.GetContentVersionByItemAndVersion(ctx, sqlccontent.GetContentVersionByItemAndVersionParams{
		ItemID:  itemID,
		Version: int32(version),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrVersionNotFound
		}
		return domain.Version{}, fmt.Errorf("get content version by item and version: %w", err)
	}
	return toDomainVersion(row), nil
}

// GetDraftVersionByItemID retrieves the latest draft / in_review / approved version for an item.
func (r *Repository) GetDraftVersionByItemID(ctx context.Context, itemID uuid.UUID) (domain.Version, error) {
	row, err := r.queries.GetDraftVersionByItemID(ctx, itemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrVersionNotFound
		}
		return domain.Version{}, fmt.Errorf("get draft content version: %w", err)
	}
	return toDomainVersion(row), nil
}

// ListVersionsByItemID lists all versions for a content item in descending version order.
func (r *Repository) ListVersionsByItemID(ctx context.Context, itemID uuid.UUID) ([]domain.Version, error) {
	rows, err := r.queries.ListContentVersionsByItemID(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list content versions by item id: %w", err)
	}
	versions := make([]domain.Version, len(rows))
	for i, row := range rows {
		versions[i] = toDomainVersion(row)
	}
	return versions, nil
}

// GetManyVersionsByIDs retrieves multiple content versions in ONE single query (preventing N+1).
func (r *Repository) GetManyVersionsByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Version, error) {
	if len(ids) == 0 {
		return []domain.Version{}, nil
	}
	rows, err := r.queries.GetManyContentVersionsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get many content versions by ids: %w", err)
	}
	versions := make([]domain.Version, len(rows))
	for i, row := range rows {
		versions[i] = toDomainVersion(row)
	}
	return versions, nil
}

// UpdateVersionDraft updates an un-published draft version.
func (r *Repository) UpdateVersionDraft(
	ctx context.Context,
	id uuid.UUID,
	kind string,
	body []byte,
	cefrLevel string,
	mediaRefs []string,
	status domain.AuthoringStatus,
) (domain.Version, error) {
	if mediaRefs == nil {
		mediaRefs = []string{}
	}
	row, err := r.queries.UpdateContentVersionDraft(ctx, sqlccontent.UpdateContentVersionDraftParams{
		ID:        id,
		Kind:      kind,
		Body:      body,
		CefrLevel: cefrLevel,
		MediaRefs: mediaRefs,
		Status:    sqlccontent.ContentAuthoringStatus(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrVersionNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			// Trigger trg_content_versions_immutable refused update
			return domain.Version{}, domain.ErrInvalidStateTransition.WithInternal("cannot update a published content version")
		}
		return domain.Version{}, fmt.Errorf("update content version draft: %w", err)
	}
	return toDomainVersion(row), nil
}

// PublishVersion updates a version's status to published.
func (r *Repository) PublishVersion(ctx context.Context, id uuid.UUID) (domain.Version, error) {
	row, err := r.queries.PublishContentVersion(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrVersionNotFound
		}
		return domain.Version{}, fmt.Errorf("publish content version: %w", err)
	}
	return toDomainVersion(row), nil
}

// GetLatestVersionNumberByItemID returns the highest version integer for an item.
func (r *Repository) GetLatestVersionNumberByItemID(ctx context.Context, itemID uuid.UUID) (int, error) {
	v, err := r.queries.GetLatestVersionNumberByItemID(ctx, itemID)
	if err != nil {
		return 0, fmt.Errorf("get latest version number: %w", err)
	}
	return int(v), nil
}

// GetPublishedVersionBySlug gets a published version by matching item slug where both item and version are published.
func (r *Repository) GetPublishedVersionBySlug(ctx context.Context, slug string) (domain.Version, error) {
	row, err := r.queries.GetPublishedVersionBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Version{}, domain.ErrContentNotPublished
		}
		return domain.Version{}, fmt.Errorf("get published version by slug: %w", err)
	}
	return toDomainVersion(row), nil
}

// BrowsePublishedVersions retrieves published versions with optional filters.
func (r *Repository) BrowsePublishedVersions(
	ctx context.Context, kind, cefrLevel *string, limit, offset int32,
) ([]domain.Version, error) {
	rows, err := r.queries.BrowsePublishedContentVersions(ctx, sqlccontent.BrowsePublishedContentVersionsParams{
		Limit:     limit,
		Offset:    offset,
		Kind:      kind,
		CefrLevel: cefrLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("browse published content versions: %w", err)
	}
	versions := make([]domain.Version, len(rows))
	for i, row := range rows {
		versions[i] = toDomainVersion(row)
	}
	return versions, nil
}

// CountPublishedVersions counts the total published versions matching filters.
func (r *Repository) CountPublishedVersions(ctx context.Context, kind, cefrLevel *string) (int64, error) {
	count, err := r.queries.CountPublishedContentVersions(ctx, sqlccontent.CountPublishedContentVersionsParams{
		Kind:      kind,
		CefrLevel: cefrLevel,
	})
	if err != nil {
		return 0, fmt.Errorf("count published content versions: %w", err)
	}
	return count, nil
}

// GetMediaAssetByObjectKey retrieves a media asset by its object key.
func (r *Repository) GetMediaAssetByObjectKey(ctx context.Context, objectKey string) (domain.MediaAsset, error) {
	row, err := r.queries.GetMediaAssetByObjectKey(ctx, objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.MediaAsset{}, fmt.Errorf("media asset not found for object key %q", objectKey)
		}
		return domain.MediaAsset{}, fmt.Errorf("get media asset by object key: %w", err)
	}
	return toDomainMediaAsset(row), nil
}

// GetMediaAssetsByObjectKeys retrieves media assets matching any of the object keys.
func (r *Repository) GetMediaAssetsByObjectKeys(ctx context.Context, objectKeys []string) ([]domain.MediaAsset, error) {
	if len(objectKeys) == 0 {
		return []domain.MediaAsset{}, nil
	}
	rows, err := r.queries.GetMediaAssetsByObjectKeys(ctx, objectKeys)
	if err != nil {
		return nil, fmt.Errorf("get media assets by object keys: %w", err)
	}
	assets := make([]domain.MediaAsset, len(rows))
	for i, row := range rows {
		assets[i] = toDomainMediaAsset(row)
	}
	return assets, nil
}

// CreateMediaAsset creates a media asset record.
func (r *Repository) CreateMediaAsset(
	ctx context.Context,
	id uuid.UUID,
	objectKey, kind string,
	durationMs *int32,
	checksum *string,
	status domain.MediaStatus,
	byteSize *int64,
	mimeType *string,
) (domain.MediaAsset, error) {
	row, err := r.queries.CreateMediaAsset(ctx, sqlccontent.CreateMediaAssetParams{
		ID:         id,
		ObjectKey:  objectKey,
		Kind:       kind,
		DurationMs: durationMs,
		Checksum:   checksum,
		Status:     sqlccontent.ContentMediaStatus(status),
		ByteSize:   byteSize,
		MimeType:   mimeType,
	})
	if err != nil {
		return domain.MediaAsset{}, fmt.Errorf("create media asset: %w", err)
	}
	return toDomainMediaAsset(row), nil
}

// UpdateMediaAssetStatus updates the processing status and metadata of a media asset.
func (r *Repository) UpdateMediaAssetStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.MediaStatus,
	durationMs *int32,
	checksum *string,
	byteSize *int64,
	mimeType *string,
) (domain.MediaAsset, error) {
	row, err := r.queries.UpdateMediaAssetStatus(ctx, sqlccontent.UpdateMediaAssetStatusParams{
		ID:         id,
		Status:     sqlccontent.ContentMediaStatus(status),
		DurationMs: durationMs,
		Checksum:   checksum,
		ByteSize:   byteSize,
		MimeType:   mimeType,
	})
	if err != nil {
		return domain.MediaAsset{}, fmt.Errorf("update media asset status: %w", err)
	}
	return toDomainMediaAsset(row), nil
}

// CreateReview records an audit trail for a review decision.
func (r *Repository) CreateReview(
	ctx context.Context,
	id, versionID, reviewerID uuid.UUID,
	decision domain.ReviewDecision,
	comments *string,
) (domain.Review, error) {
	row, err := r.queries.CreateContentReview(ctx, sqlccontent.CreateContentReviewParams{
		ID:         id,
		VersionID:  versionID,
		ReviewerID: reviewerID,
		Decision:   sqlccontent.ContentReviewDecision(decision),
		Comments:   comments,
	})
	if err != nil {
		return domain.Review{}, fmt.Errorf("create content review: %w", err)
	}
	return toDomainReview(row), nil
}

// ListReviewsForVersion retrieves reviews for a given version.
func (r *Repository) ListReviewsForVersion(ctx context.Context, versionID uuid.UUID) ([]domain.Review, error) {
	rows, err := r.queries.ListContentReviewsForVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("list content reviews for version: %w", err)
	}
	reviews := make([]domain.Review, len(rows))
	for i, row := range rows {
		reviews[i] = toDomainReview(row)
	}
	return reviews, nil
}

// AddContentTag associates an item with a taxonomy tag.
func (r *Repository) AddContentTag(ctx context.Context, itemID, taxonomyID uuid.UUID) error {
	return r.queries.AddContentTag(ctx, sqlccontent.AddContentTagParams{
		ItemID:     itemID,
		TaxonomyID: taxonomyID,
	})
}

// ClearTagsForContentItem removes all tags for an item.
func (r *Repository) ClearTagsForContentItem(ctx context.Context, itemID uuid.UUID) error {
	return r.queries.ClearTagsForContentItem(ctx, itemID)
}

// ListTagsForContentItem lists taxonomy tags attached to an item.
func (r *Repository) ListTagsForContentItem(ctx context.Context, itemID uuid.UUID) ([]domain.Taxonomy, error) {
	rows, err := r.queries.ListTagsForContentItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list tags for content item: %w", err)
	}
	tags := make([]domain.Taxonomy, len(rows))
	for i, row := range rows {
		tags[i] = domain.Taxonomy{
			ID:        row.ID,
			Namespace: row.Namespace,
			Code:      row.Code,
			Label:     row.Label,
			ParentID:  row.ParentID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}
	return tags, nil
}

// ListTagsForContentItems loads tags in a single batch for multiple item IDs.
func (r *Repository) ListTagsForContentItems(
	ctx context.Context,
	itemIDs []uuid.UUID,
) (map[uuid.UUID][]domain.TaxonomyTag, error) {
	result := make(map[uuid.UUID][]domain.TaxonomyTag, len(itemIDs))
	if len(itemIDs) == 0 {
		return result, nil
	}
	rows, err := r.queries.ListTagsForContentItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("list tags for content items: %w", err)
	}
	for _, row := range rows {
		result[row.ItemID] = append(result[row.ItemID], domain.TaxonomyTag{
			Namespace: row.Namespace,
			Code:      row.Code,
			Label:     row.Label,
		})
	}
	return result, nil
}

// GetTaxonomyByNamespaceCode looks up a taxonomy entry by namespace and code.
func (r *Repository) GetTaxonomyByNamespaceCode(ctx context.Context, namespace, code string) (domain.Taxonomy, error) {
	row, err := r.queries.GetTaxonomyByNamespaceCode(ctx, sqlccontent.GetTaxonomyByNamespaceCodeParams{
		Namespace: namespace,
		Code:      code,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Taxonomy{}, fmt.Errorf("taxonomy %s:%s not found", namespace, code)
		}
		return domain.Taxonomy{}, fmt.Errorf("get taxonomy: %w", err)
	}
	return toDomainTaxonomy(row), nil
}
