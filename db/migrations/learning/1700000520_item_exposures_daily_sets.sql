-- +goose Up
-- +goose StatementBegin

-- ----------------------------------------------------------- item_exposures
-- Tracks which activities have been presented to which learners.
-- An item shown counts as seen whether or not it was answered (§3.11).
CREATE TABLE IF NOT EXISTS learn.item_exposures (
    user_id         uuid        NOT NULL,
    activity_id     uuid        NOT NULL,
    first_served_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT pk_item_exposures PRIMARY KEY (user_id, activity_id),
    CONSTRAINT fk_item_exposures_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_item_exposures_activity FOREIGN KEY (activity_id) REFERENCES learn.activities (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_item_exposures_user_served
    ON learn.item_exposures (user_id, first_served_at ASC);

-- --------------------------------------------------------------- daily_sets
-- Caches each learner's daily practice set for a given local date in Asia/Ho_Chi_Minh.
-- Built on first open; a set nobody opens costs nothing (§3.11).
CREATE TABLE IF NOT EXISTS learn.daily_sets (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL,
    local_date   date        NOT NULL,
    activity_ids uuid[]      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_daily_sets_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT uq_daily_sets_user_date UNIQUE (user_id, local_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_sets_user_date
    ON learn.daily_sets (user_id, local_date);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.daily_sets;
DROP TABLE IF EXISTS learn.item_exposures;
-- +goose StatementEnd
