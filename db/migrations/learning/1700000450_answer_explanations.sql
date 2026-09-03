-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------- answer_explanations
-- Stores pre-generated and lazily-generated explanations for exercise questions.
-- Keyed by (content_version_id, user_answer) so an explanation is generated on
-- first request and reused indefinitely across all learners (WP17).
CREATE TABLE IF NOT EXISTS learn.answer_explanations (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    content_version_id uuid        NOT NULL,
    user_answer        text        NOT NULL,
    is_correct         boolean     NOT NULL,
    explanation_en     text        NOT NULL,
    explanation_vi     text        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_answer_explanations UNIQUE (content_version_id, user_answer)
);

-- No separate index on content_version_id.
--
-- The unique constraint above is a btree on (content_version_id, user_answer),
-- and the only read this table has -- GetAnswerExplanation -- filters on both
-- columns, so that index already serves it. A second index on the leading
-- column alone would never be chosen and would cost a write on every insert.
-- Add one when a query appears that reads by question alone.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.answer_explanations;
-- +goose StatementEnd
