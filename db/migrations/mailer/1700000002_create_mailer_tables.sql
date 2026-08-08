-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS comm.email_log (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    to_hash             text        NOT NULL,
    template            text        NOT NULL,
    locale              text        NOT NULL,
    status              text        NOT NULL,
    provider_message_id text,
    error               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_email_log_to_hash ON comm.email_log (to_hash);
CREATE INDEX IF NOT EXISTS idx_email_log_status ON comm.email_log (status);
CREATE INDEX IF NOT EXISTS idx_email_log_created_at ON comm.email_log (created_at);

CREATE TABLE IF NOT EXISTS comm.email_suppressions (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email_hash  text        NOT NULL UNIQUE,
    reason      text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_email_suppressions_email_hash ON comm.email_suppressions (email_hash);

GRANT SELECT, INSERT, UPDATE, DELETE ON comm.email_log, comm.email_suppressions TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS comm.email_suppressions;
DROP TABLE IF EXISTS comm.email_log;
-- +goose StatementEnd
