-- +goose Up
-- +goose StatementBegin

-- Enum types: closed and stable sets (DATABASE_GUIDELINE §4). Written as bare
-- CREATE TYPE rather than wrapped in a DO block with an existence guard,
-- because sqlc parses this file as its schema source and cannot see inside a
-- DO block: guarded, every enum column generates `interface{}` instead of a
-- typed Go constant.
CREATE TYPE audit.actor_role AS ENUM ('admin', 'user', 'system');
CREATE TYPE audit.event_severity AS ENUM ('low', 'medium', 'high', 'critical');

-- ------------------------------------------------------- audit.audit_logs
--
-- The trail of what happened. Partitioned monthly on created_at
-- (DATABASE_GUIDELINE §10); partitions are created by audit.ensure_partitions
-- below, never by hand.
--
-- Two deliberate departures from the standard column set in §3:
--
--   * No `updated_at`. The application role holds no UPDATE grant on this
--     table (BR-AUDIT-01), so a column recording when a row was last changed
--     would document something that cannot happen.
--   * No foreign key on `actor_id`. An audit entry must outlive the account
--     that produced it (BR-AUDIT-06), and `audit` depends only on `job` in the
--     dependency graph — a reference to core.users would be a boundary the
--     module is not allowed to cross and a cascade that erases the evidence.
CREATE TABLE audit.audit_logs (
    id             uuid        NOT NULL DEFAULT gen_random_uuid(),

    -- When the action happened, not when this row was written. The consumer
    -- takes it from the emitting module's event, which is what makes a
    -- redelivered event land in the same partition and collide with the row it
    -- already wrote instead of creating a second one.
    created_at     timestamptz NOT NULL DEFAULT now(),

    -- The idempotency key. It is the outbox event id for anything arriving
    -- through the outbox, and a generated id for a direct Recorder call.
    event_id       uuid        NOT NULL,

    actor_id       uuid,

    -- Recorded rather than resolved at read time: a revoked administrator must
    -- still read as an administrator in the entries they left behind.
    actor_role     audit.actor_role,

    action         text        NOT NULL,
    target_type    text,

    -- text, not uuid: not every auditable target is keyed by one.
    target_id      text,

    -- The names of the fields the action moved. BR-AUDIT-04 — the trail says
    -- what changed, and `before`/`after` carry values only when the emitting
    -- module sends them, redacted on the PII deny-list. An audit log holding a
    -- copy of every display name would be a second store of personal data with
    -- a longer retention period than the first.
    changed_fields text[]      NOT NULL DEFAULT '{}',
    before         jsonb,
    after          jsonb,

    -- Context that is not a field of the target: the reason an administrator
    -- gave, the request id, the event that caused this. Redacted on the same
    -- deny-list as the diff.
    meta           jsonb       NOT NULL DEFAULT '{}'::jsonb,

    -- SHA-256 of the client address under a keyed hash. The raw address is
    -- never persisted; without the key the column is not reversible, which
    -- matters because the IPv4 space is small enough to enumerate.
    ip_hash        text,

    trace_id       text,

    -- The partition key has to be in the primary key. Leading with created_at
    -- also makes it the index the newest-first search reads.
    CONSTRAINT pk_audit_logs PRIMARY KEY (created_at, id),

    -- Exactly-once, enforced by the database rather than by the consumer
    -- checking first and inserting second. At-least-once delivery means the
    -- second copy arrives eventually; a check-then-insert loses the race.
    CONSTRAINT uq_audit_logs_event UNIQUE (event_id, created_at),

    CONSTRAINT ck_audit_logs_action CHECK (action ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$'),
    CONSTRAINT ck_audit_logs_trace_id CHECK (trace_id IS NULL OR trace_id ~ '^[0-9a-f]{32}$'),
    CONSTRAINT ck_audit_logs_ip_hash CHECK (ip_hash IS NULL OR ip_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_audit_logs_changed_fields CHECK (cardinality(changed_fields) <= 64),
    CONSTRAINT ck_audit_logs_meta_object CHECK (jsonb_typeof(meta) = 'object'),
    CONSTRAINT ck_audit_logs_target CHECK (
        (target_type IS NULL AND target_id IS NULL) OR target_type IS NOT NULL
    )
) PARTITION BY RANGE (created_at);

-- Declared on the parent so every partition inherits them (§10).
--
-- All three lead with an equality column and close with created_at DESC,
-- because every search is bounded by the window and ordered newest first.
CREATE INDEX idx_audit_logs_target
    ON audit.audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor_time
    ON audit.audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_action_time
    ON audit.audit_logs (action, created_at DESC);

-- -------------------------------------------------- audit.security_events
--
-- Separate from the trail because these are triaged rather than merely
-- recorded, and because they are not always attributable to a known actor.
-- This table does take UPDATE: resolving an event is the whole point of it.
CREATE TABLE audit.security_events (
    id              uuid        NOT NULL DEFAULT gen_random_uuid(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    event_id        uuid        NOT NULL,
    kind            text        NOT NULL,
    severity        audit.event_severity NOT NULL,
    user_id         uuid,

    -- Structured context for triage. Never the request body: an event raised
    -- by an attacker would otherwise store whatever they chose to send.
    detail          jsonb       NOT NULL DEFAULT '{}'::jsonb,

    ip_hash         text,
    trace_id        text,
    resolved_at     timestamptz,
    resolved_by     uuid,
    resolution_note text,

    CONSTRAINT pk_security_events PRIMARY KEY (created_at, id),
    CONSTRAINT uq_security_events_event UNIQUE (event_id, created_at),
    CONSTRAINT ck_security_events_kind CHECK (kind ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$'),
    CONSTRAINT ck_security_events_trace_id CHECK (trace_id IS NULL OR trace_id ~ '^[0-9a-f]{32}$'),
    CONSTRAINT ck_security_events_ip_hash CHECK (ip_hash IS NULL OR ip_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_security_events_detail_object CHECK (jsonb_typeof(detail) = 'object'),

    -- A resolution is all three facts or none of them. An event closed with no
    -- explanation is indistinguishable from one closed by accident.
    CONSTRAINT ck_security_events_resolution CHECK (
        (resolved_at IS NULL AND resolved_by IS NULL AND resolution_note IS NULL)
        OR (resolved_at IS NOT NULL AND resolved_by IS NOT NULL
            AND resolution_note IS NOT NULL AND char_length(resolution_note) BETWEEN 1 AND 500)
    )
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_security_events_kind_time
    ON audit.security_events (kind, created_at DESC);
CREATE INDEX idx_security_events_severity_time
    ON audit.security_events (severity, created_at DESC);

-- The triage queue is a skewed predicate — most events are resolved, and the
-- dashboard only ever asks for the ones that are not (§6, partial indexes).
CREATE INDEX idx_security_events_open
    ON audit.security_events (created_at DESC) WHERE resolved_at IS NULL;

-- Resolving an event addresses it by id alone, and the primary key leads with
-- created_at, so without this the lookup sequentially scans every partition.
-- Non-unique, which is what lets it omit the partition key: uniqueness is
-- already held by the primary key.
CREATE INDEX idx_security_events_id ON audit.security_events (id);

-- ------------------------------------------------- partition management
--
-- SECURITY DEFINER, owned by fluentra_migrator.
--
-- The rotation job runs as fluentra_app, which has no DDL rights and must not
-- get any: a role that can create relations in this schema can also create one
-- that shadows the trail. Confining the DDL to a function with a fixed body,
-- owned by the migration role, is what lets the job do its work without
-- holding the privilege to do anything else.
--
-- `SET search_path` is not optional on a SECURITY DEFINER function. Without
-- it, a caller controlling their own search_path chooses which `format` or
-- `to_regclass` this body resolves to, and the function runs their code as its
-- owner.
CREATE FUNCTION audit.ensure_partitions(months_ahead integer DEFAULT 3)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $ensure$
DECLARE
    parent_name   text;
    month_offset  integer;
    starts_at     timestamptz;
    ends_at       timestamptz;
    partition_name text;
    created_count integer := 0;
BEGIN
    IF months_ahead IS NULL OR months_ahead < 0 OR months_ahead > 24 THEN
        RAISE EXCEPTION 'months_ahead must be between 0 and 24, got %', months_ahead;
    END IF;

    FOREACH parent_name IN ARRAY ARRAY['audit_logs', 'security_events'] LOOP
        FOR month_offset IN 0..months_ahead LOOP
            -- Anchored to UTC rather than the session time zone, so the same
            -- call from a worker in another region produces the same boundary.
            starts_at := (
                date_trunc('month', (now() AT TIME ZONE 'UTC'))
                + make_interval(months => month_offset)
            ) AT TIME ZONE 'UTC';
            ends_at := starts_at + interval '1 month';
            partition_name := parent_name || to_char(starts_at AT TIME ZONE 'UTC', '"_y"YYYY"m"MM');

            IF to_regclass('audit.' || quote_ident(partition_name)) IS NOT NULL THEN
                CONTINUE;
            END IF;

            -- The DDL keyword is concatenated rather than written into the
            -- format string. tools/docgen/check-drift.mjs regex-matches table
            -- declarations in this directory against the `tables:`
            -- front-matter, and a template it could read would look like a
            -- table this module failed to document.
            EXECUTE 'CREATE TABLE ' || format(
                'audit.%I PARTITION OF audit.%I FOR VALUES FROM (%L) TO (%L)',
                partition_name, parent_name, starts_at, ends_at);

            -- A new partition is created by fluentra_migrator, so the default
            -- privileges set in the bootstrap migration hand it SELECT, INSERT,
            -- UPDATE and DELETE. Append-only would last exactly until the first
            -- month rolled over. Take it back, then grant the subset.
            EXECUTE format('REVOKE ALL ON audit.%I FROM fluentra_app', partition_name);
            IF parent_name = 'audit_logs' THEN
                EXECUTE format('GRANT SELECT, INSERT ON audit.%I TO fluentra_app', partition_name);
            ELSE
                EXECUTE format('GRANT SELECT, INSERT, UPDATE ON audit.%I TO fluentra_app', partition_name);
            END IF;

            created_count := created_count + 1;
        END LOOP;
    END LOOP;

    RETURN created_count;
END;
$ensure$;

-- Retention (BR-AUDIT-05). Partitions whose whole range is older than the
-- retention period are DETACHED, not dropped: detaching is instant and
-- reversible, dropping is neither, and archiving the detached table to object
-- storage needs `storage`, which this module does not depend on. The detached
-- relation is left in place for that archival step, with its grants removed so
-- that nothing can write to a table no search will ever read.
CREATE FUNCTION audit.detach_expired_partitions(retain interval DEFAULT interval '2 years')
RETURNS text[]
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $detach$
DECLARE
    cutoff      timestamptz;
    candidate   record;
    month_start timestamptz;
    detached    text[] := ARRAY[]::text[];
BEGIN
    IF retain IS NULL OR retain < interval '30 days' THEN
        RAISE EXCEPTION 'retain must be at least 30 days, got %', retain;
    END IF;
    cutoff := now() - retain;

    FOR candidate IN
        SELECT parent.relname AS parent_name, child.relname AS child_name
        FROM pg_inherits
        JOIN pg_class parent ON parent.oid = pg_inherits.inhparent
        JOIN pg_class child ON child.oid = pg_inherits.inhrelid
        JOIN pg_namespace ns ON ns.oid = parent.relnamespace
        WHERE ns.nspname = 'audit'
          AND parent.relname IN ('audit_logs', 'security_events')
        ORDER BY child.relname
    LOOP
        -- The month comes from the name this module gave the partition, not
        -- from parsing relpartbound: the naming is ours to rely on, and the
        -- bound expression is a string whose format is the planner's business.
        IF candidate.child_name !~ '_y\d{4}m\d{2}$' THEN
            CONTINUE;
        END IF;
        month_start := make_timestamptz(
            (substring(candidate.child_name from '_y(\d{4})m\d{2}$'))::integer,
            (substring(candidate.child_name from '_y\d{4}m(\d{2})$'))::integer,
            1, 0, 0, 0, 'UTC');

        -- Only when the entire month is past the cutoff. A partition holding
        -- one row still inside retention is not expired.
        CONTINUE WHEN month_start + interval '1 month' > cutoff;

        EXECUTE format('ALTER TABLE audit.%I DETACH PARTITION audit.%I',
            candidate.parent_name, candidate.child_name);
        EXECUTE format('REVOKE ALL ON audit.%I FROM fluentra_app', candidate.child_name);
        detached := detached || candidate.child_name;
    END LOOP;

    RETURN detached;
END;
$detach$;

-- ------------------------------------------------------------------ grants
--
-- This is BR-AUDIT-01, and it is the only thing that makes it true. The
-- bootstrap migration grants SELECT, INSERT, UPDATE and DELETE by default on
-- every table created in this schema, so append-only is a REVOKE — not an
-- omission, and not a rule the application is trusted to follow.
REVOKE ALL ON audit.audit_logs FROM fluentra_app;
GRANT SELECT, INSERT ON audit.audit_logs TO fluentra_app;

-- security_events takes UPDATE so an event can be resolved, and still no
-- DELETE: triaging an event is not the same as making it go away.
REVOKE ALL ON audit.security_events FROM fluentra_app;
GRANT SELECT, INSERT, UPDATE ON audit.security_events TO fluentra_app;

-- Postgres grants EXECUTE on a new function to PUBLIC. On a SECURITY DEFINER
-- function that creates tables, that is the privilege escalation the role
-- split exists to prevent.
REVOKE ALL ON FUNCTION audit.ensure_partitions(integer) FROM PUBLIC;
REVOKE ALL ON FUNCTION audit.detach_expired_partitions(interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION audit.ensure_partitions(integer) TO fluentra_app;
GRANT EXECUTE ON FUNCTION audit.detach_expired_partitions(interval) TO fluentra_app;

-- The current month plus three, so a deployment that never runs the rotation
-- job still accepts writes for a quarter.
SELECT audit.ensure_partitions(3);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS audit.detach_expired_partitions(interval);
DROP FUNCTION IF EXISTS audit.ensure_partitions(integer);

-- CASCADE takes the partitions with the parent. Any partition detached by the
-- retention function is no longer attached, so it is dropped by name below if
-- it is still there.
DROP TABLE IF EXISTS audit.security_events CASCADE;
DROP TABLE IF EXISTS audit.audit_logs CASCADE;

DO $down$
DECLARE
    orphan text;
BEGIN
    FOR orphan IN
        SELECT c.relname
        FROM pg_class c
        JOIN pg_namespace ns ON ns.oid = c.relnamespace
        WHERE ns.nspname = 'audit'
          AND c.relkind = 'r'
          AND c.relname ~ '^(audit_logs|security_events)_y\d{4}m\d{2}$'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS audit.%I', orphan);
    END LOOP;
END $down$;

DROP TYPE IF EXISTS audit.event_severity;
DROP TYPE IF EXISTS audit.actor_role;
-- +goose StatementEnd
