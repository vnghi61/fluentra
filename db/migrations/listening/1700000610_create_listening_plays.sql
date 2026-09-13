-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS skill.listening_plays (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    content_version_id  uuid NOT NULL REFERENCES content.content_versions(id),
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
