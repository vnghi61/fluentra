//go:build integration

package learning_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
)

// These run the placement queries against PostgreSQL: the partial unique index,
// the version guard, the session ↔ result keys and the jsonb lookup are all
// things the in-memory repository re-implements and the SQL could get wrong.

func seedPlacementUser(t *testing.T) uuid.UUID {
	t.Helper()
	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	userID := uuid.New()
	if _, err := attemptPool.Exec(context.Background(),
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		userID, fmt.Sprintf("placement-%s@example.com", userID),
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = attemptPool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})
	return userID
}

func newPlacementSession(userID uuid.UUID, now time.Time) *domain.PlacementSession {
	estimate := domain.NewPlacementEstimate()
	estimate.Observe(domain.Observation{
		Skill: domain.SkillVocabulary, Band: domain.LevelB1, Correct: true, Options: 4,
	})
	return &domain.PlacementSession{
		ID:               uuid.New(),
		UserID:           userID,
		Status:           domain.PlacementInProgress,
		Stage:            domain.PlacementStageVocabularyGrammar,
		StartedAt:        now,
		DeadlineAt:       now.Add(domain.PlacementTimeLimit),
		Estimate:         estimate,
		Items:            []domain.PlacementItem{},
		ProductiveStatus: domain.ProductiveOffered,
	}
}

func TestPlacementStorage_OneSessionAtATimeAndStaleWritesFail_Integration(t *testing.T) {
	userID := seedPlacementUser(t)
	ctx := context.Background()
	repo := repository.New(attemptPool)
	now := time.Now().UTC().Truncate(time.Second)

	stored, err := repo.CreatePlacementSession(ctx, newPlacementSession(userID, now))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if stored.Estimate.Responses() != 1 {
		t.Errorf("the estimate did not round-trip: %+v", stored.Estimate)
	}

	if _, err := repo.CreatePlacementSession(ctx, newPlacementSession(userID, now)); !errors.Is(
		err, domain.ErrPlacementInProgress) {
		t.Fatalf("a second session in progress = %v, want ErrPlacementInProgress", err)
	}

	stale := *stored
	stored.Stage = domain.PlacementStageReadingListening
	saved, err := repo.SavePlacementProgress(ctx, stored)
	if err != nil {
		t.Fatalf("save progress: %v", err)
	}
	if saved.Version != stored.Version+1 {
		t.Errorf("version = %d, want %d", saved.Version, stored.Version+1)
	}
	if _, err := repo.SavePlacementProgress(ctx, &stale); !errors.Is(err, domain.ErrPlacementConflict) {
		t.Fatalf("a write from a stale read = %v, want ErrPlacementConflict", err)
	}
}

func TestPlacementStorage_FinishingLinksTheResultAndFindsTheAttempt_Integration(t *testing.T) {
	userID := seedPlacementUser(t)
	ctx := context.Background()
	repo := repository.New(attemptPool)
	now := time.Now().UTC().Truncate(time.Second)

	session, err := repo.CreatePlacementSession(ctx, newPlacementSession(userID, now))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	attemptID := uuid.New()
	session.Items = append(session.Items, domain.PlacementItem{
		ActivityID: uuid.New(), Kind: "writing_prompt", Skill: domain.SkillWriting, Band: domain.LevelB1,
		Part: domain.PlacementPartProductive, ServedAt: now, AttemptID: &attemptID, Status: domain.StatusGrading,
	})

	sessionID := session.ID
	result, err := repo.CreatePlacementResult(ctx, &domain.PlacementResult{
		UserID:    userID,
		SessionID: &sessionID,
		Level:     domain.LevelB1,
		PerSkill:  map[string]domain.SkillEstimate{domain.SkillVocabulary: {Band: domain.LevelB1, Responses: 8}},
		TakenAt:   now,
	})
	if err != nil {
		t.Fatalf("create result: %v", err)
	}
	session.Status = domain.PlacementCompleted
	session.ResultID = &result.ID
	session.CompletedAt = &now
	if _, err := repo.FinishPlacementSession(ctx, session); err != nil {
		t.Fatalf("finish session: %v", err)
	}

	assertFinishedSession(t, repo, userID, session.ID, result.ID, attemptID)

	perSkill := result.PerSkill
	perSkill[domain.SkillWriting] = domain.SkillEstimate{Band: domain.LevelB2, Responses: 1}
	if _, err := repo.UpdatePlacementResultPerSkill(ctx, result.ID, perSkill); err != nil {
		t.Fatalf("update per_skill: %v", err)
	}
	current, err := repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil || current == nil {
		t.Fatalf("current result: %v", err)
	}
	if current.PerSkill[domain.SkillWriting].Band != domain.LevelB2 {
		t.Errorf("per_skill = %+v, want writing B2", current.PerSkill)
	}

	// With the session finished, a new one may start.
	if _, err := repo.CreatePlacementSession(ctx, newPlacementSession(userID, now)); err != nil {
		t.Fatalf("a new session after finishing: %v", err)
	}
}

func assertFinishedSession(
	t *testing.T, repo *repository.Repository, userID, sessionID, resultID, attemptID uuid.UUID,
) {
	t.Helper()
	ctx := context.Background()
	latest, err := repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil || latest == nil || latest.ResultID == nil || *latest.ResultID != resultID {
		t.Fatalf("latest completed = %+v, %v; want the session linked to its result", latest, err)
	}
	found, err := repo.FindPlacementSessionByAttempt(ctx, userID, attemptID)
	if err != nil || found == nil || found.ID != sessionID {
		t.Fatalf("session by attempt = %+v, %v; want the session that served it", found, err)
	}
}

func TestPlacementStorage_TheSweepSeesOnlyOverdueOpenSessions_Integration(t *testing.T) {
	overdue := seedPlacementUser(t)
	current := seedPlacementUser(t)
	ctx := context.Background()
	repo := repository.New(attemptPool)
	now := time.Now().UTC().Truncate(time.Second)

	old, err := repo.CreatePlacementSession(ctx, newPlacementSession(overdue, now.Add(-time.Hour)))
	if err != nil {
		t.Fatalf("create overdue session: %v", err)
	}
	fresh, err := repo.CreatePlacementSession(ctx, newPlacementSession(current, now))
	if err != nil {
		t.Fatalf("create current session: %v", err)
	}

	ids, err := repo.ListOverduePlacementSessions(ctx, now)
	if err != nil {
		t.Fatalf("list overdue: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen[old.ID] || seen[fresh.ID] {
		t.Errorf("overdue ids %v: want %s and not %s", ids, old.ID, fresh.ID)
	}
}

func TestWeeklyPlanStorage_TheFirstPlanOfTheWeekIsThePlan_Integration(t *testing.T) {
	userID := seedPlacementUser(t)
	ctx := context.Background()
	repo := repository.New(attemptPool)
	monday := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	first := &domain.WeeklyPlan{UserID: userID, WeekStart: monday, MinutesGoal: 90, Items: []domain.WeeklyPlanItem{
		{Kind: domain.PlanItemDailyPractice, Minutes: domain.DailyPracticeMinutes},
	}}
	if stored, err := repo.CreateWeeklyPlan(ctx, first); err != nil || stored == nil {
		t.Fatalf("create plan: %+v, %v", stored, err)
	}
	second := &domain.WeeklyPlan{UserID: userID, WeekStart: monday, MinutesGoal: 300}
	if stored, err := repo.CreateWeeklyPlan(ctx, second); err != nil || stored != nil {
		t.Fatalf("a second plan for the week = %+v, %v; want nil, the first one stands", stored, err)
	}
	read, err := repo.GetWeeklyPlan(ctx, userID, monday)
	if err != nil || read == nil || read.MinutesGoal != 90 || len(read.Items) != 1 {
		t.Fatalf("plan read back = %+v, %v; want the first plan", read, err)
	}
}

// SubmitSittingAnswer created its attempt with an empty status, which the
// attempts status CHECK refuses: no exam or placement answer could be recorded
// against a real database while every unit test passed.
func TestSubmitSittingAnswer_RecordsAGradedAttempt_Integration(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()

	result, err := f.svc.SubmitSittingAnswer(ctx, contract.SittingAnswerRequest{
		UserID:         f.userID,
		ActivityID:     f.activity,
		Response:       json.RawMessage(`{"selected_option_id": "A"}`),
		IdempotencyKey: uuid.New(),
	})
	if err != nil {
		t.Fatalf("submit sitting answer: %v", err)
	}
	var status string
	if err := attemptPool.QueryRow(ctx,
		`SELECT status FROM learn.attempts WHERE id = $1`, result.AttemptID,
	).Scan(&status); err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	if status != domain.StatusGraded {
		t.Errorf("attempt status = %q, want graded", status)
	}
}
