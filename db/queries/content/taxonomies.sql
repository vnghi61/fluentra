-- name: CreateTaxonomy :one
INSERT INTO content.taxonomies (
    id, namespace, code, label, parent_id,
    description, cefr_level, position, deprecated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at;

-- name: GetTaxonomyByID :one
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE id = $1;

-- name: GetTaxonomyByNamespaceCode :one
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE namespace = $1 AND code = $2;

-- name: ListTaxonomiesByCode :many
-- (namespace, code) is the unique key, so a bare code can match more than one
-- row. Two are fetched rather than one: the caller needs to tell "found" from
-- "ambiguous", and `LIMIT 1` answered an arbitrary one of them.
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE code = $1
ORDER BY namespace
LIMIT 2;

-- name: ListTaxonomiesByNamespace :many
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE namespace = $1
ORDER BY position, code;

-- name: ListAllTaxonomies :many
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
ORDER BY namespace, position, code;

-- name: ListTaxonomiesFiltered :many
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE (sqlc.narg('namespace')::text IS NULL OR namespace = sqlc.narg('namespace'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('parent_id')::uuid IS NULL OR parent_id = sqlc.narg('parent_id'))
  AND (sqlc.narg('query')::text IS NULL OR (code ILIKE '%' || sqlc.narg('query') || '%' OR label ILIKE '%' || sqlc.narg('query') || '%'))
  AND (sqlc.arg('include_deprecated')::boolean = true OR deprecated_at IS NULL)
ORDER BY position ASC, code ASC
LIMIT sqlc.arg('result_limit')::int OFFSET sqlc.arg('result_offset')::int;

-- name: CountTaxonomiesFiltered :one
SELECT count(*)
FROM content.taxonomies
WHERE (sqlc.narg('namespace')::text IS NULL OR namespace = sqlc.narg('namespace'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('parent_id')::uuid IS NULL OR parent_id = sqlc.narg('parent_id'))
  AND (sqlc.narg('query')::text IS NULL OR (code ILIKE '%' || sqlc.narg('query') || '%' OR label ILIKE '%' || sqlc.narg('query') || '%'))
  AND (sqlc.arg('include_deprecated')::boolean = true OR deprecated_at IS NULL);

-- name: UpdateTaxonomy :one
UPDATE content.taxonomies
SET
    label = COALESCE(sqlc.narg('label'), label),
    description = COALESCE(sqlc.narg('description'), description),
    cefr_level = CASE WHEN sqlc.arg('set_cefr_level')::boolean THEN sqlc.narg('cefr_level') ELSE cefr_level END,
    parent_id = CASE WHEN sqlc.arg('set_parent_id')::boolean THEN sqlc.narg('parent_id') ELSE parent_id END,
    position = COALESCE(sqlc.narg('position'), position),
    deprecated_at = CASE WHEN sqlc.arg('set_deprecated_at')::boolean THEN sqlc.narg('deprecated_at') ELSE deprecated_at END,
    updated_at = now()
WHERE id = $1
RETURNING
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at;

-- name: DeleteTaxonomy :exec
DELETE FROM content.taxonomies
WHERE id = $1;

-- name: ListPrerequisitesForNode :many
SELECT
    t.id, t.namespace, t.code, t.label, t.parent_id,
    t.created_at, t.updated_at,
    t.description, t.cefr_level, t.position, t.deprecated_at
FROM content.taxonomy_prerequisites p
JOIN content.taxonomies t ON t.id = p.requires_node_id
WHERE p.node_id = $1
ORDER BY t.position, t.code;

-- name: ListDependantsForNode :many
SELECT
    t.id, t.namespace, t.code, t.label, t.parent_id,
    t.created_at, t.updated_at,
    t.description, t.cefr_level, t.position, t.deprecated_at
FROM content.taxonomy_prerequisites p
JOIN content.taxonomies t ON t.id = p.node_id
WHERE p.requires_node_id = $1
ORDER BY t.position, t.code;

-- name: ListAllPrerequisiteEdgesInNamespace :many
SELECT p.node_id, p.requires_node_id, p.created_at
FROM content.taxonomy_prerequisites p
JOIN content.taxonomies t ON t.id = p.node_id
WHERE t.namespace = $1;

-- name: ListAllTaxonomiesInNamespace :many
SELECT
    id, namespace, code, label, parent_id,
    created_at, updated_at,
    description, cefr_level, position, deprecated_at
FROM content.taxonomies
WHERE namespace = $1 AND deprecated_at IS NULL
ORDER BY position, code;

-- name: DeletePrerequisitesForNode :exec
DELETE FROM content.taxonomy_prerequisites
WHERE node_id = $1;

-- name: InsertPrerequisiteEdge :exec
INSERT INTO content.taxonomy_prerequisites (node_id, requires_node_id)
VALUES ($1, $2)
ON CONFLICT (node_id, requires_node_id) DO NOTHING;

-- name: CountTaggedContentByKindForTaxonomy :many
-- Published only. This count is what BR-FOUNDATION-05 gates publication on and
-- what the public topic response reports, and counting drafts would let a topic
-- publish against exercises nobody has written yet.
SELECT ci.kind, count(DISTINCT ci.id)::bigint AS item_count
FROM content.content_tags ct
JOIN content.content_items ci ON ci.id = ct.item_id
WHERE ct.taxonomy_id = $1
  AND ci.status = 'published'
GROUP BY ci.kind;

-- name: GetPublishedTopicBodyByTaxonomyID :one
SELECT cv.body
FROM content.content_tags ct
JOIN content.content_items ci ON ci.id = ct.item_id
JOIN content.content_versions cv ON cv.id = ci.current_version_id
WHERE ct.taxonomy_id = $1
  AND ci.kind = 'foundation_topic'
  AND ci.status = 'published'
LIMIT 1;
