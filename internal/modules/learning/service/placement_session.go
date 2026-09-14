package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// --------------------------------------------------------------------------
// Placement Test Lifecycle (WO13 §3.5)
// --------------------------------------------------------------------------

// GetPlacementInvitation checks eligibility and returns current placement invitation status.
func (s *Service) GetPlacementInvitation(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementInvitationDTO, error) {
	now := s.clock.Now().UTC()

	sufficient, err := s.HasSufficientPlacementPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("check pool sufficiency: %w", err)
	}

	dto := &domain.PlacementInvitationDTO{
		PoolSufficient: sufficient,
		Eligible:       sufficient,
	}

	// 1. Check for an active in-progress test
	active, err := s.repo.GetActivePlacementSessionByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check active placement session: %w", err)
	}
	if active != nil {
		if active.IsExpired(now) {
			// Session expired
			dto.HasActiveTest = false
		} else {
			dto.HasActiveTest = true
			dto.ActiveSessionID = &active.ID
			dto.Eligible = false
			return dto, nil
		}
	}

	// 2. Check for latest completed session cooldown (30 days)
	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check latest completed session: %w", err)
	}
	if latest != nil && latest.CompletedAt != nil {
		cooldownUntil := latest.CompletedAt.Add(domain.PlacementRetakeCooldown)
		if now.Before(cooldownUntil) {
			dto.Eligible = false
			dto.CooldownUntil = &cooldownUntil
		}
		// Attach last result if available
		perSkill := make(map[string]float64)
		for _, r := range latest.Responses {
			perSkill[r.Kind] = (perSkill[r.Kind] + r.Score) / 2.0
		}
		dto.LastResult = &domain.PlacementResult{
			ID:             latest.ID,
			UserID:         latest.UserID,
			EstimatedLevel: latest.AdaptiveState.PlacedLevel,
			PerSkill:       perSkill,
			SessionID:      &latest.ID,
			TakenAt:        *latest.CompletedAt,
		}
	}

	return dto, nil
}

// StartPlacementSession initializes a new adaptive placement test session.
func (s *Service) StartPlacementSession(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementSession, *lessoncontract.ActivityHierarchy, error) {
	now := s.clock.Now().UTC()

	// Check if already in progress and not expired
	active, err := s.repo.GetActivePlacementSessionByUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("check active session: %w", err)
	}
	if active != nil && !active.IsExpired(now) {
		// Return existing active session
		if active.CurrentActivityID != nil {
			actHierarchy, err := s.resolveAndRedactPlacementActivity(ctx, *active.CurrentActivityID)
			if err == nil {
				return active, actHierarchy, nil
			}
		}
		return active, nil, nil
	}

	// Check 30-day cooldown
	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("check latest completed session: %w", err)
	}
	if latest != nil && latest.CompletedAt != nil {
		if now.Before(latest.CompletedAt.Add(domain.PlacementRetakeCooldown)) {
			return nil, nil, domain.ErrPlacementCooldownActive
		}
	}

	// Check pool sufficiency
	sufficient, err := s.HasSufficientPlacementPool(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("check pool sufficiency: %w", err)
	}
	if !sufficient {
		return nil, nil, domain.ErrPlacementInsufficientPool
	}

	// Fetch user's declared level if available
	var declaredLevel string
	if s.user != nil {
		if profile, found, err := s.user.GetLearningProfile(ctx, userID); err == nil && found && profile.DeclaredLevel != nil {
			declaredLevel = *profile.DeclaredLevel
		}
	}

	// Initialize Bayesian adaptive engine state
	adaptiveState := domain.NewAdaptiveState(declaredLevel)
	nextSpec := adaptiveState.NextItem()

	// Draw unseen item from placement pool
	pickedAct, err := s.drawUnseenPlacementItem(ctx, userID, nextSpec.Level, nextSpec.SlotName)
	if err != nil {
		return nil, nil, fmt.Errorf("draw first placement item: %w", err)
	}

	// Record exposure
	_ = s.repo.RecordItemExposure(ctx, userID, pickedAct.ID)

	// Create session
	session := &domain.PlacementSession{
		UserID:            userID,
		Status:            domain.PlacementSessionStatusInProgress,
		Stage:             adaptiveState.Stage,
		ThetaEstimate:     adaptiveState.ThetaEstimate,
		PlacedLevel:       &adaptiveState.PlacedLevel,
		Confidence:        adaptiveState.Confidence,
		CurrentActivityID: &pickedAct.ID,
		CurrentItemKind:   &nextSpec.Kind,
		CurrentItemLevel:  &nextSpec.Level,
		Responses:         []domain.PlacementResponseRecord{},
		AdaptiveState:     *adaptiveState,
		StartedAt:         now,
		ExpiresAt:         now.Add(domain.PlacementSessionDuration),
	}

	stored, err := s.repo.CreatePlacementSession(ctx, session)
	if err != nil {
		return nil, nil, fmt.Errorf("store placement session: %w", err)
	}

	actHierarchy, err := s.resolveAndRedactPlacementActivity(ctx, pickedAct.ID)
	if err != nil {
		return nil, nil, err
	}

	return stored, actHierarchy, nil
}

// GetPlacementSession returns the current state of a placement session and its active item.
func (s *Service) GetPlacementSession(
	ctx context.Context, userID, sessionID uuid.UUID,
) (*domain.PlacementSession, *lessoncontract.ActivityHierarchy, error) {
	session, err := s.repo.GetPlacementSessionByID(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("get placement session: %w", err)
	}
	if session == nil {
		return nil, nil, domain.ErrPlacementSessionNotFound
	}
	if session.UserID != userID {
		return nil, nil, domain.ErrUnauthorizedAttemptAccess
	}

	if session.IsCompleted() || session.CurrentActivityID == nil {
		return session, nil, nil
	}

	actHierarchy, err := s.resolveAndRedactPlacementActivity(ctx, *session.CurrentActivityID)
	if err != nil {
		return session, nil, nil
	}
	return session, actHierarchy, nil
}

// SubmitPlacementAnswer grades the learner's response, updates the Bayesian adaptive state,
// and advances to the next item or finishes the test.
func (s *Service) SubmitPlacementAnswer(
	ctx context.Context, userID, sessionID uuid.UUID, response json.RawMessage,
) (*domain.PlacementSession, *lessoncontract.ActivityHierarchy, bool, error) {
	session, err := s.repo.GetPlacementSessionByID(ctx, sessionID)
	if err != nil {
		return nil, nil, false, fmt.Errorf("get placement session: %w", err)
	}
	if session == nil {
		return nil, nil, false, domain.ErrPlacementSessionNotFound
	}
	if session.UserID != userID {
		return nil, nil, false, domain.ErrUnauthorizedAttemptAccess
	}
	if session.IsCompleted() {
		return nil, nil, false, domain.ErrPlacementSessionCompleted
	}
	now := s.clock.Now().UTC()
	if session.IsExpired(now) {
		return nil, nil, false, domain.ErrPlacementSessionExpired
	}
	if session.CurrentActivityID == nil || session.CurrentItemKind == nil || session.CurrentItemLevel == nil {
		return nil, nil, false, errors.New("placement session has no active item to answer")
	}

	// Grade the response using the registered exercise grader
	kind := *session.CurrentItemKind
	level := *session.CurrentItemLevel
	grader, ok := s.graders.Get(kind)
	if !ok || grader == nil {
		return nil, nil, false, domain.ErrGraderNotRegistered.WithMeta("kind", kind)
	}

	gradeRes, err := grader.Grade(ctx, contract.GradeRequest{
		ActivityID: *session.CurrentActivityID,
		Response:   response,
	})
	if err != nil {
		return nil, nil, false, fmt.Errorf("grade placement response: %w", err)
	}

	var score float64
	if gradeRes.MaxScore > 0 {
		score = float64(gradeRes.Score) / float64(gradeRes.MaxScore)
	} else if gradeRes.Correct {
		score = 1.0
	}

	// Update Bayesian adaptive engine
	if err := session.AdaptiveState.RecordResponse(kind, level, score); err != nil {
		return nil, nil, false, fmt.Errorf("update adaptive state: %w", err)
	}

	// Record response
	record := domain.PlacementResponseRecord{
		ActivityID: *session.CurrentActivityID,
		Kind:       kind,
		Level:      level,
		Score:      score,
		Response:   response,
		AnsweredAt: now,
	}
	session.Responses = append(session.Responses, record)
	session.ThetaEstimate = session.AdaptiveState.ThetaEstimate
	session.PlacedLevel = &session.AdaptiveState.PlacedLevel
	session.Confidence = session.AdaptiveState.Confidence
	session.Stage = session.AdaptiveState.Stage

	// Check next item or completion
	nextSpec := session.AdaptiveState.NextItem()
	if nextSpec.Finished {
		// Complete session
		if err := s.completePlacementSessionInternal(ctx, session); err != nil {
			return nil, nil, false, fmt.Errorf("complete placement session: %w", err)
		}
		return session, nil, true, nil
	}

	// Draw next item
	pickedAct, err := s.drawUnseenPlacementItem(ctx, userID, nextSpec.Level, nextSpec.SlotName)
	if err != nil {
		// Fallback: complete if pool exhausted
		if err := s.completePlacementSessionInternal(ctx, session); err != nil {
			return nil, nil, false, fmt.Errorf("complete placement session on pool exhaustion: %w", err)
		}
		return session, nil, true, nil
	}

	_ = s.repo.RecordItemExposure(ctx, userID, pickedAct.ID)

	session.CurrentActivityID = &pickedAct.ID
	session.CurrentItemKind = &nextSpec.Kind
	session.CurrentItemLevel = &nextSpec.Level

	updated, err := s.repo.UpdatePlacementSessionProgress(ctx, session)
	if err != nil {
		return nil, nil, false, fmt.Errorf("save placement progress: %w", err)
	}

	nextActHierarchy, err := s.resolveAndRedactPlacementActivity(ctx, pickedAct.ID)
	if err != nil {
		return nil, nil, false, err
	}

	return updated, nextActHierarchy, false, nil
}

func (s *Service) completePlacementSessionInternal(
	ctx context.Context, session *domain.PlacementSession,
) error {
	session.Status = domain.PlacementSessionStatusCompleted
	session.Stage = domain.StageCompleted
	now := s.clock.Now().UTC()
	session.CompletedAt = &now

	updated, err := s.repo.CompletePlacementSession(ctx, session)
	if err != nil {
		return fmt.Errorf("complete placement session in db: %w", err)
	}
	*session = *updated

	// Calculate per-skill scores
	perSkillSum := map[string]float64{}
	perSkillCount := map[string]int{}
	for _, r := range session.Responses {
		perSkillSum[r.Kind] += r.Score
		perSkillCount[r.Kind]++
	}
	perSkill := map[string]float64{}
	for k, sum := range perSkillSum {
		if c := perSkillCount[k]; c > 0 {
			perSkill[k] = sum / float64(c)
		}
	}

	// Insert placement result
	res := &domain.PlacementResult{
		UserID:         session.UserID,
		EstimatedLevel: session.AdaptiveState.PlacedLevel,
		PerSkill:       perSkill,
		SessionID:      &session.ID,
		TakenAt:        now,
	}
	if _, err := s.repo.CreatePlacementResult(ctx, res); err != nil {
		return fmt.Errorf("create placement result: %w", err)
	}

	// Generate initial weekly plan
	weekStart := startOfWeekMonday(now)
	plan := buildWeeklyPlan(session.UserID, weekStart, session.AdaptiveState.PlacedLevel, perSkill)
	if _, err := s.repo.UpsertWeeklyPlan(ctx, plan); err != nil {
		// Log but do not abort
	}

	// Emit outbox event
	if s.events != nil {
		_, _ = s.events.Write(ctx, nil, "placement_session", "placement.completed", map[string]any{
			"user_id":      session.UserID,
			"session_id":   session.ID,
			"placed_level": session.AdaptiveState.PlacedLevel,
			"confidence":   session.AdaptiveState.Confidence,
		})
	}

	return nil
}

func (s *Service) drawUnseenPlacementItem(
	ctx context.Context, userID uuid.UUID, level, slotName string,
) (*lessoncontract.Activity, error) {
	unseen, err := s.GetUnseenPlacementItems(ctx, userID, level, slotName)
	if err != nil {
		return nil, err
	}
	if len(unseen) > 0 {
		return &unseen[0], nil
	}

	// Fallback to all activities in slot if all have been exposed
	layout, err := s.placementPool(ctx)
	if err != nil {
		return nil, err
	}
	allActs, err := s.placementSlotActivities(ctx, layout, level, slotName)
	if err != nil {
		return nil, err
	}
	if len(allActs) == 0 {
		return nil, fmt.Errorf("slot %s/%s has no items", level, slotName)
	}
	return &allActs[0], nil
}

func (s *Service) resolveAndRedactPlacementActivity(
	ctx context.Context, activityID uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	act, err := s.resolveActivityHierarchy(ctx, activityID)
	if err != nil {
		return nil, fmt.Errorf("resolve placement activity %s: %w", activityID, err)
	}
	redacted := contentcontract.RedactForLearner(act.Config)
	clone := *act
	clone.Config = redacted
	return &clone, nil
}

// SweepExpiredPlacementSessions expires stale in-progress placement sessions (WO13 §5).
func (s *Service) SweepExpiredPlacementSessions(ctx context.Context) (int64, error) {
	return s.repo.ExpireStalePlacementSessions(ctx)
}

// --------------------------------------------------------------------------
// Starting Path & Weekly Plan (WO13 §3.6)
// --------------------------------------------------------------------------

// GetStartingPath determines the learner's recommended course and starting lesson.
func (s *Service) GetStartingPath(
	ctx context.Context, userID uuid.UUID,
) (*domain.StartingPathDTO, error) {
	placedLevel := domain.BandB1 // Default fallback

	// 1. Check latest placement result
	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err == nil && latest != nil && latest.PlacedLevel != nil {
		placedLevel = *latest.PlacedLevel
	}

	var declaredLevel string
	if s.user != nil {
		if profile, found, err := s.user.GetLearningProfile(ctx, userID); err == nil && found && profile.DeclaredLevel != nil {
			declaredLevel = *profile.DeclaredLevel
			if latest == nil || latest.PlacedLevel == nil {
				placedLevel = declaredLevel
			}
		}
	}

	dto := &domain.StartingPathDTO{
		PlacedLevel:   placedLevel,
		DeclaredLevel: declaredLevel,
	}

	// Find recommended course matching placed level
	// We can find course slug based on placed level, e.g. "general-english-" + strings.ToLower(placedLevel)
	courseSlug := "general-english-" + strings.ToLower(placedLevel)
	dto.RecommendedCourseSlug = courseSlug
	dto.RecommendedCourseTitle = fmt.Sprintf("General English (%s)", placedLevel)

	return dto, nil
}

// GetWeeklyPlan retrieves or generates the personal study plan for the current week.
func (s *Service) GetWeeklyPlan(
	ctx context.Context, userID uuid.UUID,
) (*domain.WeeklyPlan, error) {
	now := s.clock.Now().UTC()
	weekStart := startOfWeekMonday(now)

	existing, err := s.repo.GetWeeklyPlanByUserAndDate(ctx, userID, weekStart)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Generate plan
	placedLevel := domain.BandB1
	perSkill := map[string]float64{
		"vocabulary": 0.8,
		"grammar":    0.7,
		"reading":    0.75,
		"listening":  0.6,
	}

	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err == nil && latest != nil && latest.PlacedLevel != nil {
		placedLevel = *latest.PlacedLevel
		for _, r := range latest.Responses {
			perSkill[r.Kind] = r.Score
		}
	}

	plan := buildWeeklyPlan(userID, weekStart, placedLevel, perSkill)
	stored, err := s.repo.UpsertWeeklyPlan(ctx, plan)
	if err != nil {
		return plan, nil
	}
	return stored, nil
}

func buildWeeklyPlan(
	userID uuid.UUID, weekStart time.Time, level string, perSkill map[string]float64,
) *domain.WeeklyPlan {
	skills := []string{"vocabulary", "grammar", "reading", "listening"}
	// Find weakest skill
	weakest := "listening"
	minScore := 2.0
	for _, sk := range skills {
		score, ok := perSkill[sk]
		if !ok {
			score = 0.5
		}
		if score < minScore {
			minScore = score
			weakest = sk
		}
	}

	// 40/30/20/10 time distribution with weakest skill boost (WO13 §3.6)
	type skillScore struct {
		skill string
		score float64
	}
	sorted := make([]skillScore, 0, len(skills))
	for _, sk := range skills {
		s := perSkill[sk]
		sorted = append(sorted, skillScore{skill: sk, score: s})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score < sorted[j].score
	})

	dist := map[string]float64{}
	weights := []float64{0.40, 0.30, 0.20, 0.10}
	for i, ss := range sorted {
		if i < len(weights) {
			dist[ss.skill] = weights[i]
		}
	}

	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	targets := make([]domain.DailyPlanTarget, 0, 7)
	for i, day := range days {
		skill := sorted[i%len(sorted)].skill
		targets = append(targets, domain.DailyPlanTarget{
			DayOfWeek:       day,
			TargetMinutes:   20,
			PrimarySkill:    skill,
			RecommendedKind: skillToDefaultKind(skill),
		})
	}

	return &domain.WeeklyPlan{
		UserID:           userID,
		WeekStartDate:    weekStart,
		PlacedLevel:      level,
		WeakestSkill:     weakest,
		TimeDistribution: dist,
		DailyTargets:     targets,
	}
}

func skillToDefaultKind(skill string) string {
	switch skill {
	case "vocabulary":
		return "vocabulary"
	case "grammar":
		return "grammar_tense_choice"
	case "reading":
		return "reading_comprehension"
	case "listening":
		return "listening_comprehension"
	case "writing":
		return "writing_prompt"
	case "speaking":
		return "speaking_task"
	default:
		return "grammar_tense_choice"
	}
}

func startOfWeekMonday(t time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	}
	tLocal := t.In(loc)
	weekday := int(tLocal.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysSinceMonday := weekday - 1
	monday := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day()-daysSinceMonday, 0, 0, 0, 0, loc)
	return monday
}
