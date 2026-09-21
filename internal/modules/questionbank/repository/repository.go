// Package repository provides persistence implementations for question bank items and statistics.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/questionbank/sqlc"
	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
)

// Repository handles database persistence for question bank.
type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// New constructs a new questionbank Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// CreateQuestion inserts a new question item.
func (r *Repository) CreateQuestion(ctx context.Context, q *domain.Question) (*domain.Question, error) {
	provBytes, err := json.Marshal(q.Provenance)
	if err != nil {
		return nil, fmt.Errorf("marshal provenance: %w", err)
	}

	diffNum := pgtype.Numeric{}
	if q.Difficulty != nil {
		_ = diffNum.Scan(strconv.FormatFloat(*q.Difficulty, 'f', 3, 64))
	}

	id := q.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	created, err := r.queries.CreateQuestion(ctx, sqlc.CreateQuestionParams{
		ID:            id,
		ContentItemID: q.ContentItemID,
		ActivityID:    q.ActivityID,
		ExamPartID:    q.ExamPartID,
		Kind:          q.Kind,
		Skill:         q.Skill,
		CefrLevel:     q.CEFRLevel,
		Difficulty:    diffNum,
		QuestionCount: int32(q.QuestionCount), //nolint:gosec // bounded to 1..20
		Fingerprint:   q.Fingerprint,
		Provenance:    provBytes,
		Status:        q.Status,
	})
	if err != nil {
		return nil, err
	}

	return toDomainQuestion(created), nil
}

// GetQuestionByID fetches a question by ID.
func (r *Repository) GetQuestionByID(ctx context.Context, id uuid.UUID) (*domain.Question, error) {
	row, err := r.queries.GetQuestionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}
	return toDomainQuestion(row), nil
}

// GetQuestionByFingerprint fetches a question by canonical fingerprint.
func (r *Repository) GetQuestionByFingerprint(ctx context.Context, fp string) (*domain.Question, error) {
	row, err := r.queries.GetQuestionByFingerprint(ctx, fp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}
	return toDomainQuestion(row), nil
}

// GetQuestionByContentItemID fetches a question by content item ID.
func (r *Repository) GetQuestionByContentItemID(
	ctx context.Context, contentItemID uuid.UUID,
) (*domain.Question, error) {
	row, err := r.queries.GetQuestionByContentItemID(ctx, contentItemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}
	return toDomainQuestion(row), nil
}

// UpdateQuestionStatus updates question status.
func (r *Repository) UpdateQuestionStatus(ctx context.Context, id uuid.UUID, status string) (*domain.Question, error) {
	row, err := r.queries.UpdateQuestionStatus(ctx, sqlc.UpdateQuestionStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}
	return toDomainQuestion(row), nil
}

// UpdateQuestionActivityID associates the published activity ID.
func (r *Repository) UpdateQuestionActivityID(
	ctx context.Context, id uuid.UUID, activityID uuid.UUID,
) (*domain.Question, error) {
	row, err := r.queries.UpdateQuestionActivityID(ctx, sqlc.UpdateQuestionActivityIDParams{
		ID:         id,
		ActivityID: &activityID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}
	return toDomainQuestion(row), nil
}

// ListQuestions returns paginated items matching criteria and total count.
func (r *Repository) ListQuestions(ctx context.Context, filter contract.Filter) ([]*domain.Question, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	total, err := r.queries.CountQuestions(ctx, sqlc.CountQuestionsParams{
		Kind:       filter.Kind,
		Skill:      filter.Skill,
		CefrLevel:  filter.CEFRLevel,
		Status:     filter.Status,
		ExamPartID: filter.ExamPartID,
		NodeCode:   filter.NodeCode,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count questions: %w", err)
	}

	rows, err := r.queries.ListQuestions(ctx, sqlc.ListQuestionsParams{
		Limit:      int32(limit),
		Offset:     int32(offset),
		Kind:       filter.Kind,
		Skill:      filter.Skill,
		CefrLevel:  filter.CEFRLevel,
		Status:     filter.Status,
		ExamPartID: filter.ExamPartID,
		NodeCode:   filter.NodeCode,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}

	items := make([]*domain.Question, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainQuestion(row))
	}

	return items, int(total), nil
}

// SamplePublishedQuestions draws N random published questions matching criteria.
func (r *Repository) SamplePublishedQuestions(
	ctx context.Context, criteria contract.SampleCriteria,
) ([]*domain.Question, error) {
	limit := criteria.Limit
	if limit <= 0 {
		limit = 10
	}

	rows, err := r.queries.SamplePublishedQuestions(ctx, sqlc.SamplePublishedQuestionsParams{
		Limit:      int32(limit), //nolint:gosec // bounded limit
		Kind:       criteria.Kind,
		Skill:      criteria.Skill,
		CefrLevel:  criteria.CEFRLevel,
		ExamPartID: criteria.ExamPartID,
	})
	if err != nil {
		return nil, fmt.Errorf("sample questions: %w", err)
	}

	items := make([]*domain.Question, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainQuestion(row))
	}
	return items, nil
}

func numericToFloat64(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

// GetQuestionStats returns empirical statistics for a question.
func (r *Repository) GetQuestionStats(ctx context.Context, questionID uuid.UUID) (*domain.QuestionStats, error) {
	row, err := r.queries.GetQuestionStats(ctx, questionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrQuestionNotFound
		}
		return nil, err
	}

	return &domain.QuestionStats{
		QuestionID:     row.QuestionID,
		Attempts:       int(row.Attempts),
		PValue:         numericToFloat64(row.PValue),
		Discrimination: numericToFloat64(row.Discrimination),
		AvgTimeMs:      int(row.AvgTimeMs),
		LastComputedAt: row.LastComputedAt,
	}, nil
}

// UpsertQuestionStats writes empirical statistics for a question.
func (r *Repository) UpsertQuestionStats(
	ctx context.Context, stats *domain.QuestionStats,
) (*domain.QuestionStats, error) {
	pValNum := pgtype.Numeric{}
	if stats.PValue != nil {
		_ = pValNum.Scan(strconv.FormatFloat(*stats.PValue, 'f', 3, 64))
	}
	discNum := pgtype.Numeric{}
	if stats.Discrimination != nil {
		_ = discNum.Scan(strconv.FormatFloat(*stats.Discrimination, 'f', 3, 64))
	}

	row, err := r.queries.UpsertQuestionStats(ctx, sqlc.UpsertQuestionStatsParams{
		QuestionID:     stats.QuestionID,
		Attempts:       int32(stats.Attempts), //nolint:gosec // attempt count
		PValue:         pValNum,
		Discrimination: discNum,
		AvgTimeMs:      int32(stats.AvgTimeMs), //nolint:gosec // avg duration
		LastComputedAt: stats.LastComputedAt,
	})
	if err != nil {
		return nil, err
	}

	return &domain.QuestionStats{
		QuestionID:     row.QuestionID,
		Attempts:       int(row.Attempts),
		PValue:         numericToFloat64(row.PValue),
		Discrimination: numericToFloat64(row.Discrimination),
		AvgTimeMs:      int(row.AvgTimeMs),
		LastComputedAt: row.LastComputedAt,
	}, nil
}

func toDomainQuestion(row sqlc.AssessQuestion) *domain.Question {
	var prov map[string]any
	if len(row.Provenance) > 0 {
		_ = json.Unmarshal(row.Provenance, &prov)
	}
	if prov == nil {
		prov = make(map[string]any)
	}

	return &domain.Question{
		ID:            row.ID,
		ContentItemID: row.ContentItemID,
		ActivityID:    row.ActivityID,
		ExamPartID:    row.ExamPartID,
		Kind:          row.Kind,
		Skill:         row.Skill,
		CEFRLevel:     row.CefrLevel,
		Difficulty:    numericToFloat64(row.Difficulty),
		QuestionCount: int(row.QuestionCount),
		Fingerprint:   row.Fingerprint,
		Provenance:    prov,
		Status:        row.Status,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}
