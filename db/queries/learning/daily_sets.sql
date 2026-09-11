-- Daily practice sets and item exposures: work order 11 §3.11.
--
-- Only the two tables learning owns here are read. The pool's course, units,
-- lessons and activities belong to `lesson` and are reached through its contract
-- (rule L2): the service resolves which activities make up a slot and passes their
-- ids to these queries.

-- name: GetDailySet :one
SELECT id, user_id, local_date, activity_ids, created_at
FROM learn.daily_sets
WHERE user_id = $1 AND local_date = $2;

-- name: CreateDailySet :one
-- DO NOTHING, not DO UPDATE. A request that loses the race for a learner's first
-- open of the day gets no row back, and so knows not to record exposures for the
-- items it drew: they were never shown.
INSERT INTO learn.daily_sets (
    user_id, local_date, activity_ids
) VALUES (
    $1, $2, $3
) ON CONFLICT (user_id, local_date) DO NOTHING
RETURNING id, user_id, local_date, activity_ids, created_at;

-- name: RecordItemExposure :exec
-- first_served_at moves forward when an item is served again, so "the items a
-- learner saw longest ago" means longest since they last saw them, and a repeated
-- item goes to the back of the queue instead of coming round every day.
INSERT INTO learn.item_exposures (
    user_id, activity_id, first_served_at
) VALUES (
    $1, $2, now()
) ON CONFLICT (user_id, activity_id) DO UPDATE
SET first_served_at = now();

-- name: ListItemExposures :many
SELECT activity_id, first_served_at
FROM learn.item_exposures
WHERE user_id = @user_id
  AND activity_id = ANY(@activity_ids::uuid[]);

-- name: HasActiveLearnerRunningLow :one
-- True when a learner active in the last fourteen days has fewer than @threshold
-- of the given activities left unseen.
SELECT EXISTS (
    SELECT 1
    FROM (
        SELECT DISTINCT user_id
        FROM learn.attempts
        WHERE created_at >= now() - interval '14 days'
    ) active
    WHERE cardinality(@activity_ids::uuid[]) - (
        SELECT count(*)
        FROM learn.item_exposures e
        WHERE e.user_id = active.user_id
          AND e.activity_id = ANY(@activity_ids::uuid[])
    ) < @threshold::int
) AS running_low;
