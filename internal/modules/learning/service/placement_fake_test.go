package service_test

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// placementStore is the fake's placement storage. It keeps copies and checks
// versions the way the guarded SQL updates do, so a test sees a stale write fail.
type placementStore struct {
	sessions map[uuid.UUID]*domain.PlacementSession
	results  map[uuid.UUID]*domain.PlacementResult
	plans    map[string]*domain.WeeklyPlan
	minutes  int
	// practiced and graded are what the week's progress queries count.
	practiced int
	graded    map[string]int
}

func (f *fakeLearningRepo) store() *placementStore {
	if f.placements == nil {
		f.placements = &placementStore{
			sessions: map[uuid.UUID]*domain.PlacementSession{},
			results:  map[uuid.UUID]*domain.PlacementResult{},
			plans:    map[string]*domain.WeeklyPlan{},
			graded:   map[string]int{},
		}
	}
	return f.placements
}

func cloneSession(s *domain.PlacementSession) *domain.PlacementSession {
	if s == nil {
		return nil
	}
	c := *s
	c.Items = append([]domain.PlacementItem{}, s.Items...)
	c.Estimate.Observations = append([]domain.Observation{}, s.Estimate.Observations...)
	c.Estimate.Exhausted = append([]string(nil), s.Estimate.Exhausted...)
	return &c
}

func cloneResult(r *domain.PlacementResult) *domain.PlacementResult {
	if r == nil {
		return nil
	}
	c := *r
	c.PerSkill = map[string]domain.SkillEstimate{}
	for skill, estimate := range r.PerSkill {
		c.PerSkill[skill] = estimate
	}
	return &c
}

func (f *fakeLearningRepo) CreatePlacementSession(
	_ context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.store().sessions {
		if existing.UserID == session.UserID && existing.Status == domain.PlacementInProgress {
			return nil, domain.ErrPlacementInProgress
		}
	}
	stored := cloneSession(session)
	stored.Version = 0
	f.store().sessions[stored.ID] = stored
	return cloneSession(stored), nil
}

func (f *fakeLearningRepo) GetPlacementSession(_ context.Context, id uuid.UUID) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneSession(f.store().sessions[id]), nil
}

func (f *fakeLearningRepo) GetOpenPlacementSession(
	_ context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.store().sessions {
		if s.UserID == userID && s.Status == domain.PlacementInProgress {
			return cloneSession(s), nil
		}
	}
	return nil, nil
}

func (f *fakeLearningRepo) GetLatestCompletedPlacementSession(
	_ context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *domain.PlacementSession
	for _, s := range f.store().sessions {
		if s.UserID != userID || s.Status != domain.PlacementCompleted || s.CompletedAt == nil {
			continue
		}
		if latest == nil || s.CompletedAt.After(*latest.CompletedAt) {
			latest = s
		}
	}
	return cloneSession(latest), nil
}

func (f *fakeLearningRepo) FindPlacementSessionByAttempt(
	_ context.Context, userID, attemptID uuid.UUID,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.store().sessions {
		if s.UserID != userID {
			continue
		}
		for _, item := range s.Items {
			if item.AttemptID != nil && *item.AttemptID == attemptID {
				return cloneSession(s), nil
			}
		}
	}
	return nil, nil
}

// guardedSession mirrors WHERE id = $1 AND version = $2 [AND status = 'in_progress'].
func (f *fakeLearningRepo) guardedSession(session *domain.PlacementSession, open bool) *domain.PlacementSession {
	stored := f.store().sessions[session.ID]
	if stored == nil || stored.Version != session.Version {
		return nil
	}
	if open && stored.Status != domain.PlacementInProgress {
		return nil
	}
	return stored
}

func (f *fakeLearningRepo) SavePlacementProgress(
	_ context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.guardedSession(session, true)
	if stored == nil {
		return nil, domain.ErrPlacementConflict
	}
	next := cloneSession(session)
	next.Status = stored.Status
	next.Version = stored.Version + 1
	f.store().sessions[next.ID] = next
	return cloneSession(next), nil
}

func (f *fakeLearningRepo) FinishPlacementSession(
	_ context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.guardedSession(session, true)
	if stored == nil {
		return nil, domain.ErrPlacementConflict
	}
	next := cloneSession(session)
	next.Stage = domain.PlacementStageDone
	next.Version = stored.Version + 1
	f.store().sessions[next.ID] = next
	return cloneSession(next), nil
}

func (f *fakeLearningRepo) SavePlacementProductive(
	_ context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.guardedSession(session, false)
	if stored == nil {
		return nil, domain.ErrPlacementConflict
	}
	next := cloneSession(stored)
	next.Items = append([]domain.PlacementItem{}, session.Items...)
	next.ProductiveStatus = session.ProductiveStatus
	next.ProductiveDeadlineAt = session.ProductiveDeadlineAt
	next.Version = stored.Version + 1
	f.store().sessions[next.ID] = next
	return cloneSession(next), nil
}

func (f *fakeLearningRepo) ListOverduePlacementSessions(_ context.Context, cutoff time.Time) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []uuid.UUID
	for id, s := range f.store().sessions {
		if s.Status == domain.PlacementInProgress && s.DeadlineAt.Before(cutoff) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeLearningRepo) CreatePlacementResult(
	_ context.Context, result *domain.PlacementResult,
) (*domain.PlacementResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := cloneResult(result)
	stored.ID = uuid.New()
	f.store().results[stored.ID] = stored
	return cloneResult(stored), nil
}

func (f *fakeLearningRepo) GetPlacementResult(_ context.Context, id uuid.UUID) (*domain.PlacementResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneResult(f.store().results[id]), nil
}

func (f *fakeLearningRepo) GetCurrentPlacementResult(
	_ context.Context, userID uuid.UUID,
) (*domain.PlacementResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var current *domain.PlacementResult
	for _, r := range f.store().results {
		if r.UserID == userID && (current == nil || r.TakenAt.After(current.TakenAt)) {
			current = r
		}
	}
	return cloneResult(current), nil
}

func (f *fakeLearningRepo) UpdatePlacementResultPerSkill(
	_ context.Context, id uuid.UUID, perSkill map[string]domain.SkillEstimate,
) (*domain.PlacementResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.store().results[id]
	if stored == nil {
		return nil, nil
	}
	stored.PerSkill = cloneResult(&domain.PlacementResult{PerSkill: perSkill}).PerSkill
	return cloneResult(stored), nil
}

func weekKey(userID uuid.UUID, weekStart time.Time) string {
	return userID.String() + "/" + weekStart.Format(time.DateOnly)
}

func (f *fakeLearningRepo) GetWeeklyPlan(
	_ context.Context, userID uuid.UUID, weekStart time.Time,
) (*domain.WeeklyPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	plan := f.store().plans[weekKey(userID, weekStart)]
	if plan == nil {
		return nil, nil
	}
	c := *plan
	return &c, nil
}

func (f *fakeLearningRepo) CreateWeeklyPlan(_ context.Context, plan *domain.WeeklyPlan) (*domain.WeeklyPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := weekKey(plan.UserID, plan.WeekStart)
	if _, exists := f.store().plans[key]; exists {
		return nil, nil
	}
	c := *plan
	f.store().plans[key] = &c
	return plan, nil
}

func (f *fakeLearningRepo) SumLearningMinutesBetween(context.Context, uuid.UUID, time.Time, time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store().minutes, nil
}

func (f *fakeLearningRepo) CountPracticedDailySetsBetween(
	context.Context, uuid.UUID, time.Time, time.Time, time.Time,
) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store().practiced, nil
}

func (f *fakeLearningRepo) CountAttemptsByGradersBetween(
	_ context.Context, _ uuid.UUID, graders []string, _, _ time.Time,
) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, grader := range graders {
		total += f.store().graded[grader]
	}
	return total, nil
}
