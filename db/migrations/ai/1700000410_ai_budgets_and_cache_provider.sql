-- +goose Up
-- +goose StatementBegin

-- 1. Add provider to ai_cache_entries so cached entries know who produced them
ALTER TABLE ai.ai_cache_entries ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '';

-- 2. Add provider dimension to ai_budgets
ALTER TABLE ai.ai_budgets DROP CONSTRAINT IF EXISTS ai_budgets_task_key;
ALTER TABLE ai.ai_budgets ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '';
ALTER TABLE ai.ai_budgets ADD CONSTRAINT uq_ai_budgets_provider_task UNIQUE (provider, task);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ai.ai_budgets DROP CONSTRAINT IF EXISTS uq_ai_budgets_provider_task;
ALTER TABLE ai.ai_budgets DROP COLUMN IF EXISTS provider;
ALTER TABLE ai.ai_budgets ADD CONSTRAINT ai_budgets_task_key UNIQUE (task);

ALTER TABLE ai.ai_cache_entries DROP COLUMN IF EXISTS provider;
-- +goose StatementEnd
