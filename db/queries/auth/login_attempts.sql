-- name: RecordLoginAttempt :one
INSERT INTO core.login_attempts (id, user_id, email_hash, ip_hash, success, failure_reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, email_hash, ip_hash, success, failure_reason, created_at;

-- name: CountRecentFailedAttemptsByAccount :one
SELECT COUNT(*)
FROM core.login_attempts
WHERE email_hash = $1 AND success = false AND created_at >= $2;

-- name: CountRecentFailedAttemptsByIP :one
SELECT COUNT(*)
FROM core.login_attempts
WHERE ip_hash = $1 AND success = false AND created_at >= $2;
