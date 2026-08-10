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
