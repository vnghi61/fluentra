-- name: AddContentTag :exec
INSERT INTO content.content_tags (item_id, taxonomy_id)
VALUES ($1, $2)
ON CONFLICT (item_id, taxonomy_id) DO NOTHING;

-- name: RemoveContentTag :exec
DELETE FROM content.content_tags
WHERE item_id = $1 AND taxonomy_id = $2;

-- name: ListTagsForContentItem :many
SELECT t.id, t.namespace, t.code, t.label, t.parent_id, t.created_at, t.updated_at
FROM content.content_tags ct
JOIN content.taxonomies t ON t.id = ct.taxonomy_id
WHERE ct.item_id = $1
ORDER BY t.namespace, t.code;

-- name: ListContentItemIDsForTaxonomy :many
SELECT ct.item_id
FROM content.content_tags ct
WHERE ct.taxonomy_id = $1
ORDER BY ct.item_id;

-- name: ClearTagsForContentItem :exec
DELETE FROM content.content_tags
WHERE item_id = $1;

-- name: ListTagsForContentItems :many
SELECT ct.item_id, t.id, t.namespace, t.code, t.label, t.parent_id, t.created_at, t.updated_at
FROM content.content_tags ct
JOIN content.taxonomies t ON t.id = ct.taxonomy_id
WHERE ct.item_id = ANY (@item_ids::uuid[])
ORDER BY t.namespace, t.code;

