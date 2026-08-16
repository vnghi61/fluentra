-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS core.user_exports (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    status        text        NOT NULL CHECK (status IN (
        'pending',
        'processing',
        'completed',
        'failed'
    )),
    object_key    text,
    requested_at  timestamptz NOT NULL DEFAULT now(),
    started_at    timestamptz,
    completed_at  timestamptz,
    expires_at    timestamptz,
    error_message text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_exports_user_status ON core.user_exports (user_id, status);
CREATE INDEX IF NOT EXISTS idx_user_exports_expires ON core.user_exports (expires_at) WHERE expires_at IS NOT NULL;

COMMENT ON TABLE core.user_exports IS 'Tracks data export requests and their artifacts';

GRANT SELECT, INSERT, UPDATE, DELETE ON core.user_exports TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.user_exports;
-- +goose StatementEnd
