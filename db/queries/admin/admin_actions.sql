-- name: LogAdminAction :one
INSERT INTO core.admin_actions (
    actor_id,
    target_id,
    action,
    reason
) VALUES (
    $1, $2, $3, $4
) RETURNING *;
