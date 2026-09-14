-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------- placement_sessions
-- Tracks adaptive placement test sessions (§3.4, §3.5).
CREATE TABLE IF NOT EXISTS learn.placement_sessions (
    id                   uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              uuid         NOT NULL,
    status               text         NOT NULL,
    stage                text         NOT NULL,
    theta_estimate       numeric(4,2) NOT NULL DEFAULT 0.00,
    placed_level         text         NULL,
    confidence           numeric(4,3) NOT NULL DEFAULT 0.200,
    current_activity_id  uuid         NULL,
    current_item_kind    text         NULL,
    current_item_level   text         NULL,
    responses            jsonb        NOT NULL DEFAULT '[]'::jsonb,
    adaptive_state       jsonb        NOT NULL DEFAULT '{}'::jsonb,
    started_at           timestamptz  NOT NULL DEFAULT now(),
    completed_at         timestamptz  NULL,
    expires_at           timestamptz  NOT NULL,
    created_at           timestamptz  NOT NULL DEFAULT now(),
    updated_at           timestamptz  NOT NULL DEFAULT now(),

    CONSTRAINT fk_placement_sessions_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_placement_sessions_current_activity FOREIGN KEY (current_activity_id) REFERENCES learn.activities (id) ON DELETE SET NULL,
    CONSTRAINT ck_placement_sessions_status CHECK (status IN ('in_progress', 'completed', 'expired')),
    CONSTRAINT ck_placement_sessions_stage CHECK (stage IN ('fast_convergence', 'receptive', 'productive', 'completed')),
    CONSTRAINT ck_placement_sessions_placed_level CHECK (placed_level IS NULL OR placed_level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_placement_sessions_current_level CHECK (current_item_level IS NULL OR current_item_level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_placement_sessions_responses_array CHECK (jsonb_typeof(responses) = 'array'),
    CONSTRAINT ck_placement_sessions_adaptive_state_object CHECK (jsonb_typeof(adaptive_state) = 'object'),
    CONSTRAINT ck_placement_sessions_completed_after_started CHECK (completed_at IS NULL OR completed_at >= started_at)
);

CREATE INDEX IF NOT EXISTS idx_placement_sessions_user_status ON learn.placement_sessions (user_id, status);
CREATE INDEX IF NOT EXISTS idx_placement_sessions_user_started ON learn.placement_sessions (user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_placement_sessions_sweep ON learn.placement_sessions (status, expires_at) WHERE status = 'in_progress';
CREATE INDEX IF NOT EXISTS idx_placement_sessions_current_activity ON learn.placement_sessions (current_activity_id);

-- ------------------------------------------------------- placement_results link
ALTER TABLE learn.placement_results
    ADD COLUMN IF NOT EXISTS session_id uuid NULL,
    ADD CONSTRAINT fk_placement_results_session FOREIGN KEY (session_id) REFERENCES learn.placement_sessions (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_placement_results_session ON learn.placement_results (session_id);

-- ------------------------------------------------------- weekly_plans
-- Caches personalized weekly study plans generated based on placement test & profile (§3.6).
CREATE TABLE IF NOT EXISTS learn.weekly_plans (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL,
    week_start_date   date        NOT NULL,
    placed_level      text        NOT NULL,
    weakest_skill     text        NOT NULL,
    time_distribution jsonb       NOT NULL DEFAULT '{}'::jsonb,
    daily_targets     jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_weekly_plans_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_weekly_plans_user_week UNIQUE (user_id, week_start_date),
    CONSTRAINT ck_weekly_plans_level CHECK (placed_level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_weekly_plans_distribution_object CHECK (jsonb_typeof(time_distribution) = 'object'),
    CONSTRAINT ck_weekly_plans_targets_array CHECK (jsonb_typeof(daily_targets) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_weekly_plans_user_week ON learn.weekly_plans (user_id, week_start_date DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.weekly_plans;
ALTER TABLE learn.placement_results DROP CONSTRAINT IF EXISTS fk_placement_results_session;
DROP INDEX IF EXISTS learn.idx_placement_results_session;
ALTER TABLE learn.placement_results DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS learn.placement_sessions CASCADE;
-- +goose StatementEnd
