-- +goose Up
-- +goose StatementBegin

-- citext is what makes BR-USER-01 ("email is unique, case-insensitive") a
-- database guarantee rather than an application convention. Doing it with
-- lower(email) in a functional index instead would leave every query free to
-- forget the lower(), which is exactly the class of bug a constraint exists to
-- remove. The extension is created here, by the module that needs it first.
CREATE EXTENSION IF NOT EXISTS citext;

-- Enum types: closed and stable sets (DATABASE_GUIDELINE §4). Anything that is
-- expected to grow — locales, countries, motivations — stays text with a check.
--
-- Written as bare CREATE TYPE, not wrapped in a DO block with an existence
-- guard, because sqlc parses this file as its schema source and cannot see
-- inside a DO block: guarded, every enum column generates `interface{}`
-- instead of a typed Go constant. Goose applies a migration exactly once, so
-- the guard bought nothing anyway.
CREATE TYPE core.user_status AS ENUM ('active', 'suspended', 'pending_deletion', 'deleted');
CREATE TYPE core.ui_theme AS ENUM ('light', 'dark', 'system');
CREATE TYPE core.cefr_level AS ENUM ('a1', 'a2', 'b1', 'b2', 'c1', 'c2');
CREATE TYPE core.target_exam AS ENUM ('ielts', 'toeic', 'none');

-- ---------------------------------------------------------------- core.users
--
-- Identity only. Every schema in the system foreign-keys to this table
-- (ADR-0004, rule DB4), which is only acceptable while it stays narrow.
-- Descriptive data goes to core.profiles; settings to core.user_preferences.
CREATE TABLE IF NOT EXISTS core.users (
    id                uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    email             citext           NOT NULL,
    status            core.user_status NOT NULL DEFAULT 'active',
    email_verified_at timestamptz,
    created_at        timestamptz      NOT NULL DEFAULT now(),
    updated_at        timestamptz      NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_email UNIQUE (email),
    CONSTRAINT ck_users_email_shape CHECK (
        char_length(email::text) BETWEEN 3 AND 254
        AND position('@' IN email::text) > 1
    )
);

-- Keyset pagination for the admin user list (`GET /api/v1/admin/users`).
CREATE INDEX IF NOT EXISTS idx_users_created_at ON core.users (created_at DESC, id DESC);

-- The deletion sweeper (BR-USER-05) scans only this state, and it is a tiny
-- fraction of the table — a skewed predicate, so the index is partial.
CREATE INDEX IF NOT EXISTS idx_users_pending_deletion
    ON core.users (updated_at)
    WHERE status = 'pending_deletion';

-- ------------------------------------------------------------- core.profiles
CREATE TABLE IF NOT EXISTS core.profiles (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL,
    display_name    text        NOT NULL,
    -- No FK: media assets are owned by the `content` module and a cross-schema
    -- FK to anything but core.users is forbidden (rule DB4).
    avatar_asset_id uuid,
    -- ISO 3166-1 alpha-2. `text` rather than `char(2)`: bpchar blank-pads, so a
    -- one-character value compares equal to a two-character one and the check
    -- below would be the only thing standing between that and a stored 'V '.
    country         text,
    -- IANA name (BR-USER-03). Postgres knows the set in pg_timezone_names, but
    -- that view is not immutable, so a CHECK cannot use it; the service
    -- validates against the tzdata the binary ships with.
    timezone        text        NOT NULL DEFAULT 'UTC',
    -- Age gate only (BR-USER-04). Never rendered, never exported to analytics.
    date_of_birth   date,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT fk_profiles_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    -- 1:1 with users. This unique constraint is also the index rule DB "every
    -- FK gets an index" asks for; a second index on user_id would be dead weight.
    CONSTRAINT uq_profiles_user UNIQUE (user_id),
    CONSTRAINT ck_profiles_display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 50),
    CONSTRAINT ck_profiles_country_format CHECK (country IS NULL OR country ~ '^[A-Z]{2}$'),
    CONSTRAINT ck_profiles_timezone_length CHECK (char_length(timezone) BETWEEN 1 AND 64),
    -- The upper bound is the service's job: now() is not immutable and a CHECK
    -- that uses it silently stops being re-evaluated after the row is written.
    CONSTRAINT ck_profiles_date_of_birth_sane CHECK (date_of_birth IS NULL OR date_of_birth > DATE '1900-01-01')
);

-- ---------------------------------------------------- core.user_preferences
CREATE TABLE IF NOT EXISTS core.user_preferences (
    id                    uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               uuid          NOT NULL,
    locale                text          NOT NULL DEFAULT 'en',
    theme                 core.ui_theme NOT NULL DEFAULT 'system',
    daily_goal_minutes    integer       NOT NULL DEFAULT 15,
    notification_channels text[]        NOT NULL DEFAULT ARRAY['in_app', 'email']::text[],
    quiet_hours_start     time,
    quiet_hours_end       time,
    ai_processing_opt_out boolean       NOT NULL DEFAULT false,
    created_at            timestamptz   NOT NULL DEFAULT now(),
    updated_at            timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT fk_user_preferences_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_user_preferences_user UNIQUE (user_id),
    CONSTRAINT ck_user_preferences_locale_format CHECK (locale ~ '^[a-z]{2}(-[A-Z]{2})?$'),
    CONSTRAINT ck_user_preferences_daily_goal CHECK (daily_goal_minutes BETWEEN 5 AND 480),
    CONSTRAINT ck_user_preferences_channels CHECK (
        notification_channels <@ ARRAY['in_app', 'email', 'push']::text[]
    ),
    -- A half-configured quiet window is not a state the service can act on.
    CONSTRAINT ck_user_preferences_quiet_hours CHECK (
        (quiet_hours_start IS NULL) = (quiet_hours_end IS NULL)
    )
);

-- --------------------------------------------------- core.learning_profiles
CREATE TABLE IF NOT EXISTS core.learning_profiles (
    id                  uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid            NOT NULL,
    -- Self-declared, not measured. The placement test result lives in `learn`.
    declared_level      core.cefr_level,
    target_level        core.cefr_level,
    target_exam         core.target_exam NOT NULL DEFAULT 'none',
    weekly_minutes_goal integer,
    motivations         text[]          NOT NULL DEFAULT ARRAY[]::text[],
    created_at          timestamptz     NOT NULL DEFAULT now(),
    updated_at          timestamptz     NOT NULL DEFAULT now(),
    CONSTRAINT fk_learning_profiles_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_learning_profiles_user UNIQUE (user_id),
    CONSTRAINT ck_learning_profiles_weekly_goal CHECK (
        weekly_minutes_goal IS NULL OR weekly_minutes_goal BETWEEN 15 AND 10080
    ),
    CONSTRAINT ck_learning_profiles_motivations CHECK (cardinality(motivations) <= 10)
);

GRANT SELECT, INSERT, UPDATE, DELETE
    ON core.users, core.profiles, core.user_preferences, core.learning_profiles
    TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.learning_profiles;
DROP TABLE IF EXISTS core.user_preferences;
DROP TABLE IF EXISTS core.profiles;
DROP TABLE IF EXISTS core.users;

DROP TYPE IF EXISTS core.target_exam;
DROP TYPE IF EXISTS core.cefr_level;
DROP TYPE IF EXISTS core.ui_theme;
DROP TYPE IF EXISTS core.user_status;

-- citext is deliberately not dropped: it is a database-wide object that other
-- modules may already depend on by the time this migration is reverted.
-- +goose StatementEnd
