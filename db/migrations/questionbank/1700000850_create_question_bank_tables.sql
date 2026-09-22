-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS assess;

CREATE TABLE IF NOT EXISTS assess.questions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    content_item_id    uuid NOT NULL UNIQUE REFERENCES content.content_items (id) ON DELETE RESTRICT,
    activity_id        uuid UNIQUE REFERENCES learn.activities (id) ON DELETE SET NULL,
    exam_part_id       uuid,
    kind               text NOT NULL,
    skill              text NOT NULL,
    cefr_level         text NOT NULL,
    difficulty         numeric(4,3),
    question_count     integer NOT NULL DEFAULT 1,
    fingerprint        text NOT NULL,
    provenance         jsonb NOT NULL,
    status             text NOT NULL DEFAULT 'draft',
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_questions_fingerprint UNIQUE (fingerprint),
    CONSTRAINT ck_questions_status CHECK (status IN ('draft', 'in_review', 'published', 'retired')),
    CONSTRAINT ck_questions_count CHECK (question_count BETWEEN 1 AND 20)
);

CREATE INDEX IF NOT EXISTS idx_questions_kind_skill_cefr
    ON assess.questions (kind, skill, cefr_level);

CREATE INDEX IF NOT EXISTS idx_questions_status
    ON assess.questions (status);

CREATE TABLE IF NOT EXISTS assess.question_stats (
    question_id        uuid PRIMARY KEY REFERENCES assess.questions (id) ON DELETE CASCADE,
    attempts           integer NOT NULL DEFAULT 0,
    p_value            numeric(4,3),
    discrimination     numeric(4,3),
    avg_time_ms        integer NOT NULL DEFAULT 0,
    last_computed_at   timestamptz NOT NULL DEFAULT now()
);

GRANT USAGE ON SCHEMA assess TO fluentra_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON assess.questions, assess.question_stats TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS assess.question_stats;
DROP TABLE IF EXISTS assess.questions;
-- +goose StatementEnd
