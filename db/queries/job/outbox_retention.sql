-- name: DeletePublishedOutboxEventsBefore :execrows
-- A delivered event is retained for an operational dispatch trail, then
-- removed rather than redacted so its payload cannot outlive the learner data
-- it described. Dead-lettered rows remain the failure record for triage.
DELETE FROM ops.outbox_events
WHERE published_at IS NOT NULL
  AND published_at < $1
  AND dead_lettered_at IS NULL;
