-- name: CreateContentVersion :one
INSERT INTO content.content_versions (id, item_id, version, kind, body, cefr_level, status, media_refs, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at;

-- name: GetContentVersionByID :one
SELECT id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at
FROM content.content_versions
WHERE id = $1;

-- name: GetContentVersionByItemAndVersion :one
SELECT id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at
FROM content.content_versions
WHERE item_id = $1 AND version = $2;

-- name: ListContentVersionsByItemID :many
SELECT id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at
FROM content.content_versions
WHERE item_id = $1
ORDER BY version DESC;

-- name: GetManyContentVersionsByIDs :many
SELECT id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at
FROM content.content_versions
WHERE id = ANY (@ids::uuid[])
ORDER BY id;

-- name: UpdateContentVersionDraft :one
-- Only draft, in_review, or approved versions can be updated.
-- A published version is rejected by trigger trg_content_versions_immutable.
UPDATE content.content_versions
SET kind = $2,
    body = $3,
    cefr_level = $4,
    media_refs = $5,
    status = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at;

-- name: PublishContentVersion :one
UPDATE content.content_versions
SET status = 'published',
    published_at = COALESCE(published_at, now()),
    updated_at = now()
WHERE id = $1
RETURNING id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at;

-- name: GetLatestVersionNumberByItemID :one
SELECT COALESCE(MAX(version), 0)::integer AS latest_version
FROM content.content_versions
WHERE item_id = $1;

-- name: GetPublishedVersionBySlug :one
SELECT v.id, v.item_id, v.version, v.kind, v.body, v.cefr_level, v.status, v.media_refs, v.published_at, v.created_at, v.updated_at
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE i.slug = $1
  AND i.status = 'published'
  AND v.status = 'published';

-- name: BrowsePublishedContentVersions :many
SELECT v.id, v.item_id, v.version, v.kind, v.body, v.cefr_level, v.status, v.media_refs, v.published_at, v.created_at, v.updated_at
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE i.status = 'published'
  AND v.status = 'published'
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR v.cefr_level = sqlc.narg('cefr_level'))
ORDER BY v.published_at DESC, v.id DESC
LIMIT $1 OFFSET $2;

-- name: CountPublishedContentVersions :one
SELECT COUNT(*)::bigint
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE i.status = 'published'
  AND v.status = 'published'
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR v.cefr_level = sqlc.narg('cefr_level'));

-- name: GetDraftVersionByItemID :one
SELECT id, item_id, version, kind, body, cefr_level, status, media_refs, published_at, created_at, updated_at
FROM content.content_versions
WHERE item_id = $1 AND status IN ('draft', 'in_review', 'approved')
ORDER BY version DESC
LIMIT 1;

