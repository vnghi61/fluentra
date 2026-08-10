-- name: CreateCredential :one
-- The ID is supplied by the caller (UUIDv7 from shared/id) rather than by the
-- column default, for the same reason CreateUser takes one: registration writes
-- the outbox row in the same transaction and needs the id before the insert.
INSERT INTO core.credentials (id, user_id, password_hash)
VALUES ($1, $2, $3)
RETURNING id, user_id, password_hash, algo_params, created_at, updated_at;

-- name: GetCredentialByUserID :one
SELECT id, user_id, password_hash, algo_params, created_at, updated_at
FROM core.credentials
WHERE user_id = $1;

-- name: ReplaceCredentialHash :one
-- Serves both rehash-on-login (same password, current parameters) and a
-- password change. The two are indistinguishable here on purpose: the row
-- carries no history, so an attacker who reads it learns nothing about when the
-- password last actually changed.
UPDATE core.credentials
SET password_hash = $2, updated_at = now()
WHERE user_id = $1
RETURNING id, user_id, password_hash, algo_params, created_at, updated_at;
