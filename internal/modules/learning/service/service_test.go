package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	testKindQuiz        = "quiz"
	testStatusCompleted = "completed"
)

type fakeLearningRepo struct {
	mu           sync.Mutex
	attempts     map[uuid.UUID]*domain.Attempt
	progress     map[string]*repository.ProgressDTO
	claimErr     error
	queryCounter atomic.Int64
}

func newFakeRepo() *fakeLearningRepo {
	return &fakeLearningRepo{
		attempts: make(map[uuid.UUID]*domain.Attempt),
		progress: make(map[string]*repository.ProgressDTO),
	}
}

func progressKey(userID uuid.UUID, scope string, scopeID uuid.UUID) string {
	return userID.String() + ":" + scope + ":" + scopeID.String()
}

func (f *fakeLearningRepo) CreateAttempt(
	_ context.Context, params repository.CreateAttemptParams,
) (*domain.Attempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	var scoreInt *int
	if params.Score != nil {
		n := int(*params.Score)
		scoreInt = &n
	}
	var dur *int64
	if params.DurationMs > 0 {
		d := int64(params.DurationMs)
		dur = &d
	}
	var keyStr *string
	if params.IdempotencyKey != nil {
		s := params.IdempotencyKey.String()
		keyStr = &s
	}

	att := &domain.Attempt{
		ID:             params.ID,
		CreatedAt:      params.CreatedAt,
		UpdatedAt:      params.CreatedAt,
		UserID:         params.UserID,
		ActivityID:     params.ActivityID,
		IdempotencyKey: keyStr,
		Response:       params.Response,
		Score:          scoreInt,
		MaxScore:       int(params.MaxScore),
		Grader:         params.Grader,
		DurationMs:     dur,
		Status:         params.Status,
	}
	f.attempts[params.ID] = att
	return att, nil
}

func (f *fakeLearningRepo) GetAttemptByID(_ context.Context, id uuid.UUID) (*domain.Attempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	att, ok := f.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	return att, nil
}

func (f *fakeLearningRepo) ClaimAttemptForGrading(
	_ context.Context, params repository.ClaimAttemptParams,
) (*domain.Attempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	if f.claimErr != nil {
		return nil, f.claimErr
	}

	att, ok := f.attempts[params.ID]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	if att.Status != domain.StatusInProgress {
		return nil, errors.New("cannot claim attempt: not in progress")
	}

	keyStr := params.IdempotencyKey.String()
	att.Status = domain.StatusGrading
	att.IdempotencyKey = &keyStr
	att.Response = params.Response
	att.UpdatedAt = time.Now().UTC()
	return att, nil
}

func (f *fakeLearningRepo) UpdateAttemptStatus(
	_ context.Context, params repository.UpdateAttemptStatusParams,
) (*domain.Attempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	att, ok := f.attempts[params.ID]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	att.Status = params.Status
	if params.Score != nil {
		sc := int(*params.Score)
		att.Score = &sc
	}
	if params.Grader != nil {
		att.Grader = params.Grader
	}
	// Always, not only when positive: attempts.duration_ms is NOT NULL DEFAULT 0,
	// so the real repository records a zero the same as any other value and a
	// fake that skips it hides the clamp.
	d := int64(params.DurationMs)
	att.DurationMs = &d
	att.UpdatedAt = time.Now().UTC()
	return att, nil
}

func (f *fakeLearningRepo) GetProgressByUserScope(
	_ context.Context, userID uuid.UUID, scope string, scopeID uuid.UUID,
) (*repository.ProgressDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	p, ok := f.progress[progressKey(userID, scope, scopeID)]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (f *fakeLearningRepo) UpsertProgress(
	_ context.Context, params repository.UpsertProgressParams,
) (*repository.ProgressDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	var scoreFloat *float64
	if params.Score != nil {
		s := float64(*params.Score)
		scoreFloat = &s
	}
	p := &repository.ProgressDTO{
		ID:          uuid.New(),
		UserID:      params.UserID,
		Scope:       params.Scope,
		ScopeID:     params.ScopeID,
		Status:      params.Status,
		Score:       scoreFloat,
		CompletedAt: params.CompletedAt,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	f.progress[progressKey(params.UserID, params.Scope, params.ScopeID)] = p
	return p, nil
}

func (f *fakeLearningRepo) ListProgressByUserScopeAndIDs(
	_ context.Context, userID uuid.UUID, scope string, scopeIDs []uuid.UUID,
) ([]repository.ProgressDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	var out []repository.ProgressDTO
	for _, id := range scopeIDs {
		if p, ok := f.progress[progressKey(userID, scope, id)]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeLearningRepo) WithTx(_ pgx.Tx) service.Repository {
	return nil
}

type fakeLessonReader struct {
	hierarchy  map[uuid.UUID]*lessoncontract.ActivityHierarchy
	lessons    map[uuid.UUID]*lessoncontract.Lesson
	unitLesson map[uuid.UUID][]*lessoncontract.Lesson
	nextLesson *lessoncontract.Lesson
}

func (r *fakeLessonReader) ResolveActivity(
	_ context.Context, activityID uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	if h, ok := r.hierarchy[activityID]; ok {
		return h, nil
	}
	return nil, apperr.New(apperr.NotFound, "ACTIVITY_NOT_FOUND", "activity not found")
}

func (r *fakeLessonReader) GetLesson(_ context.Context, id uuid.UUID) (*lessoncontract.Lesson, error) {
	if l, ok := r.lessons[id]; ok {
		return l, nil
	}
	return nil, apperr.New(apperr.NotFound, "LESSON_NOT_FOUND", "lesson not found")
}

func (r *fakeLessonReader) ListLessons(_ context.Context, unitID uuid.UUID) ([]*lessoncontract.Lesson, error) {
	if ls, ok := r.unitLesson[unitID]; ok {
		return ls, nil
	}
	return nil, nil
}

func (r *fakeLessonReader) NextLesson(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (*lessoncontract.Lesson, error) {
	return r.nextLesson, nil
}

type fakeEventWriter struct {
	events []string
}

func (w *fakeEventWriter) Write(_ context.Context, _ service.OutboxTx, _, event string, _ any) (uuid.UUID, error) {
	w.events = append(w.events, event)
	return uuid.New(), nil
}

func setupTestService() (*service.Service, *fakeLearningRepo, *fakeLessonReader, *domain.GraderRegistry) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		hierarchy:  make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:    make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson: make(map[uuid.UUID][]*lessoncontract.Lesson),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())
	_ = graders.Register("async_grader", domain.NewAsyncFakeGrader())

	clk := clock.NewFake(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		Clock:   clk,
	})
	return svc, repo, reader, graders
}

func TestStartAttempt_Success(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	userID := uuid.New()
	res, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	if res.ActivityID != activityID {
		t.Errorf("got ActivityID %s, want %s", res.ActivityID, activityID)
	}
	if res.Status != domain.StatusInProgress {
		t.Errorf("got status %s, want in_progress", res.Status)
	}
}

func TestStartAttempt_ActivityNotFound(t *testing.T) {
	svc, _, _, _ := setupTestService()
	_, err := svc.StartAttempt(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent activity")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "ACTIVITY_NOT_FOUND" {
		t.Fatalf("expected ACTIVITY_NOT_FOUND, got: %v", err)
	}
}

func TestSubmitAttempt_SynchronousGrading(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	lessonID := uuid.New()
	unitID := uuid.New()
	courseID := uuid.New()

	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		LessonID:   lessonID,
		UnitID:     unitID,
		CourseID:   courseID,
		Kind:       testKindQuiz,
	}
	reader.lessons[lessonID] = &lessoncontract.Lesson{
		ID:         lessonID,
		Activities: []lessoncontract.Activity{{ID: activityID}},
	}
	reader.unitLesson[unitID] = []*lessoncontract.Lesson{{ID: lessonID}}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	idempotencyKey := uuid.New()
	subRes, err := svc.SubmitAttempt(
		context.Background(), userID, startRes.AttemptID, idempotencyKey, json.RawMessage(`{"answer":1}`),
	)
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	if subRes.Status != domain.StatusGraded {
		t.Errorf("got status %s, want graded", subRes.Status)
	}
	if subRes.Score == nil || *subRes.Score != 100 {
		t.Errorf("got score %v, want 100", subRes.Score)
	}
	if subRes.Async {
		t.Errorf("got async true, want false")
	}
}

func TestSubmitAttempt_AsyncGrading(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       "async_grader",
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	idempotencyKey := uuid.New()
	subRes, err := svc.SubmitAttempt(
		context.Background(), userID, startRes.AttemptID, idempotencyKey, json.RawMessage(`{"audio":"..."}`),
	)
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	if subRes.Status != domain.StatusGrading {
		t.Errorf("got status %s, want grading", subRes.Status)
	}
	if !subRes.Async {
		t.Errorf("got async false, want true")
	}
	if subRes.Score != nil {
		t.Errorf("an async grader wrote a score of %d; nothing has graded it yet", *subRes.Score)
	}

	// P8.3 §7 names three assertions and this is the one that makes the flag
	// more than decorative in Phase 3: the attempt is left in `grading` and a
	// read of it says so, rather than the status living only in the submit
	// response.
	detail, err := svc.GetAttempt(context.Background(), userID, startRes.AttemptID)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if detail.Status != domain.StatusGrading {
		t.Errorf("GET /attempts/{id} reports %q, want grading", detail.Status)
	}
	if detail.Score != nil {
		t.Errorf("a score of %d is readable on an attempt still being graded", *detail.Score)
	}
	if detail.CompletedAt != nil {
		t.Error("completed_at is set on an attempt still being graded")
	}
}

func TestSubmitAttempt_IdempotentReplayReturnsStoredResult(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	idempotencyKey := uuid.New()
	res1, err := svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, idempotencyKey, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	res2, err := svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, idempotencyKey, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	if res1.Status != res2.Status || *res1.Score != *res2.Score {
		t.Errorf("replay result mismatch: %+v vs %+v", res1, res2)
	}
}

func TestSubmitAttempt_DifferentIdempotencyKeyConflict(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	idempotencyKey1 := uuid.New()
	_, err = svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, idempotencyKey1, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	idempotencyKey2 := uuid.New()
	_, err = svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, idempotencyKey2, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected conflict error for different idempotency key")
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || (appErr.Code != "ALREADY_GRADED" && appErr.Code != "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("expected conflict error code, got: %v", err)
	}
}

func TestSubmitAttempt_UnownedAttemptForbidden(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	ownerID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), ownerID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	otherUserID := uuid.New()
	_, err = svc.SubmitAttempt(context.Background(), otherUserID, startRes.AttemptID, uuid.New(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected forbidden error for unowned attempt submission")
	}

	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got: %v", err)
	}
}

func TestGetAttempt_SuccessAndForbidden(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	detail, err := svc.GetAttempt(context.Background(), userID, startRes.AttemptID)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if detail.ID != startRes.AttemptID {
		t.Errorf("got attempt ID %s, want %s", detail.ID, startRes.AttemptID)
	}

	_, err = svc.GetAttempt(context.Background(), uuid.New(), startRes.AttemptID)
	if err == nil {
		t.Fatal("expected forbidden error for other user reading attempt")
	}
}

type rollupBoundaryCase struct {
	name                 string
	lessonActivities     int
	completedActivities  int
	unitLessons          int
	completedLessons     int
	isLastLessonInCourse bool
	wantLessonCompleted  bool
	wantUnitCompleted    bool
	wantCourseCompleted  bool
}

func TestSubmitAttempt_ProgressRollupBoundaries(t *testing.T) {
	cases := []rollupBoundaryCase{
		{
			name:                 "first activity: activity progress only",
			lessonActivities:     3,
			completedActivities:  0,
			unitLessons:          2,
			completedLessons:     0,
			isLastLessonInCourse: false,
			wantLessonCompleted:  false,
			wantUnitCompleted:    false,
			wantCourseCompleted:  false,
		},
		{
			name:                 "last activity in lesson: triggers lesson completed",
			lessonActivities:     2,
			completedActivities:  1,
			unitLessons:          3,
			completedLessons:     0,
			isLastLessonInCourse: false,
			wantLessonCompleted:  true,
			wantUnitCompleted:    false,
			wantCourseCompleted:  false,
		},
		{
			name:                 "last activity in unit: triggers lesson and unit completed",
			lessonActivities:     1,
			completedActivities:  0,
			unitLessons:          2,
			completedLessons:     1,
			isLastLessonInCourse: false,
			wantLessonCompleted:  true,
			wantUnitCompleted:    true,
			wantCourseCompleted:  false,
		},
		{
			name:                 "last activity in course: triggers course completed",
			lessonActivities:     1,
			completedActivities:  0,
			unitLessons:          1,
			completedLessons:     0,
			isLastLessonInCourse: true,
			wantLessonCompleted:  true,
			wantUnitCompleted:    true,
			wantCourseCompleted:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRollupTestCase(t, tc)
		})
	}
}

func runRollupTestCase(t *testing.T, tc rollupBoundaryCase) {
	t.Helper()
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		hierarchy:  make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:    make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson: make(map[uuid.UUID][]*lessoncontract.Lesson),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	userID, courseID, unitID, lessonID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	activities := make([]lessoncontract.Activity, tc.lessonActivities)
	for i := 0; i < tc.lessonActivities; i++ {
		actID := uuid.New()
		activities[i] = lessoncontract.Activity{ID: actID, LessonID: lessonID}
		reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
			ActivityID: actID, LessonID: lessonID, UnitID: unitID, CourseID: courseID, Kind: testKindQuiz,
		}
	}

	now := time.Now().UTC()
	score100 := int32(100)
	for i := 0; i < tc.completedActivities; i++ {
		_, _ = repo.UpsertProgress(context.Background(), repository.UpsertProgressParams{
			UserID: userID, Scope: "activity", ScopeID: activities[i].ID, Status: testStatusCompleted,
			Score: &score100, CompletedAt: &now,
		})
	}

	reader.lessons[lessonID] = &lessoncontract.Lesson{ID: lessonID, UnitID: unitID, Activities: activities}
	unitLessons := make([]*lessoncontract.Lesson, tc.unitLessons)
	unitLessons[0] = reader.lessons[lessonID]
	for i := 1; i < tc.unitLessons; i++ {
		otherLID := uuid.New()
		unitLessons[i] = &lessoncontract.Lesson{ID: otherLID, UnitID: unitID}
	}
	reader.unitLesson[unitID] = unitLessons

	for i := 0; i < tc.completedLessons; i++ {
		_, _ = repo.UpsertProgress(context.Background(), repository.UpsertProgressParams{
			UserID: userID, Scope: "lesson", ScopeID: unitLessons[i+1].ID, Status: testStatusCompleted,
			Score: &score100, CompletedAt: &now,
		})
	}

	if !tc.isLastLessonInCourse {
		reader.nextLesson = &lessoncontract.Lesson{ID: uuid.New()}
	}

	events := &fakeEventWriter{}
	svc := service.New(service.Deps{
		Repo: repo, Lesson: reader, Graders: graders, Events: events, Clock: clock.NewFake(now),
	})

	targetActID := activities[tc.completedActivities].ID
	startRes, err := svc.StartAttempt(context.Background(), userID, targetActID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	subRes, err := svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, uuid.New(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}
	if subRes.Status != domain.StatusGraded {
		t.Fatalf("expected status graded, got %s", subRes.Status)
	}

	verifyRollupProgress(t, repo, userID, targetActID, lessonID, unitID, courseID, tc)
	verifyRollupEvents(t, events, tc)
}

// verifyRollupEvents is the half of the boundary assertion that the progress
// rows cannot make. `activity.completed` goes out on every completion;
// `lesson.completed` and `course.completed` only when the rollup crosses those
// boundaries, which is the case a learner notices — a course congratulating
// them at lesson three.
func verifyRollupEvents(t *testing.T, events *fakeEventWriter, tc rollupBoundaryCase) {
	t.Helper()

	count := func(name string) int {
		n := 0
		for _, e := range events.events {
			if e == name {
				n++
			}
		}
		return n
	}

	if got := count(contract.EventActivityCompleted); got != 1 {
		t.Errorf("%s published %d times, want exactly 1 on every completion", contract.EventActivityCompleted, got)
	}
	for _, want := range []struct {
		name     string
		expected bool
	}{
		{contract.EventLessonCompleted, tc.wantLessonCompleted},
		{contract.EventCourseCompleted, tc.wantCourseCompleted},
	} {
		got := count(want.name)
		if want.expected && got != 1 {
			t.Errorf("%s published %d times, want 1 — the rollup crossed this boundary", want.name, got)
		}
		if !want.expected && got != 0 {
			t.Errorf("%s published %d times, want 0 — the rollup did not cross this boundary", want.name, got)
		}
	}
}

func verifyRollupProgress(
	t *testing.T,
	repo *fakeLearningRepo,
	userID, actID, lessonID, unitID, courseID uuid.UUID,
	tc rollupBoundaryCase,
) {
	t.Helper()
	ctx := context.Background()
	actProg, _ := repo.GetProgressByUserScope(ctx, userID, "activity", actID)
	if actProg == nil || actProg.Status != testStatusCompleted {
		t.Errorf("expected activity progress completed, got %+v", actProg)
	}

	for _, scope := range []struct {
		name     string
		id       uuid.UUID
		expected bool
	}{
		{"lesson", lessonID, tc.wantLessonCompleted},
		{"unit", unitID, tc.wantUnitCompleted},
		{"course", courseID, tc.wantCourseCompleted},
	} {
		prog, _ := repo.GetProgressByUserScope(ctx, userID, scope.name, scope.id)
		completed := prog != nil && prog.Status == testStatusCompleted
		if scope.expected && !completed {
			t.Errorf("expected %s progress completed, got %+v", scope.name, prog)
		}
		// The negative is the assertion worth having. Checking only the
		// crossings it expects lets a rollup that completes everything pass
		// every row in this table.
		if !scope.expected && completed {
			t.Errorf("%s progress was completed without the rollup crossing that boundary: %+v", scope.name, prog)
		}
	}
}

func TestSubmitAttempt_ConcurrentDoubleSubmitRace(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindQuiz,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	idempotencyKey := uuid.New()
	var wg sync.WaitGroup
	results := make([]*service.SubmitAttemptResultDTO, 2)
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, submitErr := svc.SubmitAttempt(
				context.Background(), userID, startRes.AttemptID, idempotencyKey, json.RawMessage(`{}`),
			)
			results[idx] = res
			errs[idx] = submitErr
		}(i)
	}

	wg.Wait()

	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Errorf("caller %d got error: %v", i, errs[i])
		}
		if results[i] == nil || results[i].Status != domain.StatusGraded || *results[i].Score != 100 {
			t.Errorf("caller %d got invalid result: %+v", i, results[i])
		}
	}
}

func TestSubmitAttempt_GraderNotRegistered(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	actID := uuid.New()
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID,
		Kind:       "unregistered_kind",
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, actID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, uuid.New(), json.RawMessage(`{}`))
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrGraderNotRegistered.Code {
		t.Fatalf("expected %s, got: %v", domain.ErrGraderNotRegistered.Code, err)
	}
	if appErr.Kind != apperr.Validation {
		t.Errorf("unsupported kind answered %s, want a 422-class validation error", appErr.Kind)
	}
	// P8.3 §4: the kind is named. Without it the reader has a 422 and no way to
	// tell which activity kind this deployment cannot grade.
	if got := appErr.Meta["kind"]; got != "unregistered_kind" {
		t.Errorf("error does not name the offending kind: meta[kind] = %v", got)
	}
}

type errGrader struct{}

func (errGrader) Grade(_ context.Context, _ contract.GradeRequest) (contract.GradeResult, error) {
	return contract.GradeResult{}, errors.New("grader backend failed")
}

func TestSubmitAttempt_GraderError(t *testing.T) {
	svc, _, reader, graders := setupTestService()
	_ = graders.Register("failing", errGrader{})
	actID := uuid.New()
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID,
		Kind:       "failing",
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(context.Background(), userID, actID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	_, err = svc.SubmitAttempt(context.Background(), userID, startRes.AttemptID, uuid.New(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error when grader fails")
	}
}

func TestStartAttempt_NewIDError(t *testing.T) {
	repo := newFakeRepo()
	graders := domain.NewGraderRegistry()
	svc := service.New(service.Deps{
		Repo:    repo,
		Graders: graders,
		NewID: func() (uuid.UUID, error) {
			return uuid.Nil, errors.New("entropy exhausted")
		},
	})

	_, err := svc.StartAttempt(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error on newID failure")
	}
}

func TestGetAttempt_NotFound(t *testing.T) {
	svc, _, _, _ := setupTestService()
	_, err := svc.GetAttempt(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrAttemptNotFound) {
		t.Fatalf("expected ErrAttemptNotFound, got: %v", err)
	}
}

func TestService_DefaultConstructor(t *testing.T) {
	svc := service.New(service.Deps{})
	if svc == nil {
		t.Fatal("expected non-nil service from default constructor")
	}
}

// The clamps are the reason this package carries no `//nolint:gosec` on its
// integer conversions. A test that only ever passes a sane duration would let
// the clamp be deleted and the suppression comment come back.
func TestSubmitAttempt_ClampsOutOfRangeDurationAndScore(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		hierarchy:  make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:    make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson: make(map[uuid.UUID][]*lessoncontract.Lesson),
	}
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{ActivityID: activityID, Kind: testKindQuiz}

	graders := domain.NewGraderRegistry()
	// A score wider than an int32 column can hold, from a grader that is free to
	// return any int.
	_ = graders.Register(testKindQuiz, &domain.FakeGrader{
		Score: math.MaxInt32 + 1000, MaxScore: math.MaxInt32, Correct: true,
	})

	// A clock that jumps backwards between start and submit, which is what a
	// leap second or a corrected host clock looks like from here.
	clk := clock.NewFake(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
	svc := service.New(service.Deps{
		Repo: repo, Lesson: reader, Graders: graders, Events: &fakeEventWriter{}, Clock: clk,
	})

	userID := uuid.New()
	started, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	clk.Set(time.Date(2026, 8, 24, 8, 59, 0, 0, time.UTC))

	if _, err := svc.SubmitAttempt(
		context.Background(), userID, started.AttemptID, uuid.New(), json.RawMessage(`{}`),
	); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	stored, err := repo.GetAttemptByID(context.Background(), started.AttemptID)
	if err != nil {
		t.Fatalf("GetAttemptByID: %v", err)
	}
	if stored.Score == nil || *stored.Score != math.MaxInt32 {
		t.Errorf("stored score = %v, want the int32 ceiling", stored.Score)
	}
	if stored.DurationMs == nil || *stored.DurationMs != 0 {
		t.Errorf("stored duration = %v, want 0 for a clock that went backwards", stored.DurationMs)
	}
}

// An attempt that has not been graded reports no score, no feedback and no
// completion time — the fields are absent rather than filled in with a default
// that reads as a result.
func TestGetAttempt_UngradedCarriesNoResult(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{ActivityID: activityID, Kind: testKindQuiz}

	userID := uuid.New()
	started, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	detail, err := svc.GetAttempt(context.Background(), userID, started.AttemptID)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if detail.Status != domain.StatusInProgress {
		t.Errorf("status = %q, want in_progress", detail.Status)
	}
	for name, got := range map[string]any{
		"score":        detail.Score,
		"max_score":    detail.MaxScore,
		"completed_at": detail.CompletedAt,
	} {
		if got != nil && !reflect.ValueOf(got).IsNil() {
			t.Errorf("%s is set on an ungraded attempt: %v", name, got)
		}
	}
	if detail.Feedback != nil {
		t.Errorf("feedback is set on an ungraded attempt: %q", *detail.Feedback)
	}
}

// A graded attempt read back reports the stored score, and no feedback: the
// row has none to give. It used to answer with the string the fake grader
// happens to emit, which told a learner who scored 0 that they were correct.
func TestGetAttempt_GradedReportsStoredScoreAndNoFeedback(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{ActivityID: activityID, Kind: testKindQuiz}

	userID := uuid.New()
	started, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	if _, err := svc.SubmitAttempt(
		context.Background(), userID, started.AttemptID, uuid.New(), json.RawMessage(`{}`),
	); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	detail, err := svc.GetAttempt(context.Background(), userID, started.AttemptID)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if detail.Status != domain.StatusGraded {
		t.Errorf("status = %q, want graded", detail.Status)
	}
	if detail.Score == nil || *detail.Score != 100 {
		t.Errorf("score = %v, want 100", detail.Score)
	}
	if detail.CompletedAt == nil {
		t.Error("completed_at is missing on a graded attempt")
	}
	if detail.Feedback != nil {
		t.Errorf("feedback = %q; learn.attempts stores no prose to read back", *detail.Feedback)
	}
}
