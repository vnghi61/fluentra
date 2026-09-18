package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// The placement test on the server — work order 13 §3.5.

// PlacementInviteFlag switches the invitation on once the pool can serve a full
// test at every band.
const PlacementInviteFlag = "placement.invite"

// maxExhaustionRetries bounds the search for a skill that still has items.
const maxExhaustionRetries = 8

// PlacementResultDTO is a placement result as the learner sees it.
type PlacementResultDTO struct {
	ID        uuid.UUID                       `json:"id"`
	SessionID *uuid.UUID                      `json:"session_id,omitempty"`
	Level     string                          `json:"level"`
	PerSkill  map[string]domain.SkillEstimate `json:"per_skill"`
	TakenAt   time.Time                       `json:"taken_at"`
}

// PlacementItemDTO is an item to answer, redacted.
type PlacementItemDTO struct {
	ActivityID       uuid.UUID       `json:"activity_id"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Kind             string          `json:"kind"`
	Skill            string          `json:"skill"`
	Config           json.RawMessage `json:"config"`
}

// PlacementProductiveItemDTO is a writing or speaking item and where its grading stands.
type PlacementProductiveItemDTO struct {
	PlacementItemDTO
	// Status is not_answered, grading, graded or failed.
	Status string  `json:"status"`
	Band   *string `json:"band,omitempty"`
}

// PlacementSessionDTO is a session as the test screen needs it.
type PlacementSessionDTO struct {
	ID                         uuid.UUID                    `json:"id"`
	Status                     string                       `json:"status"`
	Stage                      string                       `json:"stage"`
	StartedAt                  time.Time                    `json:"started_at"`
	DeadlineAt                 time.Time                    `json:"deadline_at"`
	RemainingSeconds           int                          `json:"remaining_seconds"`
	Responses                  int                          `json:"responses"`
	MaxResponses               int                          `json:"max_responses"`
	CurrentItem                *PlacementItemDTO            `json:"current_item"`
	ProductiveStatus           string                       `json:"productive_status"`
	ProductiveDeadlineAt       *time.Time                   `json:"productive_deadline_at"`
	ProductiveRemainingSeconds int                          `json:"productive_remaining_seconds"`
	ProductiveItems            []PlacementProductiveItemDTO `json:"productive_items"`
	Result                     *PlacementResultDTO          `json:"result"`
}

// PlacementActiveSessionDTO summarises a session in progress.
type PlacementActiveSessionDTO struct {
	ID               uuid.UUID `json:"id"`
	Stage            string    `json:"stage"`
	DeadlineAt       time.Time `json:"deadline_at"`
	RemainingSeconds int       `json:"remaining_seconds"`
}

// PlacementOverviewDTO is GET /me/placement.
type PlacementOverviewDTO struct {
	Result            *PlacementResultDTO        `json:"result"`
	ActiveSession     *PlacementActiveSessionDTO `json:"active_session"`
	RetakeAvailableAt *time.Time                 `json:"retake_available_at"`
	InviteAvailable   bool                       `json:"invite_available"`
}

// Productive item grading states.
const (
	productiveNotAnswered = "not_answered"
	productiveGrading     = "grading"
	productiveFailed      = "failed"
)

// --------------------------------------------------------------------------
// Reads
// --------------------------------------------------------------------------

// GetPlacementOverview returns the current result, the session in progress, when
// a retake becomes available, and whether to invite the learner. A session past
// its deadline is finished here, so no read ever reports one as active.
func (s *Service) GetPlacementOverview(ctx context.Context, userID uuid.UUID) (*PlacementOverviewDTO, error) {
	now := s.clock.Now().UTC()
	open, err := s.openPlacementSession(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read current placement result: %w", err)
	}
	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read latest placement session: %w", err)
	}

	overview := &PlacementOverviewDTO{Result: toPlacementResultDTO(result)}
	if open != nil {
		overview.ActiveSession = &PlacementActiveSessionDTO{
			ID: open.ID, Stage: open.Stage, DeadlineAt: open.DeadlineAt, RemainingSeconds: open.RemainingSeconds(now),
		}
	}
	if latest != nil && latest.CompletedAt != nil {
		at := latest.CompletedAt.Add(domain.PlacementRetakeAfter)
		overview.RetakeAvailableAt = &at
	}
	overview.InviteAvailable = result == nil && open == nil && s.placementInviteOn(ctx, userID)
	return overview, nil
}

// GetPlacementSession returns one of the caller's sessions.
func (s *Service) GetPlacementSession(
	ctx context.Context, userID, sessionID uuid.UUID,
) (*PlacementSessionDTO, error) {
	session, err := s.ownedPlacementSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	if session.Status == domain.PlacementInProgress && session.Overdue(now) {
		if session, err = s.finishPlacementTolerant(ctx, session, now); err != nil {
			return nil, err
		}
	}
	if session, err = s.settleProductiveDeadline(ctx, session, now); err != nil {
		return nil, err
	}
	return s.placementSessionDTO(ctx, session, now)
}

// openPlacementSession returns the learner's session in progress, finishing it
// first when its time is up.
func (s *Service) openPlacementSession(
	ctx context.Context, userID uuid.UUID, now time.Time,
) (*domain.PlacementSession, error) {
	open, err := s.repo.GetOpenPlacementSession(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read open placement session: %w", err)
	}
	if open == nil || !open.Overdue(now) {
		return open, nil
	}
	if _, err := s.finishPlacementTolerant(ctx, open, now); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Service) ownedPlacementSession(
	ctx context.Context, userID, sessionID uuid.UUID,
) (*domain.PlacementSession, error) {
	session, err := s.repo.GetPlacementSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read placement session: %w", err)
	}
	// Another learner's session is not found, not forbidden: its existence is
	// not the caller's to learn.
	if session == nil || session.UserID != userID {
		return nil, domain.ErrPlacementNotFound
	}
	return session, nil
}

func (s *Service) placementInviteOn(ctx context.Context, userID uuid.UUID) bool {
	if s.flags == nil {
		return false
	}
	enabled, err := s.flags.IsEnabled(ctx, PlacementInviteFlag, userID)
	if err != nil {
		slog.WarnContext(ctx, "could not read the placement invitation flag", "error", err)
		return false
	}
	return enabled
}

// --------------------------------------------------------------------------
// Starting
// --------------------------------------------------------------------------

// StartPlacement starts a session and serves its first item.
func (s *Service) StartPlacement(ctx context.Context, userID uuid.UUID) (*PlacementSessionDTO, error) {
	now := s.clock.Now().UTC()
	if err := s.checkCanStartPlacement(ctx, userID, now); err != nil {
		return nil, err
	}
	layout, err := s.placementPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("load placement pool: %w", err)
	}
	ready, err := s.placementPoolReady(ctx, layout)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, domain.ErrPlacementUnavailable
	}

	id, err := s.newID()
	if err != nil {
		return nil, fmt.Errorf("generate placement session id: %w", err)
	}
	session := &domain.PlacementSession{
		ID:               id,
		UserID:           userID,
		Status:           domain.PlacementInProgress,
		StartedAt:        now,
		DeadlineAt:       now.Add(domain.PlacementTimeLimit),
		Estimate:         domain.NewPlacementEstimate(),
		Items:            []domain.PlacementItem{},
		ProductiveStatus: domain.ProductiveOffered,
	}
	step, activity, err := s.nextPlacementItem(ctx, layout, session, now)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, domain.ErrPlacementUnavailable
	}
	session.Stage = step.Stage
	session.Items = append(session.Items, servedItem(activity, step, domain.PlacementPartAdaptive, now))

	err = s.inPlacementTx(ctx, func(txCtx context.Context, _ OutboxTx, repo Repository) error {
		stored, createErr := repo.CreatePlacementSession(txCtx, session)
		if createErr != nil {
			return createErr
		}
		*session = *stored
		return repo.RecordItemExposure(txCtx, userID, activity.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.placementSessionDTO(ctx, session, now)
}

// checkCanStartPlacement refuses a second session, a retake inside 30 days, and
// finishes a session whose time ran out so it no longer blocks a new one.
func (s *Service) checkCanStartPlacement(ctx context.Context, userID uuid.UUID, now time.Time) error {
	open, err := s.repo.GetOpenPlacementSession(ctx, userID)
	if err != nil {
		return fmt.Errorf("read open placement session: %w", err)
	}
	if open != nil {
		if !open.Overdue(now) {
			return domain.ErrPlacementInProgress.WithMeta("session_id", open.ID.String())
		}
		if _, err := s.finishPlacementTolerant(ctx, open, now); err != nil {
			return err
		}
	}
	latest, err := s.repo.GetLatestCompletedPlacementSession(ctx, userID)
	if err != nil {
		return fmt.Errorf("read latest placement session: %w", err)
	}
	if latest != nil && latest.CompletedAt != nil {
		if at := latest.CompletedAt.Add(domain.PlacementRetakeAfter); now.Before(at) {
			return domain.ErrPlacementRetakeTooSoon.WithMeta("retake_available_at", at.Format(time.RFC3339))
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// Answering
// --------------------------------------------------------------------------

// SubmitPlacementAnswer answers the current item. Every answer is a
// learn.attempts row graded by the kind's grader; the same Idempotency-Key
// grades once and returns the session as it stands.
func (s *Service) SubmitPlacementAnswer(
	ctx context.Context, userID, sessionID, activityID, idempotencyKey uuid.UUID, response json.RawMessage,
) (*PlacementSessionDTO, error) {
	session, err := s.ownedPlacementSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	if session.Status == domain.PlacementInProgress {
		return s.answerAdaptive(ctx, session, activityID, idempotencyKey, response, now)
	}
	return s.answerProductive(ctx, session, activityID, idempotencyKey, response, now)
}

func (s *Service) answerAdaptive(
	ctx context.Context, session *domain.PlacementSession, activityID, key uuid.UUID,
	response json.RawMessage, now time.Time,
) (*PlacementSessionDTO, error) {
	if session.Overdue(now) {
		if _, err := s.finishPlacementTolerant(ctx, session, now); err != nil {
			return nil, err
		}
		return nil, domain.ErrPlacementExpired
	}
	current := session.CurrentItem()
	if current == nil || current.ActivityID != activityID {
		if s.isReplay(ctx, session.UserID, session.LastAnswered(), activityID, key) {
			return s.placementSessionDTO(ctx, session, now)
		}
		return nil, domain.ErrPlacementNotCurrentItem
	}

	activity, err := s.resolveActivityHierarchy(ctx, activityID)
	if err != nil {
		return nil, err
	}
	graded, err := s.SubmitSittingAnswer(ctx, contract.SittingAnswerRequest{
		UserID: session.UserID, ActivityID: activityID, Response: response, IdempotencyKey: key,
	})
	if err != nil {
		return nil, err
	}
	observations := placementObservations(*current, activity.Config, graded)
	for _, obs := range observations {
		session.Estimate.Observe(obs)
	}
	markAnswered(current, graded, now)
	logPlacementResponse(ctx, *current, observations)

	return s.advancePlacement(ctx, session, now)
}

// advancePlacement serves the next item, or finishes the adaptive part.
func (s *Service) advancePlacement(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*PlacementSessionDTO, error) {
	layout, err := s.placementPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("load placement pool: %w", err)
	}
	step, activity, err := s.nextPlacementItem(ctx, layout, session, now)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		finished, finishErr := s.finishPlacement(ctx, session, now)
		if finishErr != nil {
			return nil, finishErr
		}
		return s.placementSessionDTO(ctx, finished, now)
	}

	session.Stage = step.Stage
	session.Items = append(session.Items, servedItem(activity, step, domain.PlacementPartAdaptive, now))
	err = s.inPlacementTx(ctx, func(txCtx context.Context, _ OutboxTx, repo Repository) error {
		stored, saveErr := repo.SavePlacementProgress(txCtx, session)
		if saveErr != nil {
			return saveErr
		}
		*session = *stored
		return repo.RecordItemExposure(txCtx, session.UserID, activity.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.placementSessionDTO(ctx, session, now)
}

// isReplay reports whether an answer is the one already recorded for an item,
// sent again with the same key.
func (s *Service) isReplay(
	ctx context.Context, userID uuid.UUID, item *domain.PlacementItem, activityID, key uuid.UUID,
) bool {
	if item == nil || item.ActivityID != activityID || item.AttemptID == nil {
		return false
	}
	attempt, err := s.repo.GetAttemptByUserActivityIdempotencyKey(ctx, userID, activityID, key)
	return err == nil && attempt != nil && attempt.ID == *item.AttemptID
}

func markAnswered(item *domain.PlacementItem, graded *contract.SittingAnswerResult, now time.Time) {
	attemptID := graded.AttemptID
	item.AttemptID = &attemptID
	item.AnsweredAt = &now
	item.Status = graded.Status
	if !graded.Async {
		score := graded.Score
		item.Score = &score
		item.MaxScore = graded.MaxScore
	}
}

// placementObservations turns a graded answer into observations: one per
// question of a passage or clip, one for a single question. The number of
// options sets the chance of guessing.
func placementObservations(
	item domain.PlacementItem, config json.RawMessage, graded *contract.SittingAnswerResult,
) []domain.Observation {
	var body struct {
		Options   []candOption   `json:"options"`
		Questions []candQuestion `json:"questions"`
	}
	_ = json.Unmarshal(config, &body)

	if len(graded.ItemResults) == 0 {
		return []domain.Observation{{
			Skill: item.Skill, Band: item.Band, Correct: graded.Correct, Options: len(body.Options),
		}}
	}
	options := make(map[string]int, len(body.Questions))
	for _, q := range body.Questions {
		options[q.ID] = len(q.Options)
	}
	observations := make([]domain.Observation, 0, len(graded.ItemResults))
	for _, result := range graded.ItemResults {
		observations = append(observations, domain.Observation{
			Skill: item.Skill, Band: item.Band, Correct: result.Correct, Options: options[result.ID],
		})
	}
	return observations
}

// logPlacementResponse records how an item at its band was answered. A later
// work order recalibrates difficulty from these; this one does not act on them.
func logPlacementResponse(ctx context.Context, item domain.PlacementItem, observations []domain.Observation) {
	correct := 0
	for _, obs := range observations {
		if obs.Correct {
			correct++
		}
	}
	slog.InfoContext(ctx, "placement item answered",
		"activity_id", item.ActivityID, "skill", item.Skill, "band", item.Band,
		"questions", len(observations), "correct", correct)
}

// --------------------------------------------------------------------------
// Drawing items
// --------------------------------------------------------------------------

// nextPlacementItem asks the engine for the next step and draws an unseen item
// for it. A skill with nothing left at any band is marked exhausted and the
// engine is asked again, so an empty slot never stalls a test.
func (s *Service) nextPlacementItem(
	ctx context.Context, layout *placementPoolLayout, session *domain.PlacementSession, now time.Time,
) (domain.PlacementStep, *lessoncontract.Activity, error) {
	for i := 0; i < maxExhaustionRetries; i++ {
		step := domain.NextPlacementStep(session.Estimate, session.Progress(), !now.Before(session.DeadlineAt))
		if step.Done {
			return step, nil, nil
		}
		activity, band, err := s.drawPlacementActivity(
			ctx, layout, session.UserID, step.Skill, step.Band, session.Estimate.Mean(),
		)
		if err != nil {
			return step, nil, err
		}
		if activity == nil {
			session.Estimate.MarkExhausted(step.Skill)
			continue
		}
		step.Band = band
		return step, activity, nil
	}
	return domain.PlacementStep{Done: true, Stage: domain.PlacementStageDone}, nil, nil
}

// drawPlacementActivity picks at random among the learner's unseen items for a
// skill, at the band asked for or the nearest band that has one.
func (s *Service) drawPlacementActivity(
	ctx context.Context, layout *placementPoolLayout, userID uuid.UUID, skill, band string, mean float64,
) (*lessoncontract.Activity, string, error) {
	slot, ok := placementSlotForSkill(skill)
	if !ok {
		return nil, "", fmt.Errorf("no placement slot for skill %q", skill)
	}
	for _, candidate := range domain.BandSearchOrder(band, mean) {
		activities, err := s.placementSlotActivities(ctx, layout, candidate, slot.slotName)
		if err != nil {
			return nil, "", err
		}
		if slot.kind == kindListeningComprehension {
			activities = s.listeningWithAudio(ctx, activities)
		}
		unseen, err := s.unseenActivities(ctx, userID, activities)
		if err != nil {
			return nil, "", err
		}
		if len(unseen) == 0 {
			continue
		}
		if candidate != band {
			slog.InfoContext(ctx, "placement served a neighbouring band",
				"skill", skill, "asked", band, "served", candidate)
		}
		picked := unseen[0]
		if shuffled := shuffledIDs(idsOf(unseen)); len(shuffled) > 0 {
			for _, activity := range unseen {
				if activity.ID == shuffled[0] {
					picked = activity
				}
			}
		}
		return &picked, candidate, nil
	}
	return nil, "", nil
}

func (s *Service) unseenActivities(
	ctx context.Context, userID uuid.UUID, activities []lessoncontract.Activity,
) ([]lessoncontract.Activity, error) {
	if len(activities) == 0 {
		return nil, nil
	}
	exposures, err := s.repo.ListItemExposures(ctx, userID, idsOf(activities))
	if err != nil {
		return nil, fmt.Errorf("list item exposures: %w", err)
	}
	unseen := make([]lessoncontract.Activity, 0, len(activities))
	for _, activity := range activities {
		if _, served := exposures[activity.ID]; !served {
			unseen = append(unseen, activity)
		}
	}
	return unseen, nil
}

func servedItem(
	activity *lessoncontract.Activity, step domain.PlacementStep, part string, now time.Time,
) domain.PlacementItem {
	return domain.PlacementItem{
		ActivityID: activity.ID,
		Kind:       activity.Kind,
		Skill:      step.Skill,
		Band:       step.Band,
		Part:       part,
		ServedAt:   now,
	}
}

// --------------------------------------------------------------------------
// Finishing
// --------------------------------------------------------------------------

// finishPlacement ends the adaptive part with what it has. With at least eight
// responses it records the result, the skill mastery and placement.completed in
// one transaction. With fewer it records nothing, and the learner may start again
// at once.
func (s *Service) finishPlacement(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*domain.PlacementSession, error) {
	session.Stage = domain.PlacementStageDone
	session.CompletedAt = &now
	if session.Estimate.Responses() < domain.PlacementMinResponses {
		session.Status = domain.PlacementExpired
		stored, err := s.repo.FinishPlacementSession(ctx, session)
		if err != nil {
			return nil, err
		}
		return stored, nil
	}

	session.Status = domain.PlacementCompleted
	level := session.Estimate.Level()
	perSkill := session.Estimate.PerSkill()
	err := s.inPlacementTx(ctx, func(txCtx context.Context, tx OutboxTx, repo Repository) error {
		sessionID := session.ID
		result, err := repo.CreatePlacementResult(txCtx, &domain.PlacementResult{
			UserID: session.UserID, SessionID: &sessionID, Level: level, PerSkill: perSkill, TakenAt: now,
		})
		if err != nil {
			return fmt.Errorf("store placement result: %w", err)
		}
		session.ResultID = &result.ID
		for _, skill := range sortedSkills(perSkill) {
			if err := raisePlacementMastery(txCtx, repo, session.UserID, skill, perSkill[skill].Band); err != nil {
				return err
			}
		}
		stored, err := repo.FinishPlacementSession(txCtx, session)
		if err != nil {
			return err
		}
		*session = *stored
		return s.publishPlacementCompleted(txCtx, tx, session.UserID, level, perSkill, now)
	})
	if err != nil {
		return nil, err
	}
	s.invalidateLearningCaches(ctx, session.UserID)
	return session, nil
}

// finishPlacementTolerant finishes a session another request may be finishing
// at the same moment, and returns the session as it stands afterwards.
func (s *Service) finishPlacementTolerant(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*domain.PlacementSession, error) {
	finished, err := s.finishPlacement(ctx, session, now)
	if err == nil {
		return finished, nil
	}
	if !errors.Is(err, domain.ErrPlacementConflict) {
		return nil, err
	}
	reloaded, readErr := s.repo.GetPlacementSession(ctx, session.ID)
	if readErr != nil || reloaded == nil {
		return nil, err
	}
	return reloaded, nil
}

func (s *Service) publishPlacementCompleted(
	ctx context.Context, tx OutboxTx, userID uuid.UUID, level string,
	perSkill map[string]domain.SkillEstimate, now time.Time,
) error {
	if s.events == nil {
		return nil
	}
	encoded, err := json.Marshal(perSkill)
	if err != nil {
		return fmt.Errorf("encode placement per_skill: %w", err)
	}
	_, err = s.events.Write(ctx, tx, contract.Aggregate, contract.EventPlacementCompleted, contract.PlacementCompleted{
		UserID: userID, Level: level, PerSkill: encoded, OccurredAt: now,
	})
	if err != nil {
		return fmt.Errorf("write placement.completed: %w", err)
	}
	return nil
}

// raisePlacementMastery writes a placed band into learn.skill_mastery at 0.40
// confidence, and only where the learner's existing confidence is lower.
func raisePlacementMastery(ctx context.Context, repo Repository, userID uuid.UUID, skill, band string) error {
	existing, err := repo.GetSkillMastery(ctx, userID, skill)
	if err != nil {
		return fmt.Errorf("read skill mastery %s: %w", skill, err)
	}
	if existing != nil && existing.Confidence >= domain.PlacementMasteryConfidence {
		return nil
	}
	if _, err := repo.UpsertSkillMastery(ctx, userID, skill, band, domain.PlacementMasteryConfidence); err != nil {
		return fmt.Errorf("write skill mastery %s: %w", skill, err)
	}
	return nil
}

func sortedSkills(perSkill map[string]domain.SkillEstimate) []string {
	skills := make([]string, 0, len(perSkill))
	for skill := range perSkill {
		skills = append(skills, skill)
	}
	sort.Strings(skills)
	return skills
}

// SweepExpiredPlacementSessions finishes the sessions whose time ran out.
func (s *Service) SweepExpiredPlacementSessions(ctx context.Context) (int, error) {
	now := s.clock.Now().UTC()
	ids, err := s.repo.ListOverduePlacementSessions(ctx, now.Add(-domain.PlacementAnswerGrace))
	if err != nil {
		return 0, fmt.Errorf("list overdue placement sessions: %w", err)
	}
	finished := 0
	for _, id := range ids {
		session, err := s.repo.GetPlacementSession(ctx, id)
		if err != nil || session == nil || session.Status != domain.PlacementInProgress {
			continue
		}
		if _, err := s.finishPlacement(ctx, session, now); err != nil {
			if !errors.Is(err, domain.ErrPlacementConflict) {
				slog.ErrorContext(ctx, "could not finish an overdue placement session", "session_id", id, "error", err)
			}
			continue
		}
		finished++
	}
	return finished, nil
}

// inPlacementTx runs fn in one transaction, or directly on the repository when
// the service has no pool, as in unit tests.
func (s *Service) inPlacementTx(
	ctx context.Context, fn func(ctx context.Context, tx OutboxTx, repo Repository) error,
) error {
	if s.pool == nil {
		return fn(ctx, noopTx{}, s.repo)
	}
	return dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		return fn(txCtx, tx, s.repo.WithTx(tx))
	})
}

// --------------------------------------------------------------------------
// Views
// --------------------------------------------------------------------------

func (s *Service) placementSessionDTO(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*PlacementSessionDTO, error) {
	dto := &PlacementSessionDTO{
		ID:                   session.ID,
		Status:               session.Status,
		Stage:                session.Stage,
		StartedAt:            session.StartedAt,
		DeadlineAt:           session.DeadlineAt,
		Responses:            session.Estimate.Responses(),
		MaxResponses:         domain.PlacementMaxResponses,
		ProductiveStatus:     session.ProductiveStatus,
		ProductiveDeadlineAt: session.ProductiveDeadlineAt,
		ProductiveItems:      []PlacementProductiveItemDTO{},
	}
	if session.Status == domain.PlacementInProgress {
		dto.RemainingSeconds = session.RemainingSeconds(now)
		if current := session.CurrentItem(); current != nil {
			item, err := s.placementItemDTO(ctx, *current)
			if err != nil {
				return nil, err
			}
			dto.CurrentItem = item
		}
	}
	if session.ProductiveDeadlineAt != nil && session.ProductiveStatus == domain.ProductiveInProgress {
		dto.ProductiveRemainingSeconds = max(0, int(session.ProductiveDeadlineAt.Sub(now).Seconds()))
	}
	for _, item := range session.ProductiveItems() {
		productive, err := s.productiveItemDTO(ctx, *item)
		if err != nil {
			return nil, err
		}
		dto.ProductiveItems = append(dto.ProductiveItems, productive)
	}
	if session.ResultID != nil {
		result, err := s.repo.GetPlacementResult(ctx, *session.ResultID)
		if err != nil {
			return nil, fmt.Errorf("read placement result: %w", err)
		}
		dto.Result = toPlacementResultDTO(result)
	}
	return dto, nil
}

// placementItemDTO resolves an item's body and removes every answer from it
// (ADR-0025): nothing answer-bearing reaches the learner before they answer.
func (s *Service) placementItemDTO(ctx context.Context, item domain.PlacementItem) (*PlacementItemDTO, error) {
	activity, err := s.resolveActivityHierarchy(ctx, item.ActivityID)
	if err != nil {
		return nil, err
	}
	return &PlacementItemDTO{
		ActivityID:       item.ActivityID,
		ContentVersionID: activity.ContentVersionID,
		Kind:             activity.Kind,
		Skill:            item.Skill,
		Config:           contentcontract.RedactForLearner(activity.Config),
	}, nil
}

func (s *Service) productiveItemDTO(
	ctx context.Context, item domain.PlacementItem,
) (PlacementProductiveItemDTO, error) {
	base, err := s.placementItemDTO(ctx, item)
	if err != nil {
		return PlacementProductiveItemDTO{}, err
	}
	dto := PlacementProductiveItemDTO{PlacementItemDTO: *base, Status: productiveNotAnswered}
	switch {
	case item.Status == domain.StatusGraded && item.Score != nil:
		dto.Status = domain.StatusGraded
		band := domain.ProductiveBand(*item.Score, item.MaxScore)
		dto.Band = &band
	case item.Status == productiveFailed:
		dto.Status = productiveFailed
	case item.Answered():
		dto.Status = productiveGrading
	}
	return dto, nil
}

func toPlacementResultDTO(result *domain.PlacementResult) *PlacementResultDTO {
	if result == nil {
		return nil
	}
	return &PlacementResultDTO{
		ID:        result.ID,
		SessionID: result.SessionID,
		Level:     result.Level,
		PerSkill:  result.PerSkill,
		TakenAt:   result.TakenAt,
	}
}
