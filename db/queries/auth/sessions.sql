-- name: CreateSession :one
INSERT INTO core.sessions (
    id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at,
    absolute_expires_at, idle_window, trusted_device_id
)
VALUES (
    $1, $2, $3, $4, $5, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz,
    sqlc.arg(absolute_expires_at)::timestamptz, sqlc.arg(idle_window)::interval,
    sqlc.narg(trusted_device_id)
)
RETURNING id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at,
          absolute_expires_at, idle_window, trusted_device_id;

-- name: GetSession :one
SELECT id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at,
       absolute_expires_at, idle_window, trusted_device_id
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

-- ListLiveSessions answers "where am I signed in now".
--
-- Scoped by user_id in the WHERE clause rather than filtered after the read, so
-- there is no version of this that returns another account's row for a caller to
-- accidentally render.
--
-- name: ListLiveSessions :many
SELECT id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at,
       absolute_expires_at, idle_window, trusted_device_id
FROM core.sessions
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY last_seen_at DESC;

-- GetOwnedSession is the ownership check behind the 404.
--
-- Both the id and the owner are in the WHERE clause, so "no such session" and
-- "somebody else's session" produce the identical empty result and the caller
-- cannot tell them apart. Filtering in Go after a lookup by id alone would leave
-- a branch where the two differ, which is the enumeration oracle this operation
-- exists to not be.
--
-- name: GetOwnedSession :one
SELECT id, user_id, device_label, ip_hash, user_agent_hash, created_at, last_seen_at, revoked_at,
       absolute_expires_at, idle_window, trusted_device_id
FROM core.sessions
WHERE id = $1 AND user_id = $2;

-- name: RevokeAllSessionsForUser :execrows
UPDATE core.sessions
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokensBySession :execrows
UPDATE core.refresh_tokens
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE session_id = $1 AND revoked_at IS NULL;

-- RevokeRefreshTokensForUser reaches the tokens through their session, because
-- core.refresh_tokens has no user_id of its own -- the session is what an
-- account owns, and duplicating the owner onto every token row would be a second
-- copy of a fact that could disagree with the first.
--
-- name: RevokeRefreshTokensForUser :execrows
UPDATE core.refresh_tokens rt
SET revoked_at = sqlc.arg(now)::timestamptz
FROM core.sessions s
WHERE s.id = rt.session_id AND s.user_id = $1 AND rt.revoked_at IS NULL;

-- RevokeOtherSessionsForUser is what a password change uses: every device but
-- the one the change was made from.
--
-- name: RevokeOtherSessionsForUser :execrows
UPDATE core.sessions
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = $1 AND id <> sqlc.arg(keep_session_id) AND revoked_at IS NULL;

-- name: RevokeRefreshTokensForOtherSessions :execrows
UPDATE core.refresh_tokens rt
SET revoked_at = sqlc.arg(now)::timestamptz
FROM core.sessions s
WHERE s.id = rt.session_id
  AND s.user_id = $1
  AND s.id <> sqlc.arg(keep_session_id)
  AND rt.revoked_at IS NULL;

-- name: RevokeSessionsForDevice :execrows
UPDATE core.sessions
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE trusted_device_id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokensForDevice :execrows
UPDATE core.refresh_tokens rt
SET revoked_at = sqlc.arg(now)::timestamptz
FROM core.sessions s
WHERE s.id = rt.session_id AND s.trusted_device_id = $1 AND rt.revoked_at IS NULL;

-- name: DeleteSessionsForUser :exec
DELETE FROM core.sessions
WHERE user_id = $1;

-- name: DeleteRefreshTokensForUser :exec
DELETE FROM core.refresh_tokens rt
USING core.sessions s
WHERE s.id = rt.session_id AND s.user_id = $1;

