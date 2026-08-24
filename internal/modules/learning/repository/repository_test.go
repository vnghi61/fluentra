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

func TestRepository_NilQueriesSafety(t *testing.T) {
	repo := New(nil)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}

	ctx := context.Background()
	uID := uuid.New()

	att, err := repo.CreateAttempt(ctx, CreateAttemptParams{})
	if err != nil || att != nil {
		t.Errorf("expected nil result without error on nil queries, got att=%v, err=%v", att, err)
	}

	_, err = repo.GetAttemptByID(ctx, uID)
	if !errors.Is(err, domain.ErrAttemptNotFound) {
		t.Errorf("expected ErrAttemptNotFound on nil queries, got: %v", err)
	}

	_, err = repo.ClaimAttemptForGrading(ctx, ClaimAttemptParams{ID: uID, CreatedAt: time.Now()})
	if !errors.Is(err, domain.ErrAttemptNotFound) {
		t.Errorf("expected ErrAttemptNotFound on nil queries, got: %v", err)
	}

	_, err = repo.UpdateAttemptStatus(ctx, UpdateAttemptStatusParams{ID: uID, CreatedAt: time.Now()})
	if !errors.Is(err, domain.ErrAttemptNotFound) {
		t.Errorf("expected ErrAttemptNotFound on nil queries, got: %v", err)
	}

	p, err := repo.GetProgressByUserScope(ctx, uID, "activity", uID)
	if err != nil || p != nil {
		t.Errorf("expected nil progress without error, got p=%v, err=%v", p, err)
	}

	up, err := repo.UpsertProgress(ctx, UpsertProgressParams{})
	if err != nil || up != nil {
		t.Errorf("expected nil progress without error on nil queries, got up=%v, err=%v", up, err)
	}

	ps, err := repo.ListProgressByUserScopeAndIDs(ctx, uID, "activity", []uuid.UUID{uID})
	if err != nil || len(ps) != 0 {
		t.Errorf("expected empty progress list without error, got len=%d, err=%v", len(ps), err)
	}

	if repo.WithTx(nil) == nil {
		t.Fatal("expected non-nil repo with tx")
	}
}

func TestRepository_Converters(t *testing.T) {
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
