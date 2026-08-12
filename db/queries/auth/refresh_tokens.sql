-- name: CreateRefreshToken :one
INSERT INTO core.refresh_tokens (id, token_hash, family_id, session_id, issued_at, expires_at)
VALUES ($1, $2, $3, $4, sqlc.arg(now)::timestamptz, sqlc.arg(expires_at)::timestamptz)
RETURNING id, token_hash, family_id, session_id, issued_at, expires_at, used_at, revoked_at;

-- ClaimRefreshToken spends a token and returns it, in one statement.
--
-- The guard is the whole design. `used_at IS NULL` in the WHERE clause means the
-- database decides who wins a race, not the application: two concurrent refreshes
-- presenting the same token both reach this UPDATE, PostgreSQL serialises them on
-- the row lock, and the second re-evaluates the predicate against the winner's
-- committed write and matches nothing. A SELECT-then-UPDATE would let both pass
-- the check before either wrote, and would issue two live tokens from one -- which
-- is the bug this card exists to not ship, and which every sequential test passes
-- over in silence.
--
-- The session join is here rather than in a second query for the same reason:
-- revoking a session must stop refreshes immediately, and a check that happens
-- outside this statement is a check something can slip between.
--
-- name: ClaimRefreshToken :one
UPDATE core.refresh_tokens rt
SET used_at = sqlc.arg(now)::timestamptz
FROM core.sessions s
WHERE rt.token_hash = sqlc.arg(token_hash)
  AND rt.used_at IS NULL
  AND rt.revoked_at IS NULL
  AND rt.expires_at > sqlc.arg(now)::timestamptz
  AND s.id = rt.session_id
  AND s.revoked_at IS NULL
  -- The absolute cap, enforced in the same statement that spends the token.
  -- Checked here rather than after the claim so a session past its cap never
  -- spends one: the caller is going to be refused either way, and a token burnt
  -- on a refusal is a token the legitimate client no longer has if the cap turns
  -- out to have been misread.
  AND s.absolute_expires_at > sqlc.arg(now)::timestamptz
RETURNING rt.id, rt.token_hash, rt.family_id, rt.session_id, rt.issued_at, rt.expires_at, rt.used_at,
          rt.revoked_at, s.user_id, s.absolute_expires_at, s.idle_window;

-- GetRefreshTokenByHash reads a row without changing it.
--
-- It runs only after a claim matched nothing, to tell the four reasons apart: no
-- such token, already spent, already revoked, or expired. They are not equivalent
-- -- the second one is a theft report.
--
-- The session join carries the account the family belongs to. It is an inner
-- join because the foreign key makes a token without a session impossible, and a
-- left join would invite a nil check for a state the schema forbids.
--
-- name: GetRefreshTokenByHash :one
SELECT rt.id, rt.token_hash, rt.family_id, rt.session_id, rt.issued_at, rt.expires_at, rt.used_at,
       rt.revoked_at, s.user_id, s.absolute_expires_at, s.idle_window
FROM core.refresh_tokens rt
JOIN core.sessions s ON s.id = rt.session_id
WHERE rt.token_hash = $1;

-- name: RevokeRefreshFamily :execrows
UPDATE core.refresh_tokens
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: CountLiveRefreshTokensInFamily :one
SELECT count(*)
FROM core.refresh_tokens
WHERE family_id = $1 AND revoked_at IS NULL AND used_at IS NULL;
