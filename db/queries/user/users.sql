-- name: CreateUser :one
-- The ID is supplied by the caller (UUIDv7 from shared/id), not by the column
-- default: the service needs it before the insert to write the outbox event in
-- the same transaction.
INSERT INTO core.users (id, email, status)
VALUES ($1, $2, $3)
RETURNING id, email, status, email_verified_at, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, status, email_verified_at, created_at, updated_at
FROM core.users
WHERE id = $1;

-- name: GetUserByEmail :one
-- email is citext, so this matches case-insensitively without a lower() the
-- caller could forget (BR-USER-01).
SELECT id, email, status, email_verified_at, created_at, updated_at
FROM core.users
WHERE email = $1;

-- name: ListUsersByIDs :many
-- The batched read behind contract.Reader.GetManyByIDs: one query for N ids,
-- so callers cannot turn a list render into an N+1.
SELECT id, email, status, email_verified_at, created_at, updated_at
FROM core.users
WHERE id = ANY (@ids::uuid[])
ORDER BY id
LIMIT 1000;

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM core.users WHERE id = $1);

-- name: UpdateUserStatus :one
UPDATE core.users
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, email, status, email_verified_at, created_at, updated_at;

-- name: MarkUserEmailVerified :one
-- Idempotent: verifying twice keeps the first timestamp, which is the one the
-- audit trail already recorded.
UPDATE core.users
SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
WHERE id = $1
RETURNING id, email, status, email_verified_at, created_at, updated_at;

-- name: PurgeUnverifiedUsersBefore :execrows
-- Removes accounts that claimed an address and never proved it. The profile,
-- preference and credential rows go with them through ON DELETE CASCADE.
--
-- `status = 'active'` is part of the predicate on purpose: a suspended or
-- pending-deletion account is somebody's problem to resolve deliberately, and
-- sweeping it here would hide that.
DELETE FROM core.users
WHERE email_verified_at IS NULL
  AND status = 'active'
  AND created_at < @cutoff;

-- name: SearchUsersAdmin :many
SELECT
    u.id,
    u.email,
    u.status,
    u.created_at,
    u.updated_at,
    p.display_name,
    p.avatar_asset_id,
    p.timezone,
    pref.locale
FROM core.users u
JOIN core.profiles p ON p.user_id = u.id
JOIN core.user_preferences pref ON pref.user_id = u.id
WHERE (@email_prefix::text = '' OR u.email ILIKE @email_prefix || '%')
  AND (@display_name::text = '' OR p.display_name ILIKE '%' || @display_name || '%')
  AND (@status::text = '' OR u.status::text = @status)
  -- sqlc.narg, not @: `@name` generates a non-nullable Go parameter, so an
  -- absent cursor arrives as the zero UUID rather than NULL. The guard then
  -- reads `'00000000-…' IS NULL OR (created_at, id) < ('0001-01-01', '000…')`,
  -- which is false for every row — the first page of an unfiltered admin search
  -- returned nothing at all, and no test noticed because the fixtures always
  -- passed a cursor.
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR u.created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR u.created_at <= sqlc.narg('created_before')::timestamptz)
  AND (
    sqlc.narg('cursor_id')::uuid IS NULL
    OR (u.created_at, u.id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY u.created_at DESC, u.id DESC
LIMIT @result_limit;

