package ai

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UsageRecorder persists AI requests and aggregate daily usage metrics.
type UsageRecorder interface {
	Record(ctx context.Context, entry RequestLog) error
}

// RequestStatus is how one AI request ended.
//
// A named type with constants rather than a string with the permitted values
// written in a trailing comment. `ai.ai_requests` carries
// `CHECK (status IN ('success', 'failed', 'cached', 'rate_limited'))`, so a
// caller that reaches for the obvious "ok" or "error" does not get a compile
// error or a validation message -- it gets a constraint violation from inside a
// background job, and the usage record it was trying to write is simply lost.
// Usage data that goes missing when something goes wrong is worst exactly when
// it is needed.
type RequestStatus string

// The four values ck_ai_requests_status accepts.
// TestRequestStatus_MatchesTheCheckConstraint holds them to the migration.
const (
	StatusSuccess     RequestStatus = "success"
	StatusFailed      RequestStatus = "failed"
	StatusCached      RequestStatus = "cached"
	StatusRateLimited RequestStatus = "rate_limited"
)

// RequestLog represents one completed AI request execution.
type RequestLog struct {
	UserID           *uuid.UUID
	Task             Task
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMs        int
	Status           RequestStatus
	ErrorMessage     string
	CreatedAt        time.Time
}

// DBUsageRecorder writes request logs and aggregate usage into PostgreSQL.
type DBUsageRecorder struct {
	pool *pgxpool.Pool
}

// NewDBUsageRecorder creates a new database usage recorder.
func NewDBUsageRecorder(pool *pgxpool.Pool) *DBUsageRecorder {
	return &DBUsageRecorder{pool: pool}
}

// Record inserts a request audit row and updates daily usage aggregates.
func (r *DBUsageRecorder) Record(ctx context.Context, entry RequestLog) error {
	if r.pool == nil {
		return nil
	}

	const insertRequest = `
		INSERT INTO ai.ai_requests (
			user_id, task, provider, model, prompt_tokens,
			completion_tokens, latency_ms, status, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, insertRequest,
		entry.UserID,
		string(entry.Task),
		entry.Provider,
		entry.Model,
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.LatencyMs,
		string(entry.Status),
		entry.ErrorMessage,
		entry.CreatedAt,
	)
	if err != nil {
		return err
	}

	const upsertUsage = `
		INSERT INTO ai.ai_usage (
			provider, model, task, usage_date, request_count,
			total_prompt_tokens, total_completion_tokens, updated_at
		) VALUES ($1, $2, $3, CURRENT_DATE, 1, $4, $5, now())
		ON CONFLICT (provider, model, task, usage_date) DO UPDATE
		SET request_count = ai.ai_usage.request_count + 1,
		    total_prompt_tokens = ai.ai_usage.total_prompt_tokens + EXCLUDED.total_prompt_tokens,
		    total_completion_tokens = ai.ai_usage.total_completion_tokens + EXCLUDED.total_completion_tokens,
		    updated_at = now()`

	_, err = r.pool.Exec(ctx, upsertUsage,
		entry.Provider,
		entry.Model,
		string(entry.Task),
		entry.PromptTokens,
		entry.CompletionTokens,
	)
	return err
}

// NoopUsageRecorder discards logs (used in tests and offline development).
type NoopUsageRecorder struct{}

// Record does nothing.
func (NoopUsageRecorder) Record(_ context.Context, _ RequestLog) error {
	return nil
}
