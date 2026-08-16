-- name: CreateOAuthState :one
INSERT INTO core.oauth_states (
    id, state, provider, nonce, pkce_verifier_hash, redirect_to, created_at, expires_at
)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(redirect_to), sqlc.arg(now)::timestamptz,
        sqlc.arg(expires_at)::timestamptz)
RETURNING id, state, provider, nonce, pkce_verifier_hash, redirect_to, created_at, expires_at, consumed_at;

-- ConsumeOAuthState spends a state and returns it, in one statement.
--
-- The guard is the same shape as the refresh claim, and for the same reason: the
-- question "is this state still usable?" and the act of using it must not have a
-- gap between them, or two callbacks arriving together both pass the check. A
-- state is the CSRF token for the whole flow (BR-AUTH-17), so "used twice" has to
-- be impossible rather than unlikely.
--
-- name: ConsumeOAuthState :one
UPDATE core.oauth_states
SET consumed_at = sqlc.arg(now)::timestamptz
WHERE state = $1
  AND provider = $2
  AND consumed_at IS NULL
  AND expires_at > sqlc.arg(now)::timestamptz
RETURNING id, state, provider, nonce, pkce_verifier_hash, redirect_to, created_at, expires_at, consumed_at;

-- name: DeleteExpiredOAuthStates :execrows
DELETE FROM core.oauth_states
WHERE expires_at < sqlc.arg(cutoff)::timestamptz;

-- name: CreateOAuthIdentity :one
INSERT INTO core.oauth_identities (id, user_id, provider, subject, email_hash, linked_at)
VALUES ($1, $2, $3, $4, $5, sqlc.arg(now)::timestamptz)
RETURNING id, user_id, provider, subject, email_hash, linked_at;

-- name: GetOAuthIdentityBySubject :one
SELECT id, user_id, provider, subject, email_hash, linked_at
FROM core.oauth_identities
WHERE provider = $1 AND subject = $2;

-- name: GetOAuthIdentityByUser :one
SELECT id, user_id, provider, subject, email_hash, linked_at
FROM core.oauth_identities
WHERE user_id = $1 AND provider = $2;

-- name: DeleteOAuthIdentity :execrows
DELETE FROM core.oauth_identities
WHERE user_id = $1 AND provider = $2;

-- CountSignInMethods is the last-sign-in-method guard (BR-AUTH-20).
--
-- It counts what an account can actually sign in with: a password, and each
-- linked identity. Unlinking the last one would lock the learner out with no
-- recovery path, because `forgot-password` cannot help somebody who has no
-- password to reset.
--
-- name: CountSignInMethods :one
SELECT
    (SELECT count(*) FROM core.credentials c WHERE c.user_id = sqlc.arg(user_id))
    + (SELECT count(*) FROM core.oauth_identities i WHERE i.user_id = sqlc.arg(user_id)) AS methods;

-- name: DeleteOAuthIdentitiesForUser :exec
DELETE FROM core.oauth_identities
WHERE user_id = $1;

