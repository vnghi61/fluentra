-- +goose Up
-- +goose StatementBegin

-- 1. Discard old usage and budget rows (mock or unconfigured history)
DELETE FROM ai.ai_usage;
DELETE FROM ai.ai_budgets;

-- 2. Seed fresh budget rows per (provider, task) under the new names.
--
-- A provider with no row here is permitted WITHOUT LIMIT: CheckQuota returns
-- true when it finds no row, which is the right default for a table nobody has
-- filled in and the wrong thing to discover in a bill. So a slot configured in
-- AI_PROVIDER_n_NAME that is absent from this list has no ceiling at all.
-- Adding a provider to the chain means adding its two rows here.
-- Tasks are 'vocab_verify' and 'explain_answer'.
-- Providers: cerebras, groq, mistral, ollama, mock.
INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
VALUES
    -- Cerebras
    ('cerebras', 'vocab_verify', 1000, 1000000, true),
    ('cerebras', 'explain_answer', 500, 1000000, true),

    -- Groq
    ('groq', 'vocab_verify', 1000, 1000000, true),
    ('groq', 'explain_answer', 500, 1000000, true),

    -- Mistral
    ('mistral', 'vocab_verify', 1000, 1000000, true),
    ('mistral', 'explain_answer', 500, 1000000, true)
ON CONFLICT (provider, task) DO UPDATE
SET daily_request_limit = EXCLUDED.daily_request_limit,
    daily_token_limit   = EXCLUDED.daily_token_limit,
    is_active           = EXCLUDED.is_active,
    updated_at          = now();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM ai.ai_budgets WHERE provider IN ('cerebras', 'groq', 'mistral');
-- +goose StatementEnd
