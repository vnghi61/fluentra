-- name: LogAdminAction :one
INSERT INTO core.admin_actions (
    actor_id,
    target_id,
    action,
    reason
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: ListAdminActionsByTarget :many
SELECT *
FROM core.admin_actions
WHERE target_id = $1
ORDER BY occurred_at DESC
LIMIT $2 OFFSET $3;
