-- name: ListActiveQuests :many
SELECT * FROM learn.quests WHERE active ORDER BY code;

-- name: UpsertQuest :one
INSERT INTO learn.quests (code, name, description, steps, window_days, reward_xp, active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (code) DO UPDATE SET
    name        = EXCLUDED.name,
    description = EXCLUDED.description,
    steps       = EXCLUDED.steps,
    window_days = EXCLUDED.window_days,
    reward_xp   = EXCLUDED.reward_xp,
    active      = EXCLUDED.active
RETURNING *;

-- name: StartUserQuest :one
-- One live attempt per quest per window (uq_user_quests_window). A second call
-- on the same day returns the attempt already in flight rather than resetting
-- its progress to zero, which is what DO UPDATE on `progress` would do to a
-- learner who reopened the app.
INSERT INTO learn.user_quests (user_id, quest_id, started_on, expires_on)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, quest_id, started_on) DO UPDATE SET
    expires_on = EXCLUDED.expires_on
RETURNING *;

-- name: ListOpenUserQuests :many
SELECT
    q.id AS quest_id, q.code, q.name, q.description, q.steps, q.reward_xp,
    uq.id AS user_quest_id, uq.progress, uq.started_on, uq.expires_on, uq.completed_at
FROM learn.user_quests uq
JOIN learn.quests q ON q.id = uq.quest_id
WHERE uq.user_id = $1
  AND uq.completed_at IS NULL
  AND uq.expires_on >= $2
ORDER BY uq.expires_on, q.code;

-- name: UpdateQuestProgress :one
UPDATE learn.user_quests SET progress = $3
WHERE id = $1 AND user_id = $2 AND completed_at IS NULL
RETURNING *;

-- name: CompleteUserQuest :one
-- Guarded on completed_at so a quest pays its reward once even if two events
-- push it over the line at the same moment.
UPDATE learn.user_quests SET completed_at = now()
WHERE id = $1 AND user_id = $2 AND completed_at IS NULL
RETURNING *;
