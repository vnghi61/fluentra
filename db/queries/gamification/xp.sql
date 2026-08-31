-- name: AwardXP :one
-- BR-GAMIFICATION-01. The conflict path is DO NOTHING, so a redelivered event
-- inserts nothing and the caller sees no row — which is exactly the signal it
-- needs to skip publishing xp_awarded a second time. DO UPDATE would return a
-- row and make a redelivery indistinguishable from a first award.
INSERT INTO learn.xp_events (user_id, source, source_id, amount, multiplier)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, source, source_id) DO NOTHING
RETURNING *;

-- name: TotalXP :one
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM learn.xp_events
WHERE user_id = $1;

-- name: XPSince :one
-- The learner's XP inside a window. Serves both the daily cap
-- (BR-GAMIFICATION-05) and the weekly leaderboard build.
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM learn.xp_events
WHERE user_id = $1 AND awarded_at >= $2;

-- name: XPFromSourceSince :one
-- The per-source daily cap. Capping the total instead would let a single
-- source crowd out every other way of earning.
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM learn.xp_events
WHERE user_id = $1 AND source = $2 AND awarded_at >= $3;

-- name: CountAwardsFromSourceSince :one
-- How many times this source has paid out in the window, for the diminishing
-- returns curve.
SELECT COUNT(*)::bigint AS awards
FROM learn.xp_events
WHERE user_id = $1 AND source = $2 AND awarded_at >= $3;

-- name: ListXPEvents :many
SELECT * FROM learn.xp_events
WHERE user_id = $1
ORDER BY awarded_at DESC, id DESC
LIMIT $2;

-- name: WeeklyXPStandings :many
-- The raw ranking a leaderboard snapshot is built from.
--
-- Opted-in learners only (BR-GAMIFICATION-07): a learner who has not opted in
-- is not ranked, not stored, and cannot appear in anyone else's standings.
SELECT
    e.user_id,
    COALESCE(SUM(e.amount), 0)::integer AS xp
FROM learn.xp_events e
JOIN learn.streaks s ON s.user_id = e.user_id AND s.leaderboard_opt_in
WHERE e.awarded_at >= $1 AND e.awarded_at < $2
GROUP BY e.user_id
ORDER BY xp DESC, e.user_id ASC
LIMIT $3;
