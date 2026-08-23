//go:build integration

package lesson_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
	lessonhttp "github.com/fluentra/fluentra/internal/modules/lesson/transport/http"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const moduleDatabase = "fluentra_lesson_module_test"

var (
	pool      *pgxpool.Pool
	moduleDSN string
)

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		// #nosec G101 -- the compose stack's local credentials, not a secret
		base = "postgres://fluentra:fluentra@localhost:5432/fluentra?sslmode=disable"
	}

	dsn, dropDatabase, err := createDatabase(base, moduleDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	pool = created
	moduleDSN = dsn

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func createDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, drop); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	dsn, err := replaceDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), drop)
	}, nil
}

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	sources, err := migrations.Flattened()
	if err != nil {
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

type integrationGuard struct{}

func (integrationGuard) Require(_ context.Context, _ string) error { return nil }

type integrationContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (r *integrationContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	return r.versions[id], nil
}

func (r *integrationContentReader) GetManyVersions(
	_ context.Context, ids []uuid.UUID,
) (map[uuid.UUID]*contentcontract.Version, error) {
	res := make(map[uuid.UUID]*contentcontract.Version)
	for _, id := range ids {
		if v, ok := r.versions[id]; ok {
			res[id] = v
		}
	}
	return res, nil
}

func (r *integrationContentReader) Browse(
	_ context.Context, _ contentcontract.BrowseFilter,
) ([]*contentcontract.Version, int, error) {
	return nil, 0, nil
}

const (
	roleUser           = "user"
	roleAdmin          = "admin"
	statusPublished    = "published"
	skillVocabulary    = "vocabulary"
	kindMultipleChoice = "multiple_choice"
)

func TestLessonLifecycle_Integration(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	adminID := uuid.New()
	learnerID := uuid.New()

	v1ID := uuid.New()
	v2ID := uuid.New()

	contentReader := &integrationContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			v1ID: {ID: v1ID, Kind: kindMultipleChoice, Status: statusPublished},
			v2ID: {ID: v2ID, Kind: "gap_fill", Status: statusPublished},
		},
	}

	mod := lesson.New(lesson.Deps{
		Pool:     pool,
		Clock:    clock.NewFake(time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)),
		Guard:    integrationGuard{},
		Content:  contentReader,
		Unlocker: nil,
	})

	router := chi.NewRouter()
	router.Group(func(auth chi.Router) {
		mod.Routes(auth)
		auth.Group(func(admin chi.Router) {
			mod.AdminRoutes(admin)
		})
	})

	// 1. Create Course
	createReq := lessonhttp.CreateCourseRequest{
		Slug:           "ielts-integration-course",
		Title:          "IELTS Integration Course",
		Description:    "Full integration test course",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		EstimatedHours: 40,
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/admin/courses", bytes.NewReader(body))
	req = req.WithContext(httpx.WithActor(ctx, httpx.Actor{UserID: adminID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create course status = %d, want 201. Body: %s", rec.Code, rec.Body.String())
	}

	var createdCourse lessonhttp.CourseSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &createdCourse); err != nil {
		t.Fatalf("unmarshal course: %v", err)
	}

	// 2. Build the unit and the lesson.
	//
	// Through the repository, not raw INSERTs. The spec gives Phase 2 no HTTP
	// route for creating a unit or a lesson — only POST /admin/courses and
	// PUT /admin/lessons/{id}/activities exist — so this layer is the only
	// caller these methods have until P11.1's seed arrives, and writing the rows
	// by hand here would leave the mapping between the schema and the domain
	// types untested.
	repo := repository.New(pool)

	unit, err := repo.CreateUnit(ctx, repository.CreateUnitParams{
		CourseID:    createdCourse.ID,
		Position:    1,
		Title:       "Unit 1: Foundations",
		Description: "Overview",
	})
	if err != nil {
		t.Fatalf("create unit: %v", err)
	}

	lesson, err := repo.CreateLesson(ctx, repository.CreateLessonParams{
		UnitID:           unit.ID,
		Position:         1,
		Title:            "Lesson 1: Vocabulary Intro",
		SkillFocus:       skillVocabulary,
		EstimatedMinutes: 15,
		Status:           statusPublished,
	})
	if err != nil {
		t.Fatalf("create lesson: %v", err)
	}
	lessonID := lesson.ID.String()

	// A second lesson, left in draft: the published-only reads below have
	// nothing to prove unless something is deliberately unpublished.
	lesson2, err := repo.CreateLesson(ctx, repository.CreateLessonParams{
		UnitID:     unit.ID,
		Position:   2,
		Title:      "Lesson 2: Vocabulary Practice",
		SkillFocus: skillVocabulary,
		Status:     "draft",
	})
	if err != nil {
		t.Fatalf("create second lesson: %v", err)
	}

	// A draft lesson must not reach a learner, so read the unit back and check
	// the published-only query filters it out while the authoring one does not.
	allLessons, err := repo.ListLessonsByUnitID(ctx, unit.ID)
	if err != nil {
		t.Fatalf("list lessons by unit: %v", err)
	}
	if len(allLessons) != 2 {
		t.Fatalf("authoring read returned %d lessons, want both", len(allLessons))
	}

	if _, err := repo.UpdateLesson(ctx, repository.UpdateLessonParams{
		ID:               lesson.ID,
		Title:            "Lesson 1: Vocabulary Intro",
		SkillFocus:       skillVocabulary,
		EstimatedMinutes: 15,
		Status:           statusPublished,
	}); err != nil {
		t.Fatalf("update lesson: %v", err)
	}

	// Publish the course so it appears in the catalogue.
	if _, err := pool.Exec(ctx,
		`UPDATE learn.courses SET status = 'published' WHERE id = $1`, createdCourse.ID); err != nil {
		t.Fatalf("publish course: %v", err)
	}

	publishedOnly, err := repo.ListPublishedLessonsByCourseID(ctx, createdCourse.ID)
	if err != nil {
		t.Fatalf("list published lessons: %v", err)
	}
	if len(publishedOnly) != 1 || publishedOnly[0].ID != lesson.ID {
		t.Errorf("learner read returned %d lessons, want only the published one", len(publishedOnly))
	}

	// 3. Update activities via Admin endpoint
	actReq := lessonhttp.UpdateActivitiesRequest{
		Activities: []lessonhttp.ActivityInput{
			{Position: 1, Kind: kindMultipleChoice, ContentVersionID: v1ID, Weight: 2},
			{Position: 2, Kind: "gap_fill", ContentVersionID: v2ID, Weight: 3},
		},
	}
	actBody, _ := json.Marshal(actReq)
	putReq := httptest.NewRequest(http.MethodPut, "/admin/lessons/"+lessonID+"/activities", bytes.NewReader(actBody))
	putReq = putReq.WithContext(httpx.WithActor(ctx, httpx.Actor{UserID: adminID, Role: roleAdmin}))
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("update activities status = %d, want 200. Body: %s", putRec.Code, putRec.Body.String())
	}

	// 4. Publish the draft lesson through the admin endpoint this task added.
	publishSecondLesson(ctx, t, router, adminID, lesson2.ID, v1ID)

	// 5. Verify Catalogue & Details
	testLearnerEndpoints(ctx, t, router, learnerID, lessonID, lesson2.ID, v1ID)
}

// publishSecondLesson drives POST /admin/lessons/{id}/publish, first against a
// lesson with no activities at all. That 422 is the half of the publish guard an
// end-to-end run can reach here: the fake content reader holds only published
// versions, so the 409 for draft content is proved in the service suite instead.
func publishSecondLesson(
	ctx context.Context, t *testing.T, router chi.Router, adminID, lesson2ID, v1ID uuid.UUID,
) {
	t.Helper()

	publish := func(label string, want int) {
		req := httptest.NewRequest(http.MethodPost, "/admin/lessons/"+lesson2ID.String()+"/publish", nil)
		req = req.WithContext(httpx.WithActor(ctx, httpx.Actor{UserID: adminID, Role: roleAdmin}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s = %d, want %d. Body: %s", label, rec.Code, want, rec.Body.String())
		}
	}

	publish("publish a lesson with no activities", http.StatusUnprocessableEntity)

	act2Req := lessonhttp.UpdateActivitiesRequest{
		Activities: []lessonhttp.ActivityInput{
			{Position: 1, Kind: kindMultipleChoice, ContentVersionID: v1ID, Weight: 2},
		},
	}
	act2Body, _ := json.Marshal(act2Req)
	put2Req := httptest.NewRequest(
		http.MethodPut, "/admin/lessons/"+lesson2ID.String()+"/activities", bytes.NewReader(act2Body))
	put2Req = put2Req.WithContext(httpx.WithActor(ctx, httpx.Actor{UserID: adminID, Role: roleAdmin}))
	put2Rec := httptest.NewRecorder()
	router.ServeHTTP(put2Rec, put2Req)
	if put2Rec.Code != http.StatusOK {
		t.Fatalf("update lesson 2 activities = %d, want 200. Body: %s", put2Rec.Code, put2Rec.Body.String())
	}

	publish("publish lesson 2", http.StatusOK)
	// Publishing twice is a no-op, not a conflict: the admin UI retries.
	publish("re-publish lesson 2", http.StatusOK)
}

func testLearnerEndpoints(
	ctx context.Context, t *testing.T, router chi.Router,
	learnerID uuid.UUID, lessonID string, lesson2ID, v1ID uuid.UUID,
) {
	learnerGet := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(httpx.WithActor(ctx, httpx.Actor{UserID: learnerID, Role: roleUser}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	assertCatalogue(t, learnerGet("/courses"))
	assertCourseTree(t, learnerGet("/courses/ielts-integration-course"), lesson2ID)
	assertLessonDetail(t, learnerGet("/lessons/"+lessonID), v1ID)
}

func assertCatalogue(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("get courses status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}

	var catResp lessonhttp.CourseListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &catResp); err != nil {
		t.Fatalf("unmarshal catalogue: %v", err)
	}
	if len(catResp.Courses) == 0 {
		t.Fatal("catalogue empty; expected at least 1 course")
	}
}

func assertCourseTree(t *testing.T, rec *httptest.ResponseRecorder, lesson2ID uuid.UUID) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("get course detail status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}

	var detailResp lessonhttp.CourseDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal course detail: %v", err)
	}
	// Both lessons are published by now, the second through the endpoint this
	// task added, so the tree carries two.
	if len(detailResp.Units) != 1 || len(detailResp.Units[0].Lessons) != 2 {
		t.Fatalf("unexpected course detail units/lessons: %+v", detailResp)
	}

	for _, l := range detailResp.Units[0].Lessons {
		if l.ID == lesson2ID {
			return
		}
	}
	t.Errorf("the lesson published through the API is missing from the course tree")
}

func assertLessonDetail(t *testing.T, rec *httptest.ResponseRecorder, v1ID uuid.UUID) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("get lesson detail status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}

	var lessonDetail lessonhttp.LessonDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &lessonDetail); err != nil {
		t.Fatalf("unmarshal lesson detail: %v", err)
	}
	if len(lessonDetail.Activities) != 2 {
		t.Fatalf("got %d activities in lesson detail, want 2", len(lessonDetail.Activities))
	}
	if lessonDetail.Activities[0].Content == nil || lessonDetail.Activities[0].Content.ID != v1ID {
		t.Errorf("activity 0 content not resolved: %+v", lessonDetail.Activities[0])
	}
}
