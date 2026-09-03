package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
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
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	testKindQuiz        = "quiz"
	testStatusCompleted = "completed"
	testScopeLesson     = "lesson"
	testScopeActivity   = "activity"
	testSkillReading    = "reading"
	testSkillGrammar    = "grammar"
)

type fakeLearningRepo struct {
	mu           sync.Mutex
	attempts     map[uuid.UUID]*domain.Attempt
	progress     map[string]*repository.ProgressDTO
	enrollments  map[string]*domain.Enrollment
	sessions     map[uuid.UUID]*domain.LearningSession
	mastery      map[string]*domain.SkillMastery
	explanations map[string]*repository.AnswerExplanationDTO
	claimErr     error
	queryCounter atomic.Int64
	// afterGet lets a test order itself against the reads the service makes,
	// rather than against how fast a goroutine happens to run. Called under mu.
	afterGet func(*domain.Attempt)
}

func newFakeRepo() *fakeLearningRepo {
	return &fakeLearningRepo{
		attempts:     make(map[uuid.UUID]*domain.Attempt),
		progress:     make(map[string]*repository.ProgressDTO),
		enrollments:  make(map[string]*domain.Enrollment),
		sessions:     make(map[uuid.UUID]*domain.LearningSession),
		mastery:      make(map[string]*domain.SkillMastery),
		explanations: make(map[string]*repository.AnswerExplanationDTO),
	}
}

func progressKey(userID uuid.UUID, scope string, scopeID uuid.UUID) string {
	return userID.String() + ":" + scope + ":" + scopeID.String()
}

func enrollmentKey(userID, courseID uuid.UUID) string {
	return userID.String() + ":" + courseID.String()
}

func masteryKey(userID uuid.UUID, skill string) string {
	return userID.String() + ":" + skill
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
	return cloneAttempt(att), nil
}

func (f *fakeLearningRepo) GetAttemptByID(_ context.Context, id uuid.UUID) (*domain.Attempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	att, ok := f.attempts[id]
	if !ok {
		return nil, domain.ErrAttemptNotFound
	}
	if f.afterGet != nil {
		f.afterGet(att)
	}
	return cloneAttempt(att), nil
}

// cloneAttempt is what makes this fake behave like the repository it stands in
// for. The real one scans a fresh struct out of every row, so no two callers
// ever hold the same attempt; handing back the stored pointer let one goroutine
// read Status while another wrote it, under a mutex that only ever guarded the
// map.
func cloneAttempt(att *domain.Attempt) *domain.Attempt {
	if att == nil {
		return nil
	}
	copied := *att
	return &copied
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
	return cloneAttempt(att), nil
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

func (f *fakeLearningRepo) ListProgressByUserAndScope(
	_ context.Context, userID uuid.UUID, scope string, _ int32,
) ([]repository.ProgressDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	prefix := userID.String() + ":" + scope + ":"
	var out []repository.ProgressDTO
	for k, p := range f.progress {
		if strings.HasPrefix(k, prefix) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeLearningRepo) ListProgressByUser(
	_ context.Context, userID uuid.UUID, _ int32,
) ([]repository.ProgressDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	prefix := userID.String() + ":"
	var out []repository.ProgressDTO
	for k, p := range f.progress {
		if strings.HasPrefix(k, prefix) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeLearningRepo) GetEnrollmentByUserCourse(
	_ context.Context, userID, courseID uuid.UUID,
) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	e, ok := f.enrollments[enrollmentKey(userID, courseID)]
	if !ok {
		return nil, nil
	}
	copied := *e
	return &copied, nil
}

func (f *fakeLearningRepo) ListEnrollmentsByUser(
	_ context.Context, userID uuid.UUID, _ int32,
) ([]domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	var out []domain.Enrollment
	for _, e := range f.enrollments {
		if e.UserID == userID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeLearningRepo) CreateEnrollment(
	_ context.Context, userID, courseID uuid.UUID, status string, startedAt time.Time,
) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	k := enrollmentKey(userID, courseID)
	if _, exists := f.enrollments[k]; exists {
		return nil, domain.ErrAlreadyEnrolled
	}

	e := &domain.Enrollment{
		ID:        uuid.New(),
		UserID:    userID,
		CourseID:  courseID,
		Status:    status,
		StartedAt: startedAt,
		CreatedAt: startedAt,
		UpdatedAt: startedAt,
	}
	f.enrollments[k] = e
	copied := *e
	return &copied, nil
}

func (f *fakeLearningRepo) UpdateEnrollmentStatus(
	_ context.Context, userID, courseID uuid.UUID, status string, completedAt *time.Time,
) (*domain.Enrollment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	k := enrollmentKey(userID, courseID)
	e, ok := f.enrollments[k]
	if !ok {
		return nil, nil
	}
	e.Status = status
	e.CompletedAt = completedAt
	e.UpdatedAt = time.Now().UTC()
	copied := *e
	return &copied, nil
}

func (f *fakeLearningRepo) CreateLearningSession(
	_ context.Context, userID uuid.UUID, startedAt time.Time, metadata json.RawMessage,
) (*domain.LearningSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	s := &domain.LearningSession{
		ID:                  uuid.New(),
		UserID:              userID,
		StartedAt:           startedAt,
		ActivitiesCompleted: 0,
		Minutes:             0,
		Metadata:            metadata,
		CreatedAt:           startedAt,
		UpdatedAt:           startedAt,
	}
	f.sessions[s.ID] = s
	copied := *s
	return &copied, nil
}

func (f *fakeLearningRepo) GetLearningSessionByID(
	_ context.Context, id uuid.UUID,
) (*domain.LearningSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	s, ok := f.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	copied := *s
	return &copied, nil
}

func (f *fakeLearningRepo) CompleteLearningSession(
	_ context.Context, id uuid.UUID, endedAt time.Time, activitiesCompleted, minutes int32,
) (*domain.LearningSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	s, ok := f.sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	s.EndedAt = &endedAt
	s.ActivitiesCompleted = int(activitiesCompleted)
	s.Minutes = int(minutes)
	s.UpdatedAt = time.Now().UTC()
	copied := *s
	return &copied, nil
}

func (f *fakeLearningRepo) GetSkillMastery(
	_ context.Context, userID uuid.UUID, skill string,
) (*domain.SkillMastery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	m, ok := f.mastery[masteryKey(userID, skill)]
	if !ok {
		return nil, nil
	}
	copied := *m
	return &copied, nil
}

func (f *fakeLearningRepo) ListSkillMasteryByUser(
	_ context.Context, userID uuid.UUID,
) ([]domain.SkillMastery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	var out []domain.SkillMastery
	for _, m := range f.mastery {
		if m.UserID == userID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (f *fakeLearningRepo) UpsertSkillMastery(
	_ context.Context, userID uuid.UUID, skill, level string, confidence float64,
) (*domain.SkillMastery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCounter.Add(1)

	k := masteryKey(userID, skill)
	m := &domain.SkillMastery{
		ID:         uuid.New(),
		UserID:     userID,
		Skill:      skill,
		Level:      level,
		Confidence: confidence,
		UpdatedAt:  time.Now().UTC(),
		CreatedAt:  time.Now().UTC(),
	}
	f.mastery[k] = m
	copied := *m
	return &copied, nil
}

func (f *fakeLearningRepo) GetAnswerExplanation(
	_ context.Context, contentVersionID uuid.UUID, userAnswer string,
) (*repository.AnswerExplanationDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := contentVersionID.String() + ":" + userAnswer
	e, ok := f.explanations[key]
	if !ok {
		return nil, nil
	}
	copied := *e
	return &copied, nil
}

func (f *fakeLearningRepo) UpsertAnswerExplanation(
	_ context.Context, explanation repository.AnswerExplanationDTO,
) (*repository.AnswerExplanationDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := explanation.ContentVersionID.String() + ":" + explanation.UserAnswer
	cp := explanation
	f.explanations[key] = &cp
	return &cp, nil
}

func (f *fakeLearningRepo) WithTx(_ pgx.Tx) service.Repository {
	return f
}

type fakeLessonReader struct {
	// calls counts reads per method, so a test can assert what resolving one
	// answer costs rather than only what it returns. The dashboard is opened on
	// every app start and P8.5 has to hold a query budget over it.
	callMu        sync.Mutex
	calls         map[string]int
	hierarchy     map[uuid.UUID]*lessoncontract.ActivityHierarchy
	lessons       map[uuid.UUID]*lessoncontract.Lesson
	unitLesson    map[uuid.UUID][]*lessoncontract.Lesson
	courseUnits   map[uuid.UUID][]*lessoncontract.Unit
	courseLessons map[uuid.UUID][]*lessoncontract.Lesson
	courseActs    map[uuid.UUID][]uuid.UUID
	prereqs       map[uuid.UUID][]lessoncontract.PrerequisiteItem
	nextLesson    *lessoncontract.Lesson
	listUnitsErr  error
}

func (r *fakeLessonReader) record(method string) {
	r.callMu.Lock()
	defer r.callMu.Unlock()
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[method]++
}

func (r *fakeLessonReader) callCount(method string) int {
	r.callMu.Lock()
	defer r.callMu.Unlock()
	return r.calls[method]
}

func (r *fakeLessonReader) ResolveActivity(
	_ context.Context, activityID uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	r.record("ResolveActivity")
	if h, ok := r.hierarchy[activityID]; ok {
		return h, nil
	}
	return nil, apperr.New(apperr.NotFound, "ACTIVITY_NOT_FOUND", "activity not found")
}

func (r *fakeLessonReader) GetLesson(_ context.Context, id uuid.UUID) (*lessoncontract.Lesson, error) {
	r.record("GetLesson")
	if l, ok := r.lessons[id]; ok {
		return l, nil
	}
	return nil, apperr.New(apperr.NotFound, "LESSON_NOT_FOUND", "lesson not found")
}

func (r *fakeLessonReader) ListLessons(_ context.Context, unitID uuid.UUID) ([]*lessoncontract.Lesson, error) {
	r.record("ListLessons")
	if ls, ok := r.unitLesson[unitID]; ok {
		return ls, nil
	}
	return nil, nil
}

func (r *fakeLessonReader) ListUnitsByCourseID(_ context.Context, courseID uuid.UUID) ([]*lessoncontract.Unit, error) {
	r.record("ListUnitsByCourseID")
	if r.listUnitsErr != nil {
		return nil, r.listUnitsErr
	}
	if us, ok := r.courseUnits[courseID]; ok {
		return us, nil
	}
	return nil, nil
}

func (r *fakeLessonReader) ListPrerequisitesForLessons(
	_ context.Context, lessonIDs []uuid.UUID,
) ([]lessoncontract.PrerequisiteItem, error) {
	r.record("ListPrerequisitesForLessons")
	var out []lessoncontract.PrerequisiteItem
	for _, id := range lessonIDs {
		if ps, ok := r.prereqs[id]; ok {
			out = append(out, ps...)
		}
	}
	return out, nil
}

func (r *fakeLessonReader) ListActivitiesByCourseIDs(
	_ context.Context, courseIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	r.record("ListActivitiesByCourseIDs")
	res := make(map[uuid.UUID][]uuid.UUID)
	for _, id := range courseIDs {
		if acts, ok := r.courseActs[id]; ok {
			res[id] = acts
		} else {
			res[id] = []uuid.UUID{}
		}
	}
	return res, nil
}

func (r *fakeLessonReader) NextLesson(
	_ context.Context, courseID uuid.UUID, currentID *uuid.UUID,
) (*lessoncontract.Lesson, error) {
	r.record("NextLesson")
	if cls, ok := r.courseLessons[courseID]; ok && len(cls) > 0 {
		if currentID == nil {
			return cls[0], nil
		}
		for i, l := range cls {
			if l.ID == *currentID && i+1 < len(cls) {
				return cls[i+1], nil
			}
		}
		return nil, nil
	}
	if currentID == nil {
		return r.nextLesson, nil
	}
	return nil, nil
}

type fakeEventWriter struct {
	mu     sync.Mutex
	events []string
}

func (w *fakeEventWriter) Write(_ context.Context, _ service.OutboxTx, _, event string, _ any) (uuid.UUID, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, event)
	return uuid.New(), nil
}

func (w *fakeEventWriter) recorded() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.events...)
}

func setupTestService() (*service.Service, *fakeLearningRepo, *fakeLessonReader, *domain.GraderRegistry) {
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
	svc, repo, reader, _ := setupTestService()
	activityID := uuid.New()
	lessonID := uuid.New()
	unitID := uuid.New()
	courseID := uuid.New()
	userID := uuid.New()

	_, _ = repo.CreateEnrollment(context.Background(), userID, courseID, domain.StatusEnrollmentActive, time.Now().UTC())
	reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}

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
		hierarchy:   make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:     make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson:  make(map[uuid.UUID][]*lessoncontract.Lesson),
		courseUnits: make(map[uuid.UUID][]*lessoncontract.Unit),
		prereqs:     make(map[uuid.UUID][]lessoncontract.PrerequisiteItem),
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
	_, _ = repo.CreateEnrollment(context.Background(), userID, courseID, domain.StatusEnrollmentActive, now)

	if tc.isLastLessonInCourse {
		reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
	} else {
		otherUnitID := uuid.New()
		reader.courseUnits[courseID] = []*lessoncontract.Unit{
			{ID: unitID, CourseID: courseID},
			{ID: otherUnitID, CourseID: courseID},
		}
	}

	score100 := int32(100)
	for i := 0; i < tc.completedActivities; i++ {
		_, _ = repo.UpsertProgress(context.Background(), repository.UpsertProgressParams{
			UserID: userID, Scope: testScopeActivity, ScopeID: activities[i].ID, Status: testStatusCompleted,
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
			UserID: userID, Scope: testScopeLesson, ScopeID: unitLessons[i+1].ID, Status: testStatusCompleted,
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
		for _, e := range events.recorded() {
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

	for i := range 2 {
		if errs[i] != nil {
			t.Fatalf("caller %d got error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("caller %d got no result", i)
		}
		if results[i].Status != domain.StatusGraded || results[i].Score == nil || *results[i].Score != 100 {
			t.Errorf("caller %d got invalid result: %+v", i, results[i])
		}
		if results[i].Async {
			// The loser used to answer 202 here: it read the row back while the
			// winner's transaction was still open, saw `grading`, and reported an
			// asynchronous grade for a synchronous grader.
			t.Errorf("caller %d was told the grade is asynchronous", i)
		}
	}

	// P8.3 §6: the same response body twice, not a fresh grade. Everything the
	// attempt row stores has to match; `feedback` is the one field it does not
	// store, and DECISIONS.md records why the replay leaves it null.
	won, replayed := results[0], results[1]
	if won.AttemptID != replayed.AttemptID ||
		won.Status != replayed.Status ||
		*won.Score != *replayed.Score ||
		*won.MaxScore != *replayed.MaxScore ||
		*won.Correct != *replayed.Correct {
		t.Errorf("the two callers of one submission got different bodies: %+v vs %+v", won, replayed)
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

// gateGrader parks inside Grade until it is released, so a test can hold an
// attempt in `grading` for as long as it needs.
type gateGrader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGateGrader() *gateGrader {
	return &gateGrader{started: make(chan struct{}), release: make(chan struct{})}
}

func (g *gateGrader) Grade(_ context.Context, _ contract.GradeRequest) (contract.GradeResult, error) {
	g.once.Do(func() { close(g.started) })
	<-g.release
	return contract.GradeResult{Score: 90, MaxScore: 100, Correct: true, Feedback: "ok"}, nil
}

// The window the broad race test only reaches by luck: a duplicate submission
// arrives while the caller holding the claim is still inside the grader, so the
// row it reads says `grading` and carries no score.
//
// Answering from that read is what CI caught — the duplicate was told its grade
// was asynchronous, for an activity whose grader is synchronous, and the two
// callers of one submission got different bodies. Shorten the wait in
// awaitSettledAttempt to zero iterations and this fails on the Async assertion.
func TestSubmitAttempt_DuplicateWaitsForTheClaimHolder(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		hierarchy:  make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
		lessons:    make(map[uuid.UUID]*lessoncontract.Lesson),
		unitLesson: make(map[uuid.UUID][]*lessoncontract.Lesson),
	}
	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{ActivityID: activityID, Kind: testKindQuiz}

	grader := newGateGrader()
	graders := domain.NewGraderRegistry()
	if err := graders.Register(testKindQuiz, grader); err != nil {
		t.Fatalf("register grader: %v", err)
	}

	svc := service.New(service.Deps{
		Repo: repo, Lesson: reader, Graders: graders, Events: &fakeEventWriter{},
		Clock: clock.NewFake(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)),
	})

	ctx := context.Background()
	userID := uuid.New()
	started, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	key := uuid.New()
	type outcome struct {
		res *service.SubmitAttemptResultDTO
		err error
	}
	winner := make(chan outcome, 1)
	go func() {
		res, submitErr := svc.SubmitAttempt(ctx, userID, started.AttemptID, key, json.RawMessage(`{}`))
		winner <- outcome{res, submitErr}
	}()

	// The winner is now parked inside the grader and the attempt is `grading`.
	<-grader.started

	// Signalled the moment a caller reads the attempt back while it says
	// `grading` — the exact window this test exists to open.
	sawGrading := make(chan struct{}, 1)
	repo.afterGet = func(att *domain.Attempt) {
		if att.Status == domain.StatusGrading {
			select {
			case sawGrading <- struct{}{}:
			default:
			}
		}
	}

	duplicate := make(chan outcome, 1)
	go func() {
		res, submitErr := svc.SubmitAttempt(ctx, userID, started.AttemptID, key, json.RawMessage(`{}`))
		duplicate <- outcome{res, submitErr}
	}()

	// Only once the duplicate has actually read the half-finished row does the
	// winner get to finish. Releasing earlier lets the winner commit first and
	// the window never opens.
	<-sawGrading
	close(grader.release)

	won := <-winner
	dup := <-duplicate
	if won.err != nil {
		t.Fatalf("the claim holder failed: %v", won.err)
	}
	if dup.err != nil {
		t.Fatalf("the duplicate failed: %v", dup.err)
	}

	if dup.res.Async {
		t.Error("the duplicate was told a synchronous grade is asynchronous")
	}
	if dup.res.Status != domain.StatusGraded {
		t.Errorf("the duplicate saw status %q, want graded", dup.res.Status)
	}
	if dup.res.Score == nil || *dup.res.Score != 90 {
		t.Errorf("the duplicate got score %v, want the stored 90", dup.res.Score)
	}
	if won.res.Status != dup.res.Status ||
		won.res.Score == nil || dup.res.Score == nil || *won.res.Score != *dup.res.Score {
		t.Errorf("different bodies for one submission: %+v vs %+v", won.res, dup.res)
	}
}

type fakeAIClient struct {
	mu       sync.Mutex
	calls    int
	hasQuota bool
	quotaErr error
	complete func(ctx context.Context, req ai.Request) (ai.Response, error)
}

func (f *fakeAIClient) HasQuota(_ context.Context, _ ai.Task) (bool, error) {
	if f.quotaErr != nil {
		return false, f.quotaErr
	}
	return f.hasQuota, nil
}

func (f *fakeAIClient) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.complete != nil {
		return f.complete(ctx, req)
	}
	return ai.Response{
		Text: `{"text": "English explanation", "text_vi": "Giai thich tieng Viet"}`,
	}, nil
}

const testExplanationText = "English explanation"

func TestSubmitAttempt_GeneratesExplanationLazilyAndCaches(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:     map[string]int{},
		hierarchy: make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	aiClient := &fakeAIClient{hasQuota: true}
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		AI:      aiClient,
	})

	activityID := uuid.New()
	contentVersionID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		ContentVersionID: contentVersionID,
		Kind:             testKindQuiz,
		Config:           json.RawMessage(`{"prompt": "Select the correct option"}`),
	}

	ctx := context.Background()
	user1 := uuid.New()
	att1, err := svc.StartAttempt(ctx, user1, activityID)
	if err != nil {
		t.Fatalf("start attempt 1: %v", err)
	}

	// First learner answers with opt_a -> AI is called once, stored in repo
	res1, err := svc.SubmitAttempt(ctx, user1, att1.AttemptID, uuid.New(),
		json.RawMessage(`{"selected_option_id": "opt_a"}`))
	if err != nil {
		t.Fatalf("submit attempt 1: %v", err)
	}
	if res1.Explanation == nil {
		t.Fatal("expected explanation to be generated for learner 1")
	}
	if res1.Explanation.Text != testExplanationText || res1.Explanation.TextVi != "Giai thich tieng Viet" {
		t.Errorf("unexpected explanation: %+v", res1.Explanation)
	}
	if aiClient.calls != 1 {
		t.Errorf("expected 1 AI call, got %d", aiClient.calls)
	}

	// Verify repo has cached the explanation under (contentVersionID, "opt_a")
	cached, err := repo.GetAnswerExplanation(ctx, contentVersionID, "opt_a")
	if err != nil || cached == nil {
		t.Fatalf("expected cached explanation in repo, got err=%v, cached=%v", err, cached)
	}

	// Second learner answers same question with same answer -> fetched from DB cache, 0 new AI calls
	user2 := uuid.New()
	att2, err := svc.StartAttempt(ctx, user2, activityID)
	if err != nil {
		t.Fatalf("start attempt 2: %v", err)
	}
	res2, err := svc.SubmitAttempt(ctx, user2, att2.AttemptID, uuid.New(),
		json.RawMessage(`{"selected_option_id": "opt_a"}`))
	if err != nil {
		t.Fatalf("submit attempt 2: %v", err)
	}
	if res2.Explanation == nil || res2.Explanation.Text != testExplanationText {
		t.Fatalf("expected explanation from cache, got %+v", res2.Explanation)
	}
	if aiClient.calls != 1 {
		t.Errorf("expected still 1 AI call (cached), got %d", aiClient.calls)
	}
}

// The grader treats "Habit" and "habit" as one answer, so the explanation cache
// has to as well.
//
// It did not: the key was the answer with only its whitespace trimmed, so the
// same answer typed with a capital produced a second row and a second provider
// call. On a free daily quota that is spend for nothing, and it grows with every
// learner who happens to hold shift.
func TestSubmitAttempt_ExplanationCacheIgnoresCaseTheGraderIgnores(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:     map[string]int{},
		hierarchy: make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	aiClient := &fakeAIClient{hasQuota: true}
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		AI:      aiClient,
	})

	activityID := uuid.New()
	contentVersionID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		ContentVersionID: contentVersionID,
		Kind:             testKindQuiz,
		Config:           json.RawMessage(`{"prompt": "Type the word"}`),
	}

	ctx := context.Background()
	for _, answer := range []string{"habit", "Habit", "  HABIT  "} {
		user := uuid.New()
		attempt, err := svc.StartAttempt(ctx, user, activityID)
		if err != nil {
			t.Fatalf("start attempt for %q: %v", answer, err)
		}
		body := json.RawMessage(`{"text_answer": "` + answer + `"}`)
		if _, err := svc.SubmitAttempt(ctx, user, attempt.AttemptID, uuid.New(), body); err != nil {
			t.Fatalf("submit %q: %v", answer, err)
		}
	}

	if aiClient.calls != 1 {
		t.Errorf("expected 1 AI call for three spellings of the same answer, got %d", aiClient.calls)
	}
}

func TestSubmitAttempt_DifferentAnswerGeneratesNewExplanation(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:     map[string]int{},
		hierarchy: make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	aiClient := &fakeAIClient{hasQuota: true}
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		AI:      aiClient,
	})

	activityID := uuid.New()
	contentVersionID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		ContentVersionID: contentVersionID,
		Kind:             testKindQuiz,
		Config:           json.RawMessage(`{"prompt": "Select the correct option"}`),
	}

	ctx := context.Background()
	user1 := uuid.New()
	att1, err := svc.StartAttempt(ctx, user1, activityID)
	if err != nil {
		t.Fatalf("start attempt 1: %v", err)
	}
	_, err = svc.SubmitAttempt(ctx, user1, att1.AttemptID, uuid.New(),
		json.RawMessage(`{"selected_option_id": "opt_a"}`))
	if err != nil {
		t.Fatalf("submit attempt 1: %v", err)
	}
	if aiClient.calls != 1 {
		t.Fatalf("expected 1 call after first answer, got %d", aiClient.calls)
	}

	// Another learner answers same question with DIFFERENT answer opt_b -> new AI call
	user2 := uuid.New()
	att2, err := svc.StartAttempt(ctx, user2, activityID)
	if err != nil {
		t.Fatalf("start attempt 2: %v", err)
	}
	res2, err := svc.SubmitAttempt(ctx, user2, att2.AttemptID, uuid.New(),
		json.RawMessage(`{"selected_option_id": "opt_b"}`))
	if err != nil {
		t.Fatalf("submit attempt 2: %v", err)
	}
	if res2.Explanation == nil {
		t.Fatal("expected explanation for opt_b")
	}
	if aiClient.calls != 2 {
		t.Errorf("expected 2 AI calls after different answer, got %d", aiClient.calls)
	}
}

func TestSubmitAttempt_GracefulFallbackOnQuotaExhausted(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:     map[string]int{},
		hierarchy: make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	// Quota is exhausted
	aiClient := &fakeAIClient{hasQuota: false}
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		AI:      aiClient,
	})

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		ContentVersionID: uuid.New(),
		Kind:             testKindQuiz,
		Config:           json.RawMessage(`{"prompt": "Select option"}`),
	}

	ctx := context.Background()
	user := uuid.New()
	att, err := svc.StartAttempt(ctx, user, activityID)
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}

	res, err := svc.SubmitAttempt(ctx, user, att.AttemptID, uuid.New(),
		json.RawMessage(`{"selected_option_id": "opt_a"}`))
	if err != nil {
		t.Fatalf("attempt must succeed even when quota is exhausted: %v", err)
	}
	if res.Status != domain.StatusGraded {
		t.Errorf("expected status graded, got %s", res.Status)
	}
	if res.Explanation != nil {
		t.Errorf("expected explanation to be nil on quota exhaustion, got %+v", res.Explanation)
	}
	if aiClient.calls != 0 {
		t.Errorf("expected 0 AI calls when quota exhausted, got %d", aiClient.calls)
	}
}

func TestGradePreview_AttachesExplanation(t *testing.T) {
	repo := newFakeRepo()
	reader := &fakeLessonReader{
		calls:     map[string]int{},
		hierarchy: make(map[uuid.UUID]*lessoncontract.ActivityHierarchy),
	}
	graders := domain.NewGraderRegistry()
	_ = graders.Register(testKindQuiz, domain.NewFakeGrader())

	aiClient := &fakeAIClient{hasQuota: true}
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  &fakeEventWriter{},
		AI:      aiClient,
	})

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		ContentVersionID: uuid.New(),
		Kind:             testKindQuiz,
		Config:           json.RawMessage(`{"prompt": "Preview question"}`),
	}

	ctx := context.Background()
	preview, err := svc.GradePreview(ctx, activityID, json.RawMessage(`{"selected_option_id": "opt_a"}`))
	if err != nil {
		t.Fatalf("grade preview: %v", err)
	}
	if preview.Explanation == nil {
		t.Fatal("expected explanation on preview grade")
	}
	if preview.Explanation.Text != "English explanation" {
		t.Errorf("unexpected preview explanation: %+v", preview.Explanation)
	}
}
