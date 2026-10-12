-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage A.5: a daily random sample of the previous day's auto-published
-- items, so a person spot-checks what the independent verifier let through. A
-- rejection unpublishes the item: the item is archived, content.archived is
-- emitted, and questionbank stops drawing the question.
CREATE TABLE IF NOT EXISTS content.review_samples (
    version_id  uuid PRIMARY KEY REFERENCES content.content_versions (id) ON DELETE CASCADE,
    batch       text NOT NULL DEFAULT '',
    sampled_on  date NOT NULL,
    decision    text CHECK (decision IN ('kept', 'rejected')),
    note        varchar(500),
    decided_by  uuid REFERENCES core.users (id) ON DELETE SET NULL,
    decided_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- The read path is "the samples still waiting for a person".
CREATE INDEX IF NOT EXISTS idx_review_samples_open
    ON content.review_samples (created_at)
    WHERE decision IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS content.review_samples;

-- +goose StatementEnd
