-- name: CreateFileResourceIntent :one
INSERT INTO resource.resources (
    id,
    user_id,
    kind,
    title,
    object_key,
    original_filename,
    declared_mime,
    status
) VALUES (
    $1, $2, 'file', $3, $4, $5, $6, 'pending'
) RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: CreateURLResource :one
INSERT INTO resource.resources (
    id,
    user_id,
    kind,
    title,
    source_url,
    status
) VALUES (
    $1, $2, 'url', $3, $4, 'uploaded'
) RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: GetResourceByID :one
SELECT
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE id = $1;

-- name: GetResourceByIDAndUser :one
SELECT
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE id = $1 AND user_id = $2;

-- name: ListResourcesByUser :many
SELECT
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE user_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountResourcesByUser :one
SELECT count(*)
FROM resource.resources
WHERE user_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'));

-- name: GetUserResourceUsage :one
SELECT
    count(*)::bigint AS resource_count,
    coalesce(sum(byte_size), 0)::bigint AS total_bytes
FROM resource.resources
WHERE user_id = $1 AND status != 'rejected' AND status != 'failed';

-- name: ConfirmFileResource :one
UPDATE resource.resources
SET status = 'uploaded',
    updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'pending'
RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: UpdateResourceValidationSuccess :one
UPDATE resource.resources
SET status = 'validated',
    title = CASE WHEN length(btrim($2::text)) > 0 THEN $2::text ELSE title END,
    detected_mime = $3,
    byte_size = $4,
    checksum = $5,
    validated_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'uploaded'
RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: UpdateResourceValidationRejected :one
UPDATE resource.resources
SET status = 'rejected',
    failure_reason = $2,
    detected_mime = coalesce(sqlc.narg('detected_mime')::text, detected_mime),
    updated_at = now()
WHERE id = $1 AND status = 'uploaded'
RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: UpdateResourceValidationFailed :one
-- Conditional on the status the caller saw. The sweeper passes 'pending': a row
-- the learner confirmed between the sweeper's read and this write must not be
-- marked failed, and its object must not be deleted, which is why the sweeper
-- only deletes after this returns a row.
UPDATE resource.resources
SET status = 'failed',
    failure_reason = $2,
    updated_at = now()
WHERE id = $1 AND status = sqlc.arg('from_status')::text
RETURNING
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at;

-- name: DeleteResource :one
DELETE FROM resource.resources
WHERE id = $1 AND user_id = $2
RETURNING id, object_key, kind;

-- name: ListExpiredPendingResources :many
SELECT
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE status = 'pending'
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2;

-- name: ListStuckUploadedResources :many
-- Confirmed rows whose validation never finished: the job exhausted its retries
-- on a storage outage, or was lost. Without this they sit at 'uploaded' for ever,
-- which the learner sees as a spinner that never stops.
SELECT
    id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE status = 'uploaded'
  AND updated_at < $1
ORDER BY updated_at ASC
LIMIT $2;
