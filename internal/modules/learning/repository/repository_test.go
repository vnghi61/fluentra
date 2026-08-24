package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// A repository built over a nil pool answers rather than panics: the unit suite
// constructs the module without a database, and every method it reaches has to
// have a defined answer. Reads that identify a row report "not found"; the rest
// report nothing, without an error.
func TestRepository_NilQueriesReturnEmpty(t *testing.T) {
	repo := New(nil)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	ctx := context.Background()
	uID := uuid.New()

	empties := []struct {
		name string
		call func() (any, error)
	}{
		{"CreateAttempt", func() (any, error) { return repo.CreateAttempt(ctx, CreateAttemptParams{}) }},
		{"GetProgressByUserScope", func() (any, error) {
			return repo.GetProgressByUserScope(ctx, uID, "activity", uID)
		}},
		{"UpsertProgress", func() (any, error) { return repo.UpsertProgress(ctx, UpsertProgressParams{}) }},
		{"GetEnrollmentByUserCourse", func() (any, error) { return repo.GetEnrollmentByUserCourse(ctx, uID, uID) }},
		{"CreateEnrollment", func() (any, error) {
			return repo.CreateEnrollment(ctx, uID, uID, "active", time.Now())
		}},
		{"UpdateEnrollmentStatus", func() (any, error) {
			return repo.UpdateEnrollmentStatus(ctx, uID, uID, "completed", nil)
		}},
		{"CreateLearningSession", func() (any, error) { return repo.CreateLearningSession(ctx, uID, time.Now(), nil) }},
		{"GetSkillMastery", func() (any, error) { return repo.GetSkillMastery(ctx, uID, "grammar") }},
		{"UpsertSkillMastery", func() (any, error) { return repo.UpsertSkillMastery(ctx, uID, "grammar", "B1", 0.5) }},
	}
	for _, tc := range empties {
		got, err := tc.call()
		if err != nil {
			t.Errorf("%s: got error %v, want nil", tc.name, err)
		}
		if !isNilResult(got) {
			t.Errorf("%s: got %v, want a nil result", tc.name, got)
		}
	}

	lists := []struct {
		name string
		call func() (int, error)
	}{
		{"ListProgressByUserScopeAndIDs", func() (int, error) {
			rows, err := repo.ListProgressByUserScopeAndIDs(ctx, uID, "activity", []uuid.UUID{uID})
			return len(rows), err
		}},
		{"ListProgressByUserAndScope", func() (int, error) {
			rows, err := repo.ListProgressByUserAndScope(ctx, uID, "lesson", 10)
			return len(rows), err
		}},
		{"ListProgressByUser", func() (int, error) {
			rows, err := repo.ListProgressByUser(ctx, uID, 10)
			return len(rows), err
		}},
		{"ListEnrollmentsByUser", func() (int, error) {
			rows, err := repo.ListEnrollmentsByUser(ctx, uID, 10)
			return len(rows), err
		}},
		{"ListSkillMasteryByUser", func() (int, error) {
			rows, err := repo.ListSkillMasteryByUser(ctx, uID)
			return len(rows), err
		}},
	}
	for _, tc := range lists {
		n, err := tc.call()
		if err != nil || n != 0 {
			t.Errorf("%s: got %d rows and error %v, want 0 and nil", tc.name, n, err)
		}
	}

	if repo.WithTx(nil) == nil {
		t.Fatal("expected non-nil repo with tx")
	}
}

func TestRepository_NilQueriesReportNotFound(t *testing.T) {
	repo := New(nil)
	ctx := context.Background()
	uID := uuid.New()

	cases := []struct {
		name string
		call func() error
		want error
	}{
		{"GetAttemptByID", func() error { _, err := repo.GetAttemptByID(ctx, uID); return err }, domain.ErrAttemptNotFound},
		{"ClaimAttemptForGrading", func() error {
			_, err := repo.ClaimAttemptForGrading(ctx, ClaimAttemptParams{ID: uID, CreatedAt: time.Now()})
			return err
		}, domain.ErrAttemptNotFound},
		{"UpdateAttemptStatus", func() error {
			_, err := repo.UpdateAttemptStatus(ctx, UpdateAttemptStatusParams{ID: uID, CreatedAt: time.Now()})
			return err
		}, domain.ErrAttemptNotFound},
		{"GetLearningSessionByID", func() error {
			_, err := repo.GetLearningSessionByID(ctx, uID)
			return err
		}, domain.ErrSessionNotFound},
		{"CompleteLearningSession", func() error {
			_, err := repo.CompleteLearningSession(ctx, uID, time.Now(), 0, 0)
			return err
		}, domain.ErrSessionNotFound},
	}
	for _, tc := range cases {
		if err := tc.call(); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
}

// isNilResult reports whether a typed nil pointer came back through an `any`,
// which a plain `got != nil` cannot see.
func isNilResult(v any) bool {
	switch typed := v.(type) {
	case nil:
		return true
	case *domain.Attempt:
		return typed == nil
	case *domain.Enrollment:
		return typed == nil
	case *domain.LearningSession:
		return typed == nil
	case *domain.SkillMastery:
		return typed == nil
	case *ProgressDTO:
		return typed == nil
	default:
		return false
	}
}

func TestRepository_ConvertsAttemptRows(t *testing.T) {
	now := time.Now().UTC()
	key := uuid.New()
	score := int32(95)
	dur := int32(1200)
	grader := "quiz"

	row := sqlc.LearnAttempt{
		ID:             uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
		UserID:         uuid.New(),
		ActivityID:     uuid.New(),
		IdempotencyKey: &key,
		Response:       []byte(`{"answer":1}`),
		Score:          &score,
		MaxScore:       100,
		Grader:         &grader,
		DurationMs:     dur,
		Status:         domain.StatusGraded,
	}

	domainAtt := toDomainAttempt(row)
	if domainAtt.ID != row.ID || domainAtt.Status != domain.StatusGraded {
		t.Errorf("unexpected domain attempt conversion: %+v", domainAtt)
	}
	if domainAtt.Score == nil || *domainAtt.Score != 95 {
		t.Errorf("got score %v, want 95", domainAtt.Score)
	}
	if domainAtt.DurationMs == nil || *domainAtt.DurationMs != 1200 {
		t.Errorf("got duration %v, want 1200", domainAtt.DurationMs)
	}
}

func TestRepository_ConvertsProgressEnrolmentAndSessionRows(t *testing.T) {
	now := time.Now().UTC()
	score := int32(95)

	progRow := sqlc.LearnProgress{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		Scope:       "lesson",
		ScopeID:     uuid.New(),
		Status:      "completed",
		Score:       &score,
		CompletedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	progDTO := toProgressDTO(progRow)
	if progDTO.Status != "completed" || progDTO.Score == nil || *progDTO.Score != 95 {
		t.Errorf("unexpected progress DTO conversion: %+v", progDTO)
	}

	enrRow := sqlc.LearnEnrollment{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		CourseID:    uuid.New(),
		Status:      "active",
		StartedAt:   now,
		CompletedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	enrDomain := toDomainEnrollment(enrRow)
	if enrDomain.ID != enrRow.ID || enrDomain.Status != domain.StatusEnrollmentActive || enrDomain.CompletedAt == nil {
		t.Errorf("unexpected domain enrollment conversion: %+v", enrDomain)
	}

	end := now.Add(10 * time.Minute)
	sessRow := sqlc.LearnLearningSession{
		ID:                  uuid.New(),
		UserID:              uuid.New(),
		StartedAt:           now,
		EndedAt:             &end,
		Minutes:             10,
		ActivitiesCompleted: 3,
		Metadata:            []byte(`{"source":"mobile"}`),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	sessDomain := toDomainSession(sessRow)
	if sessDomain.ID != sessRow.ID || sessDomain.Minutes != 10 || sessDomain.ActivitiesCompleted != 3 {
		t.Errorf("unexpected domain learning session conversion: %+v", sessDomain)
	}

	mRow := sqlc.LearnSkillMastery{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Skill:     "vocabulary",
		Level:     "B2",
		UpdatedAt: now,
		CreatedAt: now,
	}
	_ = mRow.Confidence.Scan("0.78")
	mDomain := toDomainSkillMastery(mRow)
	if mDomain.ID != mRow.ID || mDomain.Skill != "vocabulary" || mDomain.Level != "B2" || mDomain.Confidence != 0.78 {
		t.Errorf("unexpected domain skill mastery conversion: %+v", mDomain)
	}
}

func TestRepository_MapPgError(t *testing.T) {
	if mapPgError(nil) != nil {
		t.Errorf("expected nil for nil error")
	}

	uniqueErr := &pgconn.PgError{Code: "23505"}
	if !errors.Is(mapPgError(uniqueErr), domain.ErrIdempotencyConflict) {
		t.Errorf("expected ErrIdempotencyConflict, got: %v", mapPgError(uniqueErr))
	}

	checkErr := &pgconn.PgError{Code: "23514", ConstraintName: "ck_attempts_status"}
	if !errors.Is(mapPgError(checkErr), domain.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got: %v", mapPgError(checkErr))
	}

	otherErr := errors.New("other database error")
	if mapPgError(otherErr) != otherErr {
		t.Errorf("expected unchanged error, got: %v", mapPgError(otherErr))
	}
}
