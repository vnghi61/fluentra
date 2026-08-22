package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const testKindWord = "word"

// fakeRepo is an in-memory mock repository implementing service.Repository.
type fakeRepo struct {
	items       map[uuid.UUID]domain.Item
	versions    map[uuid.UUID]domain.Version
	mediaAssets map[string]domain.MediaAsset
	reviews     map[uuid.UUID]domain.Review
	tags        map[uuid.UUID][]domain.TaxonomyTag
	queriesRun  int

	// what the last BrowsePublishedVersions call was given, so a test can
	// assert the clamp reaches the query rather than stopping at the service.
	lastLimit  int32
	lastOffset int32
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		items:       make(map[uuid.UUID]domain.Item),
		versions:    make(map[uuid.UUID]domain.Version),
		mediaAssets: make(map[string]domain.MediaAsset),
		reviews:     make(map[uuid.UUID]domain.Review),
		tags:        make(map[uuid.UUID][]domain.TaxonomyTag),
	}
}

func (f *fakeRepo) WithTx(_ pgx.Tx) service.Repository {
	return f
}

func (f *fakeRepo) CreateItem(
	_ context.Context,
	id uuid.UUID,
	kind, slug string,
	status domain.AuthoringStatus,
	ownerID uuid.UUID,
) (domain.Item, error) {
	for _, it := range f.items {
		if it.Slug == slug {
			return domain.Item{}, domain.ErrSlugAlreadyExists
		}
	}
	item := domain.Item{
		ID:        id,
		Kind:      kind,
		Slug:      slug,
		Status:    status,
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.items[id] = item
	return item, nil
}

func (f *fakeRepo) GetItemByID(_ context.Context, id uuid.UUID) (domain.Item, error) {
	item, ok := f.items[id]
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	return item, nil
}

func (f *fakeRepo) GetItemBySlug(_ context.Context, slug string) (domain.Item, error) {
	for _, item := range f.items {
		if item.Slug == slug {
			return item, nil
		}
	}
	return domain.Item{}, domain.ErrItemNotFound
}

func (f *fakeRepo) UpdateItemStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.AuthoringStatus,
) (domain.Item, error) {
	item, ok := f.items[id]
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	item.Status = status
	item.UpdatedAt = time.Now()
	f.items[id] = item
	return item, nil
}

func (f *fakeRepo) UpdateItemCurrentVersion(
	_ context.Context,
	id uuid.UUID,
	currentVersionID *uuid.UUID,
) (domain.Item, error) {
	item, ok := f.items[id]
	if !ok {
		return domain.Item{}, domain.ErrItemNotFound
	}
	item.CurrentVersionID = currentVersionID
	item.UpdatedAt = time.Now()
	f.items[id] = item
	return item, nil
}

func (f *fakeRepo) ListItemsByOwner(
	_ context.Context,
	ownerID uuid.UUID,
	_ int32,
) ([]domain.Item, error) {
	var list []domain.Item
	for _, item := range f.items {
		if item.OwnerID == ownerID {
			list = append(list, item)
		}
	}
	return list, nil
}

func (f *fakeRepo) DeleteItem(_ context.Context, id uuid.UUID) error {
	delete(f.items, id)
	return nil
}

func (f *fakeRepo) CreateVersion(
	_ context.Context,
	id, itemID uuid.UUID,
	version int,
	kind string,
	body []byte,
	cefrLevel string,
	status domain.AuthoringStatus,
	mediaRefs []string,
	publishedAt *time.Time,
) (domain.Version, error) {
	v := domain.Version{
		ID:          id,
		ItemID:      itemID,
		Version:     version,
		Kind:        kind,
		Body:        json.RawMessage(body),
		CEFRLevel:   cefrLevel,
		Status:      status,
		MediaRefs:   mediaRefs,
		PublishedAt: publishedAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	f.versions[id] = v
	return v, nil
}

func (f *fakeRepo) GetVersionByID(_ context.Context, id uuid.UUID) (domain.Version, error) {
	v, ok := f.versions[id]
	if !ok {
		return domain.Version{}, domain.ErrVersionNotFound
	}
	return v, nil
}

func (f *fakeRepo) GetVersionByItemAndVersion(
	_ context.Context,
	itemID uuid.UUID,
	version int,
) (domain.Version, error) {
	for _, v := range f.versions {
		if v.ItemID == itemID && v.Version == version {
			return v, nil
		}
	}
	return domain.Version{}, domain.ErrVersionNotFound
}

func (f *fakeRepo) GetDraftVersionByItemID(_ context.Context, itemID uuid.UUID) (domain.Version, error) {
	var latest *domain.Version
	for _, v := range f.versions {
		isDraftState := v.Status == domain.StatusDraft ||
			v.Status == domain.StatusInReview ||
			v.Status == domain.StatusApproved
		if v.ItemID == itemID && isDraftState {
			if latest == nil || v.Version > latest.Version {
				vCopy := v
				latest = &vCopy
			}
		}
	}
	if latest == nil {
		return domain.Version{}, domain.ErrVersionNotFound
	}
	return *latest, nil
}

func (f *fakeRepo) ListVersionsByItemID(_ context.Context, itemID uuid.UUID) ([]domain.Version, error) {
	var list []domain.Version
	for _, v := range f.versions {
		if v.ItemID == itemID {
			list = append(list, v)
		}
	}
	return list, nil
}

func (f *fakeRepo) GetManyVersionsByIDs(_ context.Context, ids []uuid.UUID) ([]domain.Version, error) {
	f.queriesRun++
	var list []domain.Version
	for _, id := range ids {
		if v, ok := f.versions[id]; ok {
			list = append(list, v)
		}
	}
	return list, nil
}

func (f *fakeRepo) UpdateVersionDraft(
	_ context.Context,
	id uuid.UUID,
	kind string,
	body []byte,
	cefrLevel string,
	mediaRefs []string,
	status domain.AuthoringStatus,
) (domain.Version, error) {
	v, ok := f.versions[id]
	if !ok {
		return domain.Version{}, domain.ErrVersionNotFound
	}
	v.Kind = kind
	v.Body = json.RawMessage(body)
	v.CEFRLevel = cefrLevel
	v.MediaRefs = mediaRefs
	v.Status = status
	v.UpdatedAt = time.Now()
	f.versions[id] = v
	return v, nil
}

func (f *fakeRepo) PublishVersion(_ context.Context, id uuid.UUID) (domain.Version, error) {
	v, ok := f.versions[id]
	if !ok {
		return domain.Version{}, domain.ErrVersionNotFound
	}
	now := time.Now()
	v.Status = domain.StatusPublished
	v.PublishedAt = &now
	v.UpdatedAt = now
	f.versions[id] = v
	return v, nil
}

func (f *fakeRepo) GetLatestVersionNumberByItemID(_ context.Context, itemID uuid.UUID) (int, error) {
	latest := 0
	for _, v := range f.versions {
		if v.ItemID == itemID && v.Version > latest {
			latest = v.Version
		}
	}
	return latest, nil
}

func (f *fakeRepo) GetPublishedVersionBySlug(_ context.Context, slug string) (domain.Version, error) {
	var foundItem *domain.Item
	for _, it := range f.items {
		if it.Slug == slug && it.Status == domain.StatusPublished {
			itCopy := it
			foundItem = &itCopy
			break
		}
	}
	if foundItem == nil || foundItem.CurrentVersionID == nil {
		return domain.Version{}, domain.ErrContentNotPublished
	}
	v, ok := f.versions[*foundItem.CurrentVersionID]
	if !ok || v.Status != domain.StatusPublished {
		return domain.Version{}, domain.ErrContentNotPublished
	}
	return v, nil
}

func (f *fakeRepo) BrowsePublishedVersions(
	_ context.Context,
	kind, cefrLevel *string,
	limit, offset int32,
) ([]domain.Version, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	return f.published(kind, cefrLevel), nil
}

// published is the filter both the browse and the count share. CountPublished-
// Versions goes through it rather than through BrowsePublishedVersions so that
// the recorded paging arguments stay the ones Browse actually asked for.
func (f *fakeRepo) published(kind, cefrLevel *string) []domain.Version {
	var list []domain.Version
	for _, it := range f.items {
		if it.Status != domain.StatusPublished || it.CurrentVersionID == nil {
			continue
		}
		v, ok := f.versions[*it.CurrentVersionID]
		if !ok || v.Status != domain.StatusPublished {
			continue
		}
		if kind != nil && v.Kind != *kind {
			continue
		}
		if cefrLevel != nil && v.CEFRLevel != *cefrLevel {
			continue
		}
		list = append(list, v)
	}
	return list
}

func (f *fakeRepo) CountPublishedVersions(_ context.Context, kind, cefrLevel *string) (int64, error) {
	return int64(len(f.published(kind, cefrLevel))), nil
}

func (f *fakeRepo) GetMediaAssetByObjectKey(_ context.Context, objectKey string) (domain.MediaAsset, error) {
	asset, ok := f.mediaAssets[objectKey]
	if !ok {
		return domain.MediaAsset{}, fmt.Errorf("media asset not found")
	}
	return asset, nil
}

func (f *fakeRepo) GetMediaAssetsByObjectKeys(_ context.Context, objectKeys []string) ([]domain.MediaAsset, error) {
	var list []domain.MediaAsset
	for _, key := range objectKeys {
		if asset, ok := f.mediaAssets[key]; ok {
			list = append(list, asset)
		}
	}
	return list, nil
}

func (f *fakeRepo) CreateMediaAsset(
	_ context.Context,
	id uuid.UUID,
	objectKey, kind string,
	durationMs *int32,
	checksum *string,
	status domain.MediaStatus,
	byteSize *int64,
	mimeType *string,
) (domain.MediaAsset, error) {
	asset := domain.MediaAsset{
		ID:         id,
		ObjectKey:  objectKey,
		Kind:       kind,
		DurationMS: durationMs,
		Checksum:   checksum,
		Status:     status,
		ByteSize:   byteSize,
		MIMEType:   mimeType,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	f.mediaAssets[objectKey] = asset
	return asset, nil
}

func (f *fakeRepo) UpdateMediaAssetStatus(
	_ context.Context,
	id uuid.UUID,
	status domain.MediaStatus,
	_ *int32,
	_ *string,
	_ *int64,
	_ *string,
) (domain.MediaAsset, error) {
	for key, asset := range f.mediaAssets {
		if asset.ID == id {
			asset.Status = status
			asset.UpdatedAt = time.Now()
			f.mediaAssets[key] = asset
			return asset, nil
		}
	}
	return domain.MediaAsset{}, fmt.Errorf("media asset not found")
}

func (f *fakeRepo) CreateReview(
	_ context.Context,
	id, versionID, reviewerID uuid.UUID,
	decision domain.ReviewDecision,
	comments *string,
) (domain.Review, error) {
	r := domain.Review{
		ID:         id,
		VersionID:  versionID,
		ReviewerID: reviewerID,
		Decision:   decision,
		Comments:   comments,
		CreatedAt:  time.Now(),
	}
	f.reviews[id] = r
	return r, nil
}

func (f *fakeRepo) ListReviewsForVersion(_ context.Context, versionID uuid.UUID) ([]domain.Review, error) {
	var list []domain.Review
	for _, r := range f.reviews {
		if r.VersionID == versionID {
			list = append(list, r)
		}
	}
	return list, nil
}

func (f *fakeRepo) AddContentTag(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (f *fakeRepo) ClearTagsForContentItem(_ context.Context, itemID uuid.UUID) error {
	delete(f.tags, itemID)
	return nil
}

func (f *fakeRepo) ListTagsForContentItem(_ context.Context, _ uuid.UUID) ([]domain.Taxonomy, error) {
	f.queriesRun++
	return nil, nil
}

func (f *fakeRepo) ListTagsForContentItems(
	_ context.Context,
	_ []uuid.UUID,
) (map[uuid.UUID][]domain.TaxonomyTag, error) {
	f.queriesRun++
	return f.tags, nil
}

func (f *fakeRepo) GetTaxonomyByNamespaceCode(_ context.Context, namespace, code string) (domain.Taxonomy, error) {
	return domain.Taxonomy{
		ID:        uuid.New(),
		Namespace: namespace,
		Code:      code,
		Label:     code,
	}, nil
}

type fakeEvents struct {
	events []struct {
		Aggregate string
		Event     string
		Payload   any
	}
}

func (fe *fakeEvents) Write(
	_ context.Context,
	_ service.OutboxTx,
	aggregate, event string,
	payload any,
) (uuid.UUID, error) {
	fe.events = append(fe.events, struct {
		Aggregate string
		Event     string
		Payload   any
	}{
		Aggregate: aggregate,
		Event:     event,
		Payload:   payload,
	})
	return uuid.New(), nil
}

type fakeBeginner struct{}

func (b fakeBeginner) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return fakeTx{}, nil
}

type fakeTx struct{}

func (t fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return fakeTx{}, nil }
func (t fakeTx) Commit(_ context.Context) error          { return nil }
func (t fakeTx) Rollback(_ context.Context) error        { return nil }
func (t fakeTx) CopyFrom(
	_ context.Context,
	_ pgx.Identifier,
	_ []string,
	_ pgx.CopyFromSource,
) (int64, error) {
	return 0, nil
}
func (t fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t fakeTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (t fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}
func (t fakeTx) Conn() *pgx.Conn { return nil }

func setupService() (*service.Service, *fakeRepo, *fakeEvents) {
	repo := newFakeRepo()
	events := &fakeEvents{}
	svc := service.New(service.Deps{
		Pool:   fakeBeginner{},
		Repo:   repo,
		Events: events,
		Clock:  clock.NewFake(time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)),
		NewID:  uuid.New,
	})
	return svc, repo, events
}

func testAuthoringCreationAndReview(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	authorID, reviewerID uuid.UUID,
) (domain.Item, domain.Version) {
	t.Helper()
	// 1. Create item
	item, ver, err := svc.CreateItem(ctx, authorID, service.CreateItemRequest{
		Kind:      "vocab_word",
		Slug:      "climate-change-1",
		CEFRLevel: "B1",
		Body:      json.RawMessage(`{"word":"climate"}`),
	})
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}
	if item.Status != domain.StatusDraft || ver.Status != domain.StatusDraft {
		t.Fatalf("expected draft status, got item=%q ver=%q", item.Status, ver.Status)
	}

	// 2. Submit for review
	ver, err = svc.SubmitForReview(ctx, authorID, item.ID)
	if err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}
	if ver.Status != domain.StatusInReview {
		t.Fatalf("version status = %q, want in_review", ver.Status)
	}

	// 3. Self-approval check (BR-CONTENT-03)
	_, err = svc.Review(ctx, authorID, item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionApproved,
	})
	if err == nil {
		t.Fatal("expected SelfApprovalForbidden, got nil")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "SELF_APPROVAL_FORBIDDEN" {
		t.Errorf("error = %v, want SELF_APPROVAL_FORBIDDEN", err)
	}

	// 4. Reviewer approves
	ver, err = svc.Review(ctx, reviewerID, item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionApproved,
	})
	if err != nil {
		t.Fatalf("Review approve failed: %v", err)
	}
	if ver.Status != domain.StatusApproved {
		t.Fatalf("version status = %q, want approved", ver.Status)
	}

	return item, ver
}

func testAuthoringPublish(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	repo *fakeRepo,
	events *fakeEvents,
	authorID uuid.UUID,
	item domain.Item,
	ver domain.Version,
) domain.Version {
	t.Helper()
	mediaKey := "audio/words/climate.mp3"
	_, _ = repo.CreateMediaAsset(ctx, uuid.New(), mediaKey, "audio", nil, nil, domain.MediaStatusPending, nil, nil)
	ver.MediaRefs = []string{mediaKey}
	repo.versions[ver.ID] = ver

	_, err := svc.Publish(ctx, authorID, item.ID)
	if err == nil {
		t.Fatal("expected MediaNotReady, got nil")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "MEDIA_NOT_READY" {
		t.Errorf("error = %v, want MEDIA_NOT_READY", err)
	}

	for k, asset := range repo.mediaAssets {
		asset.Status = domain.MediaStatusReady
		repo.mediaAssets[k] = asset
	}

	ver, err = svc.Publish(ctx, authorID, item.ID)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if ver.Status != domain.StatusPublished {
		t.Errorf("version status = %q, want published", ver.Status)
	}
	if len(events.events) != 1 || events.events[0].Event != "content.published" {
		t.Errorf("expected content.published event, got %+v", events.events)
	}
	return ver
}

func testAuthoringArchive(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	events *fakeEvents,
	authorID uuid.UUID,
	item domain.Item,
	ver domain.Version,
) {
	t.Helper()
	archivedItem, err := svc.Archive(ctx, authorID, item.ID)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	if archivedItem.Status != domain.StatusArchived {
		t.Errorf("archived item status = %q, want archived", archivedItem.Status)
	}
	if len(events.events) != 2 || events.events[1].Event != "content.archived" {
		t.Errorf("expected content.archived event, got %+v", events.events)
	}

	resolvedVer, err := svc.GetVersion(ctx, ver.ID)
	if err != nil {
		t.Fatalf("GetVersion on archived item failed: %v", err)
	}
	if resolvedVer.ID != ver.ID {
		t.Errorf("resolvedVer ID = %v, want %v", resolvedVer.ID, ver.ID)
	}

	_, err = svc.GetPublishedVersionBySlug(ctx, item.Slug)
	if err == nil {
		t.Fatal("expected GetPublishedVersionBySlug on archived item to fail, got nil")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "CONTENT_NOT_PUBLISHED" {
		t.Errorf("error = %v, want CONTENT_NOT_PUBLISHED", err)
	}
}

func TestAuthoringWorkflowEndToEnd(t *testing.T) {
	t.Parallel()
	svc, repo, events := setupService()
	ctx := context.Background()

	authorID := uuid.New()
	reviewerID := uuid.New()

	item, ver := testAuthoringCreationAndReview(ctx, t, svc, authorID, reviewerID)
	ver = testAuthoringPublish(ctx, t, svc, repo, events, authorID, item, ver)
	testAuthoringArchive(ctx, t, svc, events, authorID, item, ver)
}

func TestGetManyVersionsSingleQuery(t *testing.T) {
	t.Parallel()
	svc, repo, _ := setupService()
	ctx := context.Background()

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	repo.versions[id1] = domain.Version{
		ID: id1, ItemID: uuid.New(), Version: 1, Kind: testKindWord, Status: domain.StatusPublished,
	}
	repo.versions[id2] = domain.Version{
		ID: id2, ItemID: uuid.New(), Version: 1, Kind: testKindWord, Status: domain.StatusPublished,
	}
	repo.versions[id3] = domain.Version{
		ID: id3, ItemID: uuid.New(), Version: 1, Kind: testKindWord, Status: domain.StatusPublished,
	}

	repo.queriesRun = 0
	result, err := svc.GetManyVersions(ctx, []uuid.UUID{id1, id2, id3})
	if err != nil {
		t.Fatalf("GetManyVersions failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("got %d versions, want 3", len(result))
	}
	// GetManyVersions must issue exactly 2 queries: versions + batch tags.
	// A naive N+1 loop over ListTagsForContentItem would be N+1 and is now
	// caught because the singular method also increments queriesRun.
	if repo.queriesRun != 2 {
		t.Errorf("queriesRun = %d, want exactly 2 (versions + batch tags)", repo.queriesRun)
	}
}
