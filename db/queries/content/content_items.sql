-- name: CreateContentItem :one
INSERT INTO content.content_items (id, kind, slug, status, owner_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, kind, slug, current_version_id, status, owner_id, created_at, updated_at;

-- name: GetContentItemByID :one
SELECT id, kind, slug, current_version_id, status, owner_id, created_at, updated_at
FROM content.content_items
WHERE id = $1;

-- name: GetContentItemBySlug :one
SELECT id, kind, slug, current_version_id, status, owner_id, created_at, updated_at
FROM content.content_items
WHERE slug = $1;

-- name: ListContentItemsByOwner :many
SELECT id, kind, slug, current_version_id, status, owner_id, created_at, updated_at
FROM content.content_items
WHERE owner_id = $1
ORDER BY created_at DESC, id DESC
LIMIT @result_limit;

-- name: UpdateContentItemStatus :one
UPDATE content.content_items
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, kind, slug, current_version_id, status, owner_id, created_at, updated_at;

-- name: UpdateContentItemCurrentVersion :one
UPDATE content.content_items
SET current_version_id = $2, updated_at = now()
WHERE id = $1
RETURNING id, kind, slug, current_version_id, status, owner_id, created_at, updated_at;

-- name: DeleteContentItem :exec
DELETE FROM content.content_items
WHERE id = $1;

-- name: ListContentItemsFiltered :many
SELECT id, kind, slug, current_version_id, status, owner_id, created_at, updated_at
FROM content.content_items
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('query')::text IS NULL OR slug ILIKE sqlc.narg('query') || '%')
ORDER BY updated_at DESC, id DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountContentItemsFiltered :one
SELECT COUNT(*)::bigint
FROM content.content_items
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('query')::text IS NULL OR slug ILIKE sqlc.narg('query') || '%');

