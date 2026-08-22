package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// wantCode asserts that err carries the given apperr code, which is what the
// HTTP layer turns into a documented status. A bare error here means the caller
// would have seen a 500 instead.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", code)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperr.Error", err)
	}
	if appErr.Code != code {
		t.Fatalf("error code = %q, want %q", appErr.Code, code)
	}
}

// publishItem drives one item all the way to published and returns it with its
// published version. Every test that needs live content starts here rather than
// poking the fake repository, so the fixture exercises the same transitions the
// API does.
func publishItem(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	authorID, reviewerID uuid.UUID,
	slug string,
) (domain.Item, domain.Version) {
	t.Helper()

	item, _, err := svc.CreateItem(ctx, authorID, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      slug,
		CEFRLevel: domain.CEFRB1,
		Body:      json.RawMessage(`{"word":"climate"}`),
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if _, err = svc.SubmitForReview(ctx, authorID, item.ID); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	if _, err = svc.Review(ctx, reviewerID, item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionApproved,
	}); err != nil {
		t.Fatalf("Review: %v", err)
	}
	version, err := svc.Publish(ctx, authorID, item.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	item, err = svc.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItemByID: %v", err)
	}
	return item, version
}

func TestBrowseReturnsOnlyPublishedAndHonoursFilters(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	reviewer := uuid.New()

	publishItem(ctx, t, svc, author, reviewer, "published-one")
	publishItem(ctx, t, svc, author, reviewer, "published-two")

	// A draft item must never appear in a browse result (BR-CONTENT-02).
	if _, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "still-a-draft",
		CEFRLevel: domain.CEFRB1,
	}); err != nil {
		t.Fatalf("CreateItem draft: %v", err)
	}

	versions, total, err := svc.Browse(ctx, contract.BrowseFilter{})
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if len(versions) != 2 || total != 2 {
		t.Fatalf("Browse returned %d versions (total %d), want 2 and 2", len(versions), total)
	}
	for _, v := range versions {
		if v.Status != string(domain.StatusPublished) {
			t.Errorf("browse returned a %q version, want published only", v.Status)
		}
	}

	otherKind := "grammar_rule"
	versions, total, err = svc.Browse(ctx, contract.BrowseFilter{Kind: &otherKind})
	if err != nil {
		t.Fatalf("Browse by kind: %v", err)
	}
	if len(versions) != 0 || total != 0 {
		t.Errorf("filtering on an unused kind returned %d/%d, want 0/0", len(versions), total)
	}

	unusedLevel := domain.CEFRC2
	versions, _, err = svc.Browse(ctx, contract.BrowseFilter{CEFRLevel: &unusedLevel})
	if err != nil {
		t.Fatalf("Browse by level: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("filtering on an unused CEFR level returned %d, want 0", len(versions))
	}
}

func TestBrowseClampsPagingArguments(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	publishItem(ctx, t, svc, uuid.New(), uuid.New(), "paging-item")

	// Limit 0 falls back to the default page size and a negative offset to 0;
	// neither may reach the repository as-is.
	versions, total, err := svc.Browse(ctx, contract.BrowseFilter{Limit: 0, Offset: -5})
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if len(versions) != 1 || total != 1 {
		t.Fatalf("Browse returned %d/%d, want 1/1", len(versions), total)
	}
}

func TestGetPublishedVersionBySlug(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	item, version := publishItem(ctx, t, svc, uuid.New(), uuid.New(), "findable-slug")

	got, err := svc.GetPublishedVersionBySlug(ctx, item.Slug)
	if err != nil {
		t.Fatalf("GetPublishedVersionBySlug: %v", err)
	}
	if got.ID != version.ID {
		t.Errorf("got version %v, want %v", got.ID, version.ID)
	}

	_, err = svc.GetPublishedVersionBySlug(ctx, "no-such-slug")
	wantCode(t, err, "CONTENT_NOT_PUBLISHED")
}

func TestGetVersionAndGetManyVersionsEdges(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	_, err := svc.GetVersion(ctx, uuid.New())
	wantCode(t, err, "CONTENT_VERSION_NOT_FOUND")

	result, err := svc.GetManyVersions(ctx, nil)
	if err != nil {
		t.Fatalf("GetManyVersions(nil): %v", err)
	}
	if len(result) != 0 {
		t.Errorf("GetManyVersions(nil) returned %d entries, want 0", len(result))
	}

	_, version := publishItem(ctx, t, svc, uuid.New(), uuid.New(), "readable-by-id")
	single, err := svc.GetVersion(ctx, version.ID)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if single.ID != version.ID {
		t.Errorf("GetVersion returned %v, want %v", single.ID, version.ID)
	}
}

func TestGetItemByIDAndGetDraftVersion(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, version, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "draft-accessors",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	gotItem, err := svc.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItemByID: %v", err)
	}
	if gotItem.Slug != item.Slug {
		t.Errorf("slug = %q, want %q", gotItem.Slug, item.Slug)
	}

	gotVersion, err := svc.GetDraftVersion(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetDraftVersion: %v", err)
	}
	if gotVersion.ID != version.ID {
		t.Errorf("draft version = %v, want %v", gotVersion.ID, version.ID)
	}

	_, err = svc.GetItemByID(ctx, uuid.New())
	wantCode(t, err, "CONTENT_ITEM_NOT_FOUND")
}

func TestCreateItemRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  service.CreateItemRequest
		code string
	}{
		{
			name: "empty slug",
			req:  service.CreateItemRequest{Kind: testKindWord, Slug: "", CEFRLevel: domain.CEFRB1},
			code: "INVALID_SLUG",
		},
		{
			name: "slug with uppercase and spaces",
			req:  service.CreateItemRequest{Kind: testKindWord, Slug: "Not A Slug", CEFRLevel: domain.CEFRB1},
			code: "INVALID_SLUG",
		},
		{
			name: "empty kind",
			req:  service.CreateItemRequest{Kind: "", Slug: "valid-slug", CEFRLevel: domain.CEFRB1},
			code: "INVALID_CONTENT_KIND",
		},
		{
			name: "unknown CEFR level",
			req:  service.CreateItemRequest{Kind: testKindWord, Slug: "valid-slug", CEFRLevel: "Z9"},
			code: "INVALID_CEFR_LEVEL",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			svc, _, _ := setupService()
			_, _, err := svc.CreateItem(context.Background(), uuid.New(), testCase.req)
			wantCode(t, err, testCase.code)
		})
	}
}

func TestCreateItemRejectsDuplicateSlug(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	req := service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "taken-slug",
		CEFRLevel: domain.CEFRB1,
	}
	if _, _, err := svc.CreateItem(ctx, uuid.New(), req); err != nil {
		t.Fatalf("first CreateItem: %v", err)
	}

	_, _, err := svc.CreateItem(ctx, uuid.New(), req)
	wantCode(t, err, "SLUG_ALREADY_EXISTS")
}

func TestUpdateDraftEditsTheExistingDraftInPlace(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, version, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "editable-draft",
		CEFRLevel: domain.CEFRB1,
		Body:      json.RawMessage(`{"word":"first"}`),
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	level := domain.CEFRB2
	updated, err := svc.UpdateDraft(ctx, author, item.ID, service.UpdateDraftRequest{
		CEFRLevel: &level,
		Body:      json.RawMessage(`{"word":"second"}`),
		Tags:      []string{"environment"},
	})
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}

	if updated.ID != version.ID {
		t.Errorf("editing a draft created version %v, want the same version %v", updated.ID, version.ID)
	}
	if updated.Version != 1 {
		t.Errorf("version number = %d, want 1 — editing a draft must not bump it", updated.Version)
	}
	if updated.CEFRLevel != domain.CEFRB2 {
		t.Errorf("CEFR level = %q, want %q", updated.CEFRLevel, domain.CEFRB2)
	}
	if string(updated.Body) != `{"word":"second"}` {
		t.Errorf("body = %s, want the updated payload", updated.Body)
	}
}

func TestUpdateDraftKeepsBodyWhenOmitted(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "body-preserved",
		CEFRLevel: domain.CEFRB1,
		Body:      json.RawMessage(`{"word":"kept"}`),
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	updated, err := svc.UpdateDraft(ctx, author, item.ID, service.UpdateDraftRequest{})
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if string(updated.Body) != `{"word":"kept"}` {
		t.Errorf("body = %s, want the original payload preserved", updated.Body)
	}
}

func TestUpdateDraftAfterPublishOpensANewVersion(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, published := publishItem(ctx, t, svc, author, uuid.New(), "reopened-item")

	level := domain.CEFRB2
	draft, err := svc.UpdateDraft(ctx, author, item.ID, service.UpdateDraftRequest{
		CEFRLevel: &level,
		Body:      json.RawMessage(`{"word":"revised"}`),
	})
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}

	if draft.ID == published.ID {
		t.Fatal("editing a published item mutated the published version (BR-CONTENT-01)")
	}
	if draft.Version != published.Version+1 {
		t.Errorf("new version number = %d, want %d", draft.Version, published.Version+1)
	}
	if draft.Status != domain.StatusDraft {
		t.Errorf("new version status = %q, want draft", draft.Status)
	}

	// The published snapshot must be untouched and still readable.
	stillPublished, err := svc.GetVersion(ctx, published.ID)
	if err != nil {
		t.Fatalf("GetVersion on the published version: %v", err)
	}
	if stillPublished.Status != string(domain.StatusPublished) {
		t.Errorf("published version status = %q, want published", stillPublished.Status)
	}
}

func TestUpdateDraftRejectsInvalidCEFRLevel(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "level-guard",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	bad := "B9"
	_, err = svc.UpdateDraft(ctx, author, item.ID, service.UpdateDraftRequest{CEFRLevel: &bad})
	wantCode(t, err, "INVALID_CEFR_LEVEL")
}

func TestUpdateDraftUnknownItem(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()

	_, err := svc.UpdateDraft(context.Background(), uuid.New(), uuid.New(), service.UpdateDraftRequest{})
	wantCode(t, err, "CONTENT_ITEM_NOT_FOUND")
}

// approveItem drives an item to approved without publishing it, so the media
// checks can act on a version that is otherwise ready to go live.
func approveItem(
	ctx context.Context,
	t *testing.T,
	svc *service.Service,
	authorID, reviewerID uuid.UUID,
	slug string,
) (domain.Item, domain.Version) {
	t.Helper()

	item, _, err := svc.CreateItem(ctx, authorID, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      slug,
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if _, err = svc.SubmitForReview(ctx, authorID, item.ID); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	version, err := svc.Review(ctx, reviewerID, item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionApproved,
	})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	return item, version
}

func TestPublishBlocksOnMediaThatIsNotReady(t *testing.T) {
	t.Parallel()
	svc, repo, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, version := approveItem(ctx, t, svc, author, uuid.New(), "waiting-on-audio")

	// media_refs has no API surface yet (see content/TODO.md, carried into
	// P7.4), so the reference is seeded the only way a caller can today.
	stored := repo.versions[version.ID]
	stored.MediaRefs = []string{"audio/climate.mp3"}
	repo.versions[version.ID] = stored
	repo.mediaAssets["audio/climate.mp3"] = domain.MediaAsset{
		ID:        uuid.New(),
		ObjectKey: "audio/climate.mp3",
		Kind:      "audio",
		Status:    domain.MediaStatusPending,
	}

	_, err := svc.Publish(ctx, author, item.ID)
	wantCode(t, err, "MEDIA_NOT_READY")
}

func TestPublishBlocksOnMediaThatDoesNotExist(t *testing.T) {
	t.Parallel()
	svc, repo, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, version := approveItem(ctx, t, svc, author, uuid.New(), "dangling-audio")

	stored := repo.versions[version.ID]
	stored.MediaRefs = []string{"audio/missing.mp3"}
	repo.versions[version.ID] = stored

	_, err := svc.Publish(ctx, author, item.ID)
	wantCode(t, err, "MEDIA_NOT_READY")
}

func TestPublishSucceedsOnceMediaIsReady(t *testing.T) {
	t.Parallel()
	svc, repo, events := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, version := approveItem(ctx, t, svc, author, uuid.New(), "audio-ready")

	stored := repo.versions[version.ID]
	stored.MediaRefs = []string{"audio/ready.mp3"}
	repo.versions[version.ID] = stored
	repo.mediaAssets["audio/ready.mp3"] = domain.MediaAsset{
		ID:        uuid.New(),
		ObjectKey: "audio/ready.mp3",
		Kind:      "audio",
		Status:    domain.MediaStatusReady,
	}

	published, err := svc.Publish(ctx, author, item.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if published.Status != domain.StatusPublished {
		t.Errorf("status = %q, want published", published.Status)
	}
	if len(events.events) != 1 || events.events[0].Event != contract.EventContentPublished {
		t.Errorf("outbox events = %+v, want one content.published", events.events)
	}
}

func TestPublishRejectsAVersionThatWasNeverApproved(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "straight-to-live",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	_, err = svc.Publish(ctx, author, item.ID)
	wantCode(t, err, "INVALID_STATE_TRANSITION")
}

func TestPublishTwiceIsIdempotent(t *testing.T) {
	t.Parallel()
	svc, _, events := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, first := publishItem(ctx, t, svc, author, uuid.New(), "published-twice")

	second, err := svc.Publish(ctx, author, item.ID)
	if err != nil {
		t.Fatalf("second Publish: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second publish returned %v, want the same version %v", second.ID, first.ID)
	}
	if len(events.events) != 1 {
		t.Errorf("outbox events = %d, want 1 — an idempotent publish must not re-emit", len(events.events))
	}
}

func TestArchiveTwiceIsIdempotent(t *testing.T) {
	t.Parallel()
	svc, _, events := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _ := publishItem(ctx, t, svc, author, uuid.New(), "archived-twice")

	if _, err := svc.Archive(ctx, author, item.ID); err != nil {
		t.Fatalf("first Archive: %v", err)
	}
	archived, err := svc.Archive(ctx, author, item.ID)
	if err != nil {
		t.Fatalf("second Archive: %v", err)
	}
	if archived.Status != domain.StatusArchived {
		t.Errorf("status = %q, want archived", archived.Status)
	}

	archivedEvents := 0
	for _, e := range events.events {
		if e.Event == contract.EventContentArchived {
			archivedEvents++
		}
	}
	if archivedEvents != 1 {
		t.Errorf("content.archived emitted %d times, want 1", archivedEvents)
	}
}

func TestArchiveRejectsAnItemThatWasNeverPublished(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "never-live",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	_, err = svc.Archive(ctx, author, item.ID)
	wantCode(t, err, "INVALID_STATE_TRANSITION")
}

func TestArchiveUnknownItem(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()

	_, err := svc.Archive(context.Background(), uuid.New(), uuid.New())
	wantCode(t, err, "CONTENT_ITEM_NOT_FOUND")
}

func TestReviewRequestingChangesReturnsTheVersionToDraft(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	reviewer := uuid.New()

	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "needs-work",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if _, err = svc.SubmitForReview(ctx, author, item.ID); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	comments := "The definition needs a source."
	version, err := svc.Review(ctx, reviewer, item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionChangesRequested,
		Comments: &comments,
	})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if version.Status != domain.StatusDraft {
		t.Errorf("version status = %q, want draft", version.Status)
	}

	item, err = svc.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItemByID: %v", err)
	}
	if item.Status != domain.StatusDraft {
		t.Errorf("item status = %q, want draft", item.Status)
	}
}

func TestReviewRejectsAnUnknownDecision(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "bad-decision",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if _, err = svc.SubmitForReview(ctx, author, item.ID); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	_, err = svc.Review(ctx, uuid.New(), item.ID, service.ReviewDecisionRequest{Decision: "maybe"})
	wantCode(t, err, "INVALID_REVIEW_DECISION")
}

func TestReviewRejectsAVersionThatIsNotInReview(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _, err := svc.CreateItem(ctx, author, service.CreateItemRequest{
		Kind:      testKindWord,
		Slug:      "not-submitted",
		CEFRLevel: domain.CEFRB1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	_, err = svc.Review(ctx, uuid.New(), item.ID, service.ReviewDecisionRequest{
		Decision: domain.ReviewDecisionApproved,
	})
	wantCode(t, err, "INVALID_STATE_TRANSITION")
}

func TestSubmitForReviewUnknownItem(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()

	_, err := svc.SubmitForReview(context.Background(), uuid.New(), uuid.New())
	wantCode(t, err, "CONTENT_ITEM_NOT_FOUND")
}

func TestSubmitForReviewRejectsAnAlreadyApprovedVersion(t *testing.T) {
	t.Parallel()
	svc, _, _ := setupService()
	ctx := context.Background()

	author := uuid.New()
	item, _ := approveItem(ctx, t, svc, author, uuid.New(), "already-approved")

	_, err := svc.SubmitForReview(ctx, author, item.ID)
	wantCode(t, err, "INVALID_STATE_TRANSITION")
}
