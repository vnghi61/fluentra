-- name: CreateDeletionRequest :one
INSERT INTO core.user_deletions (
    id,
    user_id,
    status,
    requested_at,
    execute_at,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    now(),
    now()
)
RETURNING *;

-- name: GetPendingDeletionByUserID :one
SELECT * FROM core.user_deletions
WHERE user_id = $1 AND status IN ('pending', 'processing')
ORDER BY created_at DESC
LIMIT 1;

-- name: GetDeletionByID :one
SELECT * FROM core.user_deletions
WHERE id = $1;

-- name: CancelDeletion :one
UPDATE core.user_deletions
SET status = 'cancelled',
    cancelled_at = $2,
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateDeletionStatus :exec
UPDATE core.user_deletions
SET status = $2,
    started_at = COALESCE($3, started_at),
    completed_at = COALESCE($4, completed_at),
    error_message = COALESCE($5, error_message),
    updated_at = now()
WHERE id = $1;

-- name: GetDueDeletions :many
SELECT * FROM core.user_deletions
WHERE status = 'pending' AND execute_at <= $1
ORDER BY execute_at ASC
LIMIT $2;

-- name: GetProcessingDeletions :many
SELECT * FROM core.user_deletions
WHERE status = 'processing'
ORDER BY updated_at ASC
LIMIT $1;

-- name: AnonymiseUser :exec
UPDATE core.users
SET email = $2,
    status = 'deleted',
    updated_at = now()
WHERE id = $1;

-- name: AnonymiseProfile :exec
UPDATE core.profiles
SET display_name = 'Deleted User',
    avatar_asset_id = NULL,
    country = NULL,
    timezone = 'UTC',
    date_of_birth = NULL
WHERE user_id = $1;

-- name: DeleteUserPreferences :exec
DELETE FROM core.user_preferences
WHERE user_id = $1;

-- name: DeleteLearningProfile :exec
DELETE FROM core.learning_profiles
WHERE user_id = $1;

