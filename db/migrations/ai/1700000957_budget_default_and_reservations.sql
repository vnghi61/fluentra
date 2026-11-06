-- +goose Up
-- +goose StatementBegin

-- Every earlier budget migration lists providers by name, and production's chain
-- now leads with `9router`, which none of them name. A provider with no row was
-- permitted without limit. The `*` rows below are the ceiling for any provider
-- (and any task) that has no row of its own; a named row still wins.
INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
VALUES
    ('*', '*', 500, 1000000, true),
    ('*', 'vocab_verify', 1000, 1000000, true)
ON CONFLICT (provider, task) DO NOTHING;

-- Requests are counted here before the call, in one atomic statement, so
-- concurrent callers cannot all pass a check that ai_usage, written after the
-- call, has not caught up with.
CREATE TABLE IF NOT EXISTS ai.ai_budget_reservations (
    provider    text    NOT NULL,
    task        text    NOT NULL,
    usage_date  date    NOT NULL,
    reserved    integer NOT NULL DEFAULT 0,
    CONSTRAINT pk_ai_budget_reservations PRIMARY KEY (provider, task, usage_date)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ai.ai_budget_reservations;
DELETE FROM ai.ai_budgets WHERE provider = '*';
-- +goose StatementEnd
