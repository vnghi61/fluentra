// Package repository stores exam templates, sittings, reports and integrity signals.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/exam/sqlc"
	"github.com/fluentra/fluentra/internal/modules/exam/domain"
)

// Repository manages persistent operations for the assess schema.
type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// New constructs an exam repository.
func New(pool *pgxpool.Pool) *Repository {
	if pool == nil {
		return &Repository{}
	}
	return &Repository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// ListExams returns every exam template.
func (r *Repository) ListExams(ctx context.Context) ([]sqlc.AssessExam, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListExams(ctx)
}

// GetExamByID returns one exam template.
func (r *Repository) GetExamByID(ctx context.Context, id uuid.UUID) (*sqlc.AssessExam, error) {
	if r.queries == nil {
		return nil, nil
	}
	exam, err := r.queries.GetExamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &exam, nil
}

// GetExamBySlug returns the exam template with this slug.
func (r *Repository) GetExamBySlug(ctx context.Context, slug string) (*sqlc.AssessExam, error) {
	if r.queries == nil {
		return nil, nil
	}
	exam, err := r.queries.GetExamBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &exam, nil
}

// ListExamSections returns a template's sections in order.
func (r *Repository) ListExamSections(ctx context.Context, examID uuid.UUID) ([]sqlc.AssessExamSection, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListExamSections(ctx, examID)
}

// CreateExamAttempt inserts a sitting.
func (r *Repository) CreateExamAttempt(
	ctx context.Context, arg sqlc.CreateExamAttemptParams,
) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.CreateExamAttempt(ctx, arg)
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// GetExamAttemptByID returns a sitting whoever owns it; for jobs, not for requests.
func (r *Repository) GetExamAttemptByID(ctx context.Context, id uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.GetExamAttemptByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// GetExamAttemptForUser returns a sitting only if it belongs to userID.
func (r *Repository) GetExamAttemptForUser(ctx context.Context, id, userID uuid.UUID) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.GetExamAttemptForUser(ctx, sqlc.GetExamAttemptForUserParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// ListUserExamAttempts returns a learner's sittings, newest first.
func (r *Repository) ListUserExamAttempts(
	ctx context.Context, userID uuid.UUID, limit, offset int32,
) ([]sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListUserExamAttempts(ctx, sqlc.ListUserExamAttemptsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

// CountUserExamAttempts counts a learner's sittings.
func (r *Repository) CountUserExamAttempts(ctx context.Context, userID uuid.UUID) (int64, error) {
	if r.queries == nil {
		return 0, nil
	}
	return r.queries.CountUserExamAttempts(ctx, userID)
}

// CountUserActiveAttempts counts a learner's sittings still in progress.
func (r *Repository) CountUserActiveAttempts(ctx context.Context, userID uuid.UUID) (int64, error) {
	if r.queries == nil {
		return 0, nil
	}
	return r.queries.CountUserActiveAttempts(ctx, userID)
}

// CountUserAttemptsToday counts the sittings a learner started in [start, end).
func (r *Repository) CountUserAttemptsToday(
	ctx context.Context, userID uuid.UUID, start, end time.Time,
) (int64, error) {
	if r.queries == nil {
		return 0, nil
	}
	return r.queries.CountUserAttemptsToday(ctx, sqlc.CountUserAttemptsTodayParams{
		UserID:      userID,
		StartedAt:   start,
		StartedAt_2: end,
	})
}

// UpdateDraftAnswers stores a sitting's draft answers while it is in progress.
func (r *Repository) UpdateDraftAnswers(
	ctx context.Context, id uuid.UUID, draftAnswers []byte, updatedAt time.Time,
) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.UpdateDraftAnswers(ctx, sqlc.UpdateDraftAnswersParams{
		ID:           id,
		DraftAnswers: draftAnswers,
		UpdatedAt:    updatedAt,
	})
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// UpdateCurrentSection moves an in-progress sitting to another section.
func (r *Repository) UpdateCurrentSection(
	ctx context.Context, id uuid.UUID, section int32, updatedAt time.Time,
) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.UpdateCurrentSection(ctx, sqlc.UpdateCurrentSectionParams{
		ID:             id,
		CurrentSection: section,
		UpdatedAt:      updatedAt,
	})
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// MarkAttemptCompleted submits a sitting still in progress; pgx.ErrNoRows if it was not.
func (r *Repository) MarkAttemptCompleted(
	ctx context.Context, id uuid.UUID, submittedAt time.Time, submittedBy string,
) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.MarkAttemptCompleted(ctx, sqlc.MarkAttemptCompletedParams{
		ID:          id,
		SubmittedAt: &submittedAt,
		SubmittedBy: &submittedBy,
	})
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// MarkAttemptExpired expires a sitting still in progress; pgx.ErrNoRows if it was not.
func (r *Repository) MarkAttemptExpired(
	ctx context.Context, id uuid.UUID, submittedAt time.Time,
) (*sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	attempt, err := r.queries.MarkAttemptExpired(ctx, sqlc.MarkAttemptExpiredParams{
		ID:          id,
		SubmittedAt: &submittedAt,
	})
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

// ListExpiredInProgressAttempts returns sittings in progress whose deadline is at or before now.
func (r *Repository) ListExpiredInProgressAttempts(
	ctx context.Context, now time.Time,
) ([]sqlc.AssessExamAttempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListExpiredInProgressAttempts(ctx, now)
}

// CreateScoreReport inserts a sitting's report; attempt_id is unique.
func (r *Repository) CreateScoreReport(
	ctx context.Context, arg sqlc.CreateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	if r.queries == nil {
		return nil, nil
	}
	report, err := r.queries.CreateScoreReport(ctx, arg)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// GetScoreReportByAttemptID returns a sitting's report.
func (r *Repository) GetScoreReportByAttemptID(
	ctx context.Context, attemptID uuid.UUID,
) (*sqlc.AssessScoreReport, error) {
	if r.queries == nil {
		return nil, nil
	}
	report, err := r.queries.GetScoreReportByAttemptID(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// ListPendingScoreReports returns reports still waiting for grades, oldest first.
func (r *Repository) ListPendingScoreReports(ctx context.Context) ([]sqlc.AssessScoreReport, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListPendingScoreReports(ctx)
}

// UpdateScoreReport rewrites a sitting's report.
func (r *Repository) UpdateScoreReport(
	ctx context.Context, arg sqlc.UpdateScoreReportParams,
) (*sqlc.AssessScoreReport, error) {
	if r.queries == nil {
		return nil, nil
	}
	report, err := r.queries.UpdateScoreReport(ctx, arg)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// RecordIntegrityEvent stores one integrity signal.
func (r *Repository) RecordIntegrityEvent(ctx context.Context, arg sqlc.RecordIntegrityEventParams) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.RecordIntegrityEvent(ctx, arg)
}

// ListIntegrityEvents returns a sitting's integrity signals in order.
func (r *Repository) ListIntegrityEvents(
	ctx context.Context, attemptID uuid.UUID,
) ([]sqlc.AssessIntegrityEvent, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.ListIntegrityEvents(ctx, attemptID)
}

// ListExamVersions returns all exam versions.
func (r *Repository) ListExamVersions(ctx context.Context) ([]*domain.ExamVersion, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListExamVersions(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*domain.ExamVersion, len(rows))
	for i, row := range rows {
		res[i] = toDomainExamVersion(row)
	}
	return res, nil
}

// ListCurrentExamVersions returns all current exam versions.
func (r *Repository) ListCurrentExamVersions(ctx context.Context) ([]*domain.ExamVersion, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListCurrentExamVersions(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*domain.ExamVersion, len(rows))
	for i, row := range rows {
		res[i] = toDomainExamVersion(row)
	}
	return res, nil
}

// GetExamVersionByID returns an exam version by its ID.
func (r *Repository) GetExamVersionByID(ctx context.Context, id uuid.UUID) (*domain.ExamVersion, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetExamVersionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDomainExamVersion(row), nil
}

// GetExamVersionByCode returns an exam version by its unique code.
func (r *Repository) GetExamVersionByCode(ctx context.Context, code string) (*domain.ExamVersion, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetExamVersionByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return toDomainExamVersion(row), nil
}

// ListExamPartsByVersionID returns all exam parts for a given version.
func (r *Repository) ListExamPartsByVersionID(
	ctx context.Context, versionID uuid.UUID,
) ([]*domain.ExamPart, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListExamPartsByVersionID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	res := make([]*domain.ExamPart, len(rows))
	for i, row := range rows {
		res[i] = toDomainExamPart(row)
	}
	return res, nil
}

// GetExamPartByID returns an exam part by its ID.
func (r *Repository) GetExamPartByID(ctx context.Context, id uuid.UUID) (*domain.ExamPart, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetExamPartByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDomainExamPart(row), nil
}

// ListBlueprintsByVersionID returns all blueprints for a version.
func (r *Repository) ListBlueprintsByVersionID(
	ctx context.Context, versionID uuid.UUID,
) ([]*domain.Blueprint, error) {
	if r.queries == nil {
		return nil, nil
	}
	rows, err := r.queries.ListBlueprintsByVersionID(ctx, versionID)
	if err != nil {
		return nil, err
	}
	res := make([]*domain.Blueprint, len(rows))
	for i, row := range rows {
		res[i] = toDomainBlueprint(row)
	}
	return res, nil
}

// GetBlueprintByID returns a blueprint by its ID.
func (r *Repository) GetBlueprintByID(ctx context.Context, id uuid.UUID) (*domain.Blueprint, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetBlueprintByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDomainBlueprint(row), nil
}

// GetBlueprintByName returns a blueprint by its version ID and name.
func (r *Repository) GetBlueprintByName(
	ctx context.Context, versionID uuid.UUID, name string,
) (*domain.Blueprint, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetBlueprintByName(ctx, sqlc.GetBlueprintByNameParams{
		VersionID: versionID,
		Name:      name,
	})
	if err != nil {
		return nil, err
	}
	return toDomainBlueprint(row), nil
}

func toDomainExamVersion(row sqlc.AssessExamVersion) *domain.ExamVersion {
	return &domain.ExamVersion{
		ID:           row.ID,
		ExamFamily:   row.ExamFamily,
		Code:         row.Code,
		Title:        row.Title,
		TotalMinutes: int(row.TotalMinutes),
		Scoring:      row.Scoring,
		SourceURL:    row.SourceUrl,
		VerifiedAt:   row.VerifiedAt.Time,
		IsCurrent:    row.IsCurrent,
		Notes:        row.Notes,
	}
}

func toDomainExamPart(row sqlc.AssessExamPart) *domain.ExamPart {
	var dur *int
	if row.DurationMinutes != nil {
		d := int(*row.DurationMinutes)
		dur = &d
	}
	return &domain.ExamPart{
		ID:              row.ID,
		VersionID:       row.VersionID,
		Section:         row.Section,
		PartNumber:      int(row.PartNumber),
		Kind:            row.Kind,
		QuestionCount:   int(row.QuestionCount),
		GroupSize:       int(row.GroupSize),
		DurationMinutes: dur,
		Constraints:     row.Constraints,
	}
}

func toDomainBlueprint(row sqlc.AssessBlueprint) *domain.Blueprint {
	return &domain.Blueprint{
		ID:               row.ID,
		VersionID:        row.VersionID,
		Name:             row.Name,
		CefrDistribution: row.CefrDistribution,
		NodeDistribution: row.NodeDistribution,
	}
}
