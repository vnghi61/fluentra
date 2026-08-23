package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	lessonhttp "github.com/fluentra/fluentra/internal/modules/lesson/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// selectiveGuard denies exactly one permission, so a test can prove which
// endpoint sits behind which one.
type selectiveGuard struct{ denied string }

func (g selectiveGuard) Require(ctx context.Context, permission string) error {
	if permission == g.denied {
		return denyGuard{}.Require(ctx, permission)
	}
	return nil
}

func routerFor(t *testing.T, svc lessonhttp.LessonService, guard lessonhttp.Guard) http.Handler {
	t.Helper()
	handler, err := lessonhttp.NewHandler(svc, guard)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	router := chi.NewRouter()
	handler.Routes(router)
	handler.AdminRoutes(router)
	return router
}

func request(method, target string, body any, actor uuid.UUID) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if actor != uuid.Nil {
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actor, Role: "admin"}))
	}
	return req
}

func serve(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestEveryEndpointIsBehindItsOwnPermission denies exactly one permission per
// run and asserts the matching endpoint is the one that 403s. An endpoint wired
// to the wrong permission fails here rather than in production.
func TestEveryEndpointIsBehindItsOwnPermission(t *testing.T) {
	t.Parallel()

	id := uuid.New().String()
	cases := []struct {
		permission string
		method     string
		path       string
		body       any
	}{
		{lessonhttp.PermContentReadPublished, http.MethodGet, "/courses", nil},
		{lessonhttp.PermContentReadPublished, http.MethodGet, "/courses/ielts-core", nil},
		{lessonhttp.PermContentReadPublished, http.MethodGet, "/lessons/" + id, nil},
		{
			lessonhttp.PermContentCreate, http.MethodPost, "/admin/courses",
			lessonhttp.CreateCourseRequest{Slug: "ielts-core", Title: "Course", CEFRFrom: "B1", CEFRTo: "B2"},
		},
		{
			lessonhttp.PermContentEdit, http.MethodPut, "/admin/lessons/" + id + "/activities",
			lessonhttp.UpdateActivitiesRequest{},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.permission+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := routerFor(t, &fakeLessonService{}, selectiveGuard{denied: testCase.permission})
			rec := serve(router, request(testCase.method, testCase.path, testCase.body, uuid.New()))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("denying %s did not block %s %s: got %d",
					testCase.permission, testCase.method, testCase.path, rec.Code)
			}
		})
	}
}

// TestEndpointsRejectAMalformedID keeps a bad path parameter a documented 4xx
// rather than a uuid.Parse panic or a 500 raised deeper down.
func TestEndpointsRejectAMalformedID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/lessons/not-a-uuid", nil},
		{http.MethodPut, "/admin/lessons/not-a-uuid/activities", lessonhttp.UpdateActivitiesRequest{
			Activities: []lessonhttp.ActivityInput{
				{Position: 1, Kind: "quiz", ContentVersionID: uuid.New(), Weight: 1},
			},
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := routerFor(t, &fakeLessonService{}, allowGuard{})
			rec := serve(router, request(testCase.method, testCase.path, testCase.body, uuid.New()))
			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("expected a 4xx, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminEndpointsRequireAnActor proves the handlers do not fall back to the
// nil UUID when the request carries no authenticated actor.
func TestAdminEndpointsRequireAnActor(t *testing.T) {
	t.Parallel()

	id := uuid.New().String()
	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/admin/courses", lessonhttp.CreateCourseRequest{Slug: "s", Title: "t"}},
		{http.MethodPut, "/admin/lessons/" + id + "/activities", lessonhttp.UpdateActivitiesRequest{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := routerFor(t, &fakeLessonService{}, allowGuard{})
			rec := serve(router, request(testCase.method, testCase.path, testCase.body, uuid.Nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestListCoursesReadsTheDocumentedParameters pins the three query parameters
// the spec declares — and only those. `level` was declared and ignored once;
// this is what stops that returning.
func TestListCoursesReadsTheDocumentedParameters(t *testing.T) {
	t.Parallel()

	svc := &fakeLessonService{}
	router := routerFor(t, svc, allowGuard{})

	rec := serve(router, request(http.MethodGet, "/courses?level=B2&limit=7&offset=14", nil, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.seenLevel == nil || *svc.seenLevel != "B2" {
		t.Errorf("service received level %v, want B2", svc.seenLevel)
	}
	if svc.seenLimit != 7 || svc.seenOffset != 14 {
		t.Errorf("service received limit %d offset %d, want 7 and 14", svc.seenLimit, svc.seenOffset)
	}
}

func TestListCoursesLeavesUnparseablePagingToTheService(t *testing.T) {
	t.Parallel()

	svc := &fakeLessonService{}
	router := routerFor(t, svc, allowGuard{})

	rec := serve(router, request(http.MethodGet, "/courses?limit=many&offset=soon", nil, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.seenLevel != nil {
		t.Errorf("service received level %v, want nil when the caller did not filter", *svc.seenLevel)
	}
	if svc.seenLimit != 0 || svc.seenOffset != 0 {
		t.Errorf("filter = limit %d offset %d, want both left at 0 for the service to default",
			svc.seenLimit, svc.seenOffset)
	}
}

// TestServiceErrorsKeepTheirStatus proves the handlers hand the error to
// WriteProblem rather than flattening it into a 500.
func TestServiceErrorsKeepTheirStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		method string
		path   string
		want   int
	}{
		{"course not found", domain.ErrCourseNotFound, http.MethodGet, "/courses/missing", http.StatusNotFound},
		{"catalogue level rejected", domain.ErrInvalidCEFRLevel, http.MethodGet, "/courses?level=Z9",
			http.StatusUnprocessableEntity},
		{"lesson not found", domain.ErrLessonNotFound, http.MethodGet, "/lessons/" + uuid.New().String(),
			http.StatusNotFound},
		{"lesson locked", domain.ErrLessonLocked, http.MethodGet, "/lessons/" + uuid.New().String(),
			http.StatusForbidden},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			router := routerFor(t, &fakeLessonService{err: testCase.err}, allowGuard{})
			rec := serve(router, request(testCase.method, testCase.path, nil, uuid.New()))
			if rec.Code != testCase.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, testCase.want, rec.Body.String())
			}
		})
	}
}

// TestLockedLessonCarriesItsReason is the acceptance criterion end to end: the
// 403 body has to name the prerequisite, not just say "forbidden".
func TestLockedLessonCarriesItsReason(t *testing.T) {
	t.Parallel()

	const reason = "Complete Unit 1 Lesson 1 first"
	svc := &fakeLessonService{err: domain.ErrLessonLocked.WithMeta("lock_reason", reason)}
	router := routerFor(t, svc, allowGuard{})

	rec := serve(router, request(http.MethodGet, "/lessons/"+uuid.New().String(), nil, uuid.New()))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	var problem struct {
		Code string         `json:"code"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("unmarshal problem: %v", err)
	}
	if problem.Code != "LESSON_LOCKED" {
		t.Errorf("code = %q, want LESSON_LOCKED", problem.Code)
	}
	if got := problem.Meta["lock_reason"]; got != reason {
		t.Errorf("meta lock_reason = %v, want %q", got, reason)
	}
}

func TestCreateCourseRejectsAMalformedBody(t *testing.T) {
	t.Parallel()

	router := routerFor(t, &fakeLessonService{}, allowGuard{})
	req := httptest.NewRequest(http.MethodPost, "/admin/courses", bytes.NewReader([]byte("{not json")))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: uuid.New(), Role: "admin"}))

	rec := serve(router, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("expected a 4xx for a malformed body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCourseSurfacesAServiceRejection(t *testing.T) {
	t.Parallel()

	router := routerFor(t, &fakeLessonService{err: domain.ErrSlugAlreadyExists}, allowGuard{})
	rec := serve(router, request(http.MethodPost, "/admin/courses", lessonhttp.CreateCourseRequest{
		Slug: "taken-slug", Title: "Course", CEFRFrom: "B1", CEFRTo: "B2",
	}, uuid.New()))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}
