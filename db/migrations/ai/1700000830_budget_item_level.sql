-- +goose Up
-- +goose StatementBegin

-- 1700000830: seed AI budgets for item_level CEFR check task (Stage E)
INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
SELECT
    provider,
    task,
    500,
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
        'item_level'
    ]) AS task
ON CONFLICT (provider, task) DO UPDATE
SET daily_request_limit = EXCLUDED.daily_request_limit,
    daily_token_limit   = EXCLUDED.daily_token_limit,
    is_active           = EXCLUDED.is_active,
    updated_at          = now();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM ai.ai_budgets
WHERE task = 'item_level';

-- +goose StatementEnd
