-- name: CreateMediaAsset :one
INSERT INTO content.media_assets (id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type, created_at, updated_at;

-- name: GetMediaAssetByID :one
SELECT id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type, created_at, updated_at
FROM content.media_assets
WHERE id = $1;

-- name: GetMediaAssetByObjectKey :one
SELECT id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type, created_at, updated_at
FROM content.media_assets
WHERE object_key = $1;

-- name: UpdateMediaAssetStatus :one
UPDATE content.media_assets
SET status = $2,
    duration_ms = COALESCE($3, duration_ms),
    checksum = COALESCE($4, checksum),
    byte_size = COALESCE($5, byte_size),
    mime_type = COALESCE($6, mime_type),
    updated_at = now()
WHERE id = $1
RETURNING id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type, created_at, updated_at;

-- name: DeleteMediaAsset :exec
DELETE FROM content.media_assets
WHERE id = $1;

-- name: GetMediaAssetsByObjectKeys :many
SELECT id, object_key, kind, duration_ms, checksum, status, byte_size, mime_type, created_at, updated_at
FROM content.media_assets
WHERE object_key = ANY (@object_keys::text[]);

