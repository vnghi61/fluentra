-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS resource.extractions (
    resource_id   uuid PRIMARY KEY REFERENCES resource.resources (id) ON DELETE CASCADE,
    source        text NOT NULL,          -- 'pdf_text' | 'ocr' | 'transcript'
    text          text NOT NULL,
    char_count    integer NOT NULL,
    truncated     boolean NOT NULL DEFAULT false,
    language      text NOT NULL DEFAULT '',
    tool_version  text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_extractions_source CHECK (source IN ('pdf_text', 'ocr', 'transcript')),
    CONSTRAINT ck_extractions_size CHECK (char_count BETWEEN 0 AND 400000)
);

CREATE TABLE IF NOT EXISTS resource.classifications (
    resource_id    uuid PRIMARY KEY REFERENCES resource.resources (id) ON DELETE CASCADE,
    cefr_estimate  text,                  -- same enum as content_versions.cefr_level
    skill          text,
    node_codes     text[] NOT NULL DEFAULT '{}',
    prompt_version text NOT NULL,
    model          text NOT NULL,
    ai_request_id  uuid,                  -- ai.ai_requests.id, for traceability (brief §2)
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_classifications_cefr CHECK (cefr_estimate IS NULL OR cefr_estimate IN ('A1', 'A2', 'B1', 'B2', 'C1', 'C2'))
);

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA resource TO fluentra_app;

INSERT INTO ai.ai_budgets (provider, task, daily_request_limit, daily_token_limit, is_active)
SELECT
    provider,
    'resource_classify',
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
    ]) AS provider
ON CONFLICT (provider, task) DO UPDATE
SET daily_request_limit = EXCLUDED.daily_request_limit,
    daily_token_limit   = EXCLUDED.daily_token_limit,
    is_active           = EXCLUDED.is_active,
    updated_at          = now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM ai.ai_budgets WHERE task = 'resource_classify';
DROP TABLE IF EXISTS resource.classifications;
DROP TABLE IF EXISTS resource.extractions;
-- +goose StatementEnd
