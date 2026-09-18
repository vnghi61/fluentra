-- Placement sessions, results and weekly plans: work order 13 §3.5–§3.7.
--
-- Every write to a session is guarded by its version, so two requests that read
-- the same session cannot both advance it: the loser gets no row back.

-- name: CreatePlacementSession :one
INSERT INTO learn.placement_sessions (
    id, user_id, status, stage, started_at, deadline_at, estimate, items
) VALUES (
    @id, @user_id, 'in_progress', @stage, @started_at, @deadline_at, @estimate, @items
)
RETURNING *;

-- name: GetPlacementSession :one
SELECT * FROM learn.placement_sessions WHERE id = $1;

-- name: GetOpenPlacementSession :one
SELECT * FROM learn.placement_sessions WHERE user_id = $1 AND status = 'in_progress';

-- name: GetLatestCompletedPlacementSession :one
SELECT *
FROM learn.placement_sessions
WHERE user_id = $1 AND status = 'completed'
ORDER BY completed_at DESC
LIMIT 1;

-- name: SavePlacementProgress :one
UPDATE learn.placement_sessions
SET stage = @stage,
    estimate = @estimate,
    items = @items,
    version = version + 1,
    updated_at = now()
WHERE id = @id AND version = @version AND status = 'in_progress'
RETURNING *;

-- name: FinishPlacementSession :one
UPDATE learn.placement_sessions
SET status = @status,
    stage = 'done',
    estimate = @estimate,
    items = @items,
    result_id = @result_id,
    completed_at = @completed_at,
    version = version + 1,
    updated_at = now()
WHERE id = @id AND version = @version AND status = 'in_progress'
RETURNING *;

-- name: SavePlacementProductive :one
UPDATE learn.placement_sessions
SET items = @items,
    productive_status = @productive_status,
    productive_deadline_at = @productive_deadline_at,
    version = version + 1,
    updated_at = now()
WHERE id = @id AND version = @version
RETURNING *;

-- name: ListOverduePlacementSessions :many
SELECT id
FROM learn.placement_sessions
WHERE status = 'in_progress' AND deadline_at < @cutoff
ORDER BY deadline_at
LIMIT @max_rows;

-- name: FindPlacementSessionByAttempt :one
-- The writing and speaking attempts of a placement are graded asynchronously;
-- when one is, this finds the session that served it.
SELECT *
FROM learn.placement_sessions
WHERE user_id = @user_id
  AND items @> jsonb_build_array(jsonb_build_object('attempt_id', @attempt_id::text))
LIMIT 1;

-- name: CreateSessionPlacementResult :one
INSERT INTO learn.placement_results (user_id, estimated_level, per_skill, session_id, taken_at)
VALUES (@user_id, @estimated_level, @per_skill, @session_id, @taken_at)
RETURNING *;

-- name: GetPlacementResult :one
SELECT * FROM learn.placement_results WHERE id = $1;

-- name: GetCurrentPlacementResult :one
SELECT *
FROM learn.placement_results
WHERE user_id = $1
ORDER BY taken_at DESC
LIMIT 1;

-- name: UpdatePlacementResultPerSkill :one
UPDATE learn.placement_results
SET per_skill = @per_skill,
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: GetWeeklyPlan :one
SELECT * FROM learn.weekly_plans WHERE user_id = $1 AND week_start = $2;

-- name: CreateWeeklyPlan :one
-- DO NOTHING: the first request of the week builds the plan, and a request that
-- loses the race reads the one that won instead of replacing it.
INSERT INTO learn.weekly_plans (user_id, week_start, minutes_goal, items)
VALUES (@user_id, @week_start, @minutes_goal, @items)
ON CONFLICT (user_id, week_start) DO NOTHING
RETURNING *;

-- name: SumLearningMinutesBetween :one
SELECT COALESCE(SUM(minutes), 0)::int AS minutes
FROM learn.learning_sessions
WHERE user_id = @user_id AND started_at >= @from_time AND started_at < @to_time;

-- name: CountPracticedDailySetsBetween :one
-- A day's practice set counts as done once any of its activities was graded.
SELECT count(*)::int AS practiced
FROM learn.daily_sets d
WHERE d.user_id = @user_id
  AND d.local_date >= @from_date
  AND d.local_date < @to_date
  AND EXISTS (
      SELECT 1
      FROM learn.attempts a
      WHERE a.user_id = d.user_id
        AND a.activity_id = ANY(d.activity_ids)
        AND a.status = 'graded'
        AND a.created_at >= @from_time
  );

-- name: CountAttemptsByGradersBetween :one
SELECT count(*)::int AS attempts
FROM learn.attempts
WHERE user_id = @user_id
  AND grader = ANY(@graders::text[])
  AND status IN ('graded', 'grading')
  AND created_at >= @from_time
  AND created_at < @to_time;
