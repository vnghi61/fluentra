-- name: InsertAuditLog :execrows
-- The write behind both the outbox consumer and a direct Recorder call.
--
-- ON CONFLICT DO NOTHING on (event_id, created_at) is what makes at-least-once
-- delivery produce exactly one row. It is deliberately not a check-then-insert
-- in the service: two workers reading "no row yet" at the same moment both
-- insert, and the constraint is the only thing that sees both.
--
-- The row count tells the caller whether this delivery was the first one, which
-- is how the duplicate is observable at all — nothing else distinguishes a
-- redelivery from a fresh event.
INSERT INTO audit.audit_logs (
    id, created_at, event_id, actor_id, actor_role,
    action, target_type, target_id, changed_fields, before, after, meta, ip_hash, trace_id
) VALUES (
    @id, @created_at, @event_id, sqlc.narg(actor_id), sqlc.narg(actor_role)::audit.actor_role,
    @action, sqlc.narg(target_type), sqlc.narg(target_id), @changed_fields::text[],
    sqlc.narg(before), sqlc.narg(after), @meta, sqlc.narg(ip_hash), sqlc.narg(trace_id)
)
ON CONFLICT (event_id, created_at) DO NOTHING;

-- name: SearchAuditLogs :many
-- The admin search.
--
-- created_at is bounded on both sides and never optional. The table is
-- partitioned monthly on it, so an unbounded search reads every partition that
-- has ever existed; the service supplies a default window rather than letting
-- a caller omit one. `@window_start`/`@window_end` are what the planner prunes
-- on, and they are named that way because `from` and `to` are reserved words.
--
-- The cursor is a keyset over (created_at, id), matching the primary key, so
-- paging deep costs the same as paging shallow. `@cursor_created_at IS NULL`
-- selects the first page without a second query.
SELECT id, created_at, event_id, actor_id, actor_role,
       action, target_type, target_id, changed_fields, before, after, meta, ip_hash, trace_id
FROM audit.audit_logs
WHERE created_at >= @window_start
  AND created_at < @window_end
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(action)::text IS NULL OR action = sqlc.narg(action)::text)
  AND (sqlc.narg(target_type)::text IS NULL OR target_type = sqlc.narg(target_type)::text)
  AND (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id)::text)
  AND (
      sqlc.narg(cursor_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: InsertSecurityEvent :execrows
-- Idempotent on the same terms as an audit entry, and for the same reason.
INSERT INTO audit.security_events (
    id, created_at, updated_at, event_id, kind, severity, user_id, detail, ip_hash, trace_id
) VALUES (
    @id, @created_at, @created_at, @event_id, @kind, @severity::audit.event_severity,
    sqlc.narg(user_id), @detail, sqlc.narg(ip_hash), sqlc.narg(trace_id)
)
ON CONFLICT (event_id, created_at) DO NOTHING;

-- name: SearchSecurityEvents :many
-- The triage feed. Bounded and paged exactly like the audit search.
SELECT id, created_at, updated_at, event_id, kind, severity, user_id, detail,
       ip_hash, trace_id, resolved_at, resolved_by, resolution_note
FROM audit.security_events
WHERE created_at >= @window_start
  AND created_at < @window_end
  AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
  AND (
      sqlc.narg(severity)::audit.event_severity IS NULL
      OR severity = sqlc.narg(severity)::audit.event_severity
  )
  AND (sqlc.narg(user_id)::uuid IS NULL OR user_id = sqlc.narg(user_id)::uuid)
  AND (
      sqlc.narg(resolved)::boolean IS NULL
      OR (resolved_at IS NOT NULL) = sqlc.narg(resolved)::boolean
  )
  AND (
      sqlc.narg(cursor_created_at)::timestamptz IS NULL
      OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: GetSecurityEventByID :one
-- Resolving addresses an event by id alone, which is why idx_security_events_id
-- exists: the primary key leads with the partition key, so this would otherwise
-- scan every partition.
SELECT id, created_at, updated_at, event_id, kind, severity, user_id, detail,
       ip_hash, trace_id, resolved_at, resolved_by, resolution_note
FROM audit.security_events
WHERE id = $1;

-- name: ResolveSecurityEvent :execrows
-- Closing an open event. The `resolved_at IS NULL` predicate is what makes a
-- second resolution a conflict rather than a silent overwrite of the first
-- administrator's explanation — the row count is how the service tells the
-- difference between "not found" and "already resolved", together with the
-- read above.
UPDATE audit.security_events
SET resolved_at = @resolved_at,
    resolved_by = @resolved_by,
    resolution_note = @resolution_note,
    updated_at = @resolved_at
WHERE id = @id
  AND created_at = @created_at
  AND resolved_at IS NULL;

-- name: EnsureAuditPartitions :one
-- Creates the current month's partition and the next `months_ahead`, returning
-- how many were made. The function is SECURITY DEFINER and owned by the
-- migration role: the application role running this holds no DDL rights of its
-- own, which is the point.
SELECT audit.ensure_partitions(@months_ahead::integer) AS created_count;

-- name: DetachExpiredAuditPartitions :one
-- Detaches every partition whose whole month is older than the retention
-- period, returning their names. Detach, not drop — see the migration.
SELECT audit.detach_expired_partitions(@retain::interval)::text[] AS detached;
