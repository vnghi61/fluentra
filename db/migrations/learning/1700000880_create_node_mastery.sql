-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS learn.node_mastery (
    user_id      uuid         NOT NULL REFERENCES core.users (id),
    node_id      uuid         NOT NULL,
    attempts     integer      NOT NULL DEFAULT 0,
    correct      integer      NOT NULL DEFAULT 0,
    score        numeric(4,3) NOT NULL DEFAULT 0,
    last_seen_at timestamptz,

    PRIMARY KEY (user_id, node_id),
    CONSTRAINT ck_node_mastery_score CHECK (score >= 0 AND score <= 1),
    CONSTRAINT ck_node_mastery_attempts CHECK (attempts >= 0),
    CONSTRAINT ck_node_mastery_correct CHECK (correct >= 0 AND correct <= attempts)
);

CREATE INDEX IF NOT EXISTS idx_node_mastery_user_score
    ON learn.node_mastery (user_id, score ASC);

GRANT SELECT, INSERT, UPDATE, DELETE ON learn.node_mastery TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.node_mastery;
-- +goose StatementEnd
