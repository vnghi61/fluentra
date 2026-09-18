package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// The optional writing and speaking part — work order 13 §3.5.
//
// Offered after the result, never before it. The two items are ordinary
// asynchronous attempts; when each is graded its band joins per_skill on the same
// result and its skill mastery is written, and placement.completed is not
// published again.

// maxProductiveGradeRetries bounds the retries of a grade that lost a race with
// another write to the session.
const maxProductiveGradeRetries = 3

// StartPlacementProductive starts the writing and speaking part, or skips it.
// A skipped part can still be started later, from the result.
func (s *Service) StartPlacementProductive(
	ctx context.Context, userID, sessionID uuid.UUID, skip bool,
) (*PlacementSessionDTO, error) {
	session, err := s.ownedPlacementSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	if session, err = s.settleProductiveDeadline(ctx, session, now); err != nil {
		return nil, err
	}
	result, err := s.currentResultOf(ctx, session)
	if err != nil {
		return nil, err
	}
	if skip {
		return s.skipProductive(ctx, session, now)
	}
	return s.beginProductive(ctx, session, result, now)
}

// currentResultOf returns the session's result when it is the learner's
// current one: the last part of an older test is not offered.
func (s *Service) currentResultOf(
	ctx context.Context, session *domain.PlacementSession,
) (*domain.PlacementResult, error) {
	if session.Status != domain.PlacementCompleted || session.ResultID == nil {
		return nil, domain.ErrProductiveUnavailable
	}
	current, err := s.repo.GetCurrentPlacementResult(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if current == nil || current.ID != *session.ResultID {
		return nil, domain.ErrProductiveUnavailable
	}
	return current, nil
}

func (s *Service) skipProductive(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*PlacementSessionDTO, error) {
	switch session.ProductiveStatus {
	case domain.ProductiveSkipped:
		return s.placementSessionDTO(ctx, session, now)
	case domain.ProductiveOffered:
		session.ProductiveStatus = domain.ProductiveSkipped
		stored, err := s.repo.SavePlacementProductive(ctx, session)
		if err != nil {
			return nil, err
		}
		return s.placementSessionDTO(ctx, stored, now)
	default:
		return nil, domain.ErrProductiveUnavailable
	}
}

func (s *Service) beginProductive(
	ctx context.Context, session *domain.PlacementSession, result *domain.PlacementResult, now time.Time,
) (*PlacementSessionDTO, error) {
	switch session.ProductiveStatus {
	case domain.ProductiveInProgress:
		return s.placementSessionDTO(ctx, session, now)
	case domain.ProductiveOffered, domain.ProductiveSkipped:
	default:
		return nil, domain.ErrProductiveUnavailable
	}

	layout, err := s.placementPool(ctx)
	if err != nil {
		return nil, err
	}
	session.Items = withoutUnansweredProductive(session.Items)
	mean := float64(domain.BandIndex(result.Level) - 2)
	var drawn []uuid.UUID
	for _, skill := range []string{domain.SkillWriting, domain.SkillSpeaking} {
		activity, band, drawErr := s.drawPlacementActivity(ctx, layout, session.UserID, skill, result.Level, mean)
		if drawErr != nil {
			return nil, drawErr
		}
		if activity == nil {
			continue
		}
		step := domain.PlacementStep{Skill: skill, Band: band}
		session.Items = append(session.Items, servedItem(activity, step, domain.PlacementPartProductive, now))
		drawn = append(drawn, activity.ID)
	}
	if len(drawn) == 0 {
		return nil, domain.ErrProductiveUnavailable
	}

	deadline := now.Add(domain.ProductiveTimeLimit)
	session.ProductiveDeadlineAt = &deadline
	session.ProductiveStatus = domain.ProductiveInProgress
	err = s.inPlacementTx(ctx, func(txCtx context.Context, _ OutboxTx, repo Repository) error {
		stored, saveErr := repo.SavePlacementProductive(txCtx, session)
		if saveErr != nil {
			return saveErr
		}
		*session = *stored
		for _, id := range drawn {
			if exposeErr := repo.RecordItemExposure(txCtx, session.UserID, id); exposeErr != nil {
				return exposeErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.placementSessionDTO(ctx, session, now)
}

func withoutUnansweredProductive(items []domain.PlacementItem) []domain.PlacementItem {
	kept := make([]domain.PlacementItem, 0, len(items))
	for _, item := range items {
		if item.Part == domain.PlacementPartProductive && !item.Answered() {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

// answerProductive records a writing or speaking answer as an attempt. It counts
// toward the daily writing and recording limits like any other.
func (s *Service) answerProductive(
	ctx context.Context, session *domain.PlacementSession, activityID, key uuid.UUID,
	response json.RawMessage, now time.Time,
) (*PlacementSessionDTO, error) {
	if session.Status != domain.PlacementCompleted {
		return nil, domain.ErrPlacementFinished
	}
	item := productiveItemFor(session, activityID)
	if item == nil {
		return nil, domain.ErrPlacementNotCurrentItem
	}
	if item.Answered() {
		if s.isReplay(ctx, session.UserID, item, activityID, key) {
			return s.placementSessionDTO(ctx, session, now)
		}
		return nil, domain.ErrPlacementFinished
	}
	if session.ProductiveStatus != domain.ProductiveInProgress {
		return nil, domain.ErrPlacementFinished
	}
	if session.ProductiveDeadlineAt != nil && now.After(session.ProductiveDeadlineAt.Add(domain.PlacementAnswerGrace)) {
		if _, err := s.settleProductiveDeadline(ctx, session, now); err != nil {
			return nil, err
		}
		return nil, domain.ErrPlacementExpired
	}

	graded, err := s.SubmitSittingAnswer(ctx, contract.SittingAnswerRequest{
		UserID: session.UserID, ActivityID: activityID, Response: response, IdempotencyKey: key,
	})
	if err != nil {
		return nil, err
	}
	markAnswered(item, graded, now)
	if allProductiveAnswered(session) {
		session.ProductiveStatus = domain.ProductiveSubmitted
	}
	stored, err := s.repo.SavePlacementProductive(ctx, session)
	if err != nil {
		return nil, err
	}
	if !graded.Async {
		score := graded.Score
		s.recordPlacementGrade(ctx, stored.UserID, graded.AttemptID, domain.StatusGraded, &score, graded.MaxScore)
		if reloaded, readErr := s.repo.GetPlacementSession(ctx, stored.ID); readErr == nil && reloaded != nil {
			stored = reloaded
		}
	}
	return s.placementSessionDTO(ctx, stored, now)
}

// settleProductiveDeadline closes a writing and speaking part whose time ran
// out: submitted when anything was answered, skipped — and so startable again —
// when nothing was.
func (s *Service) settleProductiveDeadline(
	ctx context.Context, session *domain.PlacementSession, now time.Time,
) (*domain.PlacementSession, error) {
	if session.ProductiveStatus != domain.ProductiveInProgress || session.ProductiveDeadlineAt == nil {
		return session, nil
	}
	if !now.After(session.ProductiveDeadlineAt.Add(domain.PlacementAnswerGrace)) {
		return session, nil
	}
	session.ProductiveStatus = domain.ProductiveSkipped
	if anyProductiveAnswered(session) {
		session.ProductiveStatus = domain.ProductiveSubmitted
		if allProductiveSettled(session) {
			session.ProductiveStatus = domain.ProductiveGraded
		}
	}
	stored, err := s.repo.SavePlacementProductive(ctx, session)
	if errors.Is(err, domain.ErrPlacementConflict) {
		return s.repo.GetPlacementSession(ctx, session.ID)
	}
	return stored, err
}

// recordPlacementGrade applies a finished grade to the placement that served the
// attempt, if one did. It is called for every attempt on the placement pool and
// is a no-op for the adaptive part's items.
func (s *Service) recordPlacementGrade(
	ctx context.Context, userID, attemptID uuid.UUID, status string, score *int, maxScore int,
) {
	for i := 0; i < maxProductiveGradeRetries; i++ {
		session, err := s.repo.FindPlacementSessionByAttempt(ctx, userID, attemptID)
		if err != nil {
			slog.ErrorContext(ctx, "could not find the placement for a graded attempt",
				"attempt_id", attemptID, "error", err)
			return
		}
		if session == nil {
			return
		}
		err = s.applyProductiveGrade(ctx, session, attemptID, status, score, maxScore)
		if !errors.Is(err, domain.ErrPlacementConflict) {
			if err != nil {
				slog.ErrorContext(ctx, "could not record a placement grade", "attempt_id", attemptID, "error", err)
			}
			return
		}
	}
}

func (s *Service) applyProductiveGrade(
	ctx context.Context, session *domain.PlacementSession, attemptID uuid.UUID,
	status string, score *int, maxScore int,
) error {
	item := itemByAttempt(session, attemptID)
	if item == nil || item.Part != domain.PlacementPartProductive {
		return nil
	}
	item.Status = status
	item.Score = score
	item.MaxScore = maxScore
	if session.ProductiveStatus == domain.ProductiveSubmitted && allProductiveSettled(session) {
		session.ProductiveStatus = domain.ProductiveGraded
	}

	return s.inPlacementTx(ctx, func(txCtx context.Context, _ OutboxTx, repo Repository) error {
		if status == domain.StatusGraded && score != nil && session.ResultID != nil {
			if err := addProductiveSkill(txCtx, repo, session, item.Skill, *score, maxScore); err != nil {
				return err
			}
		}
		_, err := repo.SavePlacementProductive(txCtx, session)
		return err
	})
}

// addProductiveSkill puts a graded writing or speaking band on the result and
// into skill mastery. Nothing is published: placement.completed went out with
// the adaptive part.
func addProductiveSkill(
	ctx context.Context, repo Repository, session *domain.PlacementSession, skill string, score, maxScore int,
) error {
	result, err := repo.GetPlacementResult(ctx, *session.ResultID)
	if err != nil || result == nil {
		return err
	}
	band := domain.ProductiveBand(score, maxScore)
	if result.PerSkill == nil {
		result.PerSkill = map[string]domain.SkillEstimate{}
	}
	result.PerSkill[skill] = domain.SkillEstimate{Band: band, Responses: 1}
	if _, err := repo.UpdatePlacementResultPerSkill(ctx, result.ID, result.PerSkill); err != nil {
		return err
	}
	return raisePlacementMastery(ctx, repo, session.UserID, skill, band)
}

// isPoolItem reports whether an activity belongs to the exam or the placement
// pool. Neither may be started, submitted or preview graded on its own: an item
// answered outside its sitting or session would reveal its answer.
func isPoolItem(activity *lessoncontract.ActivityHierarchy) bool {
	return activity.CourseSlug == ExamPoolCourseSlug || activity.CourseSlug == PlacementPoolCourseSlug
}

// isPlacementActivity reports whether an activity belongs to the placement pool.
func isPlacementActivity(activity *lessoncontract.ActivityHierarchy) bool {
	return activity != nil && activity.CourseSlug == PlacementPoolCourseSlug
}

func productiveItemFor(session *domain.PlacementSession, activityID uuid.UUID) *domain.PlacementItem {
	for _, item := range session.ProductiveItems() {
		if item.ActivityID == activityID {
			return item
		}
	}
	return nil
}

func itemByAttempt(session *domain.PlacementSession, attemptID uuid.UUID) *domain.PlacementItem {
	for i := range session.Items {
		if id := session.Items[i].AttemptID; id != nil && *id == attemptID {
			return &session.Items[i]
		}
	}
	return nil
}

func allProductiveAnswered(session *domain.PlacementSession) bool {
	items := session.ProductiveItems()
	for _, item := range items {
		if !item.Answered() {
			return false
		}
	}
	return len(items) > 0
}

func anyProductiveAnswered(session *domain.PlacementSession) bool {
	for _, item := range session.ProductiveItems() {
		if item.Answered() {
			return true
		}
	}
	return false
}

// allProductiveSettled reports whether every answered item has finished grading.
func allProductiveSettled(session *domain.PlacementSession) bool {
	for _, item := range session.ProductiveItems() {
		if item.Answered() && item.Status != domain.StatusGraded && item.Status != productiveFailed {
			return false
		}
	}
	return true
}
