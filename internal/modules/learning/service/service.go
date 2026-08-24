// Package service implements the core attempt lifecycle, idempotent submission,
// grader dispatch, progress rollup, and outbox event publishing for the learning module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository defines data access methods required by the learning service.
type Repository interface {
	CreateAttempt(ctx context.Context, params repository.CreateAttemptParams) (*domain.Attempt, error)
	GetAttemptByID(ctx context.Context, id uuid.UUID) (*domain.Attempt, error)
	ClaimAttemptForGrading(ctx context.Context, params repository.ClaimAttemptParams) (*domain.Attempt, error)
	UpdateAttemptStatus(
		ctx context.Context, params repository.UpdateAttemptStatusParams,
	) (*domain.Attempt, error)
	GetProgressByUserScope(
		ctx context.Context, userID uuid.UUID, scope string, scopeID uuid.UUID,
	) (*repository.ProgressDTO, error)
	UpsertProgress(ctx context.Context, params repository.UpsertProgressParams) (*repository.ProgressDTO, error)
	ListProgressByUserScopeAndIDs(
		ctx context.Context, userID uuid.UUID, scope string, scopeIDs []uuid.UUID,
	) ([]repository.ProgressDTO, error)
	// WithTx returns this repository bound to tx. It returns the interface, not
	// the concrete struct: returning *repository.Repository dropped every
	// decorator the service had been given the moment the grading transaction
	// opened, so a test repository that fails on purpose inside the rollup
	// could not exist. `lesson`'s repositoryAdapter is the same shape.
	WithTx(tx pgx.Tx) Repository
}

// OutboxTx matches the database transaction interface needed for outbox writes.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter writes domain events to the outbox inside database transactions.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// StartAttemptDTO models the result of starting an attempt.
type StartAttemptDTO struct {
	AttemptID  uuid.UUID `json:"attempt_id"`
	ActivityID uuid.UUID `json:"activity_id"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
}

// SubmitAttemptResultDTO models the output of submitting an attempt.
type SubmitAttemptResultDTO struct {
	AttemptID uuid.UUID `json:"attempt_id"`
	Status    string    `json:"status"`
	Score     *int      `json:"score"`
	MaxScore  *int      `json:"max_score"`
	Correct   *bool     `json:"correct"`
	Feedback  *string   `json:"feedback"`
	Async     bool      `json:"async"`
}

// AttemptDetailDTO models the complete attempt view returned by GET /attempts/{id}.
type AttemptDetailDTO struct {
	ID          uuid.UUID       `json:"id"`
	ActivityID  uuid.UUID       `json:"activity_id"`
	UserID      uuid.UUID       `json:"user_id"`
	Status      string          `json:"status"`
	Response    json.RawMessage `json:"response,omitempty"`
	Score       *int            `json:"score,omitempty"`
	MaxScore    *int            `json:"max_score,omitempty"`
	Feedback    *string         `json:"feedback,omitempty"`
	DurationMs  *int64          `json:"duration_ms,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// Deps holds dependencies for constructing the learning service.
type Deps struct {
	Pool    *pgxpool.Pool
	Repo    Repository
	Lesson  lessoncontract.Reader
	Graders *domain.GraderRegistry
	Events  EventWriter
	Clock   clock.Clock
	NewID   func() (uuid.UUID, error)
}

// Service coordinates attempt execution, grading, progress rollups, and event emission.
type Service struct {
	pool    *pgxpool.Pool
	repo    Repository
	lesson  lessoncontract.Reader
	graders *domain.GraderRegistry
	events  EventWriter
	clock   clock.Clock
	newID   func() (uuid.UUID, error)
}

// New constructs a new Service.
func New(deps Deps) *Service {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	idGen := deps.NewID
	if idGen == nil {
		idGen = func() (uuid.UUID, error) {
			return uuid.NewV7()
		}
	}
	return &Service{
		pool:    deps.Pool,
		repo:    deps.Repo,
		lesson:  deps.Lesson,
		graders: deps.Graders,
		events:  deps.Events,
		clock:   clk,
		newID:   idGen,
	}
}

// StartAttempt begins a new in-progress attempt for an activity.
func (s *Service) StartAttempt(ctx context.Context, userID, activityID uuid.UUID) (*StartAttemptDTO, error) {
	// 1. Resolve activity existence and structural position (Trap 1)
	// Proves the activity exists before an attempt row is written against it.
	if _, err := s.resolveActivityHierarchy(ctx, activityID); err != nil {
		return nil, err
	}

	attemptID, err := s.newID()
	if err != nil {
		return nil, fmt.Errorf("generate attempt id: %w", err)
	}

	now := s.clock.Now().UTC()
	attempt, err := s.repo.CreateAttempt(ctx, repository.CreateAttemptParams{
		ID:         attemptID,
		CreatedAt:  now,
		UserID:     userID,
		ActivityID: activityID,
		// attempts.response is NOT NULL DEFAULT '{}'. The column default does not
		// apply to an explicit NULL, and CreateAttempt names every column.
		Response: json.RawMessage(`{}`),
		MaxScore: 100,
		Status:   domain.StatusInProgress,
	})
	if err != nil {
		return nil, fmt.Errorf("create attempt: %w", err)
	}

	return &StartAttemptDTO{
		AttemptID:  attempt.ID,
		ActivityID: attempt.ActivityID,
		Status:     attempt.Status,
		StartedAt:  attempt.CreatedAt,
	}, nil
}

// SubmitAttempt submits a response for an attempt, executing atomic idempotency claim,
// grader dispatch, synchronous scoring, transactional progress rollup, and outbox event publishing.
func (s *Service) SubmitAttempt(
	ctx context.Context, userID, attemptID, idempotencyKey uuid.UUID, response json.RawMessage,
) (*SubmitAttemptResultDTO, error) {
	attempt, err := s.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.UserID != userID {
		return nil, domain.ErrUnauthorizedAttemptAccess
	}

	if earlyResult, err := s.checkEarlySubmissionState(attempt, idempotencyKey); err != nil || earlyResult != nil {
		return earlyResult, err
	}

	claimed, current, err := s.claimAttempt(ctx, attempt, idempotencyKey, response)
	if err != nil {
		return nil, err
	}
	if !claimed {
		// A concurrent copy of this same submission won the claim. Return what
		// that attempt is, and do not grade it again.
		return s.buildStoredSubmissionResult(current), nil
	}

	activity, err := s.resolveActivityHierarchy(ctx, attempt.ActivityID)
	if err != nil {
		return nil, err
	}

	grader, ok := s.graders.Get(activity.Kind)
	if !ok || grader == nil {
		// The kind goes in the response. A 422 that does not say which kind is
		// unsupported sends the reader back to the database to find out.
		return nil, domain.ErrGraderNotRegistered.WithMeta("kind", activity.Kind)
	}

	gradeResult, err := grader.Grade(ctx, contract.GradeRequest{
		AttemptID:        attempt.ID,
		ActivityID:       attempt.ActivityID,
		ContentVersionID: activity.ContentVersionID,
		UserID:           userID,
		Response:         response,
	})
	if err != nil {
		return nil, fmt.Errorf("grading attempt %s: %w", attempt.ID, err)
	}

	if gradeResult.Async {
		return &SubmitAttemptResultDTO{
			AttemptID: attempt.ID,
			Status:    domain.StatusGrading,
			Async:     true,
		}, nil
	}

	return s.completeSynchronousGrading(ctx, userID, attempt, activity, gradeResult)
}

func (s *Service) checkEarlySubmissionState(
	attempt *domain.Attempt, idempotencyKey uuid.UUID,
) (*SubmitAttemptResultDTO, error) {
	if attempt.IsGraded() || attempt.IsGrading() {
		if attempt.IdempotencyKey != nil && *attempt.IdempotencyKey == idempotencyKey.String() {
			return s.buildStoredSubmissionResult(attempt), nil
		}
		return nil, domain.ErrAlreadyGraded
	}

	return nil, nil
}

// claimAttempt takes exclusive ownership of the grading transition.
//
// It answers three different outcomes and the caller must be able to tell them
// apart, which is why it does not return a bare error:
//
//   - claimed: this caller won and is the one that grades.
//   - lost, same key: a concurrent copy of this caller's own submission won.
//     The answer is that attempt's state, and grading again would score the same
//     learner twice for one attempt.
//   - lost, different key: someone else is submitting this attempt. A conflict.
//
// Returning nil for the second case is what made the loser fall through and
// grade a second time — the database refused the claim exactly as designed, and
// the caller ignored the refusal.
func (s *Service) claimAttempt(
	ctx context.Context, attempt *domain.Attempt, idempotencyKey uuid.UUID, response json.RawMessage,
) (claimed bool, current *domain.Attempt, err error) {
	if _, claimErr := s.repo.ClaimAttemptForGrading(ctx, repository.ClaimAttemptParams{
		ID:             attempt.ID,
		CreatedAt:      attempt.CreatedAt,
		IdempotencyKey: &idempotencyKey,
		Response:       response,
	}); claimErr == nil {
		return true, attempt, nil
	}

	latest, getErr := s.repo.GetAttemptByID(ctx, attempt.ID)
	if getErr != nil {
		return false, nil, getErr
	}
	if latest.IdempotencyKey == nil || *latest.IdempotencyKey != idempotencyKey.String() {
		return false, nil, domain.ErrIdempotencyConflict
	}
	return false, latest, nil
}

// resolveActivityHierarchy turns the one id the caller has into everything the
// activity's position implies (P8.3 Trap 1).
//
// There is no fallback for a missing reader. Returning a fabricated hierarchy
// with Kind "quiz" would pick a grader for an activity nobody resolved, score a
// learner against it, and roll that score up to a course id of uuid.Nil —
// silently, and only in the deployment that forgot to wire the reader.
func (s *Service) resolveActivityHierarchy(
	ctx context.Context, activityID uuid.UUID,
) (*lessoncontract.ActivityHierarchy, error) {
	if s.lesson == nil {
		return nil, fmt.Errorf("resolve activity %s: lesson reader is not configured", activityID)
	}
	return s.lesson.ResolveActivity(ctx, activityID)
}

func safeDurationMs(start, end time.Time) int32 {
	ms := end.Sub(start).Milliseconds()
	if ms < 0 {
		return 0
	}
	if ms > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(ms)
}

func safeScore(score int) int32 {
	if score < 0 {
		return 0
	}
	if score > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(score)
}

func (s *Service) completeSynchronousGrading(
	ctx context.Context,
	userID uuid.UUID,
	attempt *domain.Attempt,
	activity *lessoncontract.ActivityHierarchy,
	gradeResult contract.GradeResult,
) (*SubmitAttemptResultDTO, error) {
	now := s.clock.Now().UTC()
	durationMs := safeDurationMs(attempt.CreatedAt, now)
	scoreInt := safeScore(gradeResult.Score)
	graderName := activity.Kind

	if s.pool != nil {
		txErr := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
			txRepo := s.repo.WithTx(tx)
			return s.executeRollupTx(
				txCtx, tx, txRepo, userID, attempt, activity, gradeResult, scoreInt, durationMs, graderName, now,
			)
		})
		if txErr != nil {
			return nil, fmt.Errorf("commit grading transaction: %w", txErr)
		}
	} else {
		// No pool: the unit suite drives the same path against fakes. The error is
		// returned rather than dropped — a rollup that fails silently here is how a
		// green unit suite hides a broken transaction.
		//
		// The tx handed on is a stand-in rather than nil on purpose. Guarding the
		// outbox writes with `tx != nil` made every one of them unreachable
		// without a database, so "activity.completed is published on every
		// completion" was asserted by no test that runs in `make check`.
		if rollupErr := s.executeRollupTx(
			ctx, noopTx{}, s.repo, userID, attempt, activity, gradeResult, scoreInt, durationMs, graderName, now,
		); rollupErr != nil {
			return nil, fmt.Errorf("roll up attempt %s: %w", attempt.ID, rollupErr)
		}
	}

	score := gradeResult.Score
	maxScore := gradeResult.MaxScore
	correct := gradeResult.Correct
	feedback := gradeResult.Feedback

	return &SubmitAttemptResultDTO{
		AttemptID: attempt.ID,
		Status:    domain.StatusGraded,
		Score:     &score,
		MaxScore:  &maxScore,
		Correct:   &correct,
		Feedback:  &feedback,
		Async:     false,
	}, nil
}

// executeRollupTx performs the atomic update of the attempt, progress scopes, and outbox event emissions.
func (s *Service) executeRollupTx(
	ctx context.Context,
	tx OutboxTx,
	repo Repository,
	userID uuid.UUID,
	attempt *domain.Attempt,
	activity *lessoncontract.ActivityHierarchy,
	gradeResult contract.GradeResult,
	scoreInt, durationMs int32,
	graderName string,
	now time.Time,
) error {
	// 1. Update attempt status to graded
	_, err := repo.UpdateAttemptStatus(ctx, repository.UpdateAttemptStatusParams{
		ID:         attempt.ID,
		CreatedAt:  attempt.CreatedAt,
		Status:     domain.StatusGraded,
		Score:      &scoreInt,
		Grader:     &graderName,
		DurationMs: durationMs,
	})
	if err != nil {
		return fmt.Errorf("update attempt status: %w", err)
	}

	// 2. Rollup Activity Progress
	_, err = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       "activity",
		ScopeID:     activity.ActivityID,
		Status:      domain.ProgressCompleted,
		Score:       &scoreInt,
		CompletedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("upsert activity progress: %w", err)
	}

	// 3. Emit activity.completed outbox event
	if s.events != nil {
		actEvent := contract.ActivityCompleted{
			UserID:     userID,
			ActivityID: activity.ActivityID,
			Score:      gradeResult.Score,
			Skill:      activity.LessonSkillFocus,
			DurationMs: int(durationMs),
			OccurredAt: now,
		}
		if _, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventActivityCompleted, actEvent); err != nil {
			return fmt.Errorf("write activity.completed event: %w", err)
		}
	}

	return s.rollupLessonAndAbove(ctx, tx, repo, userID, activity, now)
}

func calculateCompletedScore(totalCount int, progress []repository.ProgressDTO) (bool, int32) {
	if totalCount == 0 {
		return false, 0
	}
	completed := 0
	var totalScore float64
	for _, p := range progress {
		if p.Status == domain.ProgressCompleted {
			completed++
			if p.Score != nil {
				totalScore += *p.Score
			}
		}
	}
	if completed < totalCount {
		return false, 0
	}
	return true, int32(totalScore / float64(totalCount))
}

func (s *Service) rollupLessonAndAbove(
	ctx context.Context,
	tx OutboxTx,
	repo Repository,
	userID uuid.UUID,
	activity *lessoncontract.ActivityHierarchy,
	now time.Time,
) error {
	if s.lesson == nil || activity.LessonID == uuid.Nil {
		return nil
	}

	lesson, err := s.lesson.GetLesson(ctx, activity.LessonID)
	if err != nil {
		return fmt.Errorf("get lesson %s: %w", activity.LessonID, err)
	}
	if lesson == nil || len(lesson.Activities) == 0 {
		return nil
	}

	actIDs := make([]uuid.UUID, len(lesson.Activities))
	for i, a := range lesson.Activities {
		actIDs[i] = a.ID
	}

	actProgress, err := repo.ListProgressByUserScopeAndIDs(ctx, userID, "activity", actIDs)
	if err != nil {
		return fmt.Errorf("list activity progress: %w", err)
	}

	completed, avgLessonScore := calculateCompletedScore(len(lesson.Activities), actProgress)
	if !completed {
		return nil
	}

	_, err = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       "lesson",
		ScopeID:     activity.LessonID,
		Status:      domain.ProgressCompleted,
		Score:       &avgLessonScore,
		CompletedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("upsert lesson progress: %w", err)
	}

	if s.events != nil {
		lessonEvent := contract.LessonCompleted{
			UserID:     userID,
			LessonID:   activity.LessonID,
			Score:      int(avgLessonScore),
			SkillFocus: lesson.SkillFocus,
			OccurredAt: now,
		}
		if _, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventLessonCompleted, lessonEvent); err != nil {
			return fmt.Errorf("write lesson.completed event: %w", err)
		}
	}

	return s.rollupUnitAndAbove(ctx, tx, repo, userID, activity, now)
}

func (s *Service) rollupUnitAndAbove(
	ctx context.Context,
	tx OutboxTx,
	repo Repository,
	userID uuid.UUID,
	activity *lessoncontract.ActivityHierarchy,
	now time.Time,
) error {
	if activity.UnitID == uuid.Nil {
		return nil
	}

	unitLessons, err := s.lesson.ListLessons(ctx, activity.UnitID)
	if err != nil {
		return fmt.Errorf("list unit lessons %s: %w", activity.UnitID, err)
	}
	if len(unitLessons) == 0 {
		return nil
	}

	unitLessonIDs := make([]uuid.UUID, len(unitLessons))
	for i, l := range unitLessons {
		unitLessonIDs[i] = l.ID
	}

	lessonProgress, err := repo.ListProgressByUserScopeAndIDs(ctx, userID, "lesson", unitLessonIDs)
	if err != nil {
		return fmt.Errorf("list unit lesson progress: %w", err)
	}

	completed, avgUnitScore := calculateCompletedScore(len(unitLessons), lessonProgress)
	if !completed {
		return nil
	}

	_, err = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       "unit",
		ScopeID:     activity.UnitID,
		Status:      domain.ProgressCompleted,
		Score:       &avgUnitScore,
		CompletedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("upsert unit progress: %w", err)
	}

	return s.rollupCourse(ctx, tx, repo, userID, activity, avgUnitScore, now)
}

func (s *Service) rollupCourse(
	ctx context.Context,
	tx OutboxTx,
	repo Repository,
	userID uuid.UUID,
	activity *lessoncontract.ActivityHierarchy,
	avgUnitScore int32,
	now time.Time,
) error {
	if activity.CourseID == uuid.Nil {
		return nil
	}

	nextLesson, err := s.lesson.NextLesson(ctx, activity.CourseID, &activity.LessonID)
	if err != nil {
		return fmt.Errorf("next lesson for course %s: %w", activity.CourseID, err)
	}

	if nextLesson == nil {
		_, err = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
			UserID:      userID,
			Scope:       "course",
			ScopeID:     activity.CourseID,
			Status:      domain.ProgressCompleted,
			Score:       &avgUnitScore,
			CompletedAt: &now,
		})
		if err != nil {
			return fmt.Errorf("upsert course progress: %w", err)
		}

		if s.events != nil {
			courseEvent := contract.CourseCompleted{
				UserID:     userID,
				CourseID:   activity.CourseID,
				OccurredAt: now,
			}
			if _, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventCourseCompleted, courseEvent); err != nil {
				return fmt.Errorf("write course.completed event: %w", err)
			}
		}
	}

	return nil
}

// GetAttempt retrieves the attempt details by ID, enforcing caller ownership.
func (s *Service) GetAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*AttemptDetailDTO, error) {
	attempt, err := s.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.UserID != userID {
		return nil, domain.ErrUnauthorizedAttemptAccess
	}

	var score *int
	var maxScore *int
	if attempt.Score != nil {
		sc := int(*attempt.Score)
		score = &sc
		mx := int(attempt.MaxScore)
		maxScore = &mx
	}

	// feedback stays nil. learn.attempts stores score, max_score, grader and
	// duration and no prose, so a read of a stored attempt has no feedback to
	// return. It used to return the string domain.FakeGrader happens to emit,
	// which read as correct only for as long as the fake was the only grader and
	// told a learner who scored 0 that they were correct.
	var completedAt *time.Time
	if attempt.IsGraded() {
		completedAt = &attempt.UpdatedAt
	}

	return &AttemptDetailDTO{
		ID:          attempt.ID,
		ActivityID:  attempt.ActivityID,
		UserID:      attempt.UserID,
		Status:      attempt.Status,
		Response:    attempt.Response,
		Score:       score,
		MaxScore:    maxScore,
		Feedback:    nil,
		DurationMs:  attempt.DurationMs,
		StartedAt:   attempt.CreatedAt,
		CompletedAt: completedAt,
	}, nil
}

func (s *Service) buildStoredSubmissionResult(attempt *domain.Attempt) *SubmitAttemptResultDTO {
	if attempt.IsGrading() {
		return &SubmitAttemptResultDTO{
			AttemptID: attempt.ID,
			Status:    domain.StatusGrading,
			Async:     true,
		}
	}

	var score *int
	var maxScore *int
	var correct *bool

	if attempt.Score != nil {
		sc := int(*attempt.Score)
		score = &sc
		mx := int(attempt.MaxScore)
		maxScore = &mx
		corr := *attempt.Score >= attempt.MaxScore
		correct = &corr
	}

	return &SubmitAttemptResultDTO{
		AttemptID: attempt.ID,
		Status:    attempt.Status,
		Score:     score,
		MaxScore:  maxScore,
		Correct:   correct,
		// No feedback: see GetAttempt. The winner's response carries the grader's
		// prose; a replay reads the row back, and the row has none.
		Feedback: nil,
		Async:    false,
	}
}

// noopTx stands in for a database transaction when the service runs without a
// pool. It swallows the statement rather than failing, so a fake EventWriter
// sees the same calls the real outbox writer would.
type noopTx struct{}

func (noopTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
