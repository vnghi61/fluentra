-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS core.login_attempts (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid        REFERENCES core.users(id) ON DELETE SET NULL,
    email_hash     bytea       NOT NULL,
    ip_hash        bytea       NOT NULL,
    success        boolean     NOT NULL,
    failure_reason text,
    created_at     timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id ON core.login_attempts(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_login_attempts_email_hash ON core.login_attempts(email_hash, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_hash ON core.login_attempts(ip_hash, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS core.login_attempts;

-- +goose StatementEnd
