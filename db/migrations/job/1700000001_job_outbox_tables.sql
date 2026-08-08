-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ops.outbox_events (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate    text        NOT NULL,
    event        text        NOT NULL,
    payload      jsonb       NOT NULL,
    attempts     integer     NOT NULL DEFAULT 0,
    published_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_unpublished 
    ON ops.outbox_events (created_at) 
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS ops.job_failures (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       text        NOT NULL,
    args       jsonb       NOT NULL,
    last_error text        NOT NULL,
    failed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_job_failures_failed_at 
    ON ops.job_failures (failed_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON ops.outbox_events, ops.job_failures TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ops.job_failures;
DROP TABLE IF EXISTS ops.outbox_events;
-- +goose StatementEnd
