-- name: UpsertLeaderboardEntry :one
-- The weekly build re-runs — on a retry, or on a redeployed worker — and must
-- correct a standing rather than fail on the unique key.
INSERT INTO learn.leaderboard_snapshots (league, week_start, user_id, xp, rank)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (league, week_start, user_id) DO UPDATE SET
    xp          = EXCLUDED.xp,
    rank        = EXCLUDED.rank,
    captured_at = now()
RETURNING *;

-- name: ListLeaderboard :many
SELECT * FROM learn.leaderboard_snapshots
WHERE league = $1 AND week_start = $2
ORDER BY rank
LIMIT $3;

-- name: GetLeaderboardEntry :one
-- A learner's own standing, so the screen can show their position without
-- paging through the whole league to find them.
SELECT * FROM learn.leaderboard_snapshots
WHERE user_id = $1 AND week_start = $2;

-- name: DeleteLeaderboardBefore :exec
-- Snapshots are a display artefact, not a record: keeping every week for ever
-- grows a table nothing reads. The XP events they were built from are the
-- durable history.
DELETE FROM learn.leaderboard_snapshots WHERE week_start < $1;
