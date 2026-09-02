-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS ai;

-- ------------------------------------------------------------- ai_requests
-- Every individual LLM invocation, logged with token counts, latency, and status.
CREATE TABLE IF NOT EXISTS ai.ai_requests (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid,
    task              text        NOT NULL,
    provider          text        NOT NULL,
    model             text        NOT NULL,
    prompt_tokens     integer     NOT NULL DEFAULT 0,
    completion_tokens integer     NOT NULL DEFAULT 0,
    latency_ms        integer     NOT NULL DEFAULT 0,
    status            text        NOT NULL DEFAULT 'success',
    error_message     text,
    created_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_ai_requests_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE SET NULL,
    CONSTRAINT ck_ai_requests_status CHECK (status IN ('success', 'failed', 'cached', 'rate_limited'))
);

CREATE INDEX IF NOT EXISTS idx_ai_requests_task_created ON ai.ai_requests (task, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_requests_user_created ON ai.ai_requests (user_id, created_at DESC) WHERE user_id IS NOT NULL;

-- ---------------------------------------------------------------- ai_usage
-- Daily aggregated usage per provider and model for quota tracking and budgeting.
CREATE TABLE IF NOT EXISTS ai.ai_usage (
    id                      uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    provider                text        NOT NULL,
    model                   text        NOT NULL,
    task                    text        NOT NULL,
    usage_date              date        NOT NULL DEFAULT CURRENT_DATE,
    request_count           integer     NOT NULL DEFAULT 0,
    total_prompt_tokens     bigint      NOT NULL DEFAULT 0,
    total_completion_tokens bigint      NOT NULL DEFAULT 0,
    updated_at              timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_ai_usage_aggregate UNIQUE (provider, model, task, usage_date)
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_date ON ai.ai_usage (usage_date DESC);

-- -------------------------------------------------------- ai_cache_entries
-- Exact-hash response cache to prevent redundant LLM invocations.
CREATE TABLE IF NOT EXISTS ai.ai_cache_entries (
    cache_key     text        PRIMARY KEY,
    task          text        NOT NULL,
    model         text        NOT NULL,
    response_text text        NOT NULL,
    expires_at    timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_cache_expires ON ai.ai_cache_entries (expires_at);

-- -------------------------------------------------------------- ai_budgets
-- Task-level daily request volume and rate ceiling caps.
CREATE TABLE IF NOT EXISTS ai.ai_budgets (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    task                 text        NOT NULL UNIQUE,
    daily_request_limit  integer     NOT NULL DEFAULT 1000,
    daily_token_limit    bigint      NOT NULL DEFAULT 1000000,
    is_active            boolean     NOT NULL DEFAULT true,
    updated_at           timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ai.ai_budgets;
DROP TABLE IF EXISTS ai.ai_cache_entries;
DROP TABLE IF EXISTS ai.ai_usage;
DROP TABLE IF EXISTS ai.ai_requests;
DROP SCHEMA IF EXISTS ai CASCADE;
-- +goose StatementEnd
