package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

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
			return domain.Taxonomy{}, domain.ErrTaxonomyNotFound
		}
		return domain.Taxonomy{}, fmt.Errorf("get taxonomy: %w", err)
	}
	return toDomainTaxonomy(row), nil
}

// ListContentItemsFiltered retrieves a paginated slice of items with optional status/kind filters.
func (r *Repository) ListContentItemsFiltered(
	ctx context.Context,
	status, kind, query *string,
	limit, offset int32,
) ([]domain.Item, error) {
	rows, err := r.queries.ListContentItemsFiltered(ctx, sqlccontent.ListContentItemsFilteredParams{
		Status:       status,
		Kind:         kind,
		Query:        query,
		ResultLimit:  limit,
		ResultOffset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list content items filtered: %w", err)
	}
	items := make([]domain.Item, len(rows))
	for i, row := range rows {
		items[i] = toDomainItem(row)
	}
	return items, nil
}

// CountContentItemsFiltered counts items matching optional status/kind filters.
func (r *Repository) CountContentItemsFiltered(
	ctx context.Context, status, kind, query *string,
) (int64, error) {
	count, err := r.queries.CountContentItemsFiltered(ctx, sqlccontent.CountContentItemsFilteredParams{
		Status: status,
		Kind:   kind,
		Query:  query,
	})
	if err != nil {
		return 0, fmt.Errorf("count content items filtered: %w", err)
	}
	return count, nil
}

// InsertItemReport creates or updates an item report by a user for a version.
func (r *Repository) InsertItemReport(
	ctx context.Context,
	versionID, userID uuid.UUID,
	reason domain.ReportReason,
	note *string,
) (domain.ItemReport, error) {
	row, err := r.queries.InsertItemReport(ctx, sqlccontent.InsertItemReportParams{
		ContentVersionID: versionID,
		UserID:           userID,
		Reason:           string(reason),
		Note:             note,
	})
	if err != nil {
		return domain.ItemReport{}, fmt.Errorf("insert item report: %w", err)
	}
	return domain.ItemReport{
		ID:               row.ID,
		ContentVersionID: row.ContentVersionID,
		UserID:           row.UserID,
		Reason:           domain.ReportReason(row.Reason),
		Note:             row.Note,
		CreatedAt:        row.CreatedAt,
	}, nil
}

// ListItemReportsByVersion returns all reports for a specific content version.
func (r *Repository) ListItemReportsByVersion(
	ctx context.Context, versionID uuid.UUID,
) ([]domain.ItemReport, error) {
	rows, err := r.queries.ListItemReportsByVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("list item reports by version: %w", err)
	}
	reports := make([]domain.ItemReport, len(rows))
	for i, row := range rows {
		reports[i] = domain.ItemReport{
			ID:               row.ID,
			ContentVersionID: row.ContentVersionID,
			UserID:           row.UserID,
			Reason:           domain.ReportReason(row.Reason),
			Note:             row.Note,
			CreatedAt:        row.CreatedAt,
		}
	}
	return reports, nil
}

// ListReportedContentVersions returns aggregated reported versions ordered by distinct reporters.
func (r *Repository) ListReportedContentVersions(
	ctx context.Context, limit, offset int32,
) ([]domain.ReportedVersionSummary, error) {
	rows, err := r.queries.ListReportedContentVersions(ctx, sqlccontent.ListReportedContentVersionsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list reported content versions: %w", err)
	}
	summaries := make([]domain.ReportedVersionSummary, len(rows))
	for i, row := range rows {
		summaries[i] = domain.ReportedVersionSummary{
			ContentVersionID: row.ContentVersionID,
			ItemID:           row.ItemID,
			Slug:             row.Slug,
			Kind:             row.Kind,
			CEFRLevel:        row.CefrLevel,
			ItemStatus:       string(row.ItemStatus),
			ReportCount:      int(row.ReportCount),
			LastReportedAt:   row.LastReportedAt,
		}
	}
	return summaries, nil
}

// CountReportedContentVersions counts distinct reported content versions.
func (r *Repository) CountReportedContentVersions(ctx context.Context) (int, error) {
	total, err := r.queries.CountReportedContentVersions(ctx)
	if err != nil {
		return 0, fmt.Errorf("count reported content versions: %w", err)
	}
	return int(total), nil
}

// GetTTSCache retrieves a cached TTS audio entry by text hash and voice.
func (r *Repository) GetTTSCache(ctx context.Context, textHash, voice string) (string, bool, error) {
	row, err := r.queries.GetTTSCache(ctx, sqlccontent.GetTTSCacheParams{
		TextHash: textHash,
		Voice:    voice,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get tts cache: %w", err)
	}
	return row.ObjectKey, true, nil
}

// UpsertTTSCache inserts or updates a TTS cache record.
func (r *Repository) UpsertTTSCache(
	ctx context.Context, textHash, voice, engine, engineVersion, objectKey string,
) error {
	_, err := r.queries.UpsertTTSCache(ctx, sqlccontent.UpsertTTSCacheParams{
		TextHash:      textHash,
		Voice:         voice,
		Engine:        engine,
		EngineVersion: engineVersion,
		ObjectKey:     objectKey,
	})
	if err != nil {
		return fmt.Errorf("upsert tts cache: %w", err)
	}
	return nil
}

// CreateTaxonomy inserts a new taxonomy node.
func (r *Repository) CreateTaxonomy(
	ctx context.Context,
	id uuid.UUID,
	namespace, code, label string,
	parentID *uuid.UUID,
	description string,
	cefrLevel *string,
	position int,
	deprecatedAt *time.Time,
) (domain.Taxonomy, error) {
	row, err := r.queries.CreateTaxonomy(ctx, sqlccontent.CreateTaxonomyParams{
		ID:           id,
		Namespace:    namespace,
		Code:         code,
		Label:        label,
		ParentID:     parentID,
		Description:  description,
		CefrLevel:    cefrLevel,
		Position:     boundedPosition(position),
		DeprecatedAt: deprecatedAt,
	})
	if err != nil {
		return domain.Taxonomy{}, fmt.Errorf("create taxonomy: %w", err)
	}
	return toDomainTaxonomy(row), nil
}

// GetTaxonomyByID retrieves a taxonomy node by ID.
func (r *Repository) GetTaxonomyByID(ctx context.Context, id uuid.UUID) (domain.Taxonomy, error) {
	row, err := r.queries.GetTaxonomyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Taxonomy{}, domain.ErrTaxonomyNodeNotFound
		}
		return domain.Taxonomy{}, fmt.Errorf("get taxonomy by id: %w", err)
	}
	return toDomainTaxonomy(row), nil
}

// ListContentItemIDsForTaxonomy returns the items tagged with a taxonomy node.
func (r *Repository) ListContentItemIDsForTaxonomy(ctx context.Context, taxonomyID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := r.queries.ListContentItemIDsForTaxonomy(ctx, taxonomyID)
	if err != nil {
		return nil, fmt.Errorf("list content items for taxonomy: %w", err)
	}
	return ids, nil
}

// GetTaxonomyByCode retrieves the one taxonomy node carrying this code.
//
// A code is unique per namespace, not globally: nothing stops `skill.LISTENING`
// and `pattern.LISTENING` both existing, and the spine is designed so that is a
// natural thing to add. This used to answer whichever row Postgres reached
// first, which made every read of an ambiguous code silently wrong and not even
// consistently so. It now says which namespaces claim the code and asks the
// caller to pick one.
func (r *Repository) GetTaxonomyByCode(ctx context.Context, code string) (domain.Taxonomy, error) {
	rows, err := r.queries.ListTaxonomiesByCode(ctx, code)
	if err != nil {
		return domain.Taxonomy{}, fmt.Errorf("get taxonomy by code: %w", err)
	}
	switch len(rows) {
	case 0:
		return domain.Taxonomy{}, domain.ErrTaxonomyNodeNotFound
	case 1:
		return toDomainTaxonomy(rows[0]), nil
	default:
		return domain.Taxonomy{}, domain.AmbiguousTaxonomyCode(code, rows[0].Namespace, rows[1].Namespace)
	}
}

// ListTaxonomiesFiltered retrieves a paginated slice of taxonomy nodes matching filters.
func (r *Repository) ListTaxonomiesFiltered(
	ctx context.Context,
	namespace, cefrLevel *string,
	parentID *uuid.UUID,
	query *string,
	includeDeprecated bool,
	limit, offset int32,
) ([]domain.Taxonomy, int64, error) {
	count, err := r.queries.CountTaxonomiesFiltered(ctx, sqlccontent.CountTaxonomiesFilteredParams{
		Namespace:         namespace,
		CefrLevel:         cefrLevel,
		ParentID:          parentID,
		Query:             query,
		IncludeDeprecated: includeDeprecated,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count taxonomies filtered: %w", err)
	}

	rows, err := r.queries.ListTaxonomiesFiltered(ctx, sqlccontent.ListTaxonomiesFilteredParams{
		Namespace:         namespace,
		CefrLevel:         cefrLevel,
		ParentID:          parentID,
		Query:             query,
		IncludeDeprecated: includeDeprecated,
		ResultLimit:       limit,
		ResultOffset:      offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list taxonomies filtered: %w", err)
	}

	items := make([]domain.Taxonomy, len(rows))
	for i, row := range rows {
		items[i] = toDomainTaxonomy(row)
	}
	return items, count, nil
}

// UpdateTaxonomy updates mutable fields of a taxonomy node.
func (r *Repository) UpdateTaxonomy(
	ctx context.Context,
	id uuid.UUID,
	label, description *string,
	cefrLevel *string, setCEFR bool,
	parentID *uuid.UUID, setParent bool,
	position *int,
	deprecatedAt *time.Time, setDeprecated bool,
) (domain.Taxonomy, error) {
	var pos32 *int32
	if position != nil {
		v := boundedPosition(*position)
		pos32 = &v
	}
	row, err := r.queries.UpdateTaxonomy(ctx, sqlccontent.UpdateTaxonomyParams{
		ID:              id,
		Label:           label,
		Description:     description,
		CefrLevel:       cefrLevel,
		SetCefrLevel:    setCEFR,
		ParentID:        parentID,
		SetParentID:     setParent,
		Position:        pos32,
		DeprecatedAt:    deprecatedAt,
		SetDeprecatedAt: setDeprecated,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Taxonomy{}, domain.ErrTaxonomyNodeNotFound
		}
		return domain.Taxonomy{}, fmt.Errorf("update taxonomy: %w", err)
	}
	return toDomainTaxonomy(row), nil
}

// DeleteTaxonomy deletes a taxonomy node.
func (r *Repository) DeleteTaxonomy(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteTaxonomy(ctx, id)
	if err != nil {
		return fmt.Errorf("delete taxonomy: %w", err)
	}
	return nil
}

// ListPrerequisitesForNode returns nodes that nodeID requires.
func (r *Repository) ListPrerequisitesForNode(ctx context.Context, nodeID uuid.UUID) ([]domain.Taxonomy, error) {
	rows, err := r.queries.ListPrerequisitesForNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list prerequisites: %w", err)
	}
	items := make([]domain.Taxonomy, len(rows))
	for i, row := range rows {
		items[i] = toDomainTaxonomy(row)
	}
	return items, nil
}

// ListDependantsForNode returns nodes that require nodeID.
func (r *Repository) ListDependantsForNode(ctx context.Context, nodeID uuid.UUID) ([]domain.Taxonomy, error) {
	rows, err := r.queries.ListDependantsForNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list dependants: %w", err)
	}
	items := make([]domain.Taxonomy, len(rows))
	for i, row := range rows {
		items[i] = toDomainTaxonomy(row)
	}
	return items, nil
}

// ListAllPrerequisiteEdgesInNamespace returns all prerequisite edges between nodes in a namespace.
func (r *Repository) ListAllPrerequisiteEdgesInNamespace(
	ctx context.Context, namespace string,
) ([]domain.PrerequisiteEdge, error) {
	rows, err := r.queries.ListAllPrerequisiteEdgesInNamespace(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list prerequisite edges in namespace: %w", err)
	}
	edges := make([]domain.PrerequisiteEdge, len(rows))
	for i, row := range rows {
		edges[i] = domain.PrerequisiteEdge{
			NodeID:         row.NodeID,
			RequiresNodeID: row.RequiresNodeID,
		}
	}
	return edges, nil
}

// ListAllTaxonomiesInNamespace returns all non-deprecated taxonomy nodes in a namespace.
func (r *Repository) ListAllTaxonomiesInNamespace(ctx context.Context, namespace string) ([]domain.Taxonomy, error) {
	rows, err := r.queries.ListAllTaxonomiesInNamespace(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list all taxonomies in namespace: %w", err)
	}
	items := make([]domain.Taxonomy, len(rows))
	for i, row := range rows {
		items[i] = toDomainTaxonomy(row)
	}
	return items, nil
}

// ReplacePrerequisites deletes old prerequisites for nodeID and inserts the new ones.
func (r *Repository) ReplacePrerequisites(ctx context.Context, nodeID uuid.UUID, requiresNodeIDs []uuid.UUID) error {
	if err := r.queries.DeletePrerequisitesForNode(ctx, nodeID); err != nil {
		return fmt.Errorf("delete prerequisites: %w", err)
	}
	for _, reqID := range requiresNodeIDs {
		if err := r.queries.InsertPrerequisiteEdge(ctx, sqlccontent.InsertPrerequisiteEdgeParams{
			NodeID:         nodeID,
			RequiresNodeID: reqID,
		}); err != nil {
			return fmt.Errorf("insert prerequisite edge: %w", err)
		}
	}
	return nil
}

// CountTaggedContentByKindForTaxonomy returns item counts grouped by kind tagged to taxonomyID.
func (r *Repository) CountTaggedContentByKindForTaxonomy(
	ctx context.Context, taxonomyID uuid.UUID,
) (map[string]int, error) {
	rows, err := r.queries.CountTaggedContentByKindForTaxonomy(ctx, taxonomyID)
	if err != nil {
		return nil, fmt.Errorf("count tagged content by kind: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.Kind] = int(row.ItemCount)
	}
	return counts, nil
}

// GetPublishedTopicBodyByTaxonomyID returns the body of a published foundation_topic tagged to taxonomyID.
func (r *Repository) GetPublishedTopicBodyByTaxonomyID(
	ctx context.Context, taxonomyID uuid.UUID,
) ([]byte, bool, error) {
	body, err := r.queries.GetPublishedTopicBodyByTaxonomyID(ctx, taxonomyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get published topic body: %w", err)
	}
	return body, true, nil
}

// ListReviewQueue returns drafts produced by the generator, oldest first.
func (r *Repository) ListReviewQueue(
	ctx context.Context, filter domain.ReviewQueueFilter,
) ([]domain.ReviewQueueItem, error) {
	rows, err := r.queries.ListReviewQueue(ctx, sqlccontent.ListReviewQueueParams{
		Purpose:      filter.Purpose,
		Kind:         filter.Kind,
		CefrLevel:    filter.CEFRLevel,
		NodeCode:     filter.NodeCode,
		Batch:        filter.Batch,
		ResultOffset: domain.NormaliseOffset(filter.Offset),
		ResultLimit:  domain.NormaliseLimit(filter.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	res := make([]domain.ReviewQueueItem, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.ReviewQueueItem{
			ID:        row.ID,
			ItemID:    row.ItemID,
			Slug:      row.Slug,
			Kind:      row.Kind,
			CEFRLevel: row.CefrLevel,
			Status:    domain.AuthoringStatus(row.Status),
			Body:      row.Body,
			CreatedAt: row.CreatedAt,
			NodeCodes: toNodeCodes(row.NodeCodes),
		})
	}
	return res, nil
}

// CountReviewQueue counts total items matching review queue filter.
func (r *Repository) CountReviewQueue(
	ctx context.Context, filter domain.ReviewQueueFilter,
) (int64, error) {
	count, err := r.queries.CountReviewQueue(ctx, sqlccontent.CountReviewQueueParams{
		Purpose:   filter.Purpose,
		Kind:      filter.Kind,
		CefrLevel: filter.CEFRLevel,
		NodeCode:  filter.NodeCode,
		Batch:     filter.Batch,
	})
	if err != nil {
		return 0, fmt.Errorf("count review queue: %w", err)
	}
	return count, nil
}

// ListReviewBatches returns one row per generation run awaiting review, oldest
// first (WO 22 Stage A.4).
func (r *Repository) ListReviewBatches(ctx context.Context, limit, offset int) ([]domain.ReviewBatch, error) {
	rows, err := r.queries.ListReviewBatches(ctx, sqlccontent.ListReviewBatchesParams{
		ResultOffset: domain.NormaliseOffset(offset),
		ResultLimit:  domain.NormaliseLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list review batches: %w", err)
	}
	res := make([]domain.ReviewBatch, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.ReviewBatch{
			Batch:     row.Batch,
			ItemCount: row.ItemCount,
			Kinds:     row.Kinds,
			CreatedAt: row.CreatedAt,
		})
	}
	return res, nil
}

// CountReviewBatches counts the distinct generation runs awaiting review.
func (r *Repository) CountReviewBatches(ctx context.Context) (int64, error) {
	count, err := r.queries.CountReviewBatches(ctx)
	if err != nil {
		return 0, fmt.Errorf("count review batches: %w", err)
	}
	return count, nil
}

// ListReviewBatchVersionIDs returns every unpublished version of one batch,
// unbounded so a batch approval is all-or-nothing.
func (r *Repository) ListReviewBatchVersionIDs(ctx context.Context, batch string) ([]uuid.UUID, error) {
	ids, err := r.queries.ListReviewBatchVersionIDs(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("list review batch version ids: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// CountPendingBatchDays counts the distinct days whose batches under a prefix
// still hold unreviewed drafts.
func (r *Repository) CountPendingBatchDays(ctx context.Context, prefix string) (int64, error) {
	count, err := r.queries.CountPendingBatchDays(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("count pending batch days: %w", err)
	}
	return count, nil
}

// CountAutoPublishedOn counts the items a verifier published on a day.
func (r *Repository) CountAutoPublishedOn(ctx context.Context, day time.Time) (int64, error) {
	count, err := r.queries.CountAutoPublishedOn(ctx, day)
	if err != nil {
		return 0, fmt.Errorf("count auto-published versions: %w", err)
	}
	return count, nil
}

// ListAutoPublishedOn draws up to sampleSize auto-published versions of a day.
func (r *Repository) ListAutoPublishedOn(
	ctx context.Context, day time.Time, sampleSize int,
) ([]uuid.UUID, error) {
	if sampleSize <= 0 {
		return []uuid.UUID{}, nil
	}
	// Clamped before the narrowing conversion: sampleSize is derived from a row
	// count, and the driver takes an int32.
	if sampleSize > math.MaxInt32 {
		sampleSize = math.MaxInt32
	}
	ids, err := r.queries.ListAutoPublishedOn(ctx, sqlccontent.ListAutoPublishedOnParams{
		Day:        day,
		SampleSize: int32(sampleSize),
	})
	if err != nil {
		return nil, fmt.Errorf("draw auto-published versions: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// InsertReviewSample queues one version for a person to spot-check.
func (r *Repository) InsertReviewSample(
	ctx context.Context, versionID uuid.UUID, batch string, sampledOn time.Time,
) error {
	if err := r.queries.InsertReviewSample(ctx, sqlccontent.InsertReviewSampleParams{
		VersionID: versionID,
		Batch:     batch,
		SampledOn: pgtype.Date{Time: sampledOn, Valid: true},
	}); err != nil {
		return fmt.Errorf("insert review sample: %w", err)
	}
	return nil
}

// ListOpenReviewSamples lists the samples still waiting for a person.
func (r *Repository) ListOpenReviewSamples(
	ctx context.Context, limit, offset int,
) ([]domain.ReviewSample, error) {
	rows, err := r.queries.ListOpenReviewSamples(ctx, sqlccontent.ListOpenReviewSamplesParams{
		ResultLimit:  domain.NormaliseLimit(limit),
		ResultOffset: domain.NormaliseOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list open review samples: %w", err)
	}
	res := make([]domain.ReviewSample, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.ReviewSample{
			VersionID: row.VersionID,
			Batch:     row.Batch,
			Kind:      row.Kind,
			CEFRLevel: row.CefrLevel,
			SampledOn: row.SampledOn.Time,
			CreatedAt: row.CreatedAt,
		})
	}
	return res, nil
}

// CountOpenReviewSamples counts the samples still waiting for a person.
func (r *Repository) CountOpenReviewSamples(ctx context.Context) (int64, error) {
	count, err := r.queries.CountOpenReviewSamples(ctx)
	if err != nil {
		return 0, fmt.Errorf("count open review samples: %w", err)
	}
	return count, nil
}

// DecideReviewSample records a person's decision on one sample.
func (r *Repository) DecideReviewSample(
	ctx context.Context, versionID uuid.UUID, decision domain.SampleDecision, note *string, decidedBy uuid.UUID,
) error {
	var by *uuid.UUID
	if decidedBy != uuid.Nil {
		by = &decidedBy
	}
	if err := r.queries.DecideReviewSample(ctx, sqlccontent.DecideReviewSampleParams{
		VersionID: versionID,
		Decision:  string(decision),
		Note:      note,
		DecidedBy: by,
	}); err != nil {
		return fmt.Errorf("decide review sample: %w", err)
	}
	return nil
}

// IsAutoPublishedVersion reports whether a version was published by the
// independent verifier.
func (r *Repository) IsAutoPublishedVersion(ctx context.Context, versionID uuid.UUID) (bool, error) {
	auto, err := r.queries.IsAutoPublishedVersion(ctx, versionID)
	if err != nil {
		return false, fmt.Errorf("check auto-published version: %w", err)
	}
	return auto, nil
}

func toNodeCodes(v interface{}) []string {
	if v == nil {
		return []string{}
	}
	switch codes := v.(type) {
	case []string:
		return codes
	case []interface{}:
		res := make([]string, 0, len(codes))
		for _, c := range codes {
			if s, ok := c.(string); ok {
				res = append(res, s)
			}
		}
		return res
	default:
		return []string{}
	}
}

// boundedPosition narrows a position to the width of the column that stores it.
//
// content.taxonomies.position is an `integer`, and an unchecked int -> int32
// conversion wraps: a position past MaxInt32 would land as a negative number and
// sort the node to the front of its strand. The service rejects such a value
// first; this is the conversion refusing to be the place it goes wrong.
func boundedPosition(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}
