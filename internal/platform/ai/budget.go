package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrQuotaExhausted is returned when every available provider has exhausted its budget.
var ErrQuotaExhausted = errors.New("ai: quota exhausted across all providers")

// BudgetChecker inspects daily provider and task usage against configured ceilings.
type BudgetChecker interface {
	CheckQuota(ctx context.Context, provider string, task Task) (bool, error)
}

// DBBudgetChecker queries ai.ai_budgets and ai.ai_usage in PostgreSQL.
type DBBudgetChecker struct {
	pool *pgxpool.Pool
}

// NewDBBudgetChecker builds a budget checker over a pgx pool.
func NewDBBudgetChecker(pool *pgxpool.Pool) *DBBudgetChecker {
	return &DBBudgetChecker{pool: pool}
}

// CheckQuota returns true if the provider has remaining request and token quota for the task today.
// If no active budget is configured for (provider, task), quota is considered available (true).
// If checking encounters an error, it logs the error and fails closed (returns false and error)
// so that unbudgeted spending is never silently permitted.
func (b *DBBudgetChecker) CheckQuota(ctx context.Context, provider string, task Task) (bool, error) {
	if b.pool == nil {
		return true, nil
	}

	const query = `
		SELECT
			b.daily_request_limit,
			b.daily_token_limit,
			b.is_active,
			COALESCE(u.request_count, 0) AS used_requests,
			COALESCE(u.total_prompt_tokens + u.total_completion_tokens, 0) AS used_tokens
		FROM ai.ai_budgets b
		LEFT JOIN (
			SELECT provider, task,
			       SUM(request_count) AS request_count,
			       SUM(total_prompt_tokens) AS total_prompt_tokens,
			       SUM(total_completion_tokens) AS total_completion_tokens
			FROM ai.ai_usage
			WHERE usage_date = CURRENT_DATE
			GROUP BY provider, task
		) u ON u.provider = b.provider AND u.task = b.task
		WHERE b.provider = $1 AND b.task = $2`

	var reqLimit int
	var tokenLimit int64
	var isActive bool
	var usedReqs int
	var usedTokens int64

	err := b.pool.QueryRow(ctx, query, provider, string(task)).Scan(
		&reqLimit, &tokenLimit, &isActive, &usedReqs, &usedTokens,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No budget row configured for this provider+task; permitted.
			return true, nil
		}
		// A check that errors must not silently allow (which spends money nobody authorized)
		// and must not silently deny either (which strands a learner for a reason they cannot see).
		// Logged with structured alert; fails closed so caller routes to fallback/queue.
		slog.ErrorContext(ctx, "ai: budget check query failed; failing closed to prevent unauthorized spend",
			"provider", provider, "task", string(task), "error", err)
		return false, fmt.Errorf("ai: budget check failed: %w", err)
	}

	if !isActive {
		return true, nil
	}

	if (reqLimit > 0 && usedReqs >= reqLimit) || (tokenLimit > 0 && usedTokens >= tokenLimit) {
		slog.WarnContext(ctx, "ai: provider daily budget limit reached for task",
			"provider", provider,
			"task", string(task),
			"used_requests", usedReqs,
			"request_limit", reqLimit,
			"used_tokens", usedTokens,
			"token_limit", tokenLimit,
		)
		return false, nil
	}

	return true, nil
}

// UsageStatus represents current daily usage against configured limits for a provider and task.
type UsageStatus struct {
	Provider          string
	Task              string
	RequestsToday     int64
	TokensToday       int64
	DailyRequestLimit *int
	DailyTokenLimit   *int64
	IsExhausted       bool
}

// UsageReporter provides a view of current AI consumption across providers and tasks.
type UsageReporter interface {
	GetUsageOverview(ctx context.Context) ([]UsageStatus, error)
}

// GetUsageOverview queries ai.ai_budgets and ai.ai_usage to summarize today's consumption.
func (b *DBBudgetChecker) GetUsageOverview(ctx context.Context) ([]UsageStatus, error) {
	if b.pool == nil {
		return []UsageStatus{}, nil
	}

	const query = `
		SELECT
			COALESCE(NULLIF(b.provider, ''), NULLIF(u.provider, ''), 'unknown') AS provider,
			COALESCE(b.task, u.task) AS task,
			COALESCE(u.requests_today, 0)::bigint AS requests_today,
			COALESCE(u.tokens_today, 0)::bigint AS tokens_today,
			b.daily_request_limit,
			b.daily_token_limit,
			CASE 
				WHEN b.daily_request_limit IS NOT NULL AND b.daily_request_limit > 0 
				     AND COALESCE(u.requests_today, 0) >= b.daily_request_limit THEN true
				WHEN b.daily_token_limit IS NOT NULL AND b.daily_token_limit > 0 
				     AND COALESCE(u.tokens_today, 0) >= b.daily_token_limit THEN true
				ELSE false
			END AS is_exhausted
		FROM ai.ai_budgets b
		FULL OUTER JOIN (
			SELECT
				provider,
				task,
				SUM(request_count)::bigint AS requests_today,
				SUM(total_prompt_tokens + total_completion_tokens)::bigint AS tokens_today
			FROM ai.ai_usage
			WHERE usage_date = CURRENT_DATE
			GROUP BY provider, task
		) u ON b.provider = u.provider AND b.task = u.task
		WHERE b.is_active IS NULL OR b.is_active = true
		ORDER BY provider, task`

	rows, err := b.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query ai usage overview: %w", err)
	}
	defer rows.Close()

	var result []UsageStatus
	for rows.Next() {
		var item UsageStatus
		if err := rows.Scan(
			&item.Provider,
			&item.Task,
			&item.RequestsToday,
			&item.TokensToday,
			&item.DailyRequestLimit,
			&item.DailyTokenLimit,
			&item.IsExhausted,
		); err != nil {
			return nil, fmt.Errorf("scan ai usage row: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai usage rows: %w", err)
	}
	if result == nil {
		result = []UsageStatus{}
	}
	return result, nil
}

// NoopBudgetChecker always permits requests (offline development and unit tests).
type NoopBudgetChecker struct{}

// CheckQuota always returns true.
func (NoopBudgetChecker) CheckQuota(_ context.Context, _ string, _ Task) (bool, error) {
	return true, nil
}

// GetUsageOverview returns empty usage for testing.
func (NoopBudgetChecker) GetUsageOverview(_ context.Context) ([]UsageStatus, error) {
	return []UsageStatus{}, nil
}
