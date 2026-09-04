-- +goose Up
-- +goose StatementBegin

-- 1. Discard old usage and budget rows (mock or unconfigured history)
DELETE FROM ai.ai_usage;
DELETE FROM ai.ai_budgets;

-- 2. Seed fresh budget rows per (provider, task) under new names with binding limits
-- Tasks are 'vocab_verify' and 'explain_answer'.
-- Providers: cerebras, groq, gemini, ollama, mock.
INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
VALUES
    -- Cerebras
    ('cerebras', 'vocab_verify', 1000, 1000000, true),
    ('cerebras', 'explain_answer', 500, 1000000, true),

    -- Groq
    ('groq', 'vocab_verify', 1000, 1000000, true),
    ('groq', 'explain_answer', 500, 1000000, true),

    -- Gemini
    ('gemini', 'vocab_verify', 1000, 1000000, true),
    ('gemini', 'explain_answer', 500, 1000000, true),

    -- Ollama
    ('ollama', 'vocab_verify', 5000, 5000000, true),
    ('ollama', 'explain_answer', 2000, 5000000, true),

    -- Mock
    ('mock', 'vocab_verify', 10000, 10000000, true),
    ('mock', 'explain_answer', 10000, 10000000, true)
ON CONFLICT (provider, task) DO UPDATE
SET daily_request_limit = EXCLUDED.daily_request_limit,
    daily_token_limit   = EXCLUDED.daily_token_limit,
    is_active           = EXCLUDED.is_active,
    updated_at          = now();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM ai.ai_budgets WHERE provider IN ('cerebras', 'groq', 'gemini', 'ollama', 'mock');
-- +goose StatementEnd
