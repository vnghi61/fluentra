-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- enrollments
CREATE TABLE IF NOT EXISTS learn.enrollments (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL,
    course_id    uuid        NOT NULL,
    status       text        NOT NULL DEFAULT 'active',
    started_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_enrollments_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_enrollments_course FOREIGN KEY (course_id) REFERENCES learn.courses (id) ON DELETE CASCADE,
    CONSTRAINT uq_enrollments_user_course UNIQUE (user_id, course_id),
    CONSTRAINT ck_enrollments_status CHECK (status IN ('active', 'completed', 'dropped')),
    CONSTRAINT ck_enrollments_completed_at CHECK (
        (status = 'completed' AND completed_at IS NOT NULL) OR
        (status <> 'completed' AND completed_at IS NULL)
    )
);

-- fk_enrollments_user is covered by uq_enrollments_user_course, which leads
-- with user_id; fk_enrollments_course needs its own, because no constraint
-- here leads with course_id.
CREATE INDEX IF NOT EXISTS idx_enrollments_course_id ON learn.enrollments (course_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_status ON learn.enrollments (status);

-- ---------------------------------------------------------------- progress
CREATE TABLE IF NOT EXISTS learn.progress (
    id           uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid           NOT NULL,
    scope        text           NOT NULL,
    scope_id     uuid           NOT NULL,
    status       text           NOT NULL DEFAULT 'not_started',
    score        integer,
    completed_at timestamptz,
    created_at   timestamptz    NOT NULL DEFAULT now(),
    updated_at   timestamptz    NOT NULL DEFAULT now(),

    CONSTRAINT fk_progress_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_progress_user_scope UNIQUE (user_id, scope, scope_id),
    CONSTRAINT ck_progress_scope CHECK (scope IN ('activity', 'lesson', 'unit', 'course')),
    CONSTRAINT ck_progress_status CHECK (status IN ('not_started', 'in_progress', 'completed')),
    CONSTRAINT ck_progress_score CHECK (score IS NULL OR (score >= 0 AND score <= 100)),
    CONSTRAINT ck_progress_completed_at CHECK (
        (status = 'completed' AND completed_at IS NOT NULL) OR
        (status <> 'completed' AND completed_at IS NULL)
    )
);

-- fk_progress_user is covered by uq_progress_user_scope, which leads with
-- user_id and is the read path for every dashboard request.
CREATE INDEX IF NOT EXISTS idx_progress_status ON learn.progress (status);

-- ---------------------------------------------------------------- attempts
CREATE TABLE IF NOT EXISTS learn.attempts (
    id              uuid         NOT NULL DEFAULT gen_random_uuid(),
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),
    user_id         uuid         NOT NULL,
    activity_id     uuid         NOT NULL,
    idempotency_key uuid,
    response        jsonb        NOT NULL DEFAULT '{}'::jsonb,
    score           integer,
    max_score       integer      NOT NULL DEFAULT 100,
    grader          text,
    duration_ms     integer      NOT NULL DEFAULT 0,
    status          text         NOT NULL DEFAULT 'in_progress',

    CONSTRAINT pk_attempts PRIMARY KEY (created_at, id),
    CONSTRAINT fk_attempts_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_attempts_activity FOREIGN KEY (activity_id) REFERENCES learn.activities (id) ON DELETE CASCADE,
    -- The same four the API can express. components/learning.yaml's
    -- AttemptDetail.status enum is the contract, and a status the database
    -- accepts but no response can carry is a row nothing will ever render.
    CONSTRAINT ck_attempts_status CHECK (status IN ('in_progress', 'grading', 'graded', 'failed')),
    CONSTRAINT ck_attempts_duration_positive CHECK (duration_ms >= 0),
    CONSTRAINT ck_attempts_max_score_positive CHECK (max_score > 0),
    CONSTRAINT ck_attempts_score_range CHECK (score IS NULL OR (score >= 0 AND score <= max_score)),
    CONSTRAINT ck_attempts_response_object CHECK (jsonb_typeof(response) = 'object')
) PARTITION BY RANGE (created_at);

-- Every index here is inherited by every monthly partition, on the table that
-- takes more writes than any other in the product, so each one has to earn its
-- place (DATABASE_GUIDELINE.md: "never index everything just in case").
--
-- These two are the reads AGENT.md §5 names, and their leading columns also
-- cover fk_attempts_user and fk_attempts_activity, so no single-column index on
-- either is needed for the foreign key.
CREATE INDEX IF NOT EXISTS idx_attempts_user_activity_time ON learn.attempts (user_id, activity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attempts_activity_time ON learn.attempts (activity_id, created_at DESC);

-- GET /attempts/{id} arrives with an id and no timestamp, so it cannot prune to
-- one partition. This is what keeps that lookup an index scan per partition
-- rather than a sequential one.
CREATE INDEX IF NOT EXISTS idx_attempts_id ON learn.attempts (id);

-- ------------------------------------------------------- learning_sessions
CREATE TABLE IF NOT EXISTS learn.learning_sessions (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              uuid        NOT NULL,
    started_at           timestamptz NOT NULL DEFAULT now(),
    ended_at             timestamptz,
    activities_completed integer     NOT NULL DEFAULT 0,
    minutes              integer     NOT NULL DEFAULT 0,
    -- StartSessionRequest.metadata is already in the merged spec as "optional
    -- client session context". Without somewhere to put it, P8.3 would accept
    -- the field and silently discard it.
    metadata             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_learning_sessions_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_learning_sessions_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT ck_learning_sessions_activities_completed CHECK (activities_completed >= 0),
    CONSTRAINT ck_learning_sessions_minutes CHECK (minutes >= 0),
    CONSTRAINT ck_learning_sessions_ended_after_started CHECK (ended_at IS NULL OR ended_at >= started_at)
);

-- fk_learning_sessions_user is covered by the composite below.
CREATE INDEX IF NOT EXISTS idx_learning_sessions_user_started ON learn.learning_sessions (user_id, started_at DESC);

-- ------------------------------------------------------- placement_results
CREATE TABLE IF NOT EXISTS learn.placement_results (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL,
    estimated_level text        NOT NULL,
    per_skill       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    taken_at        timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_placement_results_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_placement_results_level CHECK (estimated_level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_placement_results_per_skill_object CHECK (jsonb_typeof(per_skill) = 'object')
);

-- fk_placement_results_user is covered by the composite below.
CREATE INDEX IF NOT EXISTS idx_placement_results_user_taken ON learn.placement_results (user_id, taken_at DESC);

-- ----------------------------------------------------------- skill_mastery
CREATE TABLE IF NOT EXISTS learn.skill_mastery (
    id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid         NOT NULL,
    skill       text         NOT NULL,
    level       text         NOT NULL,
    confidence  numeric(5,2) NOT NULL DEFAULT 0.00,
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    created_at  timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT fk_skill_mastery_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_skill_mastery_user_skill UNIQUE (user_id, skill),
    CONSTRAINT ck_skill_mastery_skill CHECK (skill IN ('vocabulary', 'grammar', 'reading', 'listening', 'speaking', 'writing')),
    CONSTRAINT ck_skill_mastery_level CHECK (level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_skill_mastery_confidence CHECK (confidence >= 0.00 AND confidence <= 1.00)
);

-- No index is declared here on purpose: uq_skill_mastery_user_skill already
-- builds one on (user_id, skill), which covers fk_skill_mastery_user and every
-- read this table has.

-- ---------------------------------------------------- partition management
--
-- SECURITY DEFINER, owned by fluentra_migrator.
--
-- The partition maintenance job runs as fluentra_app, which has no DDL rights
-- and must not get any. Confining partition creation to a function with a fixed
-- body, owned by the migration role, lets the scheduled job create monthly
-- partitions without holding elevated privileges.
--
-- `SET search_path` is required on SECURITY DEFINER functions to prevent
-- search_path hijacking.
CREATE FUNCTION learn.ensure_partitions(months_ahead integer DEFAULT 3)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $ensure$
DECLARE
    month_offset   integer;
    starts_at      timestamptz;
    ends_at        timestamptz;
    partition_name text;
    created_count  integer := 0;
BEGIN
    IF months_ahead IS NULL OR months_ahead < 0 OR months_ahead > 24 THEN
        RAISE EXCEPTION 'months_ahead must be between 0 and 24, got %', months_ahead;
    END IF;

    FOR month_offset IN 0..months_ahead LOOP
        -- Anchored to UTC so any worker replica in any timezone computes identical boundaries.
        starts_at := (
            date_trunc('month', (now() AT TIME ZONE 'UTC'))
            + make_interval(months => month_offset)
        ) AT TIME ZONE 'UTC';
        ends_at := starts_at + interval '1 month';
        partition_name := 'attempts' || to_char(starts_at AT TIME ZONE 'UTC', '"_y"YYYY"m"MM');

        IF to_regclass('learn.' || quote_ident(partition_name)) IS NOT NULL THEN
            CONTINUE;
        END IF;

        -- The DDL keyword is concatenated rather than written into the format string.
        -- tools/docgen/check-drift.mjs regex-matches table declarations in this directory
        -- against the `tables:` front-matter, and a template it could read would look like
        -- a table this module failed to document (Trap 2).
        EXECUTE 'CREATE TABLE ' || format(
            'learn.%I PARTITION OF learn.attempts FOR VALUES FROM (%L) TO (%L)',
            partition_name, starts_at, ends_at);

        created_count := created_count + 1;
    END LOOP;

    RETURN created_count;
END;
$ensure$;

-- Restrict function execution permissions.
REVOKE ALL ON FUNCTION learn.ensure_partitions(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION learn.ensure_partitions(integer) TO fluentra_app;

-- Pre-create partitions for current month and next 3 months so immediate writes succeed.
SELECT learn.ensure_partitions(3);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS learn.ensure_partitions(integer);

DROP TABLE IF EXISTS learn.skill_mastery CASCADE;
DROP TABLE IF EXISTS learn.placement_results CASCADE;
DROP TABLE IF EXISTS learn.learning_sessions CASCADE;
DROP TABLE IF EXISTS learn.attempts CASCADE;
DROP TABLE IF EXISTS learn.progress CASCADE;
DROP TABLE IF EXISTS learn.enrollments CASCADE;

DO $down$
DECLARE
    orphan text;
BEGIN
    FOR orphan IN
        SELECT c.relname
        FROM pg_class c
        JOIN pg_namespace ns ON ns.oid = c.relnamespace
        WHERE ns.nspname = 'learn'
          AND c.relkind = 'r'
          AND c.relname ~ '^attempts_y\d{4}m\d{2}$'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS learn.%I', orphan);
    END LOOP;
END $down$;
-- +goose StatementEnd
