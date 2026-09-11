-- +goose Up
-- +goose StatementBegin

-- The level a learner's daily practice set is drawn at (work order 11 §3.11).
-- The profile's declared_level is written by nothing, so the learner picks this
-- the first time they open the set, and it is kept here rather than in one
-- browser: a level chosen on a phone has to be the level on a laptop.
--
-- Nullable, because "not chosen yet" is the state that asks the question. The
-- practice pool holds A2, B1 and B2 only, so the other CEFR levels are refused.
ALTER TABLE core.user_preferences
    ADD COLUMN IF NOT EXISTS practice_level core.cefr_level;

ALTER TABLE core.user_preferences
    ADD CONSTRAINT ck_user_preferences_practice_level CHECK (
        practice_level IS NULL OR practice_level IN ('a2', 'b1', 'b2')
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE core.user_preferences DROP CONSTRAINT IF EXISTS ck_user_preferences_practice_level;
ALTER TABLE core.user_preferences DROP COLUMN IF EXISTS practice_level;
-- +goose StatementEnd
