-- +goose Up
ALTER TABLE learn.daily_sets ADD COLUMN cefr_level text NOT NULL DEFAULT ''
    CHECK (cefr_level IN ('', 'A2', 'B1', 'B2'));
-- Old sets have no trustworthy level. Keep their history, but do not serve them
-- under a newly selected label.
ALTER TABLE learn.daily_sets DROP CONSTRAINT uq_daily_sets_user_date;
ALTER TABLE learn.daily_sets ADD CONSTRAINT uq_daily_sets_user_date_level
    UNIQUE (user_id, local_date, cefr_level);

-- +goose Down
-- Refuse to discard any sets: rollback is possible only without duplicate days.
ALTER TABLE learn.daily_sets ADD CONSTRAINT uq_daily_sets_user_date UNIQUE (user_id, local_date);
ALTER TABLE learn.daily_sets DROP CONSTRAINT uq_daily_sets_user_date_level;
ALTER TABLE learn.daily_sets DROP COLUMN cefr_level;
