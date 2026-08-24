package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// memCache implements cache.Cache in-memory for testing caching & invalidation.
type memCache[T any] struct {
	store map[string]T
}

func newMemCache[T any]() *memCache[T] {
	return &memCache[T]{store: make(map[string]T)}
}

func (m *memCache[T]) Get(_ context.Context, key string) (T, error) {
	if val, ok := m.store[key]; ok {
		return val, nil
	}
	var zero T
	return zero, cache.ErrMiss
}

func (m *memCache[T]) Set(_ context.Context, key string, value T, _ time.Duration) error {
	m.store[key] = value
	return nil
}

func (m *memCache[T]) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func (m *memCache[T]) GetOrLoad(
	ctx context.Context, key string, _ time.Duration, loader func(context.Context) (T, error),
) (T, error) {
	if val, ok := m.store[key]; ok {
		return val, nil
	}
	loaded, err := loader(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	m.store[key] = loaded
	return loaded, nil
}

func buildCourseFixture(
	courseID uuid.UUID,
	unitCount int,
	lessonsPerUnit int,
) (
	*lessoncontract.Course,
	[]*lessoncontract.Unit,
	[]*lessoncontract.Lesson,
	map[uuid.UUID]*lessoncontract.ActivityHierarchy,
	[]uuid.UUID,
) {
	course := &lessoncontract.Course{
		ID:    courseID,
		Slug:  fmt.Sprintf("course-%s", courseID.String()[:8]),
		Title: "Test Course",
	}

	var units []*lessoncontract.Unit
	var allLessons []*lessoncontract.Lesson
	hierarchies := make(map[uuid.UUID]*lessoncontract.ActivityHierarchy)
	var allActIDs []uuid.UUID

	for u := 1; u <= unitCount; u++ {
		unitID := uuid.New()
		unit := &lessoncontract.Unit{
			ID:       unitID,
			CourseID: courseID,
			Position: u,
			Title:    fmt.Sprintf("Unit %d", u),
		}
		units = append(units, unit)

		for l := 1; l <= lessonsPerUnit; l++ {
			lessonID := uuid.New()
			actID := uuid.New()
			allActIDs = append(allActIDs, actID)

			act := lessoncontract.Activity{
				ID:       actID,
				LessonID: lessonID,
				Position: 1,
				Kind:     testKindQuiz,
			}
			lesson := &lessoncontract.Lesson{
				ID:               lessonID,
				UnitID:           unitID,
				Position:         l,
				Title:            fmt.Sprintf("Lesson %d-%d", u, l),
				SkillFocus:       testSkillGrammar,
				EstimatedMinutes: 10,
				Status:           "published",
				Activities:       []lessoncontract.Activity{act},
			}
			allLessons = append(allLessons, lesson)

			hierarchies[actID] = &lessoncontract.ActivityHierarchy{
				ActivityID:       actID,
				LessonID:         lessonID,
				UnitID:           unitID,
				CourseID:         courseID,
				Kind:             testKindQuiz,
				LessonSkillFocus: testSkillGrammar,
			}
		}
	}

	return course, units, allLessons, hierarchies, allActIDs
}

// TestDashboard_QueryBudget_ConstantInCourseSize proves that the query count for
// GET /me/dashboard does not scale with course size (small fixture vs large fixture).
func TestDashboard_QueryBudget_ConstantInCourseSize(t *testing.T) {
	ctx := context.Background()

	runScenario := func(t *testing.T, unitCount, lessonsPerUnit int) (map[string]int, map[string]int) {
		t.Helper()
		repo := newFakeRepo()
		reader := &fakeLessonReader{
			calls:         map[string]int{},
			hierarchy:     make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
			lessons:       make(map[uuid.UUID]*lessoncontract.Lesson),
			unitLesson:    make(map[uuid.UUID][]*lessoncontract.Lesson),
			courseUnits:   make(map[uuid.UUID][]*lessoncontract.Unit),
			courseLessons: make(map[uuid.UUID][]*lessoncontract.Lesson),
			courseActs:    make(map[uuid.UUID][]uuid.UUID),
			prereqs:       make(map[uuid.UUID][]lessoncontract.PrerequisiteItem),
		}
		graders := domain.NewGraderRegistry()
		_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

		svc := service.New(service.Deps{
			Repo:    repo,
			Lesson:  reader,
			Graders: graders,
			Clock:   clock.NewFake(time.Now()),
		})

		userID := uuid.New()
		courseID := uuid.New()

		_, units, lessons, hierarchies, actIDs := buildCourseFixture(courseID, unitCount, lessonsPerUnit)
		reader.courseUnits[courseID] = units
		reader.courseLessons[courseID] = lessons
		reader.courseActs[courseID] = actIDs
		for _, l := range lessons {
			reader.lessons[l.ID] = l
			reader.unitLesson[l.UnitID] = append(reader.unitLesson[l.UnitID], l)
		}
		for aid, h := range hierarchies {
			reader.hierarchy[aid] = h
		}

		// User enrolls in the course
		_, err := repo.CreateEnrollment(ctx, userID, courseID, domain.StatusEnrollmentActive, time.Now())
		if err != nil {
			t.Fatalf("CreateEnrollment: %v", err)
		}

		// Reset query and call counters before dashboard resolution
		repo.queryCounter.Store(0)
		reader.callMu.Lock()
		reader.calls = map[string]int{}
		reader.callMu.Unlock()

		dash, err := svc.Dashboard(ctx, userID)
		if err != nil {
			t.Fatalf("Dashboard: %v", err)
		}
		if dash.State != domain.DashboardStateInProgress {
			t.Fatalf("got state %s, want in_progress", dash.State)
		}
		if dash.NextActivity == nil {
			t.Fatalf("expected next activity to be present")
		}
		if dash.DueReviewsCount != 0 {
			t.Errorf("due_reviews_count = %d, want 0", dash.DueReviewsCount)
		}

		readerCalls := make(map[string]int)
		reader.callMu.Lock()
		for k, v := range reader.calls {
			readerCalls[k] = v
		}
		reader.callMu.Unlock()

		return map[string]int{
			"totalQueries": int(repo.queryCounter.Load()),
		}, readerCalls
	}

	// Fixture A: 3 units, 10 lessons each = 30 lessons
	repoCountsA, readerCallsA := runScenario(t, 3, 10)

	// Fixture C: the same 3 units carrying 40 lessons each = 120 lessons. This is
	// the pair that proves the claim: the shape of the course changed by 90
	// lessons and every count has to be identical.
	repoCountsC, readerCallsC := runScenario(t, 3, 40)

	// Fixture B: 20 units, 20 lessons each = 400 lessons
	repoCountsB, readerCallsB := runScenario(t, 20, 20)

	// Constant in lessons: four times the lessons behind the same units costs
	// exactly what A cost, read for read.
	assertSameCounts(t, "lessons per unit", repoCountsA, readerCallsA, repoCountsC, readerCallsC,
		[]string{"ListUnitsByCourseID", "ListLessons", "GetLesson", "NextLesson"})

	// Constant in repository reads across course shapes, and no tree walking.
	assertSameCounts(t, "course shape", repoCountsA, readerCallsA, repoCountsB, readerCallsB,
		[]string{"GetLesson", "NextLesson"})
	if readerCallsA["GetLesson"] > 1 || readerCallsB["GetLesson"] > 1 {
		t.Errorf("GetLesson called more than once: A=%d, B=%d", readerCallsA["GetLesson"], readerCallsB["GetLesson"])
	}

	// And the one read that is *not* constant, asserted rather than avoided.
	//
	// Next-activity resolution lists the lessons of each unit (P8.4 §7), so the
	// dashboard costs one ListLessons per unit: 3 for A, 20 for B. That is the
	// budget this task inherited and chose to keep — see P8.5 §4. Pinning the
	// numbers here means collapsing it later is a deliberate edit to this test,
	// not something that drifts in unnoticed, and a regression that reintroduces
	// a per-*lesson* read fails the A/C comparison above.
	if readerCallsA["ListLessons"] != 3 {
		t.Errorf("ListLessons for 3 units: got %d, want 3 (one per unit)", readerCallsA["ListLessons"])
	}
	if readerCallsB["ListLessons"] != 20 {
		t.Errorf("ListLessons for 20 units: got %d, want 20 (one per unit)", readerCallsB["ListLessons"])
	}
}

// assertSameCounts compares two runs read for read, over the contract methods
// named and the total repository count.
func assertSameCounts(
	t *testing.T,
	what string,
	repoA map[string]int, readerA map[string]int,
	repoB map[string]int, readerB map[string]int,
	methods []string,
) {
	t.Helper()
	if repoA["totalQueries"] != repoB["totalQueries"] {
		t.Errorf("repo queries grew with %s: %d then %d", what, repoA["totalQueries"], repoB["totalQueries"])
	}
	for _, method := range methods {
		if readerA[method] != readerB[method] {
			t.Errorf("%s grew with %s: %d then %d", method, what, readerA[method], readerB[method])
		}
	}
}

// TestProgress_QueryBudget_BoundedInCourseCount verifies that /me/progress costs a bounded
// number of queries for a learner in multiple courses.
func TestProgress_QueryBudget_BoundedInCourseCount(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:         map[string]int{},
		courseActs:    make(map[uuid.UUID][]uuid.UUID),
		hierarchy:     make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:       make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson:    make(map[uuid.UUID][]*lessoncontract.Lesson),
		courseUnits:   make(map[uuid.UUID][]*lessoncontract.Unit),
		courseLessons: make(map[uuid.UUID][]*lessoncontract.Lesson),
		prereqs:       make(map[uuid.UUID][]lessoncontract.PrerequisiteItem),
	}

	svc := service.New(service.Deps{
		Repo:   repo,
		Lesson: reader,
		Clock:  clock.NewFake(time.Now()),
	})

	userID := uuid.New()
	course1 := uuid.New()
	course2 := uuid.New()
	course3 := uuid.New()

	act1, act2, act3 := uuid.New(), uuid.New(), uuid.New()
	reader.courseActs[course1] = []uuid.UUID{act1}
	reader.courseActs[course2] = []uuid.UUID{act2}
	reader.courseActs[course3] = []uuid.UUID{act3}

	_, _ = repo.CreateEnrollment(ctx, userID, course1, domain.StatusEnrollmentActive, time.Now())
	_, _ = repo.CreateEnrollment(ctx, userID, course2, domain.StatusEnrollmentActive, time.Now())
	_, _ = repo.CreateEnrollment(ctx, userID, course3, domain.StatusEnrollmentActive, time.Now())

	repo.queryCounter.Store(0)
	reader.callMu.Lock()
	reader.calls = map[string]int{}
	reader.callMu.Unlock()

	prog, err := svc.Progress(ctx, userID)
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}

	if len(prog.Courses) != 3 {
		t.Fatalf("got %d courses, want 3", len(prog.Courses))
	}

	// Assert bounded query counts:
	// ListActivitiesByCourseIDs called exactly ONCE for all 3 courses
	if count := reader.callCount("ListActivitiesByCourseIDs"); count != 1 {
		t.Errorf("ListActivitiesByCourseIDs called %d times, want 1", count)
	}

	// Four repository reads answer /me/progress: enrolments, mastery, activity
	// progress and course progress. The number is pinned rather than bounded,
	// because "bounded" is what a per-course read also looks like on a fixture
	// of one course.
	threeCourseQueries := repo.queryCounter.Load()
	if threeCourseQueries != 4 {
		t.Errorf("repo queries for 3 courses: got %d, want 4", threeCourseQueries)
	}

	// The same learner in one course costs exactly the same.
	single := uuid.New()
	act4 := uuid.New()
	reader.courseActs[course1] = []uuid.UUID{act1, act4}
	_, _ = repo.CreateEnrollment(ctx, single, course1, domain.StatusEnrollmentActive, time.Now())

	repo.queryCounter.Store(0)
	if _, err := svc.Progress(ctx, single); err != nil {
		t.Fatalf("Progress for one course: %v", err)
	}
	if oneCourseQueries := repo.queryCounter.Load(); oneCourseQueries != threeCourseQueries {
		t.Errorf("repo queries grew with course count: 1 course=%d, 3 courses=%d",
			oneCourseQueries, threeCourseQueries)
	}
}

// TestDashboard_CacheHitAndPostSubmitInvalidation proves Redis cache hit on repeat reads,
// and invalidation immediately following attempt submission.
func TestDashboard_CacheHitAndPostSubmitInvalidation(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:         map[string]int{},
		hierarchy:     make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:       make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson:    make(map[uuid.UUID][]*lessoncontract.Lesson),
		courseUnits:   make(map[uuid.UUID][]*lessoncontract.Unit),
		courseLessons: make(map[uuid.UUID][]*lessoncontract.Lesson),
		courseActs:    make(map[uuid.UUID][]uuid.UUID),
		prereqs:       make(map[uuid.UUID][]lessoncontract.PrerequisiteItem),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	dashCache := newMemCache[*domain.DashboardData]()
	progCache := newMemCache[*domain.ProgressData]()

	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Clock:   clock.NewFake(time.Now()),
		Caches: service.LearningCaches{
			Dashboard: dashCache,
			Progress:  progCache,
		},
		Env: "test",
	})

	userID := uuid.New()
	courseID := uuid.New()
	actID := uuid.New()
	lessonID := uuid.New()
	unitID := uuid.New()

	lesson := &lessoncontract.Lesson{
		ID:         lessonID,
		UnitID:     unitID,
		Position:   1,
		Title:      "Lesson 1",
		Activities: []lessoncontract.Activity{{ID: actID, LessonID: lessonID, Kind: testKindQuiz}},
	}
	reader.lessons[lessonID] = lesson
	reader.courseLessons[courseID] = []*lessoncontract.Lesson{lesson}
	reader.courseActs[courseID] = []uuid.UUID{actID}
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID,
		LessonID:   lessonID,
		UnitID:     unitID,
		CourseID:   courseID,
		Kind:       testKindQuiz,
	}

	_, _ = repo.CreateEnrollment(ctx, userID, courseID, domain.StatusEnrollmentActive, time.Now())

	// 1. Initial read loads from repository and populates cache
	repo.queryCounter.Store(0)
	d1, err := svc.Dashboard(ctx, userID)
	if err != nil {
		t.Fatalf("first Dashboard: %v", err)
	}
	if d1.State != domain.DashboardStateInProgress {
		t.Fatalf("got state %s, want in_progress", d1.State)
	}
	firstQueries := repo.queryCounter.Load()
	if firstQueries == 0 {
		t.Fatalf("expected repo queries on first read")
	}

	// 2. Second read is served entirely from cache (zero repository queries)
	repo.queryCounter.Store(0)
	d2, err := svc.Dashboard(ctx, userID)
	if err != nil {
		t.Fatalf("second Dashboard: %v", err)
	}
	if d2.State != d1.State {
		t.Errorf("cached state mismatch")
	}
	if repo.queryCounter.Load() != 0 {
		t.Errorf("second read issued %d queries, expected 0 (cache hit)", repo.queryCounter.Load())
	}

	// 3. Start attempt and submit -> triggers invalidateLearningCaches
	startDTO, err := svc.StartAttempt(ctx, userID, actID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	idempKey := uuid.New()
	_, err = svc.SubmitAttempt(ctx, userID, startDTO.AttemptID, idempKey, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	// 4. Third read detects cache invalidation and reloads fresh data
	repo.queryCounter.Store(0)
	d3, err := svc.Dashboard(ctx, userID)
	if err != nil {
		t.Fatalf("third Dashboard: %v", err)
	}
	// After completing the only lesson, state is completed
	if d3.State != domain.DashboardStateCompleted {
		t.Errorf("got state %s after attempt submission, want completed", d3.State)
	}
	if repo.queryCounter.Load() == 0 {
		t.Errorf("expected reload queries after cache invalidation, got 0")
	}
}

// TestDashboard_CacheIsolationBetweenUsers asserts that cached entries are segregated by userID.
func TestDashboard_CacheIsolationBetweenUsers(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:         map[string]int{},
		hierarchy:     make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:       make(map[uuid.UUID]*lessoncontract.Lesson),
		courseLessons: make(map[uuid.UUID][]*lessoncontract.Lesson),
		courseActs:    make(map[uuid.UUID][]uuid.UUID),
		prereqs:       make(map[uuid.UUID][]lessoncontract.PrerequisiteItem),
	}
	graders := domain.NewGraderRegistry()
	dashCache := newMemCache[*domain.DashboardData]()

	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Clock:   clock.NewFake(time.Now()),
		Caches: service.LearningCaches{
			Dashboard: dashCache,
		},
		Env: "test",
	})

	userA := uuid.New()
	userB := uuid.New()

	// User A has mastery row
	_, _ = repo.UpsertSkillMastery(ctx, userA, "vocabulary", "B2", 0.90)

	dA, err := svc.Dashboard(ctx, userA)
	if err != nil {
		t.Fatalf("Dashboard A: %v", err)
	}
	if len(dA.SkillMastery) != 1 {
		t.Fatalf("expected 1 skill mastery for user A")
	}

	// User B has no mastery rows
	dB, err := svc.Dashboard(ctx, userB)
	if err != nil {
		t.Fatalf("Dashboard B: %v", err)
	}
	if len(dB.SkillMastery) != 0 {
		t.Errorf("expected 0 skill mastery for user B, got %d (data leak!)", len(dB.SkillMastery))
	}
}

// TestProgress_CourseZeroActivities_SafePercentage verifies 0/0 activities division safety.
func TestProgress_CourseZeroActivities_SafePercentage(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		courseActs: make(map[uuid.UUID][]uuid.UUID),
	}

	svc := service.New(service.Deps{
		Repo:   repo,
		Lesson: reader,
		Clock:  clock.NewFake(time.Now()),
	})

	userID := uuid.New()
	courseID := uuid.New()

	// Course with zero activities
	reader.courseActs[courseID] = []uuid.UUID{}
	_, _ = repo.CreateEnrollment(ctx, userID, courseID, domain.StatusEnrollmentActive, time.Now())

	prog, err := svc.Progress(ctx, userID)
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}

	if len(prog.Courses) != 1 {
		t.Fatalf("expected 1 course")
	}
	c := prog.Courses[0]
	if c.TotalActivities != 0 || c.CompletedActivities != 0 || c.Percentage != 0 {
		t.Errorf("unexpected zero-activity course progress: %+v", c)
	}
	if c.Status != domain.CourseProgressNotStarted {
		t.Errorf("got status %s, want not_started", c.Status)
	}
}

// TestProgress_RoundingRule tests percentage rounding (e.g. 12/40 -> 30).
func TestProgress_RoundingRule(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		courseActs: make(map[uuid.UUID][]uuid.UUID),
	}

	svc := service.New(service.Deps{
		Repo:   repo,
		Lesson: reader,
		Clock:  clock.NewFake(time.Now()),
	})

	userID := uuid.New()
	courseID := uuid.New()

	actIDs := make([]uuid.UUID, 40)
	for i := range actIDs {
		actIDs[i] = uuid.New()
	}
	reader.courseActs[courseID] = actIDs

	_, _ = repo.CreateEnrollment(ctx, userID, courseID, domain.StatusEnrollmentActive, time.Now())

	// Complete 12 activities
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		score := int32(100)
		_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
			UserID:      userID,
			Scope:       "activity",
			ScopeID:     actIDs[i],
			Status:      domain.ProgressCompleted,
			Score:       &score,
			CompletedAt: &now,
		})
	}

	prog, err := svc.Progress(ctx, userID)
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}

	if len(prog.Courses) != 1 {
		t.Fatalf("expected 1 course")
	}
	c := prog.Courses[0]
	if c.CompletedActivities != 12 || c.TotalActivities != 40 || c.Percentage != 30 {
		t.Errorf("got completed=%d, total=%d, percentage=%d (want 12/40 -> 30)",
			c.CompletedActivities, c.TotalActivities, c.Percentage)
	}
	if c.Status != domain.CourseProgressInProgress {
		t.Errorf("got status %s, want in_progress", c.Status)
	}
}
