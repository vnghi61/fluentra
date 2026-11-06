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

// BudgetReserver is a BudgetChecker that can also count a request against the
// ceiling before the call. The router prefers it when the checker offers it.
type BudgetReserver interface {
	Reserve(ctx context.Context, provider string, task Task) (bool, error)
}

// DBBudgetChecker queries ai.ai_budgets and ai.ai_usage in PostgreSQL.
type DBBudgetChecker struct {
	pool *pgxpool.Pool
}

// NewDBBudgetChecker builds a budget checker over a pgx pool.
func NewDBBudgetChecker(pool *pgxpool.Pool) *DBBudgetChecker {
	return &DBBudgetChecker{pool: pool}
}

// budgetQuery reads the ceiling for (provider, task) and today's spend against
// it. A named row wins over a `*` row; the `*` rows (migration 1700000957) are
// what a provider nobody listed is held to. Requests count the larger of the
// atomic reservations and the recorded usage.
const budgetQuery = `
	SELECT
		b.daily_request_limit,
		b.daily_token_limit,
		b.is_active,
		GREATEST(
			COALESCE((SELECT SUM(u.request_count) FROM ai.ai_usage u
			          WHERE u.provider = $1 AND u.task = $2 AND u.usage_date = CURRENT_DATE), 0),
			COALESCE((SELECT r.reserved FROM ai.ai_budget_reservations r
			          WHERE r.provider = $1 AND r.task = $2 AND r.usage_date = CURRENT_DATE), 0)
		)::integer AS used_requests,
		COALESCE((SELECT SUM(u.total_prompt_tokens + u.total_completion_tokens) FROM ai.ai_usage u
		          WHERE u.provider = $1 AND u.task = $2 AND u.usage_date = CURRENT_DATE), 0)::bigint AS used_tokens
	FROM ai.ai_budgets b
	WHERE b.provider IN ($1, '*') AND b.task IN ($2, '*')
	ORDER BY (b.provider = '*'), (b.task = '*')
	LIMIT 1`

// reserveQuery counts one request against today's ceiling, or returns no row
// when the ceiling is reached. One statement, so concurrent callers serialise
// on the row instead of all passing a check made before any of them recorded.
const reserveQuery = `
	INSERT INTO ai.ai_budget_reservations AS r (provider, task, usage_date, reserved)
	VALUES ($1, $2, CURRENT_DATE, 1)
	ON CONFLICT (provider, task, usage_date) DO UPDATE SET reserved = r.reserved + 1
	WHERE r.reserved < $3
	RETURNING r.reserved`

type budgetRow struct {
	reqLimit   int
	tokenLimit int64
	usedReqs   int
	usedTokens int64
}

// readBudget returns the applicable budget, or ok=false when none applies.
func (b *DBBudgetChecker) readBudget(ctx context.Context, provider string, task Task) (budgetRow, bool, error) {
	var row budgetRow
	var isActive bool
	err := b.pool.QueryRow(ctx, budgetQuery, provider, string(task)).Scan(
		&row.reqLimit, &row.tokenLimit, &isActive, &row.usedReqs, &row.usedTokens,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return budgetRow{}, false, nil
	}
	if err != nil {
		// A check that errors must not silently allow (which spends money nobody authorized)
		// and must not silently deny either (which strands a learner for a reason they cannot see).
		slog.ErrorContext(ctx, "ai: budget check query failed; failing closed to prevent unauthorized spend",
			"provider", provider, "task", string(task), "error", err)
		return budgetRow{}, false, fmt.Errorf("ai: budget check failed: %w", err)
	}
	return row, isActive, nil
}

func (row budgetRow) exhausted() bool {
	return (row.reqLimit > 0 && row.usedReqs >= row.reqLimit) ||
		(row.tokenLimit > 0 && row.usedTokens >= row.tokenLimit)
}

func logBudgetReached(ctx context.Context, provider string, task Task, row budgetRow) {
	slog.WarnContext(ctx, "ai: provider daily budget limit reached for task",
		"provider", provider,
		"task", string(task),
		"used_requests", row.usedReqs,
		"request_limit", row.reqLimit,
		"used_tokens", row.usedTokens,
		"token_limit", row.tokenLimit,
	)
}

// CheckQuota reports, without spending anything, whether the provider has
// request and token quota left for the task today. No applicable budget row is
// permitted; a failed check fails closed (false and the error).
func (b *DBBudgetChecker) CheckQuota(ctx context.Context, provider string, task Task) (bool, error) {
	if b.pool == nil {
		return true, nil
	}
	row, applies, err := b.readBudget(ctx, provider, task)
	if err != nil {
		return false, err
	}
	if !applies {
		return true, nil
	}
	if row.exhausted() {
		logBudgetReached(ctx, provider, task, row)
		return false, nil
	}
	return true, nil
}

// Reserve is CheckQuota that also counts this request against the ceiling,
// atomically, before the provider is called.
func (b *DBBudgetChecker) Reserve(ctx context.Context, provider string, task Task) (bool, error) {
	if b.pool == nil {
		return true, nil
	}
	row, applies, err := b.readBudget(ctx, provider, task)
	if err != nil {
		return false, err
	}
	if !applies {
		return true, nil
	}
	if row.exhausted() {
		logBudgetReached(ctx, provider, task, row)
		return false, nil
	}
	if row.reqLimit <= 0 {
		return true, nil
	}
	var reserved int
	err = b.pool.QueryRow(ctx, reserveQuery, provider, string(task), row.reqLimit).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		row.usedReqs = row.reqLimit
		logBudgetReached(ctx, provider, task, row)
		return false, nil
	}
	if err != nil {
		slog.ErrorContext(ctx, "ai: budget reservation failed; failing closed to prevent unauthorized spend",
			"provider", provider, "task", string(task), "error", err)
		return false, fmt.Errorf("ai: budget reservation failed: %w", err)
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
