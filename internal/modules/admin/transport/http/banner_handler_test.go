package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	permFlags       = "system.flags"
	roleLearner     = "user"
	pathBanners     = "/admin/banners"
	pathActive      = "/banners/active"
	httpMethodPut   = http.MethodPut
	httpMethodDel   = http.MethodDelete
	validBannerBody = `{"title":"IELTS mock week","body":"Free until Sunday.",` +
		`"cta_label":"Start","cta_url":"/exams","tone":"promo","is_active":true,"priority":10}`
)

var testBannerID = uuid.MustParse("0199c3e4-5f60-7a82-9cde-f01234567890")

const (
	testRawKey   = "banners/raw/a/b"
	testImageURL = "https://storage.test/i.jpg"
)

// fakeBannerState is the banner half of fakeAdminService.
type fakeBannerState struct {
	called   []string
	seenIn   admindomain.BannerInput
	seenID   uuid.UUID
	seenUser uuid.UUID
	list     []admindomain.Banner
	result   admindomain.Banner
	upload   admindomain.BannerImageUpload
	image    admindomain.BannerImage
	err      error
}

func (f *fakeAdminService) ListActiveBanners(_ context.Context) ([]admindomain.Banner, error) {
	f.banner.called = append(f.banner.called, "active")
	return f.banner.list, f.banner.err
}

func (f *fakeAdminService) ListBanners(_ context.Context) ([]admindomain.Banner, error) {
	f.banner.called = append(f.banner.called, "list")
	return f.banner.list, f.banner.err
}

func (f *fakeAdminService) CreateBanner(
	_ context.Context, actorID uuid.UUID, in admindomain.BannerInput,
) (admindomain.Banner, error) {
	f.banner.called = append(f.banner.called, "create")
	f.banner.seenUser = actorID
	f.banner.seenIn = in
	return f.banner.result, f.banner.err
}

func (f *fakeAdminService) UpdateBanner(
	_ context.Context, actorID, id uuid.UUID, in admindomain.BannerInput,
) (admindomain.Banner, error) {
	f.banner.called = append(f.banner.called, "update")
	f.banner.seenUser = actorID
	f.banner.seenID = id
	f.banner.seenIn = in
	return f.banner.result, f.banner.err
}

func (f *fakeAdminService) DeleteBanner(_ context.Context, actorID, id uuid.UUID) error {
	f.banner.called = append(f.banner.called, "delete")
	f.banner.seenUser = actorID
	f.banner.seenID = id
	return f.banner.err
}

func (f *fakeAdminService) RequestBannerImageUpload(
	_ context.Context, actorID uuid.UUID, contentType string,
) (admindomain.BannerImageUpload, error) {
	f.banner.called = append(f.banner.called, "image-intent:"+contentType)
	f.banner.seenUser = actorID
	return f.banner.upload, f.banner.err
}

func (f *fakeAdminService) ProcessBannerImage(
	_ context.Context, actorID uuid.UUID, objectKey string,
) (admindomain.BannerImage, error) {
	f.banner.called = append(f.banner.called, "image-process:"+objectKey)
	f.banner.seenUser = actorID
	return f.banner.image, f.banner.err
}

func sampleBanner() admindomain.Banner {
	label, link := "Start", "/exams"
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	return admindomain.Banner{
		ID:          testBannerID,
		Title:       "IELTS mock week",
		Body:        "Free until Sunday.",
		CTALabel:    &label,
		CTAURL:      &link,
		Tone:        admindomain.BannerTonePromo,
		IsActive:    true,
		Priority:    10,
		Dismissible: true,
		StartsAt:    &start,
		CreatedBy:   testAdminID,
		CreatedAt:   start,
		UpdatedAt:   start,
	}
}

func TestBannerAdminRoutes_RefuseWithoutSystemFlags(t *testing.T) {
	svc := &fakeAdminService{}
	// A learner, and an admin holding every permission except the one that
	// guards banners: both are refused, and the service is never reached.
	srv := newServer(svc, newFakeGuard("user.list", "admin.dashboard"))
	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}

	cases := []struct{ method, path, body string }{
		{httpMethodGet, pathBanners, ""},
		{httpMethodPost, pathBanners, validBannerBody},
		{httpMethodPut, pathBanners + "/" + testBannerID.String(), validBannerBody},
		{httpMethodDel, pathBanners + "/" + testBannerID.String(), ""},
		{httpMethodPost, pathBanners + "/image-upload-intent", `{"content_type":"image/png"}`},
		{httpMethodPost, pathBanners + "/images", `{"object_key":"banners/raw/x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doRequest(srv, tc.method, tc.path, tc.body, learner)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (body %s)", rec.Code, rec.Body)
			}
		})
	}
	if len(svc.banner.called) != 0 {
		t.Fatalf("service reached without permission: %v", svc.banner.called)
	}
}

func TestBannerAdminRoutes_Unauthenticated401(t *testing.T) {
	svc := &fakeAdminService{}
	srv := newServer(svc, newFakeGuard(permFlags))

	for _, path := range []string{pathBanners, pathActive} {
		rec := doRequest(srv, httpMethodGet, path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", path, rec.Code)
		}
	}
	if len(svc.banner.called) != 0 {
		t.Fatalf("service reached without a session: %v", svc.banner.called)
	}
}

func TestListActiveBanners_AnySignedInUser_NoAdminFields(t *testing.T) {
	svc := &fakeAdminService{}
	svc.banner.list = []admindomain.Banner{sampleBanner()}
	// The guard holds nothing: the active list needs only a session.
	guard := newFakeGuard()
	srv := newServer(svc, guard)
	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}

	rec := doRequest(srv, httpMethodGet, pathActive, "", learner)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(guard.seen) != 0 {
		t.Fatalf("active banners asked the guard for %v; it needs no permission", guard.seen)
	}

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	item := resp.Items[0]
	for _, hidden := range []string{"created_by", "priority", "starts_at", "ends_at", "is_active"} {
		if _, ok := item[hidden]; ok {
			t.Fatalf("learner view leaks %q: %s", hidden, rec.Body)
		}
	}
	if item["cta_url"] != "/exams" || item["tone"] != "promo" {
		t.Fatalf("unexpected item: %v", item)
	}
}

func TestListActiveBanners_EmptyIsAnEmptyArray(t *testing.T) {
	svc := &fakeAdminService{}
	srv := newServer(svc, newFakeGuard())
	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}

	rec := doRequest(srv, httpMethodGet, pathActive, "", learner)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("want items: [], got %s", rec.Body)
	}
}

func TestCreateBanner_PassesActorAndDefaults(t *testing.T) {
	svc := &fakeAdminService{}
	svc.banner.result = sampleBanner()
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners, `{"title":"Hello"}`, actor)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if svc.banner.seenUser != testAdminID {
		t.Fatalf("created_by = %v, want the acting admin", svc.banner.seenUser)
	}
	in := svc.banner.seenIn
	if in.Title != "Hello" || in.Tone != admindomain.BannerToneInfo || !in.Dismissible || in.IsActive {
		t.Fatalf("defaults not applied: %+v", in)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["created_by"] != testAdminID.String() || resp["ends_at"] != nil {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
}

func TestCreateBanner_UnknownFieldIs400(t *testing.T) {
	svc := &fakeAdminService{}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners, `{"title":"x","html":"<b>"}`, actor)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(svc.banner.called) != 0 {
		t.Fatalf("service reached with a malformed body")
	}
}

func TestCreateBanner_ValidationErrorIs422WithFields(t *testing.T) {
	svc := &fakeAdminService{}
	_, svc.banner.err = admindomain.NormalizeBannerInput(admindomain.BannerInput{Title: ""})
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners, `{"title":""}`, actor)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"field":"title"`) {
		t.Fatalf("problem does not name the field: %s", rec.Body)
	}
}

func TestUpdateBanner_HappyPath(t *testing.T) {
	svc := &fakeAdminService{}
	svc.banner.result = sampleBanner()
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPut, pathBanners+"/"+testBannerID.String(), validBannerBody, actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if svc.banner.seenID != testBannerID || svc.banner.seenIn.Priority != 10 {
		t.Fatalf("wrong call: id %v, input %+v", svc.banner.seenID, svc.banner.seenIn)
	}
}

func TestUpdateBanner_NotFoundIs404(t *testing.T) {
	svc := &fakeAdminService{}
	svc.banner.err = admindomain.ErrBannerNotFound
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPut, pathBanners+"/"+uuid.NewString(), validBannerBody, actor)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteBanner_Returns204(t *testing.T) {
	svc := &fakeAdminService{}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodDel, pathBanners+"/"+testBannerID.String(), "", actor)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d body %q, want 204 and no body", rec.Code, rec.Body)
	}
	if svc.banner.seenID != testBannerID || svc.banner.seenUser != testAdminID {
		t.Fatalf("wrong delete call: %+v", svc.banner)
	}
}

func TestBannerRoutes_MalformedIDIs422(t *testing.T) {
	svc := &fakeAdminService{}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodDel, pathBanners+"/not-a-uuid", "", actor)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestBannerImageUploadIntent_PassesActorAndType(t *testing.T) {
	svc := &fakeAdminService{}
	expires := time.Date(2026, 10, 8, 9, 25, 0, 0, time.UTC)
	svc.banner.upload = admindomain.BannerImageUpload{
		URL: "https://storage.test/fluentra-media", Method: "POST",
		FormData: map[string]string{"key": testRawKey}, FileField: "file",
		ObjectKey: testRawKey, ExpiresAt: expires, MaxBytes: 5 << 20, ContentType: "image/png",
	}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners+"/image-upload-intent", `{"content_type":"image/png"}`, actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if svc.banner.called[0] != "image-intent:image/png" || svc.banner.seenUser != testAdminID {
		t.Fatalf("wrong call: %v by %v", svc.banner.called, svc.banner.seenUser)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["upload_url"] != "https://storage.test/fluentra-media" || resp["object_key"] != testRawKey ||
		resp["expires_at"] != "2026-10-08T09:25:00Z" || resp["max_bytes"] != float64(5<<20) {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
}

func TestProcessBannerImage_Returns201(t *testing.T) {
	svc := &fakeAdminService{}
	svc.banner.image = admindomain.BannerImage{
		Key: "banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg", URL: testImageURL,
		Width: 1600, Height: 900,
	}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners+"/images", `{"object_key":"`+testRawKey+`"}`, actor)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	want := `{"image_key":"banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg",` +
		`"image_url":"` + testImageURL + `","width":1600,"height":900}`
	if strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("body = %s, want %s", rec.Body, want)
	}
}

func TestBannerDTOs_CarryTheImage(t *testing.T) {
	banner := sampleBanner()
	banner.Image = &admindomain.BannerImage{
		Key: "banners/images/0199c3e4-5f60-7a82-9cde-f01234567899.jpg", Alt: "Mock test",
		Width: 1200, Height: 600, URL: testImageURL,
	}
	unsigned := sampleBanner()
	unsigned.ID = uuid.New()
	unsigned.Image = &admindomain.BannerImage{Key: "banners/images/x.jpg", Width: 10, Height: 10}

	svc := &fakeAdminService{}
	svc.banner.list = []admindomain.Banner{banner, unsigned}
	srv := newServer(svc, newFakeGuard(permFlags))
	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}

	rec := doRequest(srv, httpMethodGet, pathActive, "", learner)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	first, second := resp.Items[0], resp.Items[1]
	if first["image_url"] != testImageURL || first["image_alt"] != "Mock test" ||
		first["image_width"] != float64(1200) || first["image_height"] != float64(600) {
		t.Fatalf("learner image fields wrong: %v", first)
	}
	if _, leaks := first["image_key"]; leaks {
		t.Fatalf("learner view leaks the storage key: %v", first)
	}
	// An image the service could not sign is not a broken picture: it is no picture.
	if second["image_url"] != nil || second["image_width"] != nil {
		t.Fatalf("unsigned image served to a learner: %v", second)
	}
}

func TestBannerKind_RoundTrips(t *testing.T) {
	svc := &fakeAdminService{}
	slide := sampleBanner()
	slide.Kind = admindomain.BannerKindSlide
	svc.banner.result = slide
	svc.banner.list = []admindomain.Banner{slide, sampleBanner()}
	srv := newServer(svc, newFakeGuard(permFlags))
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodPost, pathBanners, `{"kind":"slide","title":"Sale"}`, actor)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	if svc.banner.seenIn.Kind != admindomain.BannerKindSlide {
		t.Fatalf("kind not passed on: %+v", svc.banner.seenIn)
	}
	if !strings.Contains(rec.Body.String(), `"kind":"slide"`) {
		t.Fatalf("response lacks the kind: %s", rec.Body)
	}

	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}
	rec = doRequest(srv, httpMethodGet, pathActive, "", learner)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// A banner stored before kinds existed reads as an announcement.
	if resp.Items[0]["kind"] != "slide" || resp.Items[1]["kind"] != "announcement" {
		t.Fatalf("kinds = %v, %v", resp.Items[0]["kind"], resp.Items[1]["kind"])
	}
}
