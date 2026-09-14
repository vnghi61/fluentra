-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS skill.listening_plays (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    -- No key to content.content_versions: DB4 allows `skill` to leave its schema only for
    -- core.users. The service resolves the version through the content contract first.
    content_version_id  uuid NOT NULL,
    context_type        varchar(32) NOT NULL,
    context_id          uuid NOT NULL,
    played_at           timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_listening_plays_context ON skill.listening_plays (user_id, content_version_id, context_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS skill.listening_plays;

-- +goose StatementEnd
