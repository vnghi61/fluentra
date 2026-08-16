-- name: CreateExportRequest :one
INSERT INTO core.user_exports (
    id,
    user_id,
    status,
    requested_at
) VALUES (
    $1,
    $2,
    'pending',
    now()
) RETURNING *;

-- name: GetPendingExportForUser :one
SELECT * FROM core.user_exports
WHERE user_id = $1
  AND status IN ('pending', 'processing')
ORDER BY requested_at DESC
LIMIT 1;

-- name: GetExportByID :one
SELECT * FROM core.user_exports
WHERE id = $1;

-- name: UpdateExportStatus :exec
UPDATE core.user_exports
SET
    status = $2,
    started_at = COALESCE($3, started_at),
    completed_at = COALESCE($4, completed_at),
    expires_at = COALESCE($5, expires_at),
    object_key = COALESCE($6, object_key),
    error_message = COALESCE($7, error_message),
    updated_at = now()
WHERE id = $1;

-- name: GetExpiredExports :many
SELECT * FROM core.user_exports
WHERE status = 'completed'
  AND expires_at < now()
ORDER BY expires_at
LIMIT $1;

-- name: DeleteExport :exec
DELETE FROM core.user_exports WHERE id = $1;
