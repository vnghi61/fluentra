-- +goose Up
-- +goose StatementBegin

-- --------------------------------------------------------- placement_sessions
-- One adaptive placement test (work order 13 §3.5). The server owns the clock:
-- deadline_at is fixed when the session starts, and an answer after it plus five
-- seconds is refused. estimate is the probability over the five bands with the
-- observations it was built from; items is every item served, in order, with the
-- attempt that answered it. version guards the read-modify-write of both.
CREATE TABLE IF NOT EXISTS learn.placement_sessions (
    id                     uuid        PRIMARY KEY,
    user_id                uuid        NOT NULL,
    status                 text        NOT NULL,
    stage                  text        NOT NULL,
    started_at             timestamptz NOT NULL,
    deadline_at            timestamptz NOT NULL,
    estimate               jsonb       NOT NULL DEFAULT '{}'::jsonb,
    items                  jsonb       NOT NULL DEFAULT '[]'::jsonb,
    version                integer     NOT NULL DEFAULT 0,
    productive_status      text        NOT NULL DEFAULT 'offered',
    productive_deadline_at timestamptz NULL,
    result_id              uuid        NULL,
    completed_at           timestamptz NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_placement_sessions_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_placement_sessions_status CHECK (status IN ('in_progress', 'completed', 'expired')),
    CONSTRAINT ck_placement_sessions_stage
        CHECK (stage IN ('vocabulary_grammar', 'reading_listening', 'refine', 'done')),
    CONSTRAINT ck_placement_sessions_productive_status
        CHECK (productive_status IN ('offered', 'skipped', 'in_progress', 'submitted', 'graded')),
    CONSTRAINT ck_placement_sessions_estimate_object CHECK (jsonb_typeof(estimate) = 'object'),
    CONSTRAINT ck_placement_sessions_items_array CHECK (jsonb_typeof(items) = 'array'),
    CONSTRAINT ck_placement_sessions_deadline CHECK (deadline_at > started_at),
    CONSTRAINT ck_placement_sessions_completed_after_started
        CHECK (completed_at IS NULL OR completed_at >= started_at)
);

-- At most one test in progress per learner. Two starts racing both pass the
-- service's check; this is what makes the second one fail.
CREATE UNIQUE INDEX IF NOT EXISTS uq_placement_sessions_one_in_progress
    ON learn.placement_sessions (user_id) WHERE status = 'in_progress';

-- Covers fk_placement_sessions_user and the retake check, which reads the
-- learner's most recent completed session.
CREATE INDEX IF NOT EXISTS idx_placement_sessions_user_completed
    ON learn.placement_sessions (user_id, completed_at DESC);

-- The expiry sweep reads only open sessions past their deadline.
CREATE INDEX IF NOT EXISTS idx_placement_sessions_open_deadline
    ON learn.placement_sessions (deadline_at) WHERE status = 'in_progress';

-- ------------------------------------------------------ placement_results link
ALTER TABLE learn.placement_results
    ADD COLUMN IF NOT EXISTS session_id uuid NULL;

ALTER TABLE learn.placement_results
    ADD CONSTRAINT fk_placement_results_session
    FOREIGN KEY (session_id) REFERENCES learn.placement_sessions (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_placement_results_session ON learn.placement_results (session_id);

ALTER TABLE learn.placement_sessions
    ADD CONSTRAINT fk_placement_sessions_result
    FOREIGN KEY (result_id) REFERENCES learn.placement_results (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_placement_sessions_result ON learn.placement_sessions (result_id);

-- --------------------------------------------------------------- weekly_plans
-- A learner's plan for one week starting Monday in Asia/Ho_Chi_Minh (§3.7). Built
-- on the first request of the week and fixed for the rest of it. Progress is not
-- stored: it is read from learning sessions and attempts when the plan is read.
CREATE TABLE IF NOT EXISTS learn.weekly_plans (
    user_id      uuid        NOT NULL,
    week_start   date        NOT NULL,
    minutes_goal integer     NOT NULL,
    items        jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT pk_weekly_plans PRIMARY KEY (user_id, week_start),
    CONSTRAINT fk_weekly_plans_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_weekly_plans_minutes_goal CHECK (minutes_goal > 0),
    CONSTRAINT ck_weekly_plans_items_array CHECK (jsonb_typeof(items) = 'array')
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.weekly_plans;
ALTER TABLE learn.placement_sessions DROP CONSTRAINT IF EXISTS fk_placement_sessions_result;
ALTER TABLE learn.placement_results DROP CONSTRAINT IF EXISTS fk_placement_results_session;
DROP INDEX IF EXISTS learn.idx_placement_results_session;
ALTER TABLE learn.placement_results DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS learn.placement_sessions;
-- +goose StatementEnd
