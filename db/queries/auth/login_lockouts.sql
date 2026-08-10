-- name: GetActiveLoginLockout :one
SELECT locked_until
FROM core.login_lockouts
WHERE scope = $1 AND subject_hash = $2 AND locked_until > $3;

-- name: AdvanceLoginLockout :one
INSERT INTO core.login_lockouts (scope, subject_hash, lockout_level, locked_until)
VALUES ($1, $2, 1, sqlc.arg(now)::timestamptz + INTERVAL '15 minutes')
ON CONFLICT (scope, subject_hash) DO UPDATE
SET lockout_level = core.login_lockouts.lockout_level + 1,
    locked_until = sqlc.arg(now)::timestamptz + make_interval(secs => LEAST(
        900::double precision * power(2, core.login_lockouts.lockout_level),
        86400::double precision
    )),
    updated_at = sqlc.arg(now)::timestamptz
WHERE core.login_lockouts.locked_until <= sqlc.arg(now)::timestamptz
RETURNING locked_until;
