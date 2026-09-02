-- +goose Up
-- +goose StatementBegin

-- Reference data: Badge catalogue
-- The badge catalogue is authored reference content required by production and all environments.
-- Inserted idempotently with ON CONFLICT (code) DO UPDATE so re-running is safe.

INSERT INTO learn.badges (code, name, description, criteria, tier) VALUES
    ('first_steps', 'First Steps', 'Earned your first 50 XP.', '{"kind": "xp_total", "threshold": 50}'::jsonb, 'bronze'),
    ('getting_serious', 'Getting Serious', 'Reached 500 XP.', '{"kind": "xp_total", "threshold": 500}'::jsonb, 'bronze'),
    ('two_thousand_club', 'Two Thousand Club', 'Reached 2,000 XP.', '{"kind": "xp_total", "threshold": 2000}'::jsonb, 'silver'),
    ('ten_thousand_club', 'Ten Thousand Club', 'Reached 10,000 XP.', '{"kind": "xp_total", "threshold": 10000}'::jsonb, 'gold'),
    ('level_five', 'Level Five', 'Reached level 5.', '{"kind": "level", "threshold": 5}'::jsonb, 'bronze'),
    ('level_ten', 'Level Ten', 'Reached level 10.', '{"kind": "level", "threshold": 10}'::jsonb, 'silver'),
    ('level_twenty', 'Level Twenty', 'Reached level 20.', '{"kind": "level", "threshold": 20}'::jsonb, 'platinum'),
    ('week_streak', 'Seven Days', 'Studied seven days in a row.', '{"kind": "streak_length", "threshold": 7}'::jsonb, 'bronze'),
    ('month_streak', 'Thirty Days', 'Studied thirty days in a row.', '{"kind": "streak_length", "threshold": 30}'::jsonb, 'gold'),
    ('hundred_streak', 'One Hundred Days', 'Studied a hundred days in a row.', '{"kind": "streak_length", "threshold": 100}'::jsonb, 'platinum')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    criteria = EXCLUDED.criteria,
    tier = EXCLUDED.tier;

-- Reference data: Quest catalogue
INSERT INTO learn.quests (code, name, description, steps, window_days, reward_xp, active) VALUES
    ('daily_practice', 'Daily Practice', 'Complete three activities today.', '[{"code": "complete_activities", "target": 3}]'::jsonb, 1, 30, true),
    ('daily_review', 'Keep It Fresh', 'Finish a review session today.', '[{"code": "complete_reviews", "target": 1}]'::jsonb, 1, 20, true),
    ('weekly_lessons', 'Steady Progress', 'Finish five lessons this week.', '[{"code": "complete_lessons", "target": 5}]'::jsonb, 7, 120, true),
    ('weekly_all_round', 'Well Rounded', 'Three lessons, ten activities and three review sessions this week.', '[{"code": "complete_lessons", "target": 3}, {"code": "complete_activities", "target": 10}, {"code": "complete_reviews", "target": 3}]'::jsonb, 7, 200, true)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    steps = EXCLUDED.steps,
    window_days = EXCLUDED.window_days,
    reward_xp = EXCLUDED.reward_xp,
    active = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM learn.quests WHERE code IN ('daily_practice', 'daily_review', 'weekly_lessons', 'weekly_all_round');
DELETE FROM learn.badges WHERE code IN ('first_steps', 'getting_serious', 'two_thousand_club', 'ten_thousand_club', 'level_five', 'level_ten', 'level_twenty', 'week_streak', 'month_streak', 'hundred_streak');
-- +goose StatementEnd
