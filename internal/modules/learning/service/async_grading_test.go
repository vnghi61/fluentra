package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type failingGrader struct {
	err error
}

func (g *failingGrader) Kind() string {
	return "failing_grader"
}

func (g *failingGrader) Grade(_ context.Context, _ contract.GradeRequest) (contract.GradeResult, error) {
	return contract.GradeResult{}, g.err
}

func setupAsyncTestService() (*service.Service, *fakeLearningRepo, *fakeLessonReader, *fakeEventWriter, *clock.Fake) {
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
	_ = graders.Register(testKindAsyncGrader, domain.NewAsyncFakeGrader())
	_ = graders.Register("failing_grader", &failingGrader{err: errors.New("simulated enqueue failure")})

	events := &fakeEventWriter{}
	clk := clock.NewFake(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
	svc := service.New(service.Deps{
		Repo:    repo,
		Lesson:  reader,
		Graders: graders,
		Events:  events,
		Clock:   clk,
	})
	return svc, repo, reader, events, clk
}

func assertAttemptStatus(t *testing.T, repo *fakeLearningRepo, id uuid.UUID, expected string) {
	t.Helper()
	att, err := repo.GetAttemptByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAttemptByID: %v", err)
	}
	if att.Status != expected {
		t.Errorf("expected repo status %s, got %s", expected, att.Status)
	}
}

func assertAttemptGraded(t *testing.T, repo *fakeLearningRepo, id uuid.UUID, expectedScore int) {
	t.Helper()
	att, err := repo.GetAttemptByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAttemptByID after complete: %v", err)
	}
	if att.Status != domain.StatusGraded {
		t.Errorf("expected repo status graded, got %s", att.Status)
	}
	if att.Score == nil || *att.Score != expectedScore {
		t.Errorf("expected score %d, got %v", expectedScore, att.Score)
	}
}

func assertActivityCompletedEventEmitted(t *testing.T, events *fakeEventWriter) {
	t.Helper()
	for _, ev := range events.recorded() {
		if ev == contract.EventActivityCompleted {
			return
		}
	}
	t.Errorf("expected %s event to be emitted, got %v", contract.EventActivityCompleted, events.recorded())
}

func TestAsyncGrading_SubmitAndComplete(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader, events, clk := setupAsyncTestService()

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID:       activityID,
		Kind:             testKindAsyncGrader,
		LessonSkillFocus: testSkillReading,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	// 1. Submit attempt -> returns 202 accepted (Async == true, Status == grading)
	rawResp := json.RawMessage(`{"text":"my essay"}`)
	submitRes, err := svc.SubmitAttempt(ctx, userID, startRes.AttemptID, uuid.New(), rawResp)
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}
	if !submitRes.Async || submitRes.Status != domain.StatusGrading {
		t.Fatalf("unexpected submit response: async=%v status=%s", submitRes.Async, submitRes.Status)
	}

	assertAttemptStatus(t, repo, startRes.AttemptID, domain.StatusGrading)

	// Verify GetAttemptForGrading returns expected attempt detail
	detail, err := svc.GetAttemptForGrading(ctx, startRes.AttemptID)
	if err != nil {
		t.Fatalf("GetAttemptForGrading: %v", err)
	}
	if detail.ID != startRes.AttemptID || detail.UserID != userID || detail.ActivityID != activityID {
		t.Errorf("unexpected attempt detail: %+v", detail)
	}

	// 2. CompleteAsyncGrading moves attempt to graded and emits activity.completed event
	ok, err := svc.CompleteAsyncGrading(ctx, startRes.AttemptID, contract.GradeResult{
		Score:    85,
		MaxScore: 100,
	})
	if err != nil || !ok {
		t.Fatalf("CompleteAsyncGrading: ok=%v err=%v", ok, err)
	}

	assertAttemptGraded(t, repo, startRes.AttemptID, 85)
	assertActivityCompletedEventEmitted(t, events)

	// Verify CountGradedAttemptsSince returns 1
	count, err := svc.CountGradedAttemptsSince(ctx, userID, testKindAsyncGrader, clk.Now().Add(-1*time.Hour))
	if err != nil || count != 1 {
		t.Errorf("CountGradedAttemptsSince: got %d, want 1, err=%v", count, err)
	}
}

func TestAsyncGrading_ProviderErrorLeavesFailed(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader, events, clk := setupAsyncTestService()

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindAsyncGrader,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	rawResp := json.RawMessage(`{"text":"essay"}`)
	if _, err := svc.SubmitAttempt(ctx, userID, startRes.AttemptID, uuid.New(), rawResp); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	ok, err := svc.FailAsyncGrading(ctx, startRes.AttemptID, "llm timeout")
	if err != nil || !ok {
		t.Fatalf("FailAsyncGrading: ok=%v err=%v", ok, err)
	}

	att, err := repo.GetAttemptByID(ctx, startRes.AttemptID)
	if err != nil {
		t.Fatalf("GetAttemptByID: %v", err)
	}
	if att.Status != domain.StatusFailed || att.Score != nil {
		t.Errorf("expected failed status with nil score, got status=%s score=%v", att.Status, att.Score)
	}

	for _, ev := range events.recorded() {
		if ev == contract.EventActivityCompleted {
			t.Errorf("unexpected event %s for failed attempt", ev)
		}
	}

	count, err := svc.CountGradedAttemptsSince(ctx, userID, testKindAsyncGrader, clk.Now().Add(-1*time.Hour))
	if err != nil || count != 0 {
		t.Errorf("expected 0 counted graded attempts, got %d (err=%v)", count, err)
	}
}

func TestAsyncGrading_AttemptAlreadyFailedBySweep(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader, _, _ := setupAsyncTestService()

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindAsyncGrader,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	rawResp := json.RawMessage(`{"text":"essay"}`)
	if _, err := svc.SubmitAttempt(ctx, userID, startRes.AttemptID, uuid.New(), rawResp); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	// Simulate sweep failing the attempt
	ok, err := svc.FailAsyncGrading(ctx, startRes.AttemptID, "stuck grading sweep")
	if err != nil || !ok {
		t.Fatalf("FailAsyncGrading: %v, ok=%v", err, ok)
	}

	// CompleteAsyncGrading arriving later must return false, nil and leave attempt unchanged
	ok, err = svc.CompleteAsyncGrading(ctx, startRes.AttemptID, contract.GradeResult{Score: 90, MaxScore: 100})
	if err != nil {
		t.Fatalf("CompleteAsyncGrading: %v", err)
	}
	if ok {
		t.Error("expected CompleteAsyncGrading to return false when already failed")
	}

	att, err := repo.GetAttemptByID(ctx, startRes.AttemptID)
	if err != nil {
		t.Fatalf("GetAttemptByID: %v", err)
	}
	if att.Status != domain.StatusFailed || att.Score != nil {
		t.Errorf("expected attempt to remain failed with nil score, got status=%s score=%v", att.Status, att.Score)
	}
}

func TestAsyncGrading_EnqueueFailureUnclaimsAttempt(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader, _, _ := setupAsyncTestService()

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       "failing_grader",
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	rawResp := json.RawMessage(`{"text":"essay"}`)
	if _, err := svc.SubmitAttempt(ctx, userID, startRes.AttemptID, uuid.New(), rawResp); err == nil {
		t.Fatal("expected SubmitAttempt to return error from failing_grader")
	}

	// Defer unclaim should restore attempt to in_progress so user can resubmit
	assertAttemptStatus(t, repo, startRes.AttemptID, domain.StatusInProgress)
}

func TestAsyncGrading_SweepStuckGrading(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader, _, clk := setupAsyncTestService()

	activityID := uuid.New()
	reader.hierarchy[activityID] = &lessoncontract.ActivityHierarchy{
		ActivityID: activityID,
		Kind:       testKindAsyncGrader,
	}

	userID := uuid.New()
	startRes, err := svc.StartAttempt(ctx, userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}

	rawResp := json.RawMessage(`{"text":"essay"}`)
	if _, err := svc.SubmitAttempt(ctx, userID, startRes.AttemptID, uuid.New(), rawResp); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	// Move attempt UpdatedAt to 2 hours before fake clock's current time
	repo.SetAttemptUpdatedAt(startRes.AttemptID, clk.Now().Add(-2*time.Hour))

	// Run sweep
	if err := svc.SweepStuckGrading(ctx); err != nil {
		t.Fatalf("SweepStuckGrading: %v", err)
	}

	assertAttemptStatus(t, repo, startRes.AttemptID, domain.StatusFailed)
}
