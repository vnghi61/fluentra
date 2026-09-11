// Package repository handles database access for the learning module.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// CreateAttemptParams contains fields to create an in-progress attempt.
type CreateAttemptParams struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	UserID         uuid.UUID
	ActivityID     uuid.UUID
	IdempotencyKey *uuid.UUID
	Response       json.RawMessage
	Score          *int32
	MaxScore       int32
	Grader         *string
	DurationMs     int32
	Status         string
}

// ClaimAttemptParams holds inputs for atomically claiming an attempt for grading.
type ClaimAttemptParams struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	IdempotencyKey *uuid.UUID
	Response       json.RawMessage
}

// UpdateAttemptStatusParams holds updates to apply when grading is complete.
type UpdateAttemptStatusParams struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	Status     string
	Score      *int32
	Grader     *string
	DurationMs int32
}

// CompleteGradingAttemptParams holds updates to apply when completing grading conditionally.
type CompleteGradingAttemptParams struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	Score      *int32
	Grader     *string
	DurationMs int32
}

// ProgressDTO models progress state for a scope.
type ProgressDTO struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Scope       string     `json:"scope"`
	ScopeID     uuid.UUID  `json:"scope_id"`
	Status      string     `json:"status"`
	Score       *float64   `json:"score,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// AnswerExplanationDTO represents an explanation for an answer to a question version.
type AnswerExplanationDTO struct {
	ID               uuid.UUID `json:"id"`
	ContentVersionID uuid.UUID `json:"content_version_id"`
	UserAnswer       string    `json:"user_answer"`
	IsCorrect        bool      `json:"is_correct"`
	Text             string    `json:"text"`
	TextVi           string    `json:"text_vi"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UpsertProgressParams holds fields to create or update progress.
type UpsertProgressParams struct {
	UserID      uuid.UUID
	Scope       string
	ScopeID     uuid.UUID
	Status      string
	Score       *int32
	CompletedAt *time.Time
}

// Repository handles database operations for learning.
type Repository struct {
	queries *sqlc.Queries
}

// New creates a new learning Repository over db.
func New(db dbx.Querier) *Repository {
	if db == nil {
		return &Repository{queries: nil}
	}
	return &Repository{
		queries: sqlc.New(db),
	}
}

// WithTx returns a Repository operating within tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		queries: r.queries.WithTx(tx),
	}
}

// CreateAttempt inserts a new attempt row.
func (r *Repository) CreateAttempt(ctx context.Context, params CreateAttemptParams) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	var respBytes []byte
	if len(params.Response) > 0 {
		respBytes = []byte(params.Response)
	}

	row, err := r.queries.CreateAttempt(ctx, sqlc.CreateAttemptParams{
		ID:             params.ID,
		CreatedAt:      params.CreatedAt,
		UserID:         params.UserID,
		ActivityID:     params.ActivityID,
		IdempotencyKey: params.IdempotencyKey,
		Response:       respBytes,
		Score:          params.Score,
		MaxScore:       params.MaxScore,
		Grader:         params.Grader,
		DurationMs:     params.DurationMs,
		Status:         params.Status,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// GetAttemptByID retrieves an attempt by its ID.
func (r *Repository) GetAttemptByID(ctx context.Context, id uuid.UUID) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	row, err := r.queries.GetAttemptByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAttemptNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// ClaimAttemptForGrading atomically flips an in-progress attempt to grading.
// Returns pgx.ErrNoRows if another concurrent caller claimed it first (Trap 4).
func (r *Repository) ClaimAttemptForGrading(ctx context.Context, params ClaimAttemptParams) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	var respBytes []byte
	if len(params.Response) > 0 {
		respBytes = []byte(params.Response)
	}

	row, err := r.queries.ClaimAttemptForGrading(ctx, sqlc.ClaimAttemptForGradingParams{
		ID:             params.ID,
		CreatedAt:      params.CreatedAt,
		IdempotencyKey: params.IdempotencyKey,
		Response:       respBytes,
	})
	if err != nil {
		return nil, err // Preserve pgx.ErrNoRows so service can distinguish race condition
	}
	return toDomainAttempt(row), nil
}

// UnclaimAttempt reverts an attempt from grading back to in_progress upon error.
func (r *Repository) UnclaimAttempt(ctx context.Context, id uuid.UUID, createdAt time.Time) error {
	if r.queries == nil {
		return domain.ErrAttemptNotFound
	}
	return r.queries.UnclaimAttempt(ctx, sqlc.UnclaimAttemptParams{
		ID:        id,
		CreatedAt: createdAt,
	})
}

// UpdateAttemptStatus updates the attempt's status, score, grader, and duration upon completion.
func (r *Repository) UpdateAttemptStatus(
	ctx context.Context, params UpdateAttemptStatusParams,
) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	row, err := r.queries.UpdateAttemptStatus(ctx, sqlc.UpdateAttemptStatusParams{
		ID:         params.ID,
		CreatedAt:  params.CreatedAt,
		Status:     params.Status,
		Score:      params.Score,
		Grader:     params.Grader,
		DurationMs: params.DurationMs,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAttemptNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// CompleteGradingAttempt conditionally updates an attempt from grading to graded.
// Returns the count of affected rows (1 on success, 0 if attempt was not in status 'grading').
func (r *Repository) CompleteGradingAttempt(
	ctx context.Context, params CompleteGradingAttemptParams,
) (int64, error) {
	if r.queries == nil {
		return 0, domain.ErrAttemptNotFound
	}
	rows, err := r.queries.CompleteGradingAttempt(ctx, sqlc.CompleteGradingAttemptParams{
		ID:         params.ID,
		CreatedAt:  params.CreatedAt,
		Score:      params.Score,
		Grader:     params.Grader,
		DurationMs: params.DurationMs,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return rows, nil
}

// FailGradingAttempt conditionally updates an attempt from grading to failed.
// Returns the count of affected rows (1 on success, 0 if attempt was not in status 'grading').
func (r *Repository) FailGradingAttempt(
	ctx context.Context, id uuid.UUID, createdAt time.Time,
) (int64, error) {
	if r.queries == nil {
		return 0, domain.ErrAttemptNotFound
	}
	rows, err := r.queries.FailGradingAttempt(ctx, sqlc.FailGradingAttemptParams{
		ID:        id,
		CreatedAt: createdAt,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return rows, nil
}

// FailStuckGradingAttempts marks attempts stuck in 'grading' beyond cutoff as failed.
func (r *Repository) FailStuckGradingAttempts(ctx context.Context, cutoff time.Time) (int64, error) {
	if r.queries == nil {
		return 0, nil
	}
	rows, err := r.queries.FailStuckGradingAttempts(ctx, cutoff)
	if err != nil {
		return 0, mapPgError(err)
	}
	return rows, nil
}

// CountGradedAttemptsSince counts attempts for a user with the given grader since a given time.
func (r *Repository) CountGradedAttemptsSince(
	ctx context.Context, userID uuid.UUID, grader string, since time.Time,
) (int, error) {
	if r.queries == nil {
		return 0, nil
	}
	var gPtr *string
	if grader != "" {
		gPtr = &grader
	}
	count, err := r.queries.CountGradedAttemptsSince(ctx, sqlc.CountGradedAttemptsSinceParams{
		UserID:    userID,
		Grader:    gPtr,
		CreatedAt: since,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return int(count), nil
}

// GetProgressByUserScope retrieves progress for a specific scope.
func (r *Repository) GetProgressByUserScope(
	ctx context.Context, userID uuid.UUID, scope string, scopeID uuid.UUID,
) (*ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetProgressByUserScope(ctx, sqlc.GetProgressByUserScopeParams{
		UserID:  userID,
		Scope:   scope,
		ScopeID: scopeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toProgressDTO(row), nil
}

// UpsertProgress creates or updates progress for a user and scope.
func (r *Repository) UpsertProgress(ctx context.Context, params UpsertProgressParams) (*ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.UpsertProgress(ctx, sqlc.UpsertProgressParams{
		UserID:      params.UserID,
		Scope:       params.Scope,
		ScopeID:     params.ScopeID,
		Status:      params.Status,
		Score:       params.Score,
		CompletedAt: params.CompletedAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toProgressDTO(row), nil
}

// ListProgressByUserScopeAndIDs retrieves progress rows for a batch of scope IDs.
func (r *Repository) ListProgressByUserScopeAndIDs(
	ctx context.Context, userID uuid.UUID, scope string, scopeIDs []uuid.UUID,
) ([]ProgressDTO, error) {
	if r.queries == nil || len(scopeIDs) == 0 {
		return nil, nil
	}
	rows, err := r.queries.ListProgressByUserScopeAndIDs(ctx, sqlc.ListProgressByUserScopeAndIDsParams{
		UserID:   userID,
		Scope:    scope,
		ScopeIds: scopeIDs,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]ProgressDTO, len(rows))
	for i, row := range rows {
		p := toProgressDTO(row)
		if p != nil {
			out[i] = *p
		}
	}
	return out, nil
}

// ListProgressByUserAndScope retrieves progress rows for a user within a specific scope.
func (r *Repository) ListProgressByUserAndScope(
	ctx context.Context, userID uuid.UUID, scope string, limit int32,
) ([]ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListProgressByUserAndScope(ctx, sqlc.ListProgressByUserAndScopeParams{
		UserID: userID,
		Scope:  scope,
		Limit:  limit,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]ProgressDTO, len(rows))
	for i, row := range rows {
		p := toProgressDTO(row)
		if p != nil {
			out[i] = *p
		}
	}
	return out, nil
}

// ListProgressByUser retrieves recent progress rows for a user across all scopes.
func (r *Repository) ListProgressByUser(
	ctx context.Context, userID uuid.UUID, limit int32,
) ([]ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListProgressByUser(ctx, sqlc.ListProgressByUserParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]ProgressDTO, len(rows))
	for i, row := range rows {
		p := toProgressDTO(row)
		if p != nil {
			out[i] = *p
		}
	}
	return out, nil
}

// GetEnrollmentByUserCourse retrieves an enrollment for a user in a course.
func (r *Repository) GetEnrollmentByUserCourse(
	ctx context.Context, userID, courseID uuid.UUID,
) (*domain.Enrollment, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetEnrollmentByUserCourse(ctx, sqlc.GetEnrollmentByUserCourseParams{
		UserID:   userID,
		CourseID: courseID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainEnrollment(row), nil
}

// ListEnrollmentsByUser retrieves all enrollments for a user.
func (r *Repository) ListEnrollmentsByUser(
	ctx context.Context, userID uuid.UUID, limit int32,
) ([]domain.Enrollment, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListEnrollmentsByUser(ctx, sqlc.ListEnrollmentsByUserParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]domain.Enrollment, len(rows))
	for i, row := range rows {
		out[i] = *toDomainEnrollment(row)
	}
	return out, nil
}

// CreateEnrollment inserts a new enrollment row.
func (r *Repository) CreateEnrollment(
	ctx context.Context, userID, courseID uuid.UUID, status string, startedAt time.Time,
) (*domain.Enrollment, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{
		UserID:    userID,
		CourseID:  courseID,
		Status:    status,
		StartedAt: startedAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainEnrollment(row), nil
}

// UpdateEnrollmentStatus updates enrollment status and completion timestamp.
func (r *Repository) UpdateEnrollmentStatus(
	ctx context.Context, userID, courseID uuid.UUID, status string, completedAt *time.Time,
) (*domain.Enrollment, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.UpdateEnrollmentStatus(ctx, sqlc.UpdateEnrollmentStatusParams{
		UserID:      userID,
		CourseID:    courseID,
		Status:      status,
		CompletedAt: completedAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainEnrollment(row), nil
}

// CreateLearningSession inserts a new learning session row.
func (r *Repository) CreateLearningSession(
	ctx context.Context, userID uuid.UUID, startedAt time.Time, metadata json.RawMessage,
) (*domain.LearningSession, error) {
	if r.queries == nil {
		return nil, nil
	}
	var metaBytes []byte
	if len(metadata) > 0 {
		metaBytes = []byte(metadata)
	}
	row, err := r.queries.CreateLearningSession(ctx, sqlc.CreateLearningSessionParams{
		UserID:    userID,
		StartedAt: startedAt,
		Metadata:  metaBytes,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainSession(row), nil
}

// GetLearningSessionByID retrieves a learning session by ID.
func (r *Repository) GetLearningSessionByID(
	ctx context.Context, id uuid.UUID,
) (*domain.LearningSession, error) {
	if r.queries == nil {
		return nil, domain.ErrSessionNotFound
	}
	row, err := r.queries.GetLearningSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainSession(row), nil
}

// CompleteLearningSession updates a learning session upon completion.
func (r *Repository) CompleteLearningSession(
	ctx context.Context, id uuid.UUID, endedAt time.Time, activitiesCompleted, minutes int32,
) (*domain.LearningSession, error) {
	if r.queries == nil {
		return nil, domain.ErrSessionNotFound
	}
	row, err := r.queries.CompleteLearningSession(ctx, sqlc.CompleteLearningSessionParams{
		ID:                  id,
		EndedAt:             &endedAt,
		ActivitiesCompleted: activitiesCompleted,
		Minutes:             minutes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainSession(row), nil
}

// GetSkillMastery retrieves a skill mastery estimate for a user and skill.
func (r *Repository) GetSkillMastery(
	ctx context.Context, userID uuid.UUID, skill string,
) (*domain.SkillMastery, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetSkillMastery(ctx, sqlc.GetSkillMasteryParams{
		UserID: userID,
		Skill:  skill,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainSkillMastery(row), nil
}

// ListSkillMasteryByUser retrieves all skill mastery records for a user.
func (r *Repository) ListSkillMasteryByUser(
	ctx context.Context, userID uuid.UUID,
) ([]domain.SkillMastery, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListSkillMasteryByUser(ctx, userID)
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]domain.SkillMastery, len(rows))
	for i, row := range rows {
		m := toDomainSkillMastery(row)
		if m != nil {
			out[i] = *m
		}
	}
	return out, nil
}

// UpsertSkillMastery creates or updates a skill mastery record.
func (r *Repository) UpsertSkillMastery(
	ctx context.Context, userID uuid.UUID, skill, level string, confidence float64,
) (*domain.SkillMastery, error) {
	if r.queries == nil {
		return nil, nil
	}
	// confidence is numeric(5,2) with a 0..1 CHECK behind it, so the value is
	// formatted to two places here rather than left to the driver. A Scan that
	// fails would leave the numeric NULL and break a NOT NULL column, so the
	// error is returned instead of dropped.
	var conf pgtype.Numeric
	if err := conf.Scan(strconv.FormatFloat(confidence, 'f', 2, 64)); err != nil {
		return nil, fmt.Errorf("encode confidence %v: %w", confidence, err)
	}

	row, err := r.queries.UpsertSkillMastery(ctx, sqlc.UpsertSkillMasteryParams{
		UserID:     userID,
		Skill:      skill,
		Level:      level,
		Confidence: conf,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainSkillMastery(row), nil
}

func toDomainAttempt(row sqlc.LearnAttempt) *domain.Attempt {
	var keyStr *string
	if row.IdempotencyKey != nil {
		s := row.IdempotencyKey.String()
		keyStr = &s
	}
	// Integers all the way through: attempts.score and max_score are integer
	// columns, and contract.GradeResult declares them int. A float here would be
	// a third representation of the same number.
	var score *int
	if row.Score != nil {
		n := int(*row.Score)
		score = &n
	}
	var dur *int64
	if row.DurationMs > 0 {
		d := int64(row.DurationMs)
		dur = &d
	}
	return &domain.Attempt{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		UserID:         row.UserID,
		ActivityID:     row.ActivityID,
		IdempotencyKey: keyStr,
		Response:       json.RawMessage(row.Response),
		Score:          score,
		MaxScore:       int(row.MaxScore),
		Grader:         row.Grader,
		DurationMs:     dur,
		Status:         row.Status,
	}
}

func toDomainEnrollment(row sqlc.LearnEnrollment) *domain.Enrollment {
	return &domain.Enrollment{
		ID:          row.ID,
		UserID:      row.UserID,
		CourseID:    row.CourseID,
		Status:      row.Status,
		StartedAt:   row.StartedAt,
		CompletedAt: row.CompletedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toDomainSession(row sqlc.LearnLearningSession) *domain.LearningSession {
	return &domain.LearningSession{
		ID:                  row.ID,
		UserID:              row.UserID,
		StartedAt:           row.StartedAt,
		EndedAt:             row.EndedAt,
		ActivitiesCompleted: int(row.ActivitiesCompleted),
		Minutes:             int(row.Minutes),
		Metadata:            json.RawMessage(row.Metadata),
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}

func toDomainSkillMastery(row sqlc.LearnSkillMastery) *domain.SkillMastery {
	var conf float64
	if f, err := row.Confidence.Float64Value(); err == nil && f.Valid {
		conf = f.Float64
	}
	return &domain.SkillMastery{
		ID:         row.ID,
		UserID:     row.UserID,
		Skill:      row.Skill,
		Level:      row.Level,
		Confidence: conf,
		UpdatedAt:  row.UpdatedAt,
		CreatedAt:  row.CreatedAt,
	}
}

func toProgressDTO(row sqlc.LearnProgress) *ProgressDTO {
	var score *float64
	if row.Score != nil {
		f := float64(*row.Score)
		score = &f
	}
	return &ProgressDTO{
		ID:          row.ID,
		UserID:      row.UserID,
		Scope:       row.Scope,
		ScopeID:     row.ScopeID,
		Status:      row.Status,
		Score:       score,
		CompletedAt: row.CompletedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// GetAnswerExplanation returns a cached explanation for a question version and learner answer.
func (r *Repository) GetAnswerExplanation(
	ctx context.Context, contentVersionID uuid.UUID, userAnswer string,
) (*AnswerExplanationDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetAnswerExplanation(ctx, sqlc.GetAnswerExplanationParams{
		ContentVersionID: contentVersionID,
		UserAnswer:       userAnswer,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return &AnswerExplanationDTO{
		ID:               row.ID,
		ContentVersionID: row.ContentVersionID,
		UserAnswer:       row.UserAnswer,
		IsCorrect:        row.IsCorrect,
		Text:             row.ExplanationEn,
		TextVi:           row.ExplanationVi,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

// UpsertAnswerExplanation inserts or updates an explanation for a question version and learner answer.
func (r *Repository) UpsertAnswerExplanation(
	ctx context.Context, explanation AnswerExplanationDTO,
) (*AnswerExplanationDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.UpsertAnswerExplanation(ctx, sqlc.UpsertAnswerExplanationParams{
		ContentVersionID: explanation.ContentVersionID,
		UserAnswer:       explanation.UserAnswer,
		IsCorrect:        explanation.IsCorrect,
		ExplanationEn:    explanation.Text,
		ExplanationVi:    explanation.TextVi,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return &AnswerExplanationDTO{
		ID:               row.ID,
		ContentVersionID: row.ContentVersionID,
		UserAnswer:       row.UserAnswer,
		IsCorrect:        row.IsCorrect,
		Text:             row.ExplanationEn,
		TextVi:           row.ExplanationVi,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

// GetPoolPracticeCourseID reads the ID of the pool-practice course.
func (r *Repository) GetPoolPracticeCourseID(ctx context.Context) (uuid.UUID, error) {
	id, err := r.queries.GetPoolPracticeCourseID(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, mapPgError(err)
	}
	return id, nil
}

// GetDailySet reads a user's practice set for a specific local date.
func (r *Repository) GetDailySet(
	ctx context.Context, userID uuid.UUID, localDate time.Time,
) (*domain.DailySet, error) {
	row, err := r.queries.GetDailySet(ctx, sqlc.GetDailySetParams{
		UserID: userID,
		LocalDate: pgtype.Date{
			Time:  localDate,
			Valid: true,
		},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return &domain.DailySet{
		ID:          row.ID,
		UserID:      row.UserID,
		LocalDate:   row.LocalDate.Time,
		ActivityIDs: row.ActivityIds,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// CreateDailySet stores a newly built daily set, or returns existing if race occurred.
func (r *Repository) CreateDailySet(
	ctx context.Context, userID uuid.UUID, localDate time.Time, activityIDs []uuid.UUID,
) (*domain.DailySet, error) {
	row, err := r.queries.CreateDailySet(ctx, sqlc.CreateDailySetParams{
		UserID: userID,
		LocalDate: pgtype.Date{
			Time:  localDate,
			Valid: true,
		},
		ActivityIds: activityIDs,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return &domain.DailySet{
		ID:          row.ID,
		UserID:      row.UserID,
		LocalDate:   row.LocalDate.Time,
		ActivityIDs: row.ActivityIds,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// RecordItemExposure writes an exposure record for a learner.
func (r *Repository) RecordItemExposure(
	ctx context.Context, userID uuid.UUID, activityID uuid.UUID,
) error {
	return r.queries.RecordItemExposure(ctx, sqlc.RecordItemExposureParams{
		UserID:     userID,
		ActivityID: activityID,
	})
}

// CountActivePoolActivitiesForSlot counts active items in a practice pool slot.
func (r *Repository) CountActivePoolActivitiesForSlot(
	ctx context.Context, levelTitle string, kind string,
) (int64, error) {
	count, err := r.queries.CountActivePoolActivitiesForSlot(ctx, sqlc.CountActivePoolActivitiesForSlotParams{
		Title: levelTitle,
		Kind:  kind,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return count, nil
}

// ListPoolActivitiesForSlot lists all active activities for a practice pool slot.
func (r *Repository) ListPoolActivitiesForSlot(
	ctx context.Context, levelTitle string, kind string,
) ([]domain.PoolActivity, error) {
	rows, err := r.queries.ListPoolActivitiesForSlot(ctx, sqlc.ListPoolActivitiesForSlotParams{
		Title: levelTitle,
		Kind:  kind,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	activities := make([]domain.PoolActivity, len(rows))
	for i, row := range rows {
		activities[i] = domain.PoolActivity{
			ID:               row.ID,
			LessonID:         row.LessonID,
			Position:         int(row.Position),
			Kind:             row.Kind,
			ContentVersionID: row.ContentVersionID,
			Config:           row.Config,
			Weight:           int(row.Weight),
		}
	}
	return activities, nil
}

// ListUnseenPoolActivitiesForSlot lists pool activities in a slot unseen by the learner.
func (r *Repository) ListUnseenPoolActivitiesForSlot(
	ctx context.Context, levelTitle string, kind string, userID uuid.UUID,
) ([]domain.PoolActivity, error) {
	rows, err := r.queries.ListUnseenPoolActivitiesForSlot(ctx, sqlc.ListUnseenPoolActivitiesForSlotParams{
		Title:  levelTitle,
		Kind:   kind,
		UserID: userID,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	activities := make([]domain.PoolActivity, len(rows))
	for i, row := range rows {
		activities[i] = domain.PoolActivity{
			ID:               row.ID,
			LessonID:         row.LessonID,
			Position:         int(row.Position),
			Kind:             row.Kind,
			ContentVersionID: row.ContentVersionID,
			Config:           row.Config,
			Weight:           int(row.Weight),
		}
	}
	return activities, nil
}

// ListSeenPoolActivitiesForSlotOldestFirst lists pool activities in a slot seen by the learner, oldest exposure first.
func (r *Repository) ListSeenPoolActivitiesForSlotOldestFirst(
	ctx context.Context, levelTitle string, kind string, userID uuid.UUID,
) ([]domain.PoolActivity, error) {
	rows, err := r.queries.ListSeenPoolActivitiesForSlotOldestFirst(ctx, sqlc.ListSeenPoolActivitiesForSlotOldestFirstParams{
		Title:  levelTitle,
		Kind:   kind,
		UserID: userID,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	activities := make([]domain.PoolActivity, len(rows))
	for i, row := range rows {
		activities[i] = domain.PoolActivity{
			ID:               row.ID,
			LessonID:         row.LessonID,
			Position:         int(row.Position),
			Kind:             row.Kind,
			ContentVersionID: row.ContentVersionID,
			Config:           row.Config,
			Weight:           int(row.Weight),
		}
	}
	return activities, nil
}

// HasActiveUserWithFewUnseenItems checks if any user active in last 14 days has < 10 unseen items in the slot.
func (r *Repository) HasActiveUserWithFewUnseenItems(
	ctx context.Context, levelTitle string, kind string,
) (bool, error) {
	has, err := r.queries.HasActiveUserWithFewUnseenItems(ctx, sqlc.HasActiveUserWithFewUnseenItemsParams{
		Title: levelTitle,
		Kind:  kind,
	})
	if err != nil {
		return false, mapPgError(err)
	}
	return has, nil
}

// GetPoolLessonID gets the lesson ID for a pool slot by level and lesson title.
func (r *Repository) GetPoolLessonID(
	ctx context.Context, levelTitle string, lessonTitle string,
) (uuid.UUID, error) {
	id, err := r.queries.GetPoolLessonID(ctx, sqlc.GetPoolLessonIDParams{
		Title:   levelTitle,
		Title_2: lessonTitle,
	})
	if err != nil {
		return uuid.Nil, mapPgError(err)
	}
	return id, nil
}

// ListActivitiesByIDs retrieves activities by a slice of IDs.
func (r *Repository) ListActivitiesByIDs(
	ctx context.Context, activityIDs []uuid.UUID,
) ([]domain.PoolActivity, error) {
	if len(activityIDs) == 0 {
		return []domain.PoolActivity{}, nil
	}
	rows, err := r.queries.ListActivitiesByIDs(ctx, activityIDs)
	if err != nil {
		return nil, mapPgError(err)
	}
	activities := make([]domain.PoolActivity, len(rows))
	for i, row := range rows {
		activities[i] = domain.PoolActivity{
			ID:               row.ID,
			LessonID:         row.LessonID,
			Position:         int(row.Position),
			Kind:             row.Kind,
			ContentVersionID: row.ContentVersionID,
			Config:           row.Config,
			Weight:           int(row.Weight),
		}
	}
	return activities, nil
}

func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			switch pgErr.ConstraintName {
			case "uq_enrollments_user_course":
				return domain.ErrAlreadyEnrolled
			default:
				return domain.ErrIdempotencyConflict
			}
		case "23503": // foreign_key_violation
			// Enrolling in a course id that does not exist is a 404, not the 500 a
			// raw constraint error would produce. learn.courses belongs to `lesson`,
			// so the foreign key is the only thing this module can ask.
			if pgErr.ConstraintName == "fk_enrollments_course" {
				return domain.ErrCourseNotFound
			}
		case "23514": // check_violation
			switch pgErr.ConstraintName {
			case "ck_attempts_status", "ck_enrollments_status":
				return domain.ErrInvalidStatus
			case "ck_learning_sessions_ended_after_started":
				return domain.ErrInvalidDuration
			}
		}
	}
	return err
}
