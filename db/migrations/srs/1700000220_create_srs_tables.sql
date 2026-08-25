-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------ review_cards
CREATE TABLE IF NOT EXISTS learn.review_cards (
    id                  uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid             NOT NULL,
    content_version_id  uuid             NOT NULL,
    skill               text             NOT NULL,
    stability           double precision NOT NULL DEFAULT 0.0,
    difficulty          double precision NOT NULL DEFAULT 0.0,
    due_at              timestamptz      NOT NULL DEFAULT now(),
    reps                integer          NOT NULL DEFAULT 0,
    lapses              integer          NOT NULL DEFAULT 0,
    state               text             NOT NULL DEFAULT 'new',
    suspended_at        timestamptz,
    created_at          timestamptz      NOT NULL DEFAULT now(),
    updated_at          timestamptz      NOT NULL DEFAULT now(),

    CONSTRAINT fk_review_cards_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_review_cards_user_content UNIQUE (user_id, content_version_id),
    CONSTRAINT ck_review_cards_skill CHECK (skill IN ('vocabulary', 'grammar', 'reading', 'listening', 'speaking', 'writing')),
    CONSTRAINT ck_review_cards_state CHECK (state IN ('new', 'learning', 'review', 'relearning')),
    CONSTRAINT ck_review_cards_stability_nonnegative CHECK (stability >= 0.0),
    CONSTRAINT ck_review_cards_difficulty_range CHECK (difficulty >= 0.0 AND difficulty <= 10.0),
    CONSTRAINT ck_review_cards_reps_nonnegative CHECK (reps >= 0),
    CONSTRAINT ck_review_cards_lapses_nonnegative CHECK (lapses >= 0)
);

-- idx_review_cards_user_due is the hottest query in the product (fetching active due cards)
CREATE INDEX IF NOT EXISTS idx_review_cards_user_due ON learn.review_cards (user_id, due_at ASC) WHERE suspended_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_review_cards_content_version ON learn.review_cards (content_version_id);

-- ------------------------------------------------------------- review_logs
CREATE TABLE IF NOT EXISTS learn.review_logs (
    id                uuid             NOT NULL DEFAULT gen_random_uuid(),
    reviewed_at       timestamptz      NOT NULL DEFAULT now(),
    card_id           uuid             NOT NULL,
    user_id           uuid             NOT NULL,
    grade             text             NOT NULL,
    elapsed_ms        integer          NOT NULL DEFAULT 0,
    stability_before  double precision NOT NULL DEFAULT 0.0,
    stability_after   double precision NOT NULL DEFAULT 0.0,
    difficulty_before double precision NOT NULL DEFAULT 0.0,
    difficulty_after  double precision NOT NULL DEFAULT 0.0,
    scheduled_days    integer          NOT NULL DEFAULT 0,
    scheduler_version text             NOT NULL DEFAULT 'v4.5',

    CONSTRAINT pk_review_logs PRIMARY KEY (reviewed_at, id),
    CONSTRAINT fk_review_logs_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_review_logs_grade CHECK (grade IN ('again', 'hard', 'good', 'easy')),
    CONSTRAINT ck_review_logs_elapsed_ms CHECK (elapsed_ms >= 0),
    CONSTRAINT ck_review_logs_scheduled_days CHECK (scheduled_days >= 0)
) PARTITION BY RANGE (reviewed_at);

CREATE INDEX IF NOT EXISTS idx_review_logs_card_time ON learn.review_logs (card_id, reviewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_logs_user_time ON learn.review_logs (user_id, reviewed_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_logs_id ON learn.review_logs (id);

-- -------------------------------------------------------------- srs_params
CREATE TABLE IF NOT EXISTS learn.srs_params (
    id                uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid,
    weights           jsonb            NOT NULL,
    request_retention double precision NOT NULL DEFAULT 0.90,
    max_interval      integer          NOT NULL DEFAULT 36500,
    created_at        timestamptz      NOT NULL DEFAULT now(),
    updated_at        timestamptz      NOT NULL DEFAULT now(),

    CONSTRAINT fk_srs_params_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_srs_params_user UNIQUE (user_id),
    CONSTRAINT ck_srs_params_retention CHECK (request_retention > 0.0 AND request_retention < 1.0),
    CONSTRAINT ck_srs_params_max_interval CHECK (max_interval > 0)
);

-- ------------------------------------------------------ review_daily_stats
CREATE TABLE IF NOT EXISTS learn.review_daily_stats (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL,
    stat_date         date        NOT NULL,
    reviews_completed integer     NOT NULL DEFAULT 0,
    new_cards_learned integer     NOT NULL DEFAULT 0,
    total_minutes     integer     NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_review_daily_stats_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_review_daily_stats_user_date UNIQUE (user_id, stat_date),
    CONSTRAINT ck_review_daily_stats_reviews_completed CHECK (reviews_completed >= 0),
    CONSTRAINT ck_review_daily_stats_new_cards CHECK (new_cards_learned >= 0),
    CONSTRAINT ck_review_daily_stats_total_minutes CHECK (total_minutes >= 0)
);

-- ---------------------------------------------------- partition management
CREATE FUNCTION learn.ensure_srs_partitions(months_ahead integer DEFAULT 3)
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
        starts_at := (
            date_trunc('month', (now() AT TIME ZONE 'UTC'))
            + make_interval(months => month_offset)
        ) AT TIME ZONE 'UTC';
        ends_at := starts_at + interval '1 month';
        partition_name := 'review_logs' || to_char(starts_at AT TIME ZONE 'UTC', '"_y"YYYY"m"MM');

        IF to_regclass('learn.' || quote_ident(partition_name)) IS NOT NULL THEN
            CONTINUE;
        END IF;

        EXECUTE 'CREATE TABLE ' || format(
            'learn.%I PARTITION OF learn.review_logs FOR VALUES FROM (%L) TO (%L)',
            partition_name, starts_at, ends_at);

        created_count := created_count + 1;
    END LOOP;

    RETURN created_count;
END;
$ensure$;

REVOKE ALL ON FUNCTION learn.ensure_srs_partitions(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION learn.ensure_srs_partitions(integer) TO fluentra_app;

-- Pre-create partitions for current and next 3 months
SELECT learn.ensure_srs_partitions(3);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS learn.ensure_srs_partitions(integer);

DROP TABLE IF EXISTS learn.review_daily_stats CASCADE;
DROP TABLE IF EXISTS learn.srs_params CASCADE;
DROP TABLE IF EXISTS learn.review_logs CASCADE;
DROP TABLE IF EXISTS learn.review_cards CASCADE;

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
          AND c.relname ~ '^review_logs_y\d{4}m\d{2}$'
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS learn.%I', orphan);
    END LOOP;
END $down$;
-- +goose StatementEnd
