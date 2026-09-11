// Package repository implements database persistence for writing feedback.
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

	"github.com/fluentra/fluentra/internal/generated/writing/sqlc"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Repository encapsulates database operations for writing feedback.
type Repository interface {
	InsertWritingFeedback(ctx context.Context, fb contract.WritingFeedback) error
	GetWritingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*contract.WritingFeedback, error)
	WithTx(tx pgx.Tx) Repository
}

type pgxRepository struct {
	q *sqlc.Queries
}

// New creates a new PostgreSQL repository for writing feedback.
func New(db sqlc.DBTX) Repository {
	if db == nil {
		return &pgxRepository{q: nil}
	}
	return &pgxRepository{
		q: sqlc.New(db),
	}
}

func (r *pgxRepository) WithTx(tx pgx.Tx) Repository {
	if r.q == nil {
		return r
	}
	return &pgxRepository{
		q: r.q.WithTx(tx),
	}
}

func (r *pgxRepository) InsertWritingFeedback(ctx context.Context, fb contract.WritingFeedback) error {
	if r.q == nil {
		return nil
	}

	criteriaJSON, err := json.Marshal(fb.Criteria)
	if err != nil {
		return fmt.Errorf("marshal writing criteria: %w", err)
	}

	annotations := fb.Annotations
	if annotations == nil {
		annotations = []contract.WritingAnnotation{}
	}
	annotationsJSON, err := json.Marshal(annotations)
	if err != nil {
		return fmt.Errorf("marshal writing annotations: %w", err)
	}

	var overallBand pgtype.Numeric
	if err := overallBand.Scan(strconv.FormatFloat(fb.OverallBand, 'f', 1, 64)); err != nil {
		return fmt.Errorf("encode overall band: %w", err)
	}

	return r.q.InsertWritingFeedback(ctx, sqlc.InsertWritingFeedbackParams{
		AttemptID:     fb.AttemptID,
		UserID:        fb.UserID,
		OverallBand:   overallBand,
		Score:         int32(fb.Score),
		Criteria:      criteriaJSON,
		Annotations:   annotationsJSON,
		FeedbackEn:    fb.FeedbackEn,
		FeedbackVi:    fb.FeedbackVi,
		PromptVersion: fb.PromptVersion,
		Model:         fb.Model,
	})
}

func (r *pgxRepository) GetWritingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*contract.WritingFeedback, error) {
	if r.q == nil {
		return nil, apperr.New(apperr.NotFound, "FEEDBACK_NOT_FOUND", "writing feedback not found")
	}

	row, err := r.q.GetWritingFeedbackByAttemptAndUser(ctx, sqlc.GetWritingFeedbackByAttemptAndUserParams{
		AttemptID: attemptID,
		UserID:    userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.New(apperr.NotFound, "FEEDBACK_NOT_FOUND", "writing feedback not found")
		}
		return nil, fmt.Errorf("get writing feedback: %w", err)
	}

	var criteria []contract.WritingCriterion
	if len(row.Criteria) > 0 {
		if err := json.Unmarshal(row.Criteria, &criteria); err != nil {
			return nil, fmt.Errorf("unmarshal criteria: %w", err)
		}
	}
	if criteria == nil {
		criteria = []contract.WritingCriterion{}
	}

	var annotations []contract.WritingAnnotation
	if len(row.Annotations) > 0 {
		if err := json.Unmarshal(row.Annotations, &annotations); err != nil {
			return nil, fmt.Errorf("unmarshal annotations: %w", err)
		}
	}
	if annotations == nil {
		annotations = []contract.WritingAnnotation{}
	}

	var band float64
	if row.OverallBand.Valid {
		f, _ := row.OverallBand.Float64Value()
		band = f.Float64
	}

	return &contract.WritingFeedback{
		AttemptID:     row.AttemptID,
		UserID:        row.UserID,
		OverallBand:   band,
		Score:         int(row.Score),
		Criteria:      criteria,
		Annotations:   annotations,
		FeedbackEn:    row.FeedbackEn,
		FeedbackVi:    row.FeedbackVi,
		PromptVersion: row.PromptVersion,
		Model:         row.Model,
		CreatedAt:     row.CreatedAt,
	}, nil
}
