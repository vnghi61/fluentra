// Package repository manages database operations for the speaking module.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/speaking/sqlc"
	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Repository wraps generated sqlc queries for speaking_feedback.
type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// New constructs a new speaking repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// InsertFeedback writes a graded speaking result.
func (r *Repository) InsertFeedback(ctx context.Context, fb *contract.SpeakingFeedback) error {
	criteriaJSON, err := json.Marshal(fb.Criteria)
	if err != nil {
		return fmt.Errorf("marshal speaking criteria: %w", err)
	}

	var accuracy pgtype.Numeric
	if fb.ReadAloudAccuracy != nil {
		accStr := fmt.Sprintf("%.2f", *fb.ReadAloudAccuracy)
		var num big.Int
		if _, ok := num.SetString(accStr, 10); ok {
			_ = accuracy.Scan(accStr)
		} else {
			_ = accuracy.Scan(accStr)
		}
	}

	var wpm *int32
	if fb.WordsPerMinute != nil {
		v := int32(*fb.WordsPerMinute)
		wpm = &v
	}

	id := fb.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC()
	if fb.CreatedAt.IsZero() {
		fb.CreatedAt = now
	}
	if fb.UpdatedAt.IsZero() {
		fb.UpdatedAt = now
	}

	_, err = r.queries.InsertSpeakingFeedback(ctx, sqlc.InsertSpeakingFeedbackParams{
		ID:                 id,
		AttemptID:          fb.AttemptID,
		UserID:             fb.UserID,
		RecordingKey:       fb.RecordingKey,
		RecordingDeletedAt: fb.RecordingDeletedAt,
		Transcript:         fb.Transcript,
		Criteria:           criteriaJSON,
		ReadAloudAccuracy:  accuracy,
		WordsPerMinute:     wpm,
		FeedbackEn:         fb.FeedbackEn,
		FeedbackVi:         fb.FeedbackVi,
		PromptVersion:      fb.PromptVersion,
		Model:              fb.Model,
		AsrModel:           fb.ASRModel,
		CreatedAt:          fb.CreatedAt,
		UpdatedAt:          fb.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("insert speaking feedback: %w", err)
	}
	fb.ID = id
	return nil
}

// GetFeedbackByAttemptID retrieves speaking feedback for a given attempt.
func (r *Repository) GetFeedbackByAttemptID(ctx context.Context, attemptID uuid.UUID) (*contract.SpeakingFeedback, error) {
	row, err := r.queries.GetSpeakingFeedbackByAttemptID(ctx, attemptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrFeedbackNotFound
		}
		return nil, fmt.Errorf("get speaking feedback: %w", err)
	}
	return toContractFeedback(row)
}

// GetFeedbackForUser retrieves speaking feedback verifying user ownership.
func (r *Repository) GetFeedbackForUser(ctx context.Context, attemptID, userID uuid.UUID) (*contract.SpeakingFeedback, error) {
	row, err := r.queries.GetSpeakingFeedbackForUser(ctx, sqlc.GetSpeakingFeedbackForUserParams{
		AttemptID: attemptID,
		UserID:    userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrFeedbackNotFound
		}
		return nil, fmt.Errorf("get speaking feedback for user: %w", err)
	}
	return toContractFeedback(row)
}

// MarkRecordingDeleted marks a recording as deleted and returns the recording key to purge from storage.
func (r *Repository) MarkRecordingDeleted(ctx context.Context, attemptID, userID uuid.UUID) (string, error) {
	key, err := r.queries.MarkRecordingDeleted(ctx, sqlc.MarkRecordingDeletedParams{
		AttemptID: attemptID,
		UserID:    userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.New(apperr.NotFound, "RECORDING_ALREADY_DELETED", "recording already deleted or not found")
		}
		return "", fmt.Errorf("mark recording deleted: %w", err)
	}
	return key, nil
}

// ListRecordingsOlderThan returns recordings created before cutoff that have not yet been purged.
func (r *Repository) ListRecordingsOlderThan(ctx context.Context, cutoff time.Time, limit int32) ([]sqlc.ListRecordingsOlderThanRow, error) {
	rows, err := r.queries.ListRecordingsOlderThan(ctx, sqlc.ListRecordingsOlderThanParams{
		CreatedAt: cutoff,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list expired recordings: %w", err)
	}
	return rows, nil
}

// MarkRecordingsDeletedBatch updates multiple feedback records as purged.
func (r *Repository) MarkRecordingsDeletedBatch(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.queries.MarkRecordingsDeletedBatch(ctx, ids); err != nil {
		return fmt.Errorf("mark recordings deleted batch: %w", err)
	}
	return nil
}

// ListUserRecordingKeys returns all active recording keys belonging to a user.
func (r *Repository) ListUserRecordingKeys(ctx context.Context, userID uuid.UUID) ([]string, error) {
	keys, err := r.queries.ListUserRecordingKeys(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user recording keys: %w", err)
	}
	return keys, nil
}

func toContractFeedback(row sqlc.SkillSpeakingFeedback) (*contract.SpeakingFeedback, error) {
	var criteria []contract.SpeakingCriterion
	if len(row.Criteria) > 0 {
		if err := json.Unmarshal(row.Criteria, &criteria); err != nil {
			return nil, fmt.Errorf("unmarshal speaking criteria: %w", err)
		}
	}

	var accuracy *float64
	if row.ReadAloudAccuracy.Valid {
		v, _ := row.ReadAloudAccuracy.Float64Value()
		if v.Valid {
			accuracy = &v.Float64
		}
	}

	var wpm *int
	if row.WordsPerMinute != nil {
		v := int(*row.WordsPerMinute)
		wpm = &v
	}

	return &contract.SpeakingFeedback{
		ID:                 row.ID,
		AttemptID:          row.AttemptID,
		UserID:             row.UserID,
		RecordingKey:       row.RecordingKey,
		RecordingDeletedAt: row.RecordingDeletedAt,
		Transcript:         row.Transcript,
		Criteria:           criteria,
		ReadAloudAccuracy:  accuracy,
		WordsPerMinute:     wpm,
		FeedbackEn:         row.FeedbackEn,
		FeedbackVi:         row.FeedbackVi,
		PromptVersion:      row.PromptVersion,
		Model:              row.Model,
		ASRModel:           row.AsrModel,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}
