package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// fakeBannerRepo returns its rows in stored order, the way the SQL hands them
// over already sorted. It applies no window of its own, so whatever the service
// returns is what the service decided to keep.
type fakeBannerRepo struct {
	rows        []admindomain.Banner
	activeCalls int
	lastLimit   int32
	created     []admindomain.BannerInput
	images      []*admindomain.BannerImage
	updated     []uuid.UUID
	deleted     []uuid.UUID
	err         error
}

func (f *fakeBannerRepo) ListBanners(_ context.Context) ([]admindomain.Banner, error) {
	return f.rows, f.err
}

func (f *fakeBannerRepo) ListActiveBanners(_ context.Context, _ time.Time, limit int32) ([]admindomain.Banner, error) {
	f.activeCalls++
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

// GetBanner finds a stored row by id, and otherwise answers with a bare
// banner so that tests about other things need not seed one.
func (f *fakeBannerRepo) GetBanner(_ context.Context, id uuid.UUID) (admindomain.Banner, error) {
	if f.err != nil {
		return admindomain.Banner{}, f.err
	}
	for _, row := range f.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return admindomain.Banner{ID: id}, nil
}

func (f *fakeBannerRepo) CreateBanner(
	_ context.Context, in admindomain.BannerInput, image *admindomain.BannerImage, createdBy uuid.UUID,
) (admindomain.Banner, error) {
	if f.err != nil {
		return admindomain.Banner{}, f.err
	}
	f.created = append(f.created, in)
	f.images = append(f.images, image)
	return admindomain.Banner{ID: uuid.New(), Title: in.Title, Image: image, CreatedBy: createdBy}, nil
}

func (f *fakeBannerRepo) UpdateBanner(
	_ context.Context, id uuid.UUID, in admindomain.BannerInput, image *admindomain.BannerImage,
) (admindomain.Banner, error) {
	if f.err != nil {
		return admindomain.Banner{}, f.err
	}
	f.updated = append(f.updated, id)
	f.images = append(f.images, image)
	return admindomain.Banner{ID: id, Title: in.Title, Image: image}, nil
}

func (f *fakeBannerRepo) DeleteBanner(_ context.Context, id uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeRecorder struct{ entries []auditcontract.Entry }

func (f *fakeRecorder) Record(_ context.Context, entry auditcontract.Entry) {
	f.entries = append(f.entries, entry)
}

var bannerNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func liveBanner(title string) admindomain.Banner {
	return admindomain.Banner{ID: uuid.New(), Title: title, IsActive: true}
}

func newBannerService(repo *fakeBannerRepo) (*adminsvc.Service, *clock.Fake, *fakeRecorder) {
	clk := clock.NewFake(bannerNow)
	rec := &fakeRecorder{}
	svc := adminsvc.New(adminsvc.Deps{Banners: repo, Clock: clk, Audit: rec})
	return svc, clk, rec
}

func titles(banners []admindomain.Banner) []string {
	out := make([]string, 0, len(banners))
	for _, b := range banners {
		out = append(out, b.Title)
	}
	return out
}

func TestListActiveBanners_KeepsOrderAndCapsAtThree(t *testing.T) {
	repo := &fakeBannerRepo{rows: []admindomain.Banner{
		liveBanner("p100"), liveBanner("p50"), liveBanner("p50-older"), liveBanner("p10"), liveBanner("p0"),
	}}
	svc, _, _ := newBannerService(repo)

	got, err := svc.ListActiveBanners(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"p100", "p50", "p50-older"}
	if gotTitles := titles(got); len(gotTitles) != 3 ||
		gotTitles[0] != want[0] || gotTitles[1] != want[1] || gotTitles[2] != want[2] {
		t.Fatalf("got %v, want %v", gotTitles, want)
	}
	handedOut := admindomain.ActiveAnnouncementLimit + admindomain.ActiveSlideLimit
	if repo.lastLimit <= int32(handedOut) {
		t.Fatalf("refresh read %d rows; it should read more than it hands out", repo.lastLimit)
	}
}

func TestListActiveBanners_CapsEachKindSeparately(t *testing.T) {
	rows := make([]admindomain.Banner, 0, 12)
	for i := range 8 {
		slide := liveBanner(fmt.Sprintf("slide-%d", i))
		slide.Kind = admindomain.BannerKindSlide
		rows = append(rows, slide)
	}
	for i := range 4 {
		rows = append(rows, liveBanner(fmt.Sprintf("note-%d", i)))
	}
	svc, _, _ := newBannerService(&fakeBannerRepo{rows: rows})

	got, err := svc.ListActiveBanners(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	counts := map[admindomain.BannerKind]int{}
	for _, b := range got {
		kind := b.Kind
		if kind == "" {
			kind = admindomain.BannerKindAnnouncement
		}
		counts[kind]++
	}
	// Eight slides ahead in priority must not crowd out the announcements.
	if counts[admindomain.BannerKindSlide] != 6 || counts[admindomain.BannerKindAnnouncement] != 3 {
		t.Fatalf("counts = %v, want 6 slides and 3 announcements", counts)
	}
}

func TestListActiveBanners_DropsRowsOutsideTheWindow(t *testing.T) {
	past, future := bannerNow.Add(-time.Hour), bannerNow.Add(time.Hour)
	notYet := liveBanner("not-yet")
	notYet.StartsAt = &future
	ended := liveBanner("ended")
	ended.EndsAt = &past
	off := admindomain.Banner{ID: uuid.New(), Title: "off"}
	inside := liveBanner("inside")
	inside.StartsAt, inside.EndsAt = &past, &future

	repo := &fakeBannerRepo{rows: []admindomain.Banner{notYet, ended, off, inside}}
	svc, _, _ := newBannerService(repo)

	got, err := svc.ListActiveBanners(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTitles := titles(got); len(gotTitles) != 1 || gotTitles[0] != "inside" {
		t.Fatalf("got %v, want [inside]", gotTitles)
	}
}

func TestListActiveBanners_EmptyIsNotNil(t *testing.T) {
	svc, _, _ := newBannerService(&fakeBannerRepo{})
	got, err := svc.ListActiveBanners(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want an empty, non-nil slice", got, err)
	}
}

func TestListActiveBanners_CachesForTheTTL(t *testing.T) {
	ending := liveBanner("ending")
	endsAt := bannerNow.Add(10 * time.Second)
	ending.EndsAt = &endsAt
	repo := &fakeBannerRepo{rows: []admindomain.Banner{ending, liveBanner("steady")}}
	svc, clk, _ := newBannerService(repo)
	ctx := context.Background()

	if _, err := svc.ListActiveBanners(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.ListActiveBanners(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.activeCalls != 1 {
		t.Fatalf("repo read %d times inside the TTL, want 1", repo.activeCalls)
	}

	// Inside the TTL a banner whose window closed is dropped without a read.
	clk.Advance(15 * time.Second)
	got, err := svc.ListActiveBanners(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTitles := titles(got); len(gotTitles) != 1 || gotTitles[0] != "steady" || repo.activeCalls != 1 {
		t.Fatalf("got %v after %d reads; want [steady] from the cache", gotTitles, repo.activeCalls)
	}

	clk.Advance(30 * time.Second)
	if _, err := svc.ListActiveBanners(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.activeCalls != 2 {
		t.Fatalf("repo read %d times after the TTL, want 2", repo.activeCalls)
	}
}

func TestBannerWrites_InvalidateTheCacheAndAudit(t *testing.T) {
	repo := &fakeBannerRepo{rows: []admindomain.Banner{liveBanner("a")}}
	svc, _, rec := newBannerService(repo)
	ctx := context.Background()
	actor := uuid.New()
	id := uuid.New()

	warm := func() {
		t.Helper()
		if _, err := svc.ListActiveBanners(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	warm()
	if _, err := svc.CreateBanner(ctx, actor, admindomain.BannerInput{Title: "New"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	warm()
	if _, err := svc.UpdateBanner(ctx, actor, id, admindomain.BannerInput{Title: "Edited"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	warm()
	if err := svc.DeleteBanner(ctx, actor, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	warm()

	if repo.activeCalls != 4 {
		t.Fatalf("repo read %d times, want 4 (each write must invalidate)", repo.activeCalls)
	}

	wantActions := []string{"admin.banner_created", "admin.banner_updated", "admin.banner_deleted"}
	if len(rec.entries) != len(wantActions) {
		t.Fatalf("audit entries = %d, want %d", len(rec.entries), len(wantActions))
	}
	for i, entry := range rec.entries {
		if entry.Action != wantActions[i] || entry.TargetType != "banner" {
			t.Fatalf("entry %d = %+v, want %s on a banner", i, entry, wantActions[i])
		}
		if entry.Meta["actor_id"] != actor.String() {
			t.Fatalf("entry %d actor = %v, want %s", i, entry.Meta["actor_id"], actor)
		}
	}
}

func TestCreateBanner_RefusesUnsafeURLsBeforeTheRepository(t *testing.T) {
	for _, link := range []string{"javascript:alert(1)", "data:text/html,x", "//evil.example", "http://x.test"} {
		repo := &fakeBannerRepo{}
		svc, _, rec := newBannerService(repo)

		_, err := svc.CreateBanner(context.Background(), uuid.New(), admindomain.BannerInput{
			Title: "t", CTALabel: ptr("Go"), CTAURL: ptr(link),
		})
		if !apperr.Is(err, apperr.Validation) {
			t.Fatalf("%s: err = %v, want a validation error", link, err)
		}
		if len(repo.created) != 0 || len(rec.entries) != 0 {
			t.Fatalf("%s: stored or audited an unsafe link", link)
		}
	}
}

func TestCreateBanner_StoresTheNormalisedInput(t *testing.T) {
	repo := &fakeBannerRepo{}
	svc, _, _ := newBannerService(repo)
	actor := uuid.New()

	banner, err := svc.CreateBanner(context.Background(), actor, admindomain.BannerInput{
		Title: "  Spaced  ", CTALabel: ptr(" Go "), CTAURL: ptr(" /exams "),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if banner.CreatedBy != actor {
		t.Fatalf("created_by = %v, want %v", banner.CreatedBy, actor)
	}
	stored := repo.created[0]
	if stored.Title != "Spaced" || *stored.CTALabel != "Go" || *stored.CTAURL != "/exams" ||
		stored.Tone != admindomain.BannerToneInfo {
		t.Fatalf("stored %+v, want trimmed text and the default tone", stored)
	}
}

func TestUpdateAndDeleteBanner_PassNotFoundThrough(t *testing.T) {
	repo := &fakeBannerRepo{err: admindomain.ErrBannerNotFound}
	svc, _, rec := newBannerService(repo)
	ctx := context.Background()

	_, err := svc.UpdateBanner(ctx, uuid.New(), uuid.New(), admindomain.BannerInput{Title: "t"})
	if !errors.Is(err, admindomain.ErrBannerNotFound) {
		t.Fatalf("update err = %v, want ErrBannerNotFound", err)
	}
	if err := svc.DeleteBanner(ctx, uuid.New(), uuid.New()); !errors.Is(err, admindomain.ErrBannerNotFound) {
		t.Fatalf("delete err = %v, want ErrBannerNotFound", err)
	}
	if len(rec.entries) != 0 {
		t.Fatalf("a failed write was audited: %+v", rec.entries)
	}
}

func TestBanners_WithoutARepository(t *testing.T) {
	svc := adminsvc.New(adminsvc.Deps{})
	got, err := svc.ListActiveBanners(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("active without a repo = %v, %v; want empty and no error", got, err)
	}
	if _, err := svc.ListBanners(context.Background()); !apperr.Is(err, apperr.Unavailable) {
		t.Fatalf("admin list without a repo = %v, want unavailable", err)
	}
}

func ptr(s string) *string { return &s }
