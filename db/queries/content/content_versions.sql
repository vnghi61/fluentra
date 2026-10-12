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

-- name: ListReviewQueue :many
SELECT
    v.id,
    v.item_id,
    i.slug,
    v.kind,
    v.cefr_level,
    v.status,
    v.body,
    v.created_at,
    COALESCE(
        (SELECT array_agg(t.code::text ORDER BY t.code)
         FROM content.content_tags ct
         JOIN content.taxonomies t ON t.id = ct.taxonomy_id
         WHERE ct.item_id = v.item_id),
        '{}'::text[]
    ) AS node_codes
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE v.status IN ('draft', 'in_review')
  -- Purpose `resource` is a learner's private practice generated from their
  -- own upload: it never enters the review queue (WO 21 D21-3, BR-RESOURCE-12).
  AND COALESCE(v.body->'_provenance'->>'purpose', '') <> 'resource'
  AND (sqlc.narg('purpose')::text IS NULL OR (v.body->'_provenance'->>'purpose' = sqlc.narg('purpose') OR i.slug ILIKE sqlc.narg('purpose') || '-%'))
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR v.cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('node_code')::text IS NULL OR EXISTS (
      SELECT 1
      FROM content.content_tags ct
      JOIN content.taxonomies t ON t.id = ct.taxonomy_id
      WHERE ct.item_id = v.item_id AND t.code = sqlc.narg('node_code')
  ))
  AND (sqlc.narg('batch')::text IS NULL OR v.body->'_provenance'->>'batch' = sqlc.narg('batch'))
ORDER BY v.created_at ASC, v.id ASC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountReviewQueue :one
SELECT COUNT(*)::bigint
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE v.status IN ('draft', 'in_review')
  -- Purpose `resource` is a learner's private practice generated from their
  -- own upload: it never enters the review queue (WO 21 D21-3, BR-RESOURCE-12).
  AND COALESCE(v.body->'_provenance'->>'purpose', '') <> 'resource'
  AND (sqlc.narg('purpose')::text IS NULL OR (v.body->'_provenance'->>'purpose' = sqlc.narg('purpose') OR i.slug ILIKE sqlc.narg('purpose') || '-%'))
  AND (sqlc.narg('kind')::text IS NULL OR v.kind = sqlc.narg('kind'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR v.cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('node_code')::text IS NULL OR EXISTS (
      SELECT 1
      FROM content.content_tags ct
      JOIN content.taxonomies t ON t.id = ct.taxonomy_id
      WHERE ct.item_id = v.item_id AND t.code = sqlc.narg('node_code')
  ))
  AND (sqlc.narg('batch')::text IS NULL OR v.body->'_provenance'->>'batch' = sqlc.narg('batch'));

-- name: ListReviewBatches :many
-- One row per generation run whose drafts are still awaiting review, so a person
-- can approve a whole batch at once (WO 22 Stage A.4).
SELECT
    (v.body->'_provenance'->>'batch')::text AS batch,
    COUNT(*)::bigint AS item_count,
    MIN(v.created_at)::timestamptz AS created_at,
    COALESCE(array_agg(DISTINCT v.kind ORDER BY v.kind), '{}'::text[])::text[] AS kinds
FROM content.content_versions v
WHERE v.status IN ('draft', 'in_review')
  AND COALESCE(v.body->'_provenance'->>'purpose', '') <> 'resource'
  AND COALESCE(v.body->'_provenance'->>'batch', '') <> ''
GROUP BY v.body->'_provenance'->>'batch'
ORDER BY MIN(v.created_at) ASC, batch ASC
LIMIT @result_limit OFFSET @result_offset;

-- name: ListReviewBatchVersionIDs :many
-- Every draft of one batch, unbounded: approving a batch must publish all of it
-- or none, and the queue's page ceiling would silently approve only the first
-- hundred (WO 22 Stage A.4).
SELECT v.id
FROM content.content_versions v
WHERE v.status IN ('draft', 'in_review')
  AND COALESCE(v.body->'_provenance'->>'purpose', '') <> 'resource'
  AND v.body->'_provenance'->>'batch' = sqlc.arg('batch')::text
ORDER BY v.created_at ASC, v.id ASC;

-- name: CountReviewBatches :one
SELECT COUNT(*)::bigint FROM (
    SELECT 1
    FROM content.content_versions v
    WHERE v.status IN ('draft', 'in_review')
      AND COALESCE(v.body->'_provenance'->>'purpose', '') <> 'resource'
      AND COALESCE(v.body->'_provenance'->>'batch', '') <> ''
    GROUP BY v.body->'_provenance'->>'batch'
) batches;

-- name: CountPendingBatchDays :one
-- How many distinct days a family of generation batches (one exam's, by batch
-- prefix) left doubts nobody has reviewed yet. The daily exam job skips an
-- exam whose doubts from two earlier days still wait (WO 22 Stage O trap 1).
SELECT COUNT(DISTINCT batch_day)::bigint FROM (
    SELECT (MIN(v.created_at) AT TIME ZONE 'UTC')::date AS batch_day
    FROM content.content_versions v
    WHERE v.status IN ('draft', 'in_review')
      AND starts_with(v.body->'_provenance'->>'batch', sqlc.arg('prefix')::text)
    GROUP BY v.body->'_provenance'->>'batch'
) batches;
