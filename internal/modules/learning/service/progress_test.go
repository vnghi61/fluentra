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
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// assertUnlockStates checks a whole batch answer at once, so the tests below stay
// about the rules rather than about map lookups.
func assertUnlockStates(t *testing.T, got map[uuid.UUID]bool, want map[uuid.UUID]bool, when string) {
	t.Helper()
	for id, expected := range want {
		if got[id] != expected {
			t.Errorf("%s: lesson %s unlocked=%v, want %v", when, id, got[id], expected)
		}
	}
}

func TestUnlockChecker_PrerequisiteRules(t *testing.T) {
	svc, repo, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()

	lesson1 := uuid.New()
	lesson2 := uuid.New()
	lesson3 := uuid.New()
	lessonNoPrereq := uuid.New()

	// lesson2 requires lesson1 with min_score 80
	// lesson3 requires lesson1 with min_score 0 (any pass)
	reader.prereqs[lesson2] = []lessoncontract.PrerequisiteItem{
		{LessonID: lesson2, RequiresLessonID: lesson1, MinScore: 80},
	}
	reader.prereqs[lesson3] = []lessoncontract.PrerequisiteItem{
		{LessonID: lesson3, RequiresLessonID: lesson1, MinScore: 0},
	}

	completeLesson := func(score int32) {
		t.Helper()
		now := time.Now().UTC()
		if _, err := repo.UpsertProgress(ctx, repository.UpsertProgressParams{
			UserID:      userID,
			Scope:       testScopeLesson,
			ScopeID:     lesson1,
			Status:      domain.ProgressCompleted,
			Score:       &score,
			CompletedAt: &now,
		}); err != nil {
			t.Fatalf("UpsertProgress: %v", err)
		}
	}

	check := func(when string, actor uuid.UUID, ids []uuid.UUID, want map[uuid.UUID]bool) {
		t.Helper()
		got, err := svc.IsUnlocked(ctx, actor, ids)
		if err != nil {
			t.Fatalf("%s: IsUnlocked: %v", when, err)
		}
		assertUnlockStates(t, got, want, when)
	}

	// 1. Nothing completed: only the lessons without prerequisites are open.
	check("nothing completed", userID,
		[]uuid.UUID{lesson1, lesson2, lesson3, lessonNoPrereq},
		map[uuid.UUID]bool{lesson1: true, lessonNoPrereq: true, lesson2: false, lesson3: false})

	// 2. Prerequisite completed at 70: the min_score 80 lesson stays locked.
	completeLesson(70)
	check("prerequisite scored 70", userID,
		[]uuid.UUID{lesson2, lesson3},
		map[uuid.UUID]bool{lesson2: false, lesson3: true})

	// 3. Prerequisite re-scored at 85: it opens.
	completeLesson(85)
	check("prerequisite scored 85", userID,
		[]uuid.UUID{lesson2},
		map[uuid.UUID]bool{lesson2: true})

	// 4. An anonymous caller has no progress, so anything gated stays gated.
	check("anonymous", uuid.Nil,
		[]uuid.UUID{lesson2, lessonNoPrereq},
		map[uuid.UUID]bool{lesson2: false, lessonNoPrereq: true})

	// 5. An empty request is an empty answer, not a nil map.
	empty, err := svc.IsUnlocked(ctx, userID, []uuid.UUID{})
	if err != nil {
		t.Fatalf("IsUnlocked empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty map for empty lessonIDs, got %d entries", len(empty))
	}
}

func TestEnrollment_SuccessAndConflict(t *testing.T) {
	svc, _, _, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	enrollment, err := svc.Enroll(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if enrollment.UserID != userID || enrollment.CourseID != courseID {
		t.Errorf("enrollment user/course mismatch")
	}
	if enrollment.Status != domain.StatusEnrollmentActive {
		t.Errorf("got status %s, want active", enrollment.Status)
	}

	// Duplicate enroll should return 409 conflict
	_, err = svc.Enroll(ctx, userID, courseID)
	if err == nil {
		t.Fatalf("expected error on duplicate enroll, got nil")
	}
	if !domain.IsAlreadyEnrolled(err) {
		t.Errorf("expected ErrAlreadyEnrolled, got %v", err)
	}
}

func TestProgressOf_ReaderContract(t *testing.T) {
	svc, repo, _, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	scopeID1 := uuid.New()
	scopeID2 := uuid.New()

	score90 := int32(90)
	now := time.Now().UTC()
	_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       testScopeLesson,
		ScopeID:     scopeID1,
		Status:      domain.ProgressCompleted,
		Score:       &score90,
		CompletedAt: &now,
	})
	_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       testScopeLesson,
		ScopeID:     scopeID2,
		Status:      domain.ProgressInProgress,
		Score:       nil,
		CompletedAt: nil,
	})

	progressList, err := svc.ProgressOf(ctx, userID, contract.ScopeLesson)
	if err != nil {
		t.Fatalf("ProgressOf: %v", err)
	}
	if len(progressList) != 2 {
		t.Fatalf("expected 2 progress records, got %d", len(progressList))
	}
}

// submitGraded runs one activity through start → submit and fails the test if it
// does not come back graded.
func submitGraded(t *testing.T, svc *service.Service, userID, activityID uuid.UUID) {
	t.Helper()
	started, err := svc.StartAttempt(context.Background(), userID, activityID)
	if err != nil {
		t.Fatalf("StartAttempt %s: %v", activityID, err)
	}
	result, err := svc.SubmitAttempt(
		context.Background(), userID, started.AttemptID, uuid.New(), json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("SubmitAttempt %s: %v", activityID, err)
	}
	if result.Status != domain.StatusGraded {
		t.Fatalf("attempt on %s: got status %s, want graded", activityID, result.Status)
	}
}

// TestCourseRollup_UnitCompleteness is the boundary the old rollup got wrong: it
// asked whether this was the course's last lesson, not whether every unit was
// done, so finishing one unit out of two published course.completed.
func TestCourseRollup_UnitCompleteness(t *testing.T) {
	svc, repo, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	unit1, unit2 := uuid.New(), uuid.New()
	lesson1, lesson2 := uuid.New(), uuid.New()
	act1, act2 := uuid.New(), uuid.New()

	if _, err := svc.Enroll(ctx, userID, courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	reader.courseUnits[courseID] = []*lessoncontract.Unit{
		{ID: unit1, CourseID: courseID},
		{ID: unit2, CourseID: courseID},
	}
	for _, u := range []struct {
		unitID, lessonID, activityID uuid.UUID
	}{
		{unit1, lesson1, act1},
		{unit2, lesson2, act2},
	} {
		reader.hierarchy[u.activityID] = &lessoncontract.ActivityHierarchy{
			ActivityID: u.activityID, LessonID: u.lessonID, UnitID: u.unitID,
			CourseID: courseID, Kind: testKindQuiz,
		}
		reader.lessons[u.lessonID] = &lessoncontract.Lesson{
			ID: u.lessonID, UnitID: u.unitID,
			Activities: []lessoncontract.Activity{{ID: u.activityID}},
		}
		reader.unitLesson[u.unitID] = []*lessoncontract.Lesson{{ID: u.lessonID}}
	}

	courseCompleted := func(when string) bool {
		t.Helper()
		progress, err := repo.GetProgressByUserScope(ctx, userID, "course", courseID)
		if err != nil {
			t.Fatalf("%s: read course progress: %v", when, err)
		}
		return progress != nil && progress.Status == domain.ProgressCompleted
	}
	enrollmentStatus := func(when string) *domain.Enrollment {
		t.Helper()
		enrollment, err := repo.GetEnrollmentByUserCourse(ctx, userID, courseID)
		if err != nil {
			t.Fatalf("%s: read enrolment: %v", when, err)
		}
		return enrollment
	}

	// One unit of two: the course is not finished, whatever position that unit
	// holds in it.
	submitGraded(t, svc, userID, act1)
	if courseCompleted("after unit 1") {
		t.Errorf("course reported complete with unit 2 untouched")
	}
	if got := enrollmentStatus("after unit 1").Status; got != domain.StatusEnrollmentActive {
		t.Errorf("enrolment status after one unit: got %s, want active", got)
	}

	// Both units: the course completes, and the enrolment closes with it.
	submitGraded(t, svc, userID, act2)
	if !courseCompleted("after unit 2") {
		t.Errorf("course should be complete once every unit is")
	}
	enrollment := enrollmentStatus("after unit 2")
	if enrollment.Status != domain.StatusEnrollmentCompleted {
		t.Errorf("enrolment status: got %s, want completed", enrollment.Status)
	}
	if enrollment.CompletedAt == nil {
		t.Errorf("completed enrolment has no completed_at; the CHECK constraint requires one")
	}
}

func TestMastery_IncrementalEstimation(t *testing.T) {
	svc, repo, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()

	_, _ = svc.Enroll(ctx, userID, courseID)

	reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID, LessonID: lessonID, UnitID: unitID, CourseID: courseID,
		Kind: testKindQuiz, LessonSkillFocus: "grammar",
	}
	reader.lessons[lessonID] = &lessoncontract.Lesson{
		ID: lessonID, UnitID: unitID, SkillFocus: "grammar", Activities: []lessoncontract.Activity{{ID: actID}},
	}
	reader.unitLesson[unitID] = []*lessoncontract.Lesson{{ID: lessonID}}

	// Submit attempt (scores 100)
	start, err := svc.StartAttempt(ctx, userID, actID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	_, err = svc.SubmitAttempt(ctx, userID, start.AttemptID, uuid.New(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	mastery, err := repo.GetSkillMastery(ctx, userID, "grammar")
	if err != nil {
		t.Fatalf("GetSkillMastery: %v", err)
	}
	if mastery == nil {
		t.Fatalf("expected mastery to be recorded for grammar")
	}
	if mastery.Level != "C2" {
		t.Errorf("got level %s, want C2 for score 100", mastery.Level)
	}
}

func TestMastery_UnrecognizedSkillIgnored(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()

	_, _ = svc.Enroll(ctx, userID, courseID)

	reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID, LessonID: lessonID, UnitID: unitID, CourseID: courseID,
		Kind: testKindQuiz, LessonSkillFocus: "non_standard_skill_tag",
	}
	reader.lessons[lessonID] = &lessoncontract.Lesson{
		ID: lessonID, UnitID: unitID, SkillFocus: "non_standard_skill_tag",
		Activities: []lessoncontract.Activity{{ID: actID}},
	}
	reader.unitLesson[unitID] = []*lessoncontract.Lesson{{ID: lessonID}}

	// Submit attempt (scores 100) - must succeed without error
	start, err := svc.StartAttempt(ctx, userID, actID)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	res, err := svc.SubmitAttempt(ctx, userID, start.AttemptID, uuid.New(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("SubmitAttempt failed with unrecognised skill: %v", err)
	}
	if res.Status != domain.StatusGraded {
		t.Errorf("got status %s, want graded", res.Status)
	}
}

func TestNextActivity_Resolution(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()

	// 1. Learner with no enrollments -> not_started
	res, err := svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateNotStarted || res.NextActivity != nil {
		t.Errorf("expected not_started with nil next activity, got state=%s", res.State)
	}

	// 2. Enrolled learner with activities -> in_progress
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()
	_, _ = svc.Enroll(ctx, userID, courseID)

	reader.nextLesson = &lessoncontract.Lesson{
		ID:               lessonID,
		UnitID:           unitID,
		Title:            "Unit 1 Lesson 1",
		SkillFocus:       testSkillReading,
		EstimatedMinutes: 15,
		Activities:       []lessoncontract.Activity{{ID: actID, Kind: testKindQuiz}},
	}
	reader.lessons[lessonID] = reader.nextLesson

	res, err = svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateInProgress {
		t.Fatalf("expected state in_progress, got %s", res.State)
	}
	if res.NextActivity == nil {
		t.Fatalf("expected next activity to be populated")
	}
	if res.NextActivity.ActivityID != actID {
		t.Errorf("got activity ID %s, want %s", res.NextActivity.ActivityID, actID)
	}
	if res.NextActivity.Title != "Unit 1 Lesson 1" {
		t.Errorf("got title %q, want %q", res.NextActivity.Title, "Unit 1 Lesson 1")
	}
	if res.NextActivity.Skill != testSkillReading {
		t.Errorf("got skill %s, want reading", res.NextActivity.Skill)
	}
}

func TestLearningSession_Lifecycle(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	repo := newFakeRepo()
	events := &fakeEventWriter{}
	svc := service.New(service.Deps{
		Repo:   repo,
		Events: events,
		Clock:  clk,
	})
	ctx := context.Background()
	userID := uuid.New()

	// 1. Start session
	sess, err := svc.StartSession(ctx, userID, json.RawMessage(`{"source":"mobile"}`))
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if sess.UserID != userID {
		t.Errorf("user mismatch")
	}
	if sess.IsCompleted() {
		t.Errorf("session should not be completed yet")
	}

	// Advance clock by 25 minutes
	clk.Advance(25 * time.Minute)

	// 2. Complete session
	acts := 4
	completedSess, err := svc.CompleteSession(ctx, userID, sess.ID, &acts)
	if err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	if !completedSess.IsCompleted() {
		t.Errorf("session should be completed")
	}
	if completedSess.Minutes != 25 {
		t.Errorf("got minutes %d, want 25", completedSess.Minutes)
	}
	if completedSess.ActivitiesCompleted != 4 {
		t.Errorf("got activities %d, want 4", completedSess.ActivitiesCompleted)
	}

	recorded := events.recorded()
	if len(recorded) == 0 || recorded[0] != contract.EventLearningSessionCompleted {
		t.Errorf("expected session_completed event to be published, got %v", recorded)
	}

	// 3. Unowned session returns 404 (Trap 6)
	otherUser := uuid.New()
	_, err = svc.CompleteSession(ctx, otherUser, sess.ID, nil)
	if err == nil {
		t.Fatalf("expected error on completing unowned session, got nil")
	}
	if !domain.IsSessionNotFound(err) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestStartAttempt_EnforceEnrollmentAndLock(t *testing.T) {
	svc, _, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	unitID := uuid.New()
	lesson1 := uuid.New()
	lesson2 := uuid.New()
	act1 := uuid.New()
	act2 := uuid.New()

	reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
	reader.hierarchy[act1] = &lessoncontract.ActivityHierarchy{
		ActivityID: act1, LessonID: lesson1, UnitID: unitID, CourseID: courseID, Kind: testKindQuiz,
	}
	reader.hierarchy[act2] = &lessoncontract.ActivityHierarchy{
		ActivityID: act2, LessonID: lesson2, UnitID: unitID, CourseID: courseID, Kind: testKindQuiz,
	}

	// lesson2 requires lesson1
	reader.prereqs[lesson2] = []lessoncontract.PrerequisiteItem{
		{LessonID: lesson2, RequiresLessonID: lesson1, MinScore: 0},
	}

	// 1. Not enrolled in course -> ErrNotEnrolled
	_, err := svc.StartAttempt(ctx, userID, act1)
	if err == nil {
		t.Fatalf("expected ErrNotEnrolled when not enrolled, got nil")
	}
	if !domain.IsNotEnrolled(err) {
		t.Errorf("expected ErrNotEnrolled, got %v", err)
	}

	// 2. Enroll user
	_, err = svc.Enroll(ctx, userID, courseID)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	// 3. Attempt in locked lesson -> ErrLessonLocked
	_, err = svc.StartAttempt(ctx, userID, act2)
	if err == nil {
		t.Fatalf("expected ErrLessonLocked, got nil")
	}
	if !domain.IsLessonLocked(err) {
		t.Errorf("expected ErrLessonLocked, got %v", err)
	}

	// 4. Attempt in unlocked lesson1 -> success
	start, err := svc.StartAttempt(ctx, userID, act1)
	if err != nil {
		t.Fatalf("StartAttempt on unlocked lesson: %v", err)
	}
	if start.ActivityID != act1 {
		t.Errorf("got activity ID %s, want %s", start.ActivityID, act1)
	}
}

func TestNextActivity_AllCompletedAndEmptyLessons(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	lesson1 := uuid.New()
	lesson2 := uuid.New()

	// 1. Service with nil lesson reader returns error
	svcNoLesson := service.New(service.Deps{Repo: newFakeRepo()})
	_, err := svcNoLesson.NextActivity(ctx, userID)
	if err == nil {
		t.Errorf("expected error when lesson reader is nil")
	}

	// 2. All lessons completed in course -> StateCompleted
	svc, repo, reader, _ := setupTestService()
	_, _ = svc.Enroll(ctx, userID, courseID)

	reader.nextLesson = &lessoncontract.Lesson{ID: lesson1}
	score100 := int32(100)
	now := time.Now().UTC()
	_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       testScopeLesson,
		ScopeID:     lesson1,
		Status:      domain.ProgressCompleted,
		Score:       &score100,
		CompletedAt: &now,
	})

	res, err := svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateCompleted {
		t.Errorf("got state %s, want completed", res.State)
	}

	// 3. Lesson with empty activities -> StateInProgress, NextActivity is nil
	reader.nextLesson = &lessoncontract.Lesson{ID: lesson2, Activities: nil}
	reader.lessons[lesson2] = &lessoncontract.Lesson{ID: lesson2, Activities: nil}
	res, err = svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateInProgress || res.NextActivity != nil {
		t.Errorf("expected state in_progress with nil NextActivity, got %+v", res)
	}
}

func TestSession_IdempotentCompleteAndInvalidDuration(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	repo := newFakeRepo()
	svc := service.New(service.Deps{Repo: repo, Clock: clk})
	ctx := context.Background()
	userID := uuid.New()

	sess, err := svc.StartSession(ctx, userID, nil)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// 1. Invalid duration: clock moved backwards
	clk.Advance(-5 * time.Minute)
	_, err = svc.CompleteSession(ctx, userID, sess.ID, nil)
	if err == nil {
		t.Fatalf("expected ErrInvalidDuration when clock is behind started_at, got nil")
	}

	// 2. Normal completion
	clk.Advance(15 * time.Minute)
	completedSess, err := svc.CompleteSession(ctx, userID, sess.ID, nil)
	if err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	// 3. Idempotent second completion returns same completed session
	completedAgain, err := svc.CompleteSession(ctx, userID, sess.ID, nil)
	if err != nil {
		t.Fatalf("second CompleteSession: %v", err)
	}
	if completedAgain.ID != completedSess.ID {
		t.Errorf("id mismatch")
	}
}

func TestIsUnlocked_NilReader(t *testing.T) {
	svcNoLesson := service.New(service.Deps{Repo: newFakeRepo()})
	_, err := svcNoLesson.IsUnlocked(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil {
		t.Errorf("expected error when lesson reader is nil")
	}
}

func TestCompleteSession_WithEventWriter(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	repo := newFakeRepo()
	events := &fakeEventWriter{}
	svc := service.New(service.Deps{Repo: repo, Clock: clk, Events: events})
	ctx := context.Background()
	userID := uuid.New()

	sess, err := svc.StartSession(ctx, userID, []byte(`{"device":"tablet"}`))
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	clk.Advance(20 * time.Minute)
	acts := 5
	completed, err := svc.CompleteSession(ctx, userID, sess.ID, &acts)
	if err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	if completed.Minutes != 20 || completed.ActivitiesCompleted != 5 {
		t.Errorf("unexpected session completion: %+v", completed)
	}

	rec := events.recorded()
	if len(rec) != 1 || rec[0] != contract.EventLearningSessionCompleted {
		t.Errorf("expected EventLearningSessionCompleted event, got: %v", rec)
	}
}

func TestNextActivity_AllDroppedEnrollments(t *testing.T) {
	svc, repo, _, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()

	_, _ = repo.CreateEnrollment(ctx, userID, courseID, "dropped", time.Now())
	res, err := svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateCompleted {
		t.Errorf("expected state completed when no active enrollment, got: %s", res.State)
	}
}

// TestUnlockChecker_ReadsAreBatched holds the shape of the answer, not just its
// value. `lesson` calls this once for a whole course tree, so a checker that
// loops internally would move the N+1 rather than remove it — and no assertion
// on the returned map can tell the two apart.
func TestUnlockChecker_ReadsAreBatched(t *testing.T) {
	svc, repo, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()

	const lessonCount = 40
	required := uuid.New()
	lessonIDs := make([]uuid.UUID, 0, lessonCount)
	for i := 0; i < lessonCount; i++ {
		id := uuid.New()
		lessonIDs = append(lessonIDs, id)
		reader.prereqs[id] = []lessoncontract.PrerequisiteItem{
			{LessonID: id, RequiresLessonID: required, MinScore: 50},
		}
	}

	score := int32(90)
	now := time.Now().UTC()
	_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       testScopeLesson,
		ScopeID:     required,
		Status:      domain.ProgressCompleted,
		Score:       &score,
		CompletedAt: &now,
	})

	before := repo.queryCounter.Load()
	unlocked, err := svc.IsUnlocked(ctx, userID, lessonIDs)
	if err != nil {
		t.Fatalf("IsUnlocked: %v", err)
	}
	if len(unlocked) != lessonCount {
		t.Fatalf("got %d answers, want %d", len(unlocked), lessonCount)
	}

	if got := reader.callCount("ListPrerequisitesForLessons"); got != 1 {
		t.Errorf("prerequisite reads: got %d, want 1 for %d lessons", got, lessonCount)
	}
	if got := repo.queryCounter.Load() - before; got != 1 {
		t.Errorf("progress reads: got %d, want 1 for %d lessons", got, lessonCount)
	}
}

// TestNextActivity_ReadsAreBoundedByUnits is the one assertion that separates
// resolving the next activity from walking the course. Walking it with
// NextLesson costs a call per finished lesson, which is what this fails on.
func TestNextActivity_ReadsAreBoundedByUnits(t *testing.T) {
	svc, repo, reader, _ := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	_, _ = svc.Enroll(ctx, userID, courseID)

	const units, lessonsPerUnit = 3, 10
	now := time.Now().UTC()
	score := int32(100)
	var target uuid.UUID
	courseUnits := make([]*lessoncontract.Unit, 0, units)
	completed := 0
	for u := 0; u < units; u++ {
		unitID := uuid.New()
		courseUnits = append(courseUnits, &lessoncontract.Unit{ID: unitID, CourseID: courseID})
		unitLessons := make([]*lessoncontract.Lesson, 0, lessonsPerUnit)
		for l := 0; l < lessonsPerUnit; l++ {
			lessonID := uuid.New()
			unitLessons = append(unitLessons, &lessoncontract.Lesson{ID: lessonID, UnitID: unitID})
			if completed < 25 {
				_, _ = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
					UserID:      userID,
					Scope:       testScopeLesson,
					ScopeID:     lessonID,
					Status:      domain.ProgressCompleted,
					Score:       &score,
					CompletedAt: &now,
				})
				completed++
				continue
			}
			if target == uuid.Nil {
				target = lessonID
				reader.lessons[lessonID] = &lessoncontract.Lesson{
					ID: lessonID, UnitID: unitID, Title: "Unit 3 Lesson 6",
					SkillFocus: testSkillReading, EstimatedMinutes: 5,
					Activities: []lessoncontract.Activity{{ID: uuid.New(), Kind: testKindQuiz}},
				}
			}
		}
		reader.unitLesson[unitID] = unitLessons
	}
	reader.courseUnits[courseID] = courseUnits

	res, err := svc.NextActivity(ctx, userID)
	if err != nil {
		t.Fatalf("NextActivity: %v", err)
	}
	if res.State != domain.StateInProgress || res.NextActivity == nil {
		t.Fatalf("got %+v, want in_progress with a next activity", res)
	}
	if res.NextActivity.LessonID != target {
		t.Errorf("got lesson %s, want the first unfinished one %s", res.NextActivity.LessonID, target)
	}

	if got := reader.callCount("NextLesson"); got != 0 {
		t.Errorf("NextLesson calls: got %d, want 0 — the walk is what this test exists to prevent", got)
	}
	if got := reader.callCount("GetLesson"); got != 1 {
		t.Errorf("GetLesson calls: got %d, want 1 (only the chosen lesson)", got)
	}
	if got := reader.callCount("ListLessons"); got != units {
		t.Errorf("ListLessons calls: got %d, want %d (one per unit)", got, units)
	}
}

// A read that fails on the way to the next activity is an error, not an empty
// dashboard: an empty state and a broken query look identical to the client, and
// only one of them should be rendered as "you have nothing to do".
func TestNextActivity_PropagatesReaderFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("units cannot be listed", func(t *testing.T) {
		svc, _, reader, _ := setupTestService()
		userID := uuid.New()
		courseID := uuid.New()
		if _, err := svc.Enroll(ctx, userID, courseID); err != nil {
			t.Fatalf("Enroll: %v", err)
		}
		reader.listUnitsErr = errors.New("lesson module unavailable")

		if _, err := svc.NextActivity(ctx, userID); err == nil {
			t.Error("expected the unit read failure to surface")
		}
	})

	t.Run("the chosen lesson cannot be read", func(t *testing.T) {
		svc, _, reader, _ := setupTestService()
		userID := uuid.New()
		courseID := uuid.New()
		unitID := uuid.New()
		if _, err := svc.Enroll(ctx, userID, courseID); err != nil {
			t.Fatalf("Enroll: %v", err)
		}
		// Listed in the unit, absent from the reader's lesson map: GetLesson fails.
		reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
		reader.unitLesson[unitID] = []*lessoncontract.Lesson{{ID: uuid.New(), UnitID: unitID}}

		if _, err := svc.NextActivity(ctx, userID); err == nil {
			t.Error("expected the lesson read failure to surface")
		}
	})
}

// A grader marks on its own scale — GradeResult carries MaxScore for that reason.
// Mastery is a CEFR band over a percentage, so 8 out of 10 is a B2, not the A1 a
// raw 8 would map to.
func TestMastery_UsesTheGradersScale(t *testing.T) {
	svc, repo, reader, graders := setupTestService()
	ctx := context.Background()
	userID := uuid.New()
	courseID := uuid.New()
	unitID := uuid.New()
	lessonID := uuid.New()
	actID := uuid.New()

	const kind = "ten_point_quiz"
	if err := graders.Register(kind, &domain.FakeGrader{Score: 8, MaxScore: 10, Correct: true}); err != nil {
		t.Fatalf("register grader: %v", err)
	}

	if _, err := svc.Enroll(ctx, userID, courseID); err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	reader.courseUnits[courseID] = []*lessoncontract.Unit{{ID: unitID, CourseID: courseID}}
	reader.hierarchy[actID] = &lessoncontract.ActivityHierarchy{
		ActivityID: actID, LessonID: lessonID, UnitID: unitID, CourseID: courseID,
		Kind: kind, LessonSkillFocus: testSkillReading,
	}
	reader.lessons[lessonID] = &lessoncontract.Lesson{
		ID: lessonID, UnitID: unitID, SkillFocus: testSkillReading,
		Activities: []lessoncontract.Activity{{ID: actID}},
	}
	reader.unitLesson[unitID] = []*lessoncontract.Lesson{{ID: lessonID}}

	submitGraded(t, svc, userID, actID)

	mastery, err := repo.GetSkillMastery(ctx, userID, testSkillReading)
	if err != nil {
		t.Fatalf("GetSkillMastery: %v", err)
	}
	if mastery == nil {
		t.Fatal("no mastery recorded")
	}
	if mastery.Level != domain.LevelB2 {
		t.Errorf("got level %s for 8/10, want B2 — a raw score of 8 would be A1", mastery.Level)
	}
}
