-- +goose Up
-- +goose StatementBegin

-- 1700000470 seeded two tasks for three providers, and said the important part
-- in its own comment: "A provider with no row here is permitted WITHOUT LIMIT".
-- Both halves of that list have since gone stale.
--
-- The provider half: the chain configured in AI_PROVIDER_n_NAME now leads with
-- `opencode` and carries `groq-backup`, and neither has ever had a row. The task
-- half is the larger gap — internal/platform/ai/ai.go declares ten tasks, and
-- only `vocab_verify` and `explain_answer` were ever budgeted, so the eight that
-- actually burn the most (generation and solving) ran with no ceiling at all.
--
-- The admin AI usage screen reads this table with a FULL OUTER JOIN against
-- today's usage, so an unbudgeted pair does not merely go unenforced: it appears
-- with "∞" in the limit column, which is an accurate report of a table nobody
-- filled in rather than a statement about the provider's own plan.
--
-- Note on OpenCode specifically: its Go plan meters in dollars ($12/5h, $30/week,
-- $60/month), not in requests or tokens, so no row here can mirror it. These are
-- Fluentra's own ceiling — a runaway guard — and the plan's dollar cap remains
-- the real limit.
INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
SELECT
    provider,
    task,
    -- vocab_verify is the cheap high-frequency one and keeps its 1000; every
    -- other task keeps explain_answer's 500, which is the conservative of the
    -- two numbers 1700000470 chose.
    CASE WHEN task = 'vocab_verify' THEN 1000 ELSE 500 END,
    1000000,
    true
FROM
    unnest(ARRAY[
        'opencode',
        'groq',
        'groq-backup',
        'cerebras',
        'mistral'
    ]) AS provider,
    unnest(ARRAY[
        'vocab_verify',
        'explain_answer',
        'vocab_enrich_examples',
        'writing_grade',
        'speaking_grade',
        'practice_generate',
        'practice_solve',
        'listening_generate',
        'placement_generate',
        'placement_solve'
    ]) AS task
ON CONFLICT (provider, task) DO UPDATE
SET daily_request_limit = EXCLUDED.daily_request_limit,
    daily_token_limit   = EXCLUDED.daily_token_limit,
    is_active           = EXCLUDED.is_active,
    updated_at          = now();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Returns the table to what 1700000470 left: three providers, two tasks.
DELETE FROM ai.ai_budgets
WHERE provider IN ('opencode', 'groq-backup')
   OR task NOT IN ('vocab_verify', 'explain_answer');
-- +goose StatementEnd
