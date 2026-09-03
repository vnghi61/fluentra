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

// NoopBudgetChecker always permits requests (offline development and unit tests).
type NoopBudgetChecker struct{}

// CheckQuota always returns true.
func (NoopBudgetChecker) CheckQuota(_ context.Context, _ string, _ Task) (bool, error) {
	return true, nil
}
