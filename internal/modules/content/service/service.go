// Package service implements the business logic and authoring workflows for the content module.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository specifies the persistence interface required by the service.
type Repository interface {
	CreateItem(
		ctx context.Context,
		id uuid.UUID,
		kind, slug string,
		status domain.AuthoringStatus,
		ownerID uuid.UUID,
	) (domain.Item, error)
	GetItemByID(ctx context.Context, id uuid.UUID) (domain.Item, error)
	GetItemBySlug(ctx context.Context, slug string) (domain.Item, error)
	UpdateItemStatus(ctx context.Context, id uuid.UUID, status domain.AuthoringStatus) (domain.Item, error)
	UpdateItemCurrentVersion(ctx context.Context, id uuid.UUID, currentVersionID *uuid.UUID) (domain.Item, error)
	ListItemsByOwner(ctx context.Context, ownerID uuid.UUID, limit int32) ([]domain.Item, error)
	DeleteItem(ctx context.Context, id uuid.UUID) error

	CreateVersion(
		ctx context.Context,
		id, itemID uuid.UUID,
		version int,
		kind string,
		body []byte,
		cefrLevel string,
		status domain.AuthoringStatus,
		mediaRefs []string,
		publishedAt *time.Time,
	) (domain.Version, error)
	GetVersionByID(ctx context.Context, id uuid.UUID) (domain.Version, error)
	GetVersionByItemAndVersion(ctx context.Context, itemID uuid.UUID, version int) (domain.Version, error)
	GetDraftVersionByItemID(ctx context.Context, itemID uuid.UUID) (domain.Version, error)
	ListVersionsByItemID(ctx context.Context, itemID uuid.UUID) ([]domain.Version, error)
	GetManyVersionsByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Version, error)
	UpdateVersionDraft(
		ctx context.Context,
		id uuid.UUID,
		kind string,
		body []byte,
		cefrLevel string,
		mediaRefs []string,
		status domain.AuthoringStatus,
	) (domain.Version, error)
	PublishVersion(ctx context.Context, id uuid.UUID) (domain.Version, error)
	GetLatestVersionNumberByItemID(ctx context.Context, itemID uuid.UUID) (int, error)
	GetPublishedVersionBySlug(ctx context.Context, slug string) (domain.Version, error)
	BrowsePublishedVersions(ctx context.Context, kind, cefrLevel *string, limit, offset int32) ([]domain.Version, error)
	CountPublishedVersions(ctx context.Context, kind, cefrLevel *string) (int64, error)

	GetMediaAssetByObjectKey(ctx context.Context, objectKey string) (domain.MediaAsset, error)
	GetMediaAssetsByObjectKeys(ctx context.Context, objectKeys []string) ([]domain.MediaAsset, error)
	CreateMediaAsset(
		ctx context.Context,
		id uuid.UUID,
		objectKey, kind string,
		durationMs *int32,
		checksum *string,
		status domain.MediaStatus,
		byteSize *int64,
		mimeType *string,
	) (domain.MediaAsset, error)
	UpdateMediaAssetStatus(
		ctx context.Context,
		id uuid.UUID,
		status domain.MediaStatus,
		durationMs *int32,
		checksum *string,
		byteSize *int64,
		mimeType *string,
	) (domain.MediaAsset, error)

	CreateReview(
		ctx context.Context,
		id, versionID, reviewerID uuid.UUID,
		decision domain.ReviewDecision,
		comments *string,
	) (domain.Review, error)
	ListReviewsForVersion(ctx context.Context, versionID uuid.UUID) ([]domain.Review, error)

	AddContentTag(ctx context.Context, itemID, taxonomyID uuid.UUID) error
	ClearTagsForContentItem(ctx context.Context, itemID uuid.UUID) error
	ListTagsForContentItem(ctx context.Context, itemID uuid.UUID) ([]domain.Taxonomy, error)
	ListTagsForContentItems(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID][]domain.TaxonomyTag, error)
	GetTaxonomyByNamespaceCode(ctx context.Context, namespace, code string) (domain.Taxonomy, error)

	WithTx(tx pgx.Tx) Repository
}

// OutboxTx matches the transaction interface used by outbox writers.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter represents the outbox event dispatch interface.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// Deps encapsulates dependencies required to instantiate Service.
type Deps struct {
	Pool   dbx.Beginner
	Repo   Repository
	Events EventWriter
	Clock  clock.Clock
	NewID  func() uuid.UUID
}

// Service orchestrates the content module's use cases and authoring state machine.
type Service struct {
	pool   dbx.Beginner
	repo   Repository
	events EventWriter
	clock  clock.Clock
	newID  func() uuid.UUID
}

// New creates a new content Service.
func New(deps Deps) *Service {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	idFn := deps.NewID
	if idFn == nil {
		idFn = uuid.New
	}
	return &Service{
		pool:   deps.Pool,
		repo:   deps.Repo,
		events: deps.Events,
		clock:  clk,
		newID:  idFn,
	}
}

// Ensure Service implements contract.Reader.
var _ contract.Reader = (*Service)(nil)

// =========================================================================
// Reader Implementation (contract.Reader)
// =========================================================================

// GetVersion retrieves a single content version by ID.
// Note: An archived item's version remains readable by direct ID lookup (archive-mid-session trap).
func (s *Service) GetVersion(ctx context.Context, id uuid.UUID) (*contract.Version, error) {
	v, err := s.repo.GetVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	tags, err := s.repo.ListTagsForContentItem(ctx, v.ItemID)
	if err != nil {
		return nil, err
	}
	tagStrings := make([]string, len(tags))
	for i, t := range tags {
		tagStrings[i] = t.Code
	}

	return toContractVersion(v, tagStrings), nil
}

// GetManyVersions retrieves multiple content versions in ONE single query,
// avoiding N+1 queries during lesson rendering.
func (s *Service) GetManyVersions(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*contract.Version, error) {
	result := make(map[uuid.UUID]*contract.Version, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	// Single query with = ANY(@ids::uuid[])
	versions, err := s.repo.GetManyVersionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Extract unique item IDs for batch tag resolution
	itemIDsMap := make(map[uuid.UUID]struct{}, len(versions))
	for _, v := range versions {
		itemIDsMap[v.ItemID] = struct{}{}
	}
	itemIDs := make([]uuid.UUID, 0, len(itemIDsMap))
	for itemID := range itemIDsMap {
		itemIDs = append(itemIDs, itemID)
	}

	tagsByItem, err := s.repo.ListTagsForContentItems(ctx, itemIDs)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		tags := tagsByItem[v.ItemID]
		tagStrings := make([]string, len(tags))
		for i, t := range tags {
			tagStrings[i] = t.Code
		}
		result[v.ID] = toContractVersion(v, tagStrings)
	}

	return result, nil
}

// Browse lists published content versions according to filter parameters.
func (s *Service) Browse(ctx context.Context, filter contract.BrowseFilter) ([]*contract.Version, int, error) {
	// #nosec G115 -- limit and offset from user filters are bounded within positive int32 ranges
	limit := int32(filter.Limit)
	if limit <= 0 {
		limit = 20
	}
	// #nosec G115 -- limit and offset from user filters are bounded within positive int32 ranges
	offset := int32(filter.Offset)
	if offset < 0 {
		offset = 0
	}

	versions, err := s.repo.BrowsePublishedVersions(ctx, filter.Kind, filter.CEFRLevel, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountPublishedVersions(ctx, filter.Kind, filter.CEFRLevel)
	if err != nil {
		return nil, 0, err
	}

	itemIDsMap := make(map[uuid.UUID]struct{}, len(versions))
	for _, v := range versions {
		itemIDsMap[v.ItemID] = struct{}{}
	}
	itemIDs := make([]uuid.UUID, 0, len(itemIDsMap))
	for itemID := range itemIDsMap {
		itemIDs = append(itemIDs, itemID)
	}

	tagsByItem, err := s.repo.ListTagsForContentItems(ctx, itemIDs)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*contract.Version, len(versions))
	for i, v := range versions {
		tags := tagsByItem[v.ItemID]
		tagStrings := make([]string, len(tags))
		for j, t := range tags {
			tagStrings[j] = t.Code
		}
		result[i] = toContractVersion(v, tagStrings)
	}

	return result, int(total), nil
}

// GetPublishedVersionBySlug fetches a published content version by matching the item's slug.
func (s *Service) GetPublishedVersionBySlug(ctx context.Context, slug string) (*contract.Version, error) {
	v, err := s.repo.GetPublishedVersionBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	tags, err := s.repo.ListTagsForContentItem(ctx, v.ItemID)
	if err != nil {
		return nil, err
	}
	tagStrings := make([]string, len(tags))
	for i, t := range tags {
		tagStrings[i] = t.Code
	}

	return toContractVersion(v, tagStrings), nil
}

// =========================================================================
// Authoring & Workflow Operations
// =========================================================================

// CreateItemRequest parameters for creating a new material item.
type CreateItemRequest struct {
	Kind      string
	Slug      string
	CEFRLevel string
	Body      json.RawMessage
	Tags      []string
}

// CreateItem creates a draft content item and its initial version 1 snapshot.
func (s *Service) CreateItem(
	ctx context.Context,
	actorID uuid.UUID,
	req CreateItemRequest,
) (domain.Item, domain.Version, error) {
	if err := domain.ValidateSlug(req.Slug); err != nil {
		return domain.Item{}, domain.Version{}, err
	}
	if err := domain.ValidateKind(req.Kind); err != nil {
		return domain.Item{}, domain.Version{}, err
	}
	if err := domain.ValidateCEFRLevel(req.CEFRLevel); err != nil {
		return domain.Item{}, domain.Version{}, err
	}

	bodyBytes := []byte(req.Body)
	if len(bodyBytes) == 0 {
		bodyBytes = []byte("{}")
	}

	itemID := s.newID()
	versionID := s.newID()

	var item domain.Item
	var version domain.Version

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		var err error
		item, err = txRepo.CreateItem(ctx, itemID, req.Kind, req.Slug, domain.StatusDraft, actorID)
		if err != nil {
			return err
		}

		version, err = txRepo.CreateVersion(
			ctx,
			versionID,
			itemID,
			1,
			req.Kind,
			bodyBytes,
			req.CEFRLevel,
			domain.StatusDraft,
			[]string{},
			nil,
		)
		if err != nil {
			return err
		}

		item, err = txRepo.UpdateItemCurrentVersion(ctx, itemID, &versionID)
		if err != nil {
			return err
		}

		// Handle tags
		for _, tagStr := range req.Tags {
			tax, err := txRepo.GetTaxonomyByNamespaceCode(ctx, "topic", tagStr)
			if err == nil {
				_ = txRepo.AddContentTag(ctx, itemID, tax.ID)
			}
		}

		return nil
	})

	if err != nil {
		return domain.Item{}, domain.Version{}, err
	}

	return item, version, nil
}

// UpdateDraftRequest parameters for updating an in-progress draft.
type UpdateDraftRequest struct {
	CEFRLevel *string
	Body      json.RawMessage
	Tags      []string
}

func (s *Service) createNewDraftFromPublished(
	ctx context.Context,
	txRepo Repository,
	item domain.Item,
	req UpdateDraftRequest,
) (domain.Version, error) {
	latestVer, err := txRepo.GetLatestVersionNumberByItemID(ctx, item.ID)
	if err != nil {
		return domain.Version{}, err
	}

	newVerID := s.newID()
	cefr := "B1"
	if req.CEFRLevel != nil {
		cefr = *req.CEFRLevel
	}

	bodyBytes := []byte(req.Body)
	if len(bodyBytes) == 0 {
		bodyBytes = []byte("{}")
	}

	v, err := txRepo.CreateVersion(
		ctx,
		newVerID,
		item.ID,
		latestVer+1,
		item.Kind,
		bodyBytes,
		cefr,
		domain.StatusDraft,
		[]string{},
		nil,
	)
	if err != nil {
		return domain.Version{}, err
	}

	_, err = txRepo.UpdateItemStatus(ctx, item.ID, domain.StatusDraft)
	if err != nil {
		return domain.Version{}, err
	}
	_, err = txRepo.UpdateItemCurrentVersion(ctx, item.ID, &newVerID)
	if err != nil {
		return domain.Version{}, err
	}

	return v, nil
}

func (s *Service) updateExistingDraftVersion(
	ctx context.Context,
	txRepo Repository,
	itemID uuid.UUID,
	item domain.Item,
	draftVersion domain.Version,
	req UpdateDraftRequest,
) (domain.Version, error) {
	cefr := draftVersion.CEFRLevel
	if req.CEFRLevel != nil {
		cefr = *req.CEFRLevel
	}

	bodyBytes := []byte(req.Body)
	if len(bodyBytes) == 0 {
		bodyBytes = []byte(draftVersion.Body)
	}

	if err := domain.ValidateTransition(draftVersion.Status, domain.StatusDraft); err != nil {
		return domain.Version{}, err
	}

	updatedVersion, err := txRepo.UpdateVersionDraft(
		ctx,
		draftVersion.ID,
		draftVersion.Kind,
		bodyBytes,
		cefr,
		draftVersion.MediaRefs,
		domain.StatusDraft,
	)
	if err != nil {
		return domain.Version{}, err
	}

	if item.Status != domain.StatusDraft {
		_, err = txRepo.UpdateItemStatus(ctx, itemID, domain.StatusDraft)
		if err != nil {
			return domain.Version{}, err
		}
	}

	if req.Tags != nil {
		_ = txRepo.ClearTagsForContentItem(ctx, itemID)
		for _, tagStr := range req.Tags {
			tax, err := txRepo.GetTaxonomyByNamespaceCode(ctx, "topic", tagStr)
			if err == nil {
				_ = txRepo.AddContentTag(ctx, itemID, tax.ID)
			}
		}
	}

	return updatedVersion, nil
}

// UpdateDraft modifies an existing draft or initializes a new draft version if previous version is published.
func (s *Service) UpdateDraft(
	ctx context.Context,
	actorID, itemID uuid.UUID,
	req UpdateDraftRequest,
) (domain.Version, error) {
	_ = actorID
	if req.CEFRLevel != nil {
		if err := domain.ValidateCEFRLevel(*req.CEFRLevel); err != nil {
			return domain.Version{}, err
		}
	}

	var updatedVersion domain.Version

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return err
		}

		draftVersion, err := txRepo.GetDraftVersionByItemID(ctx, itemID)
		if err != nil {
			if errors.Is(err, domain.ErrVersionNotFound) {
				updatedVersion, err = s.createNewDraftFromPublished(ctx, txRepo, item, req)
				return err
			}
			return err
		}

		updatedVersion, err = s.updateExistingDraftVersion(ctx, txRepo, itemID, item, draftVersion, req)
		return err
	})

	if err != nil {
		return domain.Version{}, err
	}

	return updatedVersion, nil
}

// SubmitForReview transitions a draft version to in_review.
func (s *Service) SubmitForReview(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error) {
	_ = actorID
	var version domain.Version

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		_, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return err
		}

		draftVersion, err := txRepo.GetDraftVersionByItemID(ctx, itemID)
		if err != nil {
			return err
		}

		if err := domain.ValidateTransition(draftVersion.Status, domain.StatusInReview); err != nil {
			return err
		}

		version, err = txRepo.UpdateVersionDraft(
			ctx,
			draftVersion.ID,
			draftVersion.Kind,
			[]byte(draftVersion.Body),
			draftVersion.CEFRLevel,
			draftVersion.MediaRefs,
			domain.StatusInReview,
		)
		if err != nil {
			return err
		}

		_, err = txRepo.UpdateItemStatus(ctx, itemID, domain.StatusInReview)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.Version{}, err
	}

	return version, nil
}

// ReviewDecisionRequest encapsulates review parameters.
type ReviewDecisionRequest struct {
	Decision domain.ReviewDecision
	Comments *string
}

// Review records an editorial decision on a version in review.
// Enforces BR-CONTENT-03: An author cannot approve their own version.
func (s *Service) Review(
	ctx context.Context,
	reviewerID, itemID uuid.UUID,
	req ReviewDecisionRequest,
) (domain.Version, error) {
	var version domain.Version

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return err
		}

		// BR-CONTENT-03: Self-approval is forbidden
		if item.OwnerID == reviewerID {
			return domain.ErrSelfApprovalForbidden
		}

		draftVersion, err := txRepo.GetDraftVersionByItemID(ctx, itemID)
		if err != nil {
			return err
		}

		var nextStatus domain.AuthoringStatus
		switch req.Decision {
		case domain.ReviewDecisionApproved:
			nextStatus = domain.StatusApproved
		case domain.ReviewDecisionChangesRequested:
			nextStatus = domain.StatusDraft
		default:
			return domain.ErrInvalidReviewDecision
		}

		if err := domain.ValidateTransition(draftVersion.Status, nextStatus); err != nil {
			return err
		}

		version, err = txRepo.UpdateVersionDraft(
			ctx,
			draftVersion.ID,
			draftVersion.Kind,
			[]byte(draftVersion.Body),
			draftVersion.CEFRLevel,
			draftVersion.MediaRefs,
			nextStatus,
		)
		if err != nil {
			return err
		}

		_, err = txRepo.UpdateItemStatus(ctx, itemID, nextStatus)
		if err != nil {
			return err
		}

		// Record review audit entry
		reviewID := s.newID()
		_, err = txRepo.CreateReview(ctx, reviewID, draftVersion.ID, reviewerID, req.Decision, req.Comments)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return domain.Version{}, err
	}

	return version, nil
}

func (s *Service) verifyMediaAssetsReady(
	ctx context.Context,
	txRepo Repository,
	mediaRefs []string,
) error {
	if len(mediaRefs) == 0 {
		return nil
	}
	assets, err := txRepo.GetMediaAssetsByObjectKeys(ctx, mediaRefs)
	if err != nil {
		return err
	}
	if len(assets) != len(mediaRefs) {
		return domain.ErrMediaNotReady.WithInternal("one or more referenced media assets do not exist")
	}
	for _, asset := range assets {
		if asset.Status != domain.MediaStatusReady {
			return domain.ErrMediaNotReady.WithInternal(
				fmt.Sprintf(
					"media asset %s is in status %q, want %q",
					asset.ObjectKey,
					asset.Status,
					domain.MediaStatusReady,
				),
			)
		}
	}
	return nil
}

// Publish finalizes an approved version, making it immutable and emitting content.published outbox event.
// Enforces BR-CONTENT-04: Publishing is blocked until referenced media assets are ready.
func (s *Service) Publish(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error) {
	_ = actorID
	var publishedVersion domain.Version

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return err
		}

		// Idempotent publish if already published
		if item.Status == domain.StatusPublished && item.CurrentVersionID != nil {
			currVer, err := txRepo.GetVersionByID(ctx, *item.CurrentVersionID)
			if err == nil && currVer.Status == domain.StatusPublished {
				publishedVersion = currVer
				return nil
			}
		}

		draftVersion, err := txRepo.GetDraftVersionByItemID(ctx, itemID)
		if err != nil {
			return err
		}

		if err := domain.ValidateTransition(draftVersion.Status, domain.StatusPublished); err != nil {
			return err
		}

		// BR-CONTENT-04: Verify all media refs are ready
		if err := s.verifyMediaAssetsReady(ctx, txRepo, draftVersion.MediaRefs); err != nil {
			return err
		}

		publishedVersion, err = txRepo.PublishVersion(ctx, draftVersion.ID)
		if err != nil {
			return err
		}

		_, err = txRepo.UpdateItemStatus(ctx, itemID, domain.StatusPublished)
		if err != nil {
			return err
		}

		_, err = txRepo.UpdateItemCurrentVersion(ctx, itemID, &draftVersion.ID)
		if err != nil {
			return err
		}

		// Outbox event: content.published
		if s.events != nil {
			now := s.clock.Now().UTC()
			eventPayload := contract.Published{
				ItemID:     item.ID,
				VersionID:  draftVersion.ID,
				Kind:       draftVersion.Kind,
				CEFRLevel:  draftVersion.CEFRLevel,
				OccurredAt: now,
			}
			_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventContentPublished, eventPayload)
			if err != nil {
				return fmt.Errorf("write outbox event %s: %w", contract.EventContentPublished, err)
			}
		}

		return nil
	})

	if err != nil {
		return domain.Version{}, err
	}

	return publishedVersion, nil
}

// Archive archives a published content item.
// Note: The published version remains readable by direct ID lookup, but is hidden from discovery.
func (s *Service) Archive(ctx context.Context, actorID, itemID uuid.UUID) (domain.Item, error) {
	_ = actorID
	var archivedItem domain.Item

	err := dbx.InTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)

		item, err := txRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return err
		}

		// Idempotent archive if already archived
		if item.Status == domain.StatusArchived {
			archivedItem = item
			return nil
		}

		if err := domain.ValidateTransition(item.Status, domain.StatusArchived); err != nil {
			return err
		}

		archivedItem, err = txRepo.UpdateItemStatus(ctx, itemID, domain.StatusArchived)
		if err != nil {
			return err
		}

		// Outbox event: content.archived
		if s.events != nil && item.CurrentVersionID != nil {
			now := s.clock.Now().UTC()
			eventPayload := contract.Archived{
				ItemID:     item.ID,
				VersionID:  *item.CurrentVersionID,
				OccurredAt: now,
			}
			_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventContentArchived, eventPayload)
			if err != nil {
				return fmt.Errorf("write outbox event %s: %w", contract.EventContentArchived, err)
			}
		}

		return nil
	})

	if err != nil {
		return domain.Item{}, err
	}

	return archivedItem, nil
}

// GetItemByID returns a content item by ID.
func (s *Service) GetItemByID(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	return s.repo.GetItemByID(ctx, id)
}

// GetDraftVersion returns the working draft version for an item.
func (s *Service) GetDraftVersion(ctx context.Context, itemID uuid.UUID) (domain.Version, error) {
	return s.repo.GetDraftVersionByItemID(ctx, itemID)
}

func toContractVersion(v domain.Version, tags []string) *contract.Version {
	var pubAt *time.Time
	if v.PublishedAt != nil {
		t := *v.PublishedAt
		pubAt = &t
	}
	return &contract.Version{
		ID:          v.ID,
		ItemID:      v.ItemID,
		Version:     v.Version,
		Kind:        v.Kind,
		Body:        v.Body,
		CEFRLevel:   v.CEFRLevel,
		MediaRefs:   v.MediaRefs,
		Tags:        tags,
		Status:      string(v.Status),
		PublishedAt: pubAt,
	}
}
