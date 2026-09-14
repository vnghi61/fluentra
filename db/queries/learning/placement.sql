-- Placement test sessions and weekly plans: work order 13 §3.5, §3.6.

-- name: GetActivePlacementSessionByUser :one
SELECT id, user_id, status, stage, theta_estimate, placed_level, confidence,
       current_activity_id, current_item_kind, current_item_level,
       responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at
FROM learn.placement_sessions
WHERE user_id = $1 AND status = 'in_progress';

-- name: GetPlacementSessionByID :one
SELECT id, user_id, status, stage, theta_estimate, placed_level, confidence,
       current_activity_id, current_item_kind, current_item_level,
       responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at
FROM learn.placement_sessions
WHERE id = $1;

-- name: GetLatestCompletedPlacementSession :one
SELECT id, user_id, status, stage, theta_estimate, placed_level, confidence,
       current_activity_id, current_item_kind, current_item_level,
       responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at
FROM learn.placement_sessions
WHERE user_id = $1 AND status = 'completed'
ORDER BY completed_at DESC
LIMIT 1;

-- name: CreatePlacementSession :one
INSERT INTO learn.placement_sessions (
    user_id, status, stage, theta_estimate, placed_level, confidence,
    current_activity_id, current_item_kind, current_item_level,
    responses, adaptive_state, started_at, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9,
    $10, $11, $12, $13
)
RETURNING id, user_id, status, stage, theta_estimate, placed_level, confidence,
          current_activity_id, current_item_kind, current_item_level,
          responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at;

-- name: UpdatePlacementSessionProgress :one
UPDATE learn.placement_sessions
SET stage = $2,
    theta_estimate = $3,
    placed_level = $4,
    confidence = $5,
    current_activity_id = $6,
    current_item_kind = $7,
    current_item_level = $8,
    responses = $9,
    adaptive_state = $10,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, status, stage, theta_estimate, placed_level, confidence,
          current_activity_id, current_item_kind, current_item_level,
          responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at;

-- name: CompletePlacementSession :one
UPDATE learn.placement_sessions
SET status = 'completed',
    stage = 'completed',
    theta_estimate = $2,
    placed_level = $3,
    confidence = $4,
    current_activity_id = NULL,
    current_item_kind = NULL,
    current_item_level = NULL,
    responses = $5,
    adaptive_state = $6,
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, status, stage, theta_estimate, placed_level, confidence,
          current_activity_id, current_item_kind, current_item_level,
          responses, adaptive_state, started_at, completed_at, expires_at, created_at, updated_at;

-- name: ExpireStalePlacementSessions :execrows
UPDATE learn.placement_sessions
SET status = 'expired',
    updated_at = now()
WHERE status = 'in_progress' AND expires_at < now();

-- name: CreatePlacementResultWithSession :one
INSERT INTO learn.placement_results (
    user_id, estimated_level, per_skill, session_id, taken_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, user_id, estimated_level, per_skill, session_id, taken_at, created_at, updated_at;

-- name: GetWeeklyPlanByUserAndDate :one
SELECT id, user_id, week_start_date, placed_level, weakest_skill, time_distribution, daily_targets, created_at, updated_at
FROM learn.weekly_plans
WHERE user_id = $1 AND week_start_date = $2;

-- name: UpsertWeeklyPlan :one
INSERT INTO learn.weekly_plans (
    user_id, week_start_date, placed_level, weakest_skill, time_distribution, daily_targets
) VALUES (
    $1, $2, $3, $4, $5, $6
) ON CONFLICT (user_id, week_start_date) DO UPDATE
SET placed_level = EXCLUDED.placed_level,
    weakest_skill = EXCLUDED.weakest_skill,
    time_distribution = EXCLUDED.time_distribution,
    daily_targets = EXCLUDED.daily_targets,
    updated_at = now()
RETURNING id, user_id, week_start_date, placed_level, weakest_skill, time_distribution, daily_targets, created_at, updated_at;
