-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- xp_events
--
-- Every XP award, one row, never updated.
--
-- NOT partitioned, and that is a deliberate departure from the module spec,
-- which asks for monthly partitions *and* a unique constraint on
-- (user_id, source, source_id) for idempotency. PostgreSQL cannot give both:
-- a unique constraint on a partitioned table must contain the partition key,
-- so the constraint would have to become (user_id, source, source_id,
-- awarded_at) — which stops deduplicating the moment a redelivery lands a
-- microsecond later, i.e. always. BR-GAMIFICATION-01 says a redelivered event
-- must not double-award, and that rule is worth more than a partition this
-- table does not yet need: review_logs is partitioned because it takes a row
-- per card per review, while this takes at most a handful per learner per day
-- and is capped by BR-GAMIFICATION-05 on top of that.
--
-- Partition it when the volume argues for it, and carry the idempotency key in
-- a separate unpartitioned table at that point.
CREATE TABLE IF NOT EXISTS learn.xp_events (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL,
    source     text        NOT NULL,
    -- The id of the thing that earned the XP: an activity, a lesson, a review
    -- session, an upload verification. Text rather than uuid because not every
    -- source has one — a daily-goal award is keyed by the date.
    source_id  text        NOT NULL,
    amount     integer     NOT NULL,
    multiplier numeric(4, 2) NOT NULL DEFAULT 1.00,
    awarded_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_xp_events_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    -- BR-GAMIFICATION-01. The award path inserts ON CONFLICT DO NOTHING, so a
    -- redelivered event is a no-op rather than a second award.
    CONSTRAINT uq_xp_events_idempotency UNIQUE (user_id, source, source_id),
    CONSTRAINT ck_xp_events_amount CHECK (amount >= 0),
    CONSTRAINT ck_xp_events_multiplier CHECK (multiplier > 0.0 AND multiplier <= 10.0)
);

-- The two reads this table serves: a learner's running total, and their awards
-- inside one day for the daily cap.
CREATE INDEX IF NOT EXISTS idx_xp_events_user_time ON learn.xp_events (user_id, awarded_at DESC);
CREATE INDEX IF NOT EXISTS idx_xp_events_user_source_time
    ON learn.xp_events (user_id, source, awarded_at DESC);

-- --------------------------------------------------------------- streaks
--
-- One row per learner, and the row every per-learner gamification setting hangs
-- off. `daily_goal_xp` and `leaderboard_opt_in` live here rather than in a
-- table of their own because there is exactly one of each per learner and the
-- streak row is already that: a second table keyed by user_id would be the same
-- row under another name. They are not in `core.user_preferences` because that
-- table belongs to `user`, and L2 forbids reaching into it.
CREATE TABLE IF NOT EXISTS learn.streaks (
    user_id            uuid        PRIMARY KEY,
    current_length     integer     NOT NULL DEFAULT 0,
    longest_length     integer     NOT NULL DEFAULT 0,
    -- A date, not a timestamp: the boundary is the learner's own local day
    -- (BR-GAMIFICATION-02), so the day is resolved on write and stored as one.
    last_active_on     date,
    freezes_available  integer     NOT NULL DEFAULT 2,
    freeze_used_on     date,
    daily_goal_xp      integer     NOT NULL DEFAULT 50,
    leaderboard_opt_in boolean     NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_streaks_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_streaks_current CHECK (current_length >= 0),
    CONSTRAINT ck_streaks_longest CHECK (longest_length >= current_length),
    CONSTRAINT ck_streaks_freezes CHECK (freezes_available >= 0 AND freezes_available <= 5),
    CONSTRAINT ck_streaks_daily_goal CHECK (daily_goal_xp > 0 AND daily_goal_xp <= 1000)
);

-- Drives the daily streak sweep, which looks only at learners with a live streak.
CREATE INDEX IF NOT EXISTS idx_streaks_active
    ON learn.streaks (last_active_on) WHERE current_length > 0;
CREATE INDEX IF NOT EXISTS idx_streaks_leaderboard
    ON learn.streaks (user_id) WHERE leaderboard_opt_in;

-- ---------------------------------------------------------------- badges
CREATE TABLE IF NOT EXISTS learn.badges (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text        NOT NULL,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    -- {"kind": "xp_total", "threshold": 1000} and similar. The evaluator reads
    -- `kind` and dispatches; an unknown kind awards nothing rather than failing.
    criteria    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    tier        text        NOT NULL DEFAULT 'bronze',
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_badges_code UNIQUE (code),
    CONSTRAINT ck_badges_tier CHECK (tier IN ('bronze', 'silver', 'gold', 'platinum'))
);

-- --------------------------------------------------------- badges_earned
CREATE TABLE IF NOT EXISTS learn.badges_earned (
    id        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   uuid        NOT NULL,
    badge_id  uuid        NOT NULL,
    earned_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_badges_earned_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_badges_earned_badge FOREIGN KEY (badge_id) REFERENCES learn.badges (id) ON DELETE CASCADE,
    -- BR-GAMIFICATION-06: the evaluator re-runs on many events, and inserts
    -- ON CONFLICT DO NOTHING against this.
    CONSTRAINT uq_badges_earned UNIQUE (user_id, badge_id)
);

CREATE INDEX IF NOT EXISTS idx_badges_earned_user ON learn.badges_earned (user_id, earned_at DESC);
-- uq_badges_earned leads with user_id, so nothing covers the badge foreign key.
CREATE INDEX IF NOT EXISTS idx_badges_earned_badge ON learn.badges_earned (badge_id);

-- ---------------------------------------------------------------- quests
CREATE TABLE IF NOT EXISTS learn.quests (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text        NOT NULL,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    -- [{"code": "complete_lessons", "target": 3}, …]
    steps       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    -- `window_days`, not `window`: WINDOW is a reserved word in SQL, and a
    -- column that has to be quoted in every statement is a column that will
    -- eventually not be.
    window_days integer     NOT NULL DEFAULT 1,
    reward_xp   integer     NOT NULL DEFAULT 0,
    active      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_quests_code UNIQUE (code),
    CONSTRAINT ck_quests_window CHECK (window_days > 0 AND window_days <= 365),
    CONSTRAINT ck_quests_reward CHECK (reward_xp >= 0)
);

CREATE INDEX IF NOT EXISTS idx_quests_active ON learn.quests (code) WHERE active;

-- ----------------------------------------------------------- user_quests
CREATE TABLE IF NOT EXISTS learn.user_quests (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL,
    quest_id     uuid        NOT NULL,
    -- {"complete_lessons": 2} — counters against the quest's steps.
    progress     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    started_on   date        NOT NULL DEFAULT CURRENT_DATE,
    expires_on   date        NOT NULL,
    completed_at timestamptz,

    CONSTRAINT fk_user_quests_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_quests_quest FOREIGN KEY (quest_id) REFERENCES learn.quests (id) ON DELETE CASCADE,
    -- One live attempt per quest per window.
    CONSTRAINT uq_user_quests_window UNIQUE (user_id, quest_id, started_on),
    CONSTRAINT ck_user_quests_window CHECK (expires_on >= started_on)
);

CREATE INDEX IF NOT EXISTS idx_user_quests_open
    ON learn.user_quests (user_id, expires_on) WHERE completed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_quests_quest ON learn.user_quests (quest_id);

-- ------------------------------------------------- leaderboard_snapshots
--
-- Materialised weekly (BR-GAMIFICATION-07). A snapshot rather than a live
-- ranking so the standings a learner sees do not shuffle under them mid-week,
-- and so ranking never runs a full-table sort on the request path.
CREATE TABLE IF NOT EXISTS learn.leaderboard_snapshots (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    league      text        NOT NULL,
    -- The ISO week the standing covers, stored as its Monday.
    week_start  date        NOT NULL,
    user_id     uuid        NOT NULL,
    xp          integer     NOT NULL DEFAULT 0,
    rank        integer     NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_leaderboard_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_leaderboard_entry UNIQUE (league, week_start, user_id),
    CONSTRAINT ck_leaderboard_rank CHECK (rank > 0),
    CONSTRAINT ck_leaderboard_xp CHECK (xp >= 0),
    CONSTRAINT ck_leaderboard_league CHECK (league IN ('bronze', 'silver', 'gold', 'diamond'))
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_standings
    ON learn.leaderboard_snapshots (league, week_start, rank);
CREATE INDEX IF NOT EXISTS idx_leaderboard_user
    ON learn.leaderboard_snapshots (user_id, week_start DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.leaderboard_snapshots CASCADE;
DROP TABLE IF EXISTS learn.user_quests CASCADE;
DROP TABLE IF EXISTS learn.quests CASCADE;
DROP TABLE IF EXISTS learn.badges_earned CASCADE;
DROP TABLE IF EXISTS learn.badges CASCADE;
DROP TABLE IF EXISTS learn.streaks CASCADE;
DROP TABLE IF EXISTS learn.xp_events CASCADE;
-- +goose StatementEnd
