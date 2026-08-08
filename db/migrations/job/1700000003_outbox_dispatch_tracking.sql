-- +goose Up
-- +goose StatementBegin
ALTER TABLE ops.outbox_events
    ADD COLUMN IF NOT EXISTS event_id         uuid        NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS dead_lettered_at timestamptz,
    ADD COLUMN IF NOT EXISTS last_error       text;

-- Consumers deduplicate on event_id, so it must be unique even though the
-- table's own primary key is not what crosses the module boundary.
CREATE UNIQUE INDEX IF NOT EXISTS ux_outbox_events_event_id
    ON ops.outbox_events (event_id);

-- The publisher polls by "due and not finished"; the old index did not know
-- about backoff or dead-lettering.
DROP INDEX IF EXISTS ops.idx_outbox_events_unpublished;
CREATE INDEX IF NOT EXISTS idx_outbox_events_due
    ON ops.outbox_events (next_attempt_at, created_at)
    WHERE published_at IS NULL AND dead_lettered_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ops.idx_outbox_events_due;
CREATE INDEX IF NOT EXISTS idx_outbox_events_unpublished
    ON ops.outbox_events (created_at)
    WHERE published_at IS NULL;

DROP INDEX IF EXISTS ops.ux_outbox_events_event_id;

ALTER TABLE ops.outbox_events
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS dead_lettered_at,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS event_id;
-- +goose StatementEnd
