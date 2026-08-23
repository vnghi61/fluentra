//go:build integration

package lesson_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/repository"
	lessonservice "github.com/fluentra/fluentra/internal/modules/lesson/service"
	lessonhttp "github.com/fluentra/fluentra/internal/modules/lesson/transport/http"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// The service suite proves the caching logic against an in-memory Cache[T].
// That fake is the caching code's own idea of a cache, so on its own it cannot
// fail the way the real pair can: a key the service builds and a key it deletes
// can disagree, `GetOrLoad` writes its value back on a goroutine rather than
// before it returns, and a Redis that is simply unreachable takes a different
// branch from one that misses. Everything below runs against a real Redis and
// counts the queries Postgres actually received.

// queryTracer counts the statements a pool sends. It is what makes "the second
// read does not hit the database" an assertion rather than a stopwatch.
type queryTracer struct {
	queries atomic.Int64
}

func (q *queryTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData,
) context.Context {
	q.queries.Add(1)
	return ctx
}

func (q *queryTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {}

func newTracedPool(t *testing.T) (*pgxpool.Pool, *queryTracer) {
	t.Helper()

	config, err := pgxpool.ParseConfig(moduleDSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	tracer := &queryTracer{}
	config.ConnConfig.Tracer = tracer

	traced, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("traced pool: %v", err)
	}
	t.Cleanup(traced.Close)
	return traced, tracer
}

func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("reach redis at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func lessonCaches(client redis.Cmdable) lessonservice.LessonCaches {
	return lessonservice.LessonCaches{
		Detail:    cache.NewRedisCache[*lessonservice.LessonDetailDTO](client),
		Tree:      cache.NewRedisCache[*lessonservice.CourseTreeData](client),
		Catalogue: cache.NewRedisCache[*lessonservice.CatalogueData](client),
		Gen:       cache.NewRedisCache[int64](client),
	}
}

// cacheEnv gives each test its own key namespace. The suite runs with -race
// against one shared Redis, and a fixed env would make two tests each other's
// flakes — and would leave keys behind for the next run to read as hits.
func cacheEnv(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("itest-%s-%d", t.Name(), time.Now().UnixNano())
}

// waitForKey blocks until a key exists. GetOrLoad writes its loaded value back
// on a goroutine, so a second read issued immediately would race the first
// read's Set and miss for a reason that has nothing to do with invalidation.
func waitForKey(t *testing.T, client *redis.Client, key string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, err := client.Exists(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("EXISTS %s: %v", key, err)
		}
		if n == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s was never written to redis", key)
}

type cacheFixture struct {
	router    chi.Router
	redis     *redis.Client
	tracer    *queryTracer
	module    *lesson.Module
	env       string
	slug      string
	courseID  uuid.UUID
	lessonID  uuid.UUID
	draftID   uuid.UUID
	versionID uuid.UUID
	adminID   uuid.UUID
	learnerID uuid.UUID
}

// newCacheFixture builds a published course with one published lesson and one
// draft lesson, both carrying an activity, served through a module wired to a
// real Redis.
func newCacheFixture(t *testing.T) *cacheFixture {
	t.Helper()

	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	client := newRedisClient(t)
	traced, tracer := newTracedPool(t)
	env := cacheEnv(t)
	versionID := uuid.New()

	fixture := &cacheFixture{
		redis:     client,
		tracer:    tracer,
		env:       env,
		slug:      fmt.Sprintf("cache-course-%d", time.Now().UnixNano()),
		versionID: versionID,
		adminID:   uuid.New(),
		learnerID: uuid.New(),
	}

	fixture.module = lesson.New(lesson.Deps{
		Pool:   traced,
		Guard:  integrationGuard{},
		Caches: lessonCaches(client),
		Env:    env,
		Content: &integrationContentReader{versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Kind: kindMultipleChoice, Status: statusPublished},
		}},
	})

	fixture.router = chi.NewRouter()
	fixture.router.Group(func(auth chi.Router) {
		fixture.module.Routes(auth)
		auth.Group(func(admin chi.Router) {
			fixture.module.AdminRoutes(admin)
		})
	})

	// Seeding goes through the untraced pool so the fixture's own writes are
	// not counted as the reads under test.
	repo := repository.New(pool)

	course, err := repo.CreateCourse(ctx, repository.CreateCourseParams{
		Slug:           fixture.slug,
		Title:          "Cache Course",
		Description:    "Course tree caching",
		CEFRFrom:       "B1",
		CEFRTo:         "B2",
		Status:         statusPublished,
		EstimatedHours: 10,
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	fixture.courseID = course.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, course.ID)
		_ = client.Del(context.Background(), fixture.keys()...).Err()
	})

	unit, err := repo.CreateUnit(ctx, repository.CreateUnitParams{
		CourseID: course.ID,
		Position: 1,
		Title:    "Unit 1",
	})
	if err != nil {
		t.Fatalf("create unit: %v", err)
	}

	published, err := repo.CreateLesson(ctx, repository.CreateLessonParams{
		UnitID:           unit.ID,
		Position:         1,
		Title:            "Published Lesson",
		SkillFocus:       skillVocabulary,
		EstimatedMinutes: 15,
		Status:           statusPublished,
	})
	if err != nil {
		t.Fatalf("create published lesson: %v", err)
	}
	fixture.lessonID = published.ID

	draft, err := repo.CreateLesson(ctx, repository.CreateLessonParams{
		UnitID:     unit.ID,
		Position:   2,
		Title:      "Draft Lesson",
		SkillFocus: skillVocabulary,
		Status:     "draft",
	})
	if err != nil {
		t.Fatalf("create draft lesson: %v", err)
	}
	fixture.draftID = draft.ID

	for _, id := range []uuid.UUID{published.ID, draft.ID} {
		fixture.putActivities(t, id)
	}

	return fixture
}

func (f *cacheFixture) treeKey() string {
	return cache.Key(f.env, "lesson", "tree", f.slug, 1)
}

func (f *cacheFixture) detailKey(id uuid.UUID) string {
	return cache.Key(f.env, "lesson", "detail", id.String(), 1)
}

func (f *cacheFixture) generationKey() string {
	return cache.Key(f.env, "lesson", "catalogue", "generation", 1)
}

// generation reads the counter folded into every catalogue key.
func (f *cacheFixture) generation(t *testing.T) int64 {
	t.Helper()

	raw, err := f.redis.Get(context.Background(), f.generationKey()).Result()
	if errors.Is(err, redis.Nil) {
		return 1
	}
	if err != nil {
		t.Fatalf("read generation counter: %v", err)
	}
	gen, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatalf("generation counter %q is not a number: %v", raw, err)
	}
	return gen
}

func (f *cacheFixture) keys() []string {
	return []string{
		f.treeKey(),
		f.detailKey(f.lessonID),
		f.detailKey(f.draftID),
		f.generationKey(),
	}
}

func (f *cacheFixture) putActivities(t *testing.T, lessonID uuid.UUID) {
	t.Helper()

	body, _ := json.Marshal(lessonhttp.UpdateActivitiesRequest{
		Activities: []lessonhttp.ActivityInput{
			{Position: 1, Kind: kindMultipleChoice, ContentVersionID: f.versionID, Weight: 2},
		},
	})
	req := httptest.NewRequest(
		http.MethodPut, "/admin/lessons/"+lessonID.String()+"/activities", bytes.NewReader(body))
	req = req.WithContext(httpx.WithActor(context.Background(), httpx.Actor{UserID: f.adminID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed activities for %s = %d. Body: %s", lessonID, rec.Code, rec.Body.String())
	}
}

func (f *cacheFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(httpx.WithActor(context.Background(), httpx.Actor{UserID: f.learnerID, Role: roleUser}))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f *cacheFixture) publish(t *testing.T, lessonID uuid.UUID) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/admin/lessons/"+lessonID.String()+"/publish", nil)
	req = req.WithContext(httpx.WithActor(context.Background(), httpx.Actor{UserID: f.adminID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish %s = %d. Body: %s", lessonID, rec.Code, rec.Body.String())
	}
}

func TestCourseTreeIsCachedAndInvalidatedByPublish_Integration(t *testing.T) {
	f := newCacheFixture(t)
	ctx := context.Background()

	if rec := f.get(t, "/courses/"+f.slug); rec.Code != http.StatusOK {
		t.Fatalf("first course read = %d. Body: %s", rec.Code, rec.Body.String())
	}
	waitForKey(t, f.redis, f.treeKey())

	before := f.tracer.queries.Load()
	if rec := f.get(t, "/courses/"+f.slug); rec.Code != http.StatusOK {
		t.Fatalf("second course read = %d", rec.Code)
	}
	if after := f.tracer.queries.Load(); after != before {
		t.Errorf("the second course read issued %d queries, want 0", after-before)
	}

	// Publishing the draft lesson must remove the tree the reads were served
	// from. This is the assertion the whole task turns on: without it the
	// author reloads the course and does not see what they just published.
	f.publish(t, f.draftID)

	exists, err := f.redis.Exists(ctx, f.treeKey()).Result()
	if err != nil {
		t.Fatalf("EXISTS tree key: %v", err)
	}
	if exists != 0 {
		t.Fatal("publishing a lesson left the course tree in redis")
	}

	rec := f.get(t, "/courses/"+f.slug)
	if rec.Code != http.StatusOK {
		t.Fatalf("course read after publish = %d", rec.Code)
	}
	var detail lessonhttp.CourseDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal course detail: %v", err)
	}
	if len(detail.Units) != 1 || len(detail.Units[0].Lessons) != 2 {
		t.Fatalf("the reread tree does not carry the newly published lesson: %+v", detail)
	}
}

func TestCatalogueGenerationIsBumpedByPublish_Integration(t *testing.T) {
	f := newCacheFixture(t)

	if rec := f.get(t, "/courses"); rec.Code != http.StatusOK {
		t.Fatalf("first catalogue read = %d. Body: %s", rec.Code, rec.Body.String())
	}

	// The catalogue key carries the generation, so it cannot be named from the
	// outside as directly as the tree key; wait for the read to settle and
	// prove the hit through the query count instead.
	time.Sleep(200 * time.Millisecond)
	before := f.tracer.queries.Load()
	if rec := f.get(t, "/courses"); rec.Code != http.StatusOK {
		t.Fatalf("second catalogue read = %d", rec.Code)
	}
	if after := f.tracer.queries.Load(); after != before {
		t.Errorf("the second catalogue read issued %d queries, want 0", after-before)
	}

	// An absolute value would be brittle: the fixture's own activity writes
	// invalidate too, so what a publish has to guarantee is the increment.
	genBefore := f.generation(t)

	f.publish(t, f.draftID)

	if genAfter := f.generation(t); genAfter != genBefore+1 {
		t.Errorf("generation counter went %d -> %d, want +1 for one publish", genBefore, genAfter)
	}

	// A bumped generation names a key nothing has written, so the next read
	// must reach the database again.
	before = f.tracer.queries.Load()
	if rec := f.get(t, "/courses"); rec.Code != http.StatusOK {
		t.Fatalf("catalogue read after publish = %d", rec.Code)
	}
	if f.tracer.queries.Load() == before {
		t.Error("the catalogue was still served from cache after a publish bumped the generation")
	}
}

func TestContentArchivedInvalidatesLessonDetail_Integration(t *testing.T) {
	f := newCacheFixture(t)
	ctx := context.Background()

	if rec := f.get(t, "/lessons/"+f.lessonID.String()); rec.Code != http.StatusOK {
		t.Fatalf("lesson read = %d. Body: %s", rec.Code, rec.Body.String())
	}
	waitForKey(t, f.redis, f.detailKey(f.lessonID))

	// Through the bus, not by calling the service: the wiring in
	// cmd/worker/main.go is what makes this consumer exist, and a direct call
	// would pass even if the topic or the payload shape were wrong.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	if err := f.module.Subscribe(bus); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	payload, _ := json.Marshal(contentcontract.Archived{
		ItemID:     uuid.New(),
		VersionID:  f.versionID,
		OccurredAt: time.Now(),
	})
	if err := bus.Publish(ctx, eventbus.Message{
		ID:      uuid.New(),
		Topic:   contentcontract.EventContentArchived,
		Payload: payload,
	}); err != nil {
		t.Fatalf("publish content.archived: %v", err)
	}

	exists, err := f.redis.Exists(ctx, f.detailKey(f.lessonID)).Result()
	if err != nil {
		t.Fatalf("EXISTS detail key: %v", err)
	}
	if exists != 0 {
		t.Error("content.archived left the lesson detail of an archived version in redis")
	}
}

func TestLessonPublishedConsumerIsTheInvalidationBackstop_Integration(t *testing.T) {
	f := newCacheFixture(t)
	ctx := context.Background()

	if rec := f.get(t, "/courses/"+f.slug); rec.Code != http.StatusOK {
		t.Fatalf("course read = %d", rec.Code)
	}
	waitForKey(t, f.redis, f.treeKey())

	// The worker runs the same consumer against the same Redis. Delivering the
	// event on its own — as it would be delivered to a process that did not
	// serve the publish — must clear the tree.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	if err := f.module.Subscribe(bus); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	payload, _ := json.Marshal(lessoncontract.Published{
		LessonID:   f.lessonID,
		CourseID:   f.courseID,
		SkillFocus: skillVocabulary,
		OccurredAt: time.Now(),
	})
	if err := bus.Publish(ctx, eventbus.Message{
		ID:      uuid.New(),
		Topic:   lessoncontract.EventLessonPublished,
		Payload: payload,
	}); err != nil {
		t.Fatalf("publish lesson.published: %v", err)
	}

	exists, err := f.redis.Exists(ctx, f.treeKey()).Result()
	if err != nil {
		t.Fatalf("EXISTS tree key: %v", err)
	}
	if exists != 0 {
		t.Error("the lesson.published consumer did not clear the course tree")
	}
}

func TestReadsSurviveAnUnreachableRedis_Integration(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	f := newCacheFixture(t)

	// A client pointed at a port nothing listens on is the closest a test can
	// get to `docker stop redis` without taking the container away from every
	// other test in the run. Every call fails to connect, which is the branch
	// that matters: an outage is not a miss.
	down := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = down.Close() })
	if err := down.Ping(context.Background()).Err(); err == nil {
		t.Skip("something is listening on 127.0.0.1:1; cannot simulate an outage")
	}

	traced, _ := newTracedPool(t)
	degraded := lesson.New(lesson.Deps{
		Pool:   traced,
		Guard:  integrationGuard{},
		Caches: lessonCaches(down),
		Env:    f.env,
		Content: &integrationContentReader{versions: map[uuid.UUID]*contentcontract.Version{
			f.versionID: {ID: f.versionID, Kind: kindMultipleChoice, Status: statusPublished},
		}},
	})

	router := chi.NewRouter()
	router.Group(func(auth chi.Router) {
		degraded.Routes(auth)
		auth.Group(func(admin chi.Router) {
			degraded.AdminRoutes(admin)
		})
	})

	serve := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(httpx.WithActor(
			context.Background(), httpx.Actor{UserID: f.learnerID, Role: roleUser}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve("/courses/" + f.slug); rec.Code != http.StatusOK {
		t.Errorf("GET /courses/{slug} with redis down = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	if rec := serve("/lessons/" + f.lessonID.String()); rec.Code != http.StatusOK {
		t.Errorf("GET /lessons/{id} with redis down = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
	if rec := serve("/courses"); rec.Code != http.StatusOK {
		t.Errorf("GET /courses with redis down = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}

	// A publish must not fail because the invalidation could not be written.
	req := httptest.NewRequest(http.MethodPost, "/admin/lessons/"+f.draftID.String()+"/publish", nil)
	req = req.WithContext(httpx.WithActor(context.Background(), httpx.Actor{UserID: f.adminID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("publish with redis down = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}
}
