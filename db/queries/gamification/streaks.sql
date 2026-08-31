-- name: GetStreak :one
SELECT * FROM learn.streaks WHERE user_id = $1;

-- name: EnsureStreak :one
-- Every learner has a streak row from the first time gamification looks at
-- them, so nothing downstream has to handle its absence.
INSERT INTO learn.streaks (user_id) VALUES ($1)
ON CONFLICT (user_id) DO UPDATE SET updated_at = learn.streaks.updated_at
RETURNING *;

-- name: ExtendStreak :one
-- Records a day on which the learner met their goal.
--
-- The decision of whether the day continues the streak or restarts it is the
-- domain's, in the learner's own timezone, and arrives here as
-- `new_length`. This statement does not recompute it: a date subtraction in
-- SQL is in the server's timezone, which is how a learner who travels loses a
-- streak they did not break (BR-GAMIFICATION-02).
--
-- `last_active_on` guards the write. A second call for a day already recorded
-- changes nothing, so a redelivered session event cannot advance the streak
-- twice.
UPDATE learn.streaks SET
    current_length = $2,
    longest_length = GREATEST(longest_length, $2),
    last_active_on = $3,
    updated_at     = now()
WHERE user_id = $1
  AND (last_active_on IS NULL OR last_active_on < $3)
RETURNING *;

-- name: BreakStreak :one
UPDATE learn.streaks SET
    current_length = 0,
    updated_at     = now()
WHERE user_id = $1
RETURNING *;

-- name: ConsumeFreeze :one
-- BR-GAMIFICATION-04. Both the count and the day are guarded in the WHERE
-- clause, so two concurrent callers cannot spend the same freeze and a freeze
-- cannot be spent twice in one day.
UPDATE learn.streaks SET
    freezes_available = freezes_available - 1,
    freeze_used_on    = $2,
    last_active_on    = $2,
    updated_at        = now()
WHERE user_id = $1
  AND freezes_available > 0
  AND (freeze_used_on IS NULL OR freeze_used_on < $2)
RETURNING *;

-- name: GrantFreeze :one
-- Replenishment, capped by ck_streaks_freezes.
UPDATE learn.streaks SET
    freezes_available = LEAST(freezes_available + $2, 5),
    updated_at        = now()
WHERE user_id = $1
RETURNING *;

-- name: SetDailyGoal :one
UPDATE learn.streaks SET
    daily_goal_xp = $2,
    updated_at    = now()
WHERE user_id = $1
RETURNING *;

-- name: SetLeaderboardOptIn :one
UPDATE learn.streaks SET
    leaderboard_opt_in = $2,
    updated_at         = now()
WHERE user_id = $1
RETURNING *;

-- name: ListStreaksAtRisk :many
-- Live streaks whose last active day is older than the cutoff: the sweep's
-- input. A learner with no streak has nothing to break and is not selected.
SELECT * FROM learn.streaks
WHERE current_length > 0
  AND (last_active_on IS NULL OR last_active_on < $1)
ORDER BY user_id
LIMIT $2;
