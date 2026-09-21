-- name: InsertRenditionPending :one
INSERT INTO resource.renditions (
    resource_id,
    kind,
    status
) VALUES (
    $1, $2, 'pending'
)
ON CONFLICT (resource_id, kind) DO NOTHING
RETURNING id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at;

-- name: ClaimPendingRenditions :many
WITH candidate AS (
    SELECT id
    FROM resource.renditions
    WHERE status = 'pending'
      AND attempts < 3
      AND (attempts = 0 OR updated_at < now() - interval '5 minutes')
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE resource.renditions r
SET attempts = r.attempts + 1,
    updated_at = now()
FROM candidate
WHERE r.id = candidate.id
RETURNING r.id, r.resource_id, r.kind, r.status, r.object_key, r.mime_type, r.width, r.height, r.duration_ms, r.byte_size, r.tool_version, r.attempts, r.failure_reason, r.created_at, r.updated_at;

-- name: UpdateRenditionReady :one
UPDATE resource.renditions
SET status = 'ready',
    object_key = $2,
    mime_type = $3,
    width = $4,
    height = $5,
    duration_ms = $6,
    byte_size = $7,
    tool_version = $8,
    failure_reason = '',
    updated_at = now()
WHERE id = $1
RETURNING id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at;

-- name: UpdateRenditionSkipped :one
UPDATE resource.renditions
SET status = 'skipped',
    failure_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at;

-- name: UpdateRenditionFailed :one
UPDATE resource.renditions
SET status = CASE WHEN attempts >= 3 THEN 'failed' ELSE 'pending' END,
    failure_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at;

-- name: ListReadyRenditionsByResourceID :many
SELECT id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at
FROM resource.renditions
WHERE resource_id = $1 AND status = 'ready'
ORDER BY kind ASC;

-- name: ListRenditionsByResourceID :many
SELECT id, resource_id, kind, status, object_key, mime_type, width, height, duration_ms, byte_size, tool_version, attempts, failure_reason, created_at, updated_at
FROM resource.renditions
WHERE resource_id = $1
ORDER BY kind ASC;

-- name: ListRenditionKeysByUserID :many
SELECT r.object_key
FROM resource.renditions r
JOIN resource.resources res ON res.id = r.resource_id
WHERE res.user_id = $1 AND r.object_key IS NOT NULL;

-- name: ListValidatedFileResourcesForRenditions :many
SELECT id, user_id, kind, title, object_key, original_filename, declared_mime, detected_mime,
    byte_size, checksum, source_url, status, failure_reason, created_at, updated_at, validated_at
FROM resource.resources
WHERE status = 'validated' AND kind = 'file' AND object_key IS NOT NULL
ORDER BY validated_at ASC NULLS LAST
LIMIT $1;
