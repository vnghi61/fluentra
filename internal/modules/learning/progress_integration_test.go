//go:build integration

// Enrolment, sessions and mastery against a real PostgreSQL instance.
//
// The service suite drives these through an in-memory repository, which cannot
// fail a CHECK constraint, cannot violate a foreign key, and stores a float where
// the column is numeric(5,2). Everything asserted here is a claim about the
// database rather than about the fake: that a second enrolment is a conflict
// rather than a second row, that a course id with no course is a 404 rather than
// a 500, and that a confidence written as 0.78 reads back as 0.78.
package learning_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
)

func newProgressRepository(t *testing.T) *repository.Repository {
	t.Helper()
	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return repository.New(attemptPool)
}

func TestEnrollment_ConstraintsAreTheSourceOfTruth_Integration(t *testing.T) {
	repo := newProgressRepository(t)
	f := newAttemptFixture(t)
	ctx := context.Background()

	// The fixture already enrolled this learner, so a second enrolment must be
	// the unique constraint talking, not a check-then-write in the service.
	_, err := f.svc.Enroll(ctx, f.userID, f.courseID)
	if !domain.IsAlreadyEnrolled(err) {
		t.Errorf("second enrolment: got %v, want ALREADY_ENROLLED", err)
	}

	// A course id with no course behind it is the foreign key talking.
	_, err = f.svc.Enroll(ctx, f.userID, uuid.New())
	if !domain.IsCourseNotFound(err) {
		t.Errorf("enrolment in an unknown course: got %v, want COURSE_NOT_FOUND", err)
	}

	enrollment, err := repo.GetEnrollmentByUserCourse(ctx, f.userID, f.courseID)
	if err != nil {
		t.Fatalf("read enrolment: %v", err)
	}
	if enrollment == nil || enrollment.Status != domain.StatusEnrollmentActive {
		t.Fatalf("got %+v, want an active enrolment", enrollment)
	}

	// ck_enrollments_completed_at couples the two columns: completing without a
	// timestamp is rejected by the database, which is why the rollup sets both.
	completedAt := time.Now().UTC()
	updated, err := repo.UpdateEnrollmentStatus(
		ctx, f.userID, f.courseID, domain.StatusEnrollmentCompleted, &completedAt,
	)
	if err != nil {
		t.Fatalf("complete enrolment: %v", err)
	}
	if updated == nil || updated.CompletedAt == nil {
		t.Fatalf("got %+v, want a completed enrolment carrying completed_at", updated)
	}

	if _, err := repo.UpdateEnrollmentStatus(
		ctx, f.userID, f.courseID, domain.StatusEnrollmentCompleted, nil,
	); err == nil {
		t.Error("completing an enrolment with a null completed_at was accepted; the CHECK forbids it")
	}
}

func TestLearningSession_RoundTrip_Integration(t *testing.T) {
	repo := newProgressRepository(t)
	f := newAttemptFixture(t)
	ctx := context.Background()

	started, err := f.svc.StartSession(ctx, f.userID, []byte(`{"source":"integration"}`))
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if started.Minutes != 0 || started.ActivitiesCompleted != 0 {
		t.Errorf("new session: got %+v, want zeroed counters", started)
	}

	stored, err := repo.GetLearningSessionByID(ctx, started.ID)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	// jsonb re-renders what it stored, so the comparison is on the decoded value
	// rather than on the bytes.
	var storedMeta map[string]string
	if err := json.Unmarshal(stored.Metadata, &storedMeta); err != nil {
		t.Fatalf("decode metadata %s: %v", stored.Metadata, err)
	}
	if storedMeta["source"] != "integration" {
		t.Errorf("metadata: got %v, want the object the caller sent", storedMeta)
	}

	// Another learner's session id is a 404, not somebody else's session.
	if _, err := f.svc.CompleteSession(ctx, uuid.New(), started.ID, nil); !domain.IsSessionNotFound(err) {
		t.Errorf("completing another learner's session: got %v, want SESSION_NOT_FOUND", err)
	}

	count := 3
	completed, err := f.svc.CompleteSession(ctx, f.userID, started.ID, &count)
	if err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}
	if completed.EndedAt == nil || completed.ActivitiesCompleted != count {
		t.Errorf("completed session: got %+v, want ended with %d activities", completed, count)
	}
	if completed.Minutes < 0 {
		t.Errorf("minutes: got %d, want a server-computed non-negative duration", completed.Minutes)
	}

	// ck_learning_sessions_activities_completed is >= 0, and the service rejects
	// the request before the constraint turns it into a 500.
	negative := -1
	if _, err := f.svc.CompleteSession(ctx, f.userID, started.ID, &negative); err == nil {
		t.Error("a negative activity count was accepted")
	}
}

func TestSkillMastery_NumericRoundTrip_Integration(t *testing.T) {
	repo := newProgressRepository(t)
	f := newAttemptFixture(t)
	ctx := context.Background()

	if _, err := repo.UpsertSkillMastery(ctx, f.userID, "vocabulary", "B2", 0.78); err != nil {
		t.Fatalf("upsert mastery: %v", err)
	}
	stored, err := repo.GetSkillMastery(ctx, f.userID, "vocabulary")
	if err != nil {
		t.Fatalf("read mastery: %v", err)
	}
	// numeric(5,2): the column keeps two places, and a test that expects more
	// fails for a reason that has nothing to do with the estimator.
	if stored == nil || stored.Level != "B2" || stored.Confidence != 0.78 {
		t.Fatalf("got %+v, want B2 at 0.78", stored)
	}

	if _, err := repo.UpsertSkillMastery(ctx, f.userID, "vocabulary", "C1", 0.9); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	rows, err := repo.ListSkillMasteryByUser(ctx, f.userID)
	if err != nil {
		t.Fatalf("list mastery: %v", err)
	}
	if len(rows) != 1 || rows[0].Level != "C1" {
		t.Fatalf("got %+v, want one row upserted to C1", rows)
	}

	// The CHECK is the reason updateSkillMastery normalises first: an unmapped
	// skill focus reaching this call is what would abort a grading transaction.
	if _, err := repo.UpsertSkillMastery(ctx, f.userID, "non_standard_skill_tag", "B1", 0.5); err == nil {
		t.Error("a skill outside the CHECK was accepted")
	}
}

func TestProgressReader_ReadsWhatTheRollupWrote_Integration(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()

	started, err := f.svc.StartAttempt(ctx, f.userID, f.activity)
	if err != nil {
		t.Fatalf("StartAttempt: %v", err)
	}
	if _, err := f.svc.SubmitAttempt(
		ctx, f.userID, started.AttemptID, uuid.New(), []byte(`{"choice":1}`),
	); err != nil {
		t.Fatalf("SubmitAttempt: %v", err)
	}

	for _, scope := range []contract.ProgressScope{
		contract.ScopeActivity, contract.ScopeLesson, contract.ScopeUnit, contract.ScopeCourse,
	} {
		rows, err := f.svc.ProgressOf(ctx, f.userID, scope)
		if err != nil {
			t.Fatalf("ProgressOf %s: %v", scope, err)
		}
		if len(rows) != 1 {
			t.Fatalf("%s: got %d rows, want 1", scope, len(rows))
		}
		if rows[0].Scope != scope || rows[0].Status != domain.ProgressCompleted {
			t.Errorf("%s: got %+v, want a completed row in that scope", scope, rows[0])
		}
	}

	// The rollup closed the enrolment in the same transaction as the course row.
	enrollments, err := repository.New(attemptPool).ListEnrollmentsByUser(ctx, f.userID, 10)
	if err != nil {
		t.Fatalf("list enrolments: %v", err)
	}
	if len(enrollments) != 1 || !enrollments[0].IsCompleted() {
		t.Fatalf("got %+v, want the enrolment completed with the course", enrollments)
	}

	// Mastery came from the same transaction, off the lesson's skill focus.
	mastery, err := repository.New(attemptPool).GetSkillMastery(ctx, f.userID, "vocabulary")
	if err != nil {
		t.Fatalf("read mastery: %v", err)
	}
	if mastery == nil {
		t.Fatal("no mastery recorded for the graded attempt")
	}
}
