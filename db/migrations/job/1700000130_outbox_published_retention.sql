-- +goose NO TRANSACTION
-- +goose Up

-- Retention only ever visits successfully published rows. Pending events still
-- need delivery and dead-lettered rows are the operator's failure record, so
-- neither belongs in this index or in the pruning predicate.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_outbox_events_published_retention
    ON ops.outbox_events (published_at)
    WHERE published_at IS NOT NULL AND dead_lettered_at IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS ops.idx_outbox_events_published_retention;
