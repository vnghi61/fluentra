-- name: CreateSession :one
INSERT INTO core.sessions (id, user_id, ip_hash, user_agent_hash, created_at, last_seen_at)
VALUES ($1, $2, $3, $4, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz)
RETURNING id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at;

-- name: GetSession :one
SELECT id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at
FROM core.sessions
WHERE id = $1;

-- name: TouchSession :execrows
UPDATE core.sessions
SET last_seen_at = sqlc.arg(now)::timestamptz
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeSession :execrows
UPDATE core.sessions
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE id = $1 AND revoked_at IS NULL;
