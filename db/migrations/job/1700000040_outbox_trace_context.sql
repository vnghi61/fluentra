-- +goose Up
-- +goose StatementBegin

-- The W3C traceparent of the transaction that wrote the event.
--
-- Without it, an event crosses a process boundary and loses its trace: the
-- publisher and every consumer start a fresh one, so the audit row a profile
-- update produces links to the worker that recorded it rather than to the
-- request that caused it. BR-AUDIT-07 promises the second.
--
-- Nullable, because an event written outside a span has none — a seed script, a
-- migration, a test — and refusing those would be refusing to record them.
--
-- The version prefix is 1700000040 rather than 1700000004 because goose applies
-- migrations in version order across every module folder and this repository
-- does not enable out-of-order application. A number below db/migrations/audit's
-- 1700000030 would be skipped on any database that has already run it.
ALTER TABLE ops.outbox_events
    ADD COLUMN IF NOT EXISTS traceparent text;

-- The W3C shape: `<version>-<trace-id>-<parent-id>-<flags>`, all lowercase hex.
-- Checked here because the publisher hands this value straight to the
-- propagator, and a malformed one is silently dropped there — the trace would
-- go missing with nothing to show for it.
ALTER TABLE ops.outbox_events
    ADD CONSTRAINT ck_outbox_events_traceparent CHECK (
        traceparent IS NULL
        OR traceparent ~ '^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$'
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ops.outbox_events
    DROP CONSTRAINT IF EXISTS ck_outbox_events_traceparent;
ALTER TABLE ops.outbox_events
    DROP COLUMN IF EXISTS traceparent;
-- +goose StatementEnd
