-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS core.user_deletions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES core.users(id),
    status        text NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    requested_at  timestamptz NOT NULL DEFAULT now(),
    execute_at    timestamptz NOT NULL,
    started_at    timestamptz,
    completed_at  timestamptz,
    cancelled_at  timestamptz,
    error_message text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_deletions_active ON core.user_deletions(user_id)
    WHERE status IN ('pending', 'processing');

CREATE INDEX IF NOT EXISTS idx_user_deletions_due ON core.user_deletions(execute_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_user_deletions_user_id ON core.user_deletions(user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON core.user_deletions TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.user_deletions;
-- +goose StatementEnd
