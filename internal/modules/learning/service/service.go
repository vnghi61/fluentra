// Package service implements the core attempt lifecycle, idempotent submission,
// grader dispatch, progress rollup, and outbox event publishing for the learning module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	srscontract "github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

const (
	cacheVersion = 1
	dashboardTTL = 2 * time.Minute
	progressTTL  = 5 * time.Minute
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
	ListProgressByUserAndScope(
		ctx context.Context, userID uuid.UUID, scope string, limit int32,
	) ([]repository.ProgressDTO, error)
	GetEnrollmentByUserCourse(
		ctx context.Context, userID, courseID uuid.UUID,
	) (*domain.Enrollment, error)
	ListEnrollmentsByUser(
		ctx context.Context, userID uuid.UUID, limit int32,
	) ([]domain.Enrollment, error)
	CreateEnrollment(
		ctx context.Context, userID, courseID uuid.UUID, status string, startedAt time.Time,
	) (*domain.Enrollment, error)
	UpdateEnrollmentStatus(
		ctx context.Context, userID, courseID uuid.UUID, status string, completedAt *time.Time,
	) (*domain.Enrollment, error)
	CreateLearningSession(
		ctx context.Context, userID uuid.UUID, startedAt time.Time, metadata json.RawMessage,
	) (*domain.LearningSession, error)
	GetLearningSessionByID(
		ctx context.Context, id uuid.UUID,
	) (*domain.LearningSession, error)
	CompleteLearningSession(
		ctx context.Context, id uuid.UUID, endedAt time.Time, activitiesCompleted, minutes int32,
	) (*domain.LearningSession, error)
	GetSkillMastery(
		ctx context.Context, userID uuid.UUID, skill string,
	) (*domain.SkillMastery, error)
	ListSkillMasteryByUser(
		ctx context.Context, userID uuid.UUID,
	) ([]domain.SkillMastery, error)
	UpsertSkillMastery(
		ctx context.Context, userID uuid.UUID, skill, level string, confidence float64,
	) (*domain.SkillMastery, error)
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

// LearningCaches holds typed cache clients for the learning service.
type LearningCaches struct {
	Dashboard cache.Cache[*domain.DashboardData]
	Progress  cache.Cache[*domain.ProgressData]
}

// Deps holds dependencies for constructing the learning service.
type Deps struct {
	Pool     *pgxpool.Pool
	Repo     Repository
	Lesson   lessoncontract.Reader
	SRSDue   srscontract.QueueReader
	SRSCards srscontract.CardWriter
	Graders  *domain.GraderRegistry
	Events   EventWriter
	Metrics  telemetry.Instruments
	Clock    clock.Clock
	NewID    func() (uuid.UUID, error)
	Caches   LearningCaches
	Env      string
}

// Service coordinates attempt execution, grading, progress rollups, and event emission.
type Service struct {
	pool     *pgxpool.Pool
	repo     Repository
	lesson   lessoncontract.Reader
	srsDue   srscontract.QueueReader
	srsCards srscontract.CardWriter
	graders  *domain.GraderRegistry
	events   EventWriter
	metrics  telemetry.Instruments
	clock    clock.Clock
	newID    func() (uuid.UUID, error)
	caches   LearningCaches
	env      string
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
		pool:     deps.Pool,
		repo:     deps.Repo,
		lesson:   deps.Lesson,
		srsDue:   deps.SRSDue,
		srsCards: deps.SRSCards,
		graders:  deps.Graders,
		events:   deps.Events,
		metrics:  deps.Metrics,
		clock:    clk,
		newID:    idGen,
		caches:   deps.Caches,
		env:      deps.Env,
	}
}

// StartAttempt begins a new in-progress attempt for an activity.
func (s *Service) StartAttempt(ctx context.Context, userID, activityID uuid.UUID) (*StartAttemptDTO, error) {
	// 1. Resolve activity existence and structural position (Trap 1)
	// Proves the activity exists before an attempt row is written against it.
	activity, err := s.resolveActivityHierarchy(ctx, activityID)
	if err != nil {
		return nil, err
	}

	// 2. Check course enrollment (Trap 6: do not auto-enroll, reject if not enrolled)
	if activity.CourseID != uuid.Nil {
		enrollment, err := s.repo.GetEnrollmentByUserCourse(ctx, userID, activity.CourseID)
		if err != nil {
			return nil, fmt.Errorf("check enrollment: %w", err)
		}
		if enrollment == nil || !enrollment.IsActive() {
			return nil, domain.ErrNotEnrolled
		}
	}

	// 3. Check lesson unlock status (Trap 6)
	if activity.LessonID != uuid.Nil {
		unlockedMap, err := s.IsUnlocked(ctx, userID, []uuid.UUID{activity.LessonID})
		if err != nil {
			return nil, fmt.Errorf("check lesson unlocking: %w", err)
		}
		if unlockedMap != nil && !unlockedMap[activity.LessonID] {
			return nil, domain.ErrLessonLocked
		}
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

	if earlyResult, err := s.checkEarlySubmissionState(ctx, attempt, idempotencyKey); err != nil || earlyResult != nil {
		return earlyResult, err
	}

	claimed, current, err := s.claimAttempt(ctx, attempt, idempotencyKey, response)
	if err != nil {
		return nil, err
	}
	if !claimed {
		// A concurrent copy of this same submission won the claim. Wait for it to
		// commit, return what it stored, and do not grade it again.
		settled, waitErr := s.awaitSettledAttempt(ctx, current)
		if waitErr != nil {
			return nil, waitErr
		}
		return s.buildStoredSubmissionResult(settled), nil
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
	ctx context.Context, attempt *domain.Attempt, idempotencyKey uuid.UUID,
) (*SubmitAttemptResultDTO, error) {
	if !attempt.IsGraded() && !attempt.IsGrading() {
		return nil, nil
	}
	if attempt.IdempotencyKey == nil || *attempt.IdempotencyKey != idempotencyKey.String() {
		return nil, domain.ErrAlreadyGraded
	}

	settled, err := s.awaitSettledAttempt(ctx, attempt)
	if err != nil {
		return nil, err
	}
	return s.buildStoredSubmissionResult(settled), nil
}

// How long a duplicate submission waits for the caller holding the claim to
// commit. Short, because the transaction it waits on is one grade and one
// rollup; bounded, because a genuinely asynchronous grader never settles here.
const (
	claimSettleAttempts = 40
	claimSettleInterval = 5 * time.Millisecond
)

// awaitSettledAttempt re-reads an attempt another caller is grading right now,
// until it leaves `grading`.
//
// The loser of the claim cannot see the winner's work: the grade, the rollup and
// the status change are one transaction, so until it commits the loser reads the
// row as the claim left it — `grading`, with no score. Answering from that read
// hands back a 202 for an attempt a synchronous grader is finishing microseconds
// later, and the two callers of one submission get different bodies. P8.3 §6
// asks for the same body twice.
//
// The wait is bounded and the timeout is not an error: a grader that really is
// asynchronous leaves the attempt in `grading` for as long as its job takes, and
// 202 is the right answer then.
func (s *Service) awaitSettledAttempt(
	ctx context.Context, attempt *domain.Attempt,
) (*domain.Attempt, error) {
	latest := attempt
	for range claimSettleAttempts {
		if !latest.IsGrading() {
			return latest, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(claimSettleInterval):
		}
		fresh, err := s.repo.GetAttemptByID(ctx, attempt.ID)
		if err != nil {
			return nil, err
		}
		latest = fresh
	}
	return latest, nil
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

	if s.srsCards != nil && len(gradeResult.ReviewItems) > 0 {
		if err := s.srsCards.UpsertCards(ctx, userID, gradeResult.ReviewItems); err != nil {
			slog.WarnContext(ctx, "failed to upsert srs review cards after grading", "user_id", userID, "error", err)
		}
	}

	s.invalidateLearningCaches(ctx, userID)

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
		// The counter sits beside the outbox write, not instead of it: the event
		// is the record, the metric is what a dashboard can draw. Emitting from
		// here rather than from a consumer means the funnel counts what happened
		// even before anything subscribes.
		s.metrics.RecordFunnelStep(ctx, telemetry.FunnelActivityCompleted)
	}

	// 4. Update incremental skill mastery if focus is a valid skill (Trap 4)
	if err := updateSkillMastery(
		ctx, repo, userID, activity.LessonSkillFocus, gradeResult.Score, gradeResult.MaxScore,
	); err != nil {
		return err
	}

	return s.rollupLessonAndAbove(ctx, tx, repo, userID, activity, now)
}

// updateSkillMastery folds one attempt score into the learner's estimate for the
// lesson's skill.
//
// An unrecognised skill focus is skipped, not an error: learn.skill_mastery.skill
// is a CHECK over six values while learn.lessons.skill_focus is free text, so a
// content author's spelling would otherwise abort the grading transaction and
// fail the learner's submission (Trap 4). A read that genuinely fails is still
// returned — inside a transaction the next statement would fail anyway, and with
// a message that names nothing.
func updateSkillMastery(
	ctx context.Context, repo Repository, userID uuid.UUID, skillFocus string, score, maxScore int,
) error {
	normSkill, ok := domain.NormalizeSkill(skillFocus)
	if !ok {
		return nil
	}
	existing, err := repo.GetSkillMastery(ctx, userID, normSkill)
	if err != nil {
		return fmt.Errorf("get skill mastery %s: %w", normSkill, err)
	}
	level, confidence, _ := domain.EstimateMastery(existing, percentageOf(score, maxScore))
	if _, err := repo.UpsertSkillMastery(ctx, userID, normSkill, level, confidence); err != nil {
		return fmt.Errorf("upsert skill mastery %s: %w", normSkill, err)
	}
	return nil
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
		s.metrics.RecordFunnelStep(ctx, telemetry.FunnelLessonCompleted)
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
	_ int32,
	now time.Time,
) error {
	if activity.CourseID == uuid.Nil || s.lesson == nil {
		return nil
	}

	units, err := s.lesson.ListUnitsByCourseID(ctx, activity.CourseID)
	if err != nil {
		return fmt.Errorf("list course units %s: %w", activity.CourseID, err)
	}
	if len(units) == 0 {
		return nil
	}

	unitIDs := make([]uuid.UUID, len(units))
	for i, u := range units {
		unitIDs[i] = u.ID
	}

	unitProgress, err := repo.ListProgressByUserScopeAndIDs(ctx, userID, "unit", unitIDs)
	if err != nil {
		return fmt.Errorf("list unit progress for course %s: %w", activity.CourseID, err)
	}

	completed, avgCourseScore := calculateCompletedScore(len(units), unitProgress)
	if !completed {
		return nil
	}

	_, err = repo.UpsertProgress(ctx, repository.UpsertProgressParams{
		UserID:      userID,
		Scope:       "course",
		ScopeID:     activity.CourseID,
		Status:      domain.ProgressCompleted,
		Score:       &avgCourseScore,
		CompletedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("upsert course progress: %w", err)
	}

	// Update enrollment to completed in the same transaction (Trap 3)
	_, err = repo.UpdateEnrollmentStatus(ctx, userID, activity.CourseID, domain.StatusEnrollmentCompleted, &now)
	if err != nil {
		return fmt.Errorf("update enrollment status: %w", err)
	}

	if s.events != nil {
		courseEvent := contract.CourseCompleted{
			UserID:     userID,
			CourseID:   activity.CourseID,
			OccurredAt: now,
		}
		s.metrics.RecordFunnelStep(ctx, telemetry.FunnelCourseCompleted)
		if _, err := s.events.Write(ctx, tx, contract.Aggregate, contract.EventCourseCompleted, courseEvent); err != nil {
			return fmt.Errorf("write course.completed event: %w", err)
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

// Enroll registers a user into a course.
func (s *Service) Enroll(ctx context.Context, userID, courseID uuid.UUID) (*domain.Enrollment, error) {
	existing, err := s.repo.GetEnrollmentByUserCourse(ctx, userID, courseID)
	if err != nil {
		return nil, fmt.Errorf("get enrollment: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrAlreadyEnrolled
	}

	now := s.clock.Now().UTC()
	enrollment, err := s.repo.CreateEnrollment(ctx, userID, courseID, domain.StatusEnrollmentActive, now)
	if err == nil {
		s.metrics.RecordFunnelStep(ctx, telemetry.FunnelEnrolled)
	}
	if err != nil {
		return nil, fmt.Errorf("create enrollment: %w", err)
	}
	s.invalidateLearningCaches(ctx, userID)
	return enrollment, nil
}

// IsUnlocked answers whether a learner has met prerequisites for a batch of lessons.
// Implements contract.UnlockChecker.
func (s *Service) IsUnlocked(
	ctx context.Context, userID uuid.UUID, lessonIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if len(lessonIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	if s.lesson == nil {
		return nil, fmt.Errorf("lesson reader not configured")
	}

	prereqs, err := s.lesson.ListPrerequisitesForLessons(ctx, lessonIDs)
	if err != nil {
		return nil, fmt.Errorf("list prerequisites: %w", err)
	}

	prereqsByLesson := make(map[uuid.UUID][]lessoncontract.PrerequisiteItem)
	reqLessonIDs := make([]uuid.UUID, 0, len(prereqs))
	for _, p := range prereqs {
		prereqsByLesson[p.LessonID] = append(prereqsByLesson[p.LessonID], p)
		reqLessonIDs = append(reqLessonIDs, p.RequiresLessonID)
	}

	progressMap := make(map[uuid.UUID]repository.ProgressDTO)
	if len(reqLessonIDs) > 0 && userID != uuid.Nil {
		progressList, err := s.repo.ListProgressByUserScopeAndIDs(ctx, userID, "lesson", reqLessonIDs)
		if err != nil {
			return nil, fmt.Errorf("list prerequisite progress: %w", err)
		}
		for _, prog := range progressList {
			progressMap[prog.ScopeID] = prog
		}
	}

	result := make(map[uuid.UUID]bool, len(lessonIDs))
	for _, id := range lessonIDs {
		reqs := prereqsByLesson[id]
		switch {
		case len(reqs) == 0:
			result[id] = true
		case userID == uuid.Nil:
			result[id] = false
		default:
			result[id] = prerequisitesMet(reqs, progressMap)
		}
	}

	return result, nil
}

// prerequisitesMet reports whether every prerequisite is complete and scored at
// or above its min_score. The score half is not optional: a prerequisite carries
// a threshold, and a checker that reads only the status unlocks a lesson for a
// learner who failed the one before it.
func prerequisitesMet(
	reqs []lessoncontract.PrerequisiteItem, progress map[uuid.UUID]repository.ProgressDTO,
) bool {
	for _, req := range reqs {
		prog, ok := progress[req.RequiresLessonID]
		if !ok || prog.Status != domain.ProgressCompleted {
			return false
		}
		if req.MinScore > 0 && (prog.Score == nil || int(*prog.Score) < req.MinScore) {
			return false
		}
	}
	return true
}

// ProgressOf returns all progress records for a given user and scope.
// Implements contract.ProgressReader.
func (s *Service) ProgressOf(
	ctx context.Context, userID uuid.UUID, scope contract.ProgressScope,
) ([]contract.Progress, error) {
	dtos, err := s.repo.ListProgressByUserAndScope(ctx, userID, string(scope), progressScanLimit)
	if err != nil {
		return nil, fmt.Errorf("list progress of %s: %w", scope, err)
	}
	out := make([]contract.Progress, len(dtos))
	for i, d := range dtos {
		var sc *int
		if d.Score != nil {
			sInt := int(*d.Score)
			sc = &sInt
		}
		out[i] = contract.Progress{
			UserID:      d.UserID,
			Scope:       contract.ProgressScope(d.Scope),
			ScopeID:     d.ScopeID,
			Status:      d.Status,
			Score:       sc,
			CompletedAt: d.CompletedAt,
		}
	}
	return out, nil
}

// Scope-scan limits. Progress rows are one per activity, lesson, unit and
// course a learner has touched, so a bound is needed; these are high enough that
// no Phase 2 learner reaches them and low enough that a corrupt row count cannot
// pull the whole table into memory.
const (
	progressScanLimit   int32 = 1000
	enrollmentScanLimit int32 = 50
)

// NextActivity resolves the single activity the learner should do now.
//
// The three states are the contract, not a convenience: DashboardResponse.state
// is not_started | in_progress | completed, and a learner who has started nothing
// gets an explicit state rather than null or a 404 (Trap 5).
//
// The reads it issues are bounded by the number of units in the course, not the
// number of lessons: this is the endpoint every app open hits, and P8.5 has to be
// able to assert a query count over it.
func (s *Service) NextActivity(ctx context.Context, userID uuid.UUID) (*domain.NextActivityResolution, error) {
	if s.lesson == nil {
		return nil, fmt.Errorf("lesson reader not configured")
	}

	active, state, err := s.activeEnrollment(ctx, userID)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return &domain.NextActivityResolution{State: state}, nil
	}

	lessons, err := s.courseLessonsInOrder(ctx, active.CourseID)
	if err != nil {
		return nil, err
	}
	if len(lessons) == 0 {
		return &domain.NextActivityResolution{State: domain.StateCompleted}, nil
	}

	completedLessons, err := s.completedScopeIDs(ctx, userID, string(contract.ScopeLesson))
	if err != nil {
		return nil, err
	}

	var target *lessoncontract.Lesson
	for _, l := range lessons {
		if l != nil && !completedLessons[l.ID] {
			target = l
			break
		}
	}
	if target == nil {
		return &domain.NextActivityResolution{State: domain.StateCompleted}, nil
	}

	return s.nextActivityInLesson(ctx, userID, active.CourseID, target.ID)
}

// activeEnrollment returns the learner's first active enrolment, or nil with the
// state that answers for its absence: no enrolment at all is not_started, and
// enrolments that have all been completed or dropped is completed.
func (s *Service) activeEnrollment(
	ctx context.Context, userID uuid.UUID,
) (*domain.Enrollment, string, error) {
	enrollments, err := s.repo.ListEnrollmentsByUser(ctx, userID, enrollmentScanLimit)
	if err != nil {
		return nil, "", fmt.Errorf("list enrollments: %w", err)
	}
	if len(enrollments) == 0 {
		return nil, domain.StateNotStarted, nil
	}
	for i := range enrollments {
		if enrollments[i].IsActive() {
			return &enrollments[i], domain.StateInProgress, nil
		}
	}
	return nil, domain.StateCompleted, nil
}

// courseLessonsInOrder lists a course's lessons in unit-then-position order.
//
// One read per unit, not one per lesson: walking the course with NextLesson costs
// a call for every lesson already finished, which is the N+1 the batched
// UnlockChecker exists to avoid, on the same page. A course whose units cannot be
// listed falls back to a single NextLesson call rather than to nothing.
func (s *Service) courseLessonsInOrder(
	ctx context.Context, courseID uuid.UUID,
) ([]*lessoncontract.Lesson, error) {
	units, err := s.lesson.ListUnitsByCourseID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list course units %s: %w", courseID, err)
	}
	if len(units) == 0 {
		first, firstErr := s.lesson.NextLesson(ctx, courseID, nil)
		if firstErr != nil {
			return nil, fmt.Errorf("find first lesson of course %s: %w", courseID, firstErr)
		}
		if first == nil {
			return nil, nil
		}
		return []*lessoncontract.Lesson{first}, nil
	}

	var ordered []*lessoncontract.Lesson
	for _, u := range units {
		if u == nil {
			continue
		}
		unitLessons, listErr := s.lesson.ListLessons(ctx, u.ID)
		if listErr != nil {
			return nil, fmt.Errorf("list lessons of unit %s: %w", u.ID, listErr)
		}
		ordered = append(ordered, unitLessons...)
	}
	return ordered, nil
}

// completedScopeIDs reads the learner's completed rows for one progress scope.
func (s *Service) completedScopeIDs(
	ctx context.Context, userID uuid.UUID, scope string,
) (map[uuid.UUID]bool, error) {
	rows, err := s.repo.ListProgressByUserAndScope(ctx, userID, scope, progressScanLimit)
	if err != nil {
		return nil, fmt.Errorf("list %s progress: %w", scope, err)
	}
	completed := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		if row.Status == domain.ProgressCompleted {
			completed[row.ScopeID] = true
		}
	}
	return completed, nil
}

// nextActivityInLesson picks the first activity of a lesson the learner has not
// completed. Title and estimated minutes come from the lesson: learn.activities
// has no title column, and the spec's example is a lesson name (Trap 5).
func (s *Service) nextActivityInLesson(
	ctx context.Context, userID, courseID, lessonID uuid.UUID,
) (*domain.NextActivityResolution, error) {
	lessonDetail, err := s.lesson.GetLesson(ctx, lessonID)
	if err != nil {
		return nil, fmt.Errorf("get lesson %s: %w", lessonID, err)
	}
	if lessonDetail == nil || len(lessonDetail.Activities) == 0 {
		return &domain.NextActivityResolution{State: domain.StateInProgress}, nil
	}

	actIDs := make([]uuid.UUID, len(lessonDetail.Activities))
	for i, a := range lessonDetail.Activities {
		actIDs[i] = a.ID
	}
	actProgress, err := s.repo.ListProgressByUserScopeAndIDs(
		ctx, userID, string(contract.ScopeActivity), actIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list activity progress: %w", err)
	}
	completedActs := make(map[uuid.UUID]bool, len(actProgress))
	for _, ap := range actProgress {
		if ap.Status == domain.ProgressCompleted {
			completedActs[ap.ScopeID] = true
		}
	}

	for _, a := range lessonDetail.Activities {
		if completedActs[a.ID] {
			continue
		}
		estMins := lessonDetail.EstimatedMinutes
		return &domain.NextActivityResolution{
			State: domain.StateInProgress,
			NextActivity: &domain.NextActivity{
				ActivityID:       a.ID,
				LessonID:         lessonDetail.ID,
				UnitID:           lessonDetail.UnitID,
				CourseID:         courseID,
				Title:            lessonDetail.Title,
				Kind:             a.Kind,
				Skill:            lessonDetail.SkillFocus,
				EstimatedMinutes: &estMins,
			},
		}, nil
	}

	return &domain.NextActivityResolution{State: domain.StateInProgress}, nil
}

// StartSession initiates a new study session.
func (s *Service) StartSession(
	ctx context.Context, userID uuid.UUID, metadata json.RawMessage,
) (*domain.LearningSession, error) {
	now := s.clock.Now().UTC()
	return s.repo.CreateLearningSession(ctx, userID, now, metadata)
}

// CompleteSession finalizes a study session, calculating server-side minutes and emitting an event.
//
// The duration is computed here and never read from the request: a client-supplied
// number of minutes is the same class of mistake as a client-supplied score
// (BR-LEARNING-01). `activities_completed` is in the spec's request body, so it is
// accepted, but a negative count is rejected before the CHECK constraint sees it.
func (s *Service) CompleteSession(
	ctx context.Context, userID, sessionID uuid.UUID, activitiesCompleted *int,
) (*domain.LearningSession, error) {
	if activitiesCompleted != nil && *activitiesCompleted < 0 {
		return nil, domain.ErrInvalidActivityCount
	}

	session, err := s.repo.GetLearningSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.UserID != userID {
		// Another learner's session is a 404 (Trap 6). An id is not an authorisation.
		return nil, domain.ErrSessionNotFound
	}
	if session.IsCompleted() {
		return session, nil
	}

	now := s.clock.Now().UTC()
	if now.Before(session.StartedAt) {
		return nil, domain.ErrInvalidDuration
	}

	minutes := safeCount(int(now.Sub(session.StartedAt).Minutes()))

	actCount := session.ActivitiesCompleted
	if activitiesCompleted != nil {
		actCount = *activitiesCompleted
	}

	event := contract.SessionCompleted{
		UserID:     userID,
		SessionID:  sessionID,
		Minutes:    int(minutes),
		Activities: actCount,
		OccurredAt: now,
	}

	var completed *domain.LearningSession
	finish := func(txCtx context.Context, tx OutboxTx, repo Repository) error {
		var updateErr error
		completed, updateErr = repo.CompleteLearningSession(
			txCtx, sessionID, now, safeCount(actCount), minutes,
		)
		if updateErr != nil {
			return updateErr
		}
		if s.events == nil {
			return nil
		}
		if _, err := s.events.Write(
			txCtx, tx, contract.Aggregate, contract.EventLearningSessionCompleted, event,
		); err != nil {
			return fmt.Errorf("write learning.session_completed event: %w", err)
		}
		return nil
	}

	if s.pool == nil {
		// No pool: the unit suite drives the same path against fakes, exactly as
		// completeSynchronousGrading does, so the outbox write stays reachable.
		if err := finish(ctx, noopTx{}, s.repo); err != nil {
			return nil, err
		}
		s.invalidateLearningCaches(ctx, userID)
		return completed, nil
	}

	if txErr := dbx.InTx(ctx, s.pool, func(txCtx context.Context, tx pgx.Tx) error {
		return finish(txCtx, tx, s.repo.WithTx(tx))
	}); txErr != nil {
		return nil, fmt.Errorf("commit session completion transaction: %w", txErr)
	}
	s.invalidateLearningCaches(ctx, userID)
	return completed, nil
}

// percentageOf turns a grader's score into the 0-100 figure the CEFR bands are
// defined over. A grader is free to mark out of 10 or out of 40 — GradeResult
// carries MaxScore precisely because the scale is the grader's choice — and
// feeding a raw 8-out-of-10 to the estimator would read as an A1.
func percentageOf(score, maxScore int) float64 {
	if maxScore <= 0 {
		return float64(score)
	}
	pct := float64(score) / float64(maxScore) * 100
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// safeCount clamps a non-negative count into int32, the width both
// learning_sessions.activities_completed and .minutes are declared at.
// The clamp is the same habit as safeScore and safeDurationMs above, and it is
// there so no //nolint has to be.
func safeCount(v int) int32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v)
}

// Dashboard returns aggregated dashboard state for the learner.
func (s *Service) Dashboard(ctx context.Context, userID uuid.UUID) (*domain.DashboardData, error) {
	if s.caches.Dashboard != nil && s.env != "" && userID != uuid.Nil {
		key := cache.Key(s.env, "learning", "dashboard", userID.String(), cacheVersion)
		loader := func(lCtx context.Context) (*domain.DashboardData, error) {
			return s.loadDashboard(lCtx, userID)
		}
		return s.caches.Dashboard.GetOrLoad(ctx, key, dashboardTTL, loader)
	}
	return s.loadDashboard(ctx, userID)
}

func (s *Service) loadDashboard(ctx context.Context, userID uuid.UUID) (*domain.DashboardData, error) {
	res, err := s.NextActivity(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve next activity: %w", err)
	}

	masteries, err := s.repo.ListSkillMasteryByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list skill mastery: %w", err)
	}
	if masteries == nil {
		masteries = []domain.SkillMastery{}
	}

	dueCount := 0
	if s.srsDue != nil {
		if c, err := s.srsDue.DueCount(ctx, userID); err == nil {
			dueCount = c
		} else {
			slog.WarnContext(ctx, "failed to get srs due count for dashboard", "user_id", userID, "error", err)
		}
	}

	data := &domain.DashboardData{
		State:           res.State,
		NextActivity:    res.NextActivity,
		DueReviewsCount: dueCount,
		SkillMastery:    masteries,
	}
	return data, nil
}

// Progress returns rolled-up progress across enrolled courses and estimated skill masteries.
func (s *Service) Progress(ctx context.Context, userID uuid.UUID) (*domain.ProgressData, error) {
	if s.caches.Progress != nil && s.env != "" && userID != uuid.Nil {
		key := cache.Key(s.env, "learning", "progress", userID.String(), cacheVersion)
		loader := func(lCtx context.Context) (*domain.ProgressData, error) {
			return s.loadProgress(lCtx, userID)
		}
		return s.caches.Progress.GetOrLoad(ctx, key, progressTTL, loader)
	}
	return s.loadProgress(ctx, userID)
}

func (s *Service) loadProgress(ctx context.Context, userID uuid.UUID) (*domain.ProgressData, error) {
	// The same bound next-activity resolution uses. Two limits for one learner's
	// enrolments would mean the dashboard and the progress page disagree about
	// which courses exist the day somebody enrols in more than the smaller one.
	enrollments, err := s.repo.ListEnrollmentsByUser(ctx, userID, enrollmentScanLimit)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}

	masteries, err := s.repo.ListSkillMasteryByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list skill mastery: %w", err)
	}
	if masteries == nil {
		masteries = []domain.SkillMastery{}
	}

	if len(enrollments) == 0 {
		return &domain.ProgressData{
			Courses: []domain.CourseProgressData{},
			Skills:  masteries,
		}, nil
	}

	courseIDs := make([]uuid.UUID, len(enrollments))
	for i, e := range enrollments {
		courseIDs[i] = e.CourseID
	}

	// One read per fact, none of them per course: the activity ids of every
	// enrolled course, the learner's completed activities among them, and the
	// course-scope progress rows. Four reads answer for a learner in one course
	// or in forty (Trap 1).
	courseActs, err := s.courseActivityIDs(ctx, courseIDs)
	if err != nil {
		return nil, err
	}
	completedActs, err := s.completedActivities(ctx, userID, courseActs)
	if err != nil {
		return nil, err
	}
	courseProgress, err := s.courseProgressRows(ctx, userID, courseIDs)
	if err != nil {
		return nil, err
	}

	coursesData := make([]domain.CourseProgressData, len(enrollments))
	for i, enr := range enrollments {
		coursesData[i] = courseProgressOf(enr, courseActs[enr.CourseID], completedActs, courseProgress)
	}

	return &domain.ProgressData{
		Courses: coursesData,
		Skills:  masteries,
	}, nil
}

// courseActivityIDs asks `lesson` for the activity ids of every course at once.
// `learn.progress` cannot attribute an activity to a course on its own, and this
// module may not read `learn.activities` (Trap 1).
func (s *Service) courseActivityIDs(
	ctx context.Context, courseIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	if s.lesson == nil {
		return map[uuid.UUID][]uuid.UUID{}, nil
	}
	acts, err := s.lesson.ListActivitiesByCourseIDs(ctx, courseIDs)
	if err != nil {
		return nil, fmt.Errorf("list activities for courses: %w", err)
	}
	if acts == nil {
		return map[uuid.UUID][]uuid.UUID{}, nil
	}
	return acts, nil
}

// completedActivities reads the learner's activity-scope progress for every
// activity in the given courses, in one query.
func (s *Service) completedActivities(
	ctx context.Context, userID uuid.UUID, courseActs map[uuid.UUID][]uuid.UUID,
) (map[uuid.UUID]bool, error) {
	var allActivityIDs []uuid.UUID
	for _, ids := range courseActs {
		allActivityIDs = append(allActivityIDs, ids...)
	}
	completed := make(map[uuid.UUID]bool, len(allActivityIDs))
	if len(allActivityIDs) == 0 {
		return completed, nil
	}

	rows, err := s.repo.ListProgressByUserScopeAndIDs(
		ctx, userID, string(contract.ScopeActivity), allActivityIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list activity progress: %w", err)
	}
	for _, row := range rows {
		if row.Status == domain.ProgressCompleted {
			completed[row.ScopeID] = true
		}
	}
	return completed, nil
}

// courseProgressRows reads the course-scope progress rows, which carry the score
// and the completion timestamp the rollup wrote.
func (s *Service) courseProgressRows(
	ctx context.Context, userID uuid.UUID, courseIDs []uuid.UUID,
) (map[uuid.UUID]repository.ProgressDTO, error) {
	rows, err := s.repo.ListProgressByUserScopeAndIDs(
		ctx, userID, string(contract.ScopeCourse), courseIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list course progress: %w", err)
	}
	byCourse := make(map[uuid.UUID]repository.ProgressDTO, len(rows))
	for _, row := range rows {
		byCourse[row.ScopeID] = row
	}
	return byCourse, nil
}

// courseProgressOf assembles one course's row of the response.
//
// The status is the learner's progress through the course, not the enrolment's
// own status: `not_started`, `in_progress` and `completed` answer "how far am
// I", while an enrolment is `active` or `completed` (Trap 5).
func courseProgressOf(
	enrollment domain.Enrollment,
	activityIDs []uuid.UUID,
	completedActs map[uuid.UUID]bool,
	courseProgress map[uuid.UUID]repository.ProgressDTO,
) domain.CourseProgressData {
	total := len(activityIDs)
	completed := 0
	for _, id := range activityIDs {
		if completedActs[id] {
			completed++
		}
	}

	var score *int
	var completedAt *time.Time
	if row, ok := courseProgress[enrollment.CourseID]; ok {
		if row.Score != nil {
			rounded := int(*row.Score)
			score = &rounded
		}
		completedAt = row.CompletedAt
	}
	if completedAt == nil && enrollment.IsCompleted() {
		completedAt = enrollment.CompletedAt
	}

	return domain.CourseProgressData{
		CourseID:            enrollment.CourseID,
		Status:              domain.DeriveCourseProgressStatus(completed, total, enrollment.IsCompleted()),
		CompletedActivities: completed,
		TotalActivities:     total,
		Percentage:          domain.CalculatePercentage(completed, total),
		Score:               score,
		CompletedAt:         completedAt,
	}
}

func (s *Service) invalidateLearningCaches(ctx context.Context, userID uuid.UUID) {
	if s.env == "" || userID == uuid.Nil {
		return
	}
	if s.caches.Dashboard != nil {
		dashboardKey := cache.Key(s.env, "learning", "dashboard", userID.String(), cacheVersion)
		if err := s.caches.Dashboard.Delete(ctx, dashboardKey); err != nil {
			slog.WarnContext(ctx, "failed to invalidate learner dashboard cache",
				"user_id", userID, "error", err)
		}
	}
	if s.caches.Progress != nil {
		progressKey := cache.Key(s.env, "learning", "progress", userID.String(), cacheVersion)
		if err := s.caches.Progress.Delete(ctx, progressKey); err != nil {
			slog.WarnContext(ctx, "failed to invalidate learner progress cache",
				"user_id", userID, "error", err)
		}
	}
}
