-- name: CreateTaxonomy :one
INSERT INTO content.taxonomies (id, namespace, code, label, parent_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, namespace, code, label, parent_id, created_at, updated_at;

-- name: GetTaxonomyByID :one
SELECT id, namespace, code, label, parent_id, created_at, updated_at
FROM content.taxonomies
WHERE id = $1;

-- name: GetTaxonomyByNamespaceCode :one
SELECT id, namespace, code, label, parent_id, created_at, updated_at
FROM content.taxonomies
WHERE namespace = $1 AND code = $2;

-- name: ListTaxonomiesByNamespace :many
SELECT id, namespace, code, label, parent_id, created_at, updated_at
FROM content.taxonomies
WHERE namespace = $1
ORDER BY code;

-- name: ListAllTaxonomies :many
SELECT id, namespace, code, label, parent_id, created_at, updated_at
FROM content.taxonomies
ORDER BY namespace, code;

-- name: DeleteTaxonomy :exec
DELETE FROM content.taxonomies
WHERE id = $1;
