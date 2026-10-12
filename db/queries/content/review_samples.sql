-- name: CountAutoPublishedOn :one
-- How many items an independent verifier published on a given day. The sample
-- size is a share of this, so it has to be counted before the draw.
SELECT COUNT(*)::bigint
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE v.status = 'published'
  AND i.status = 'published'
  AND v.published_at >= sqlc.arg('day')::timestamptz
  AND v.published_at < sqlc.arg('day')::timestamptz + interval '1 day'
  AND v.body->'_provenance'->'verification'->>'verdict' = 'confirmed';

-- name: ListAutoPublishedOn :many
-- A random draw of a day's auto-published versions, for the sample queue.
SELECT v.id
FROM content.content_versions v
JOIN content.content_items i ON i.id = v.item_id
WHERE v.status = 'published'
  AND i.status = 'published'
  AND v.published_at >= sqlc.arg('day')::timestamptz
  AND v.published_at < sqlc.arg('day')::timestamptz + interval '1 day'
  AND v.body->'_provenance'->'verification'->>'verdict' = 'confirmed'
ORDER BY random()
LIMIT sqlc.arg('sample_size')::int;

-- name: InsertReviewSample :exec
INSERT INTO content.review_samples (version_id, batch, sampled_on)
VALUES (sqlc.arg('version_id'), sqlc.arg('batch'), sqlc.arg('sampled_on')::date)
ON CONFLICT (version_id) DO NOTHING;

-- name: ListOpenReviewSamples :many
SELECT s.version_id, s.batch, s.sampled_on, s.created_at, v.kind, v.cefr_level
FROM content.review_samples s
JOIN content.content_versions v ON v.id = s.version_id
WHERE s.decision IS NULL
ORDER BY s.created_at ASC, s.version_id ASC
LIMIT sqlc.arg('result_limit') OFFSET sqlc.arg('result_offset');

-- name: CountOpenReviewSamples :one
SELECT COUNT(*)::bigint
FROM content.review_samples
WHERE decision IS NULL;

-- name: DecideReviewSample :exec
UPDATE content.review_samples
SET decision = sqlc.arg('decision')::text,
    note = sqlc.arg('note'),
    decided_by = sqlc.arg('decided_by'),
    decided_at = clock_timestamp()
WHERE version_id = sqlc.arg('version_id');

-- name: IsAutoPublishedVersion :one
SELECT EXISTS (
    SELECT 1
    FROM content.content_versions v
    WHERE v.id = sqlc.arg('version_id')
      AND v.status = 'published'
      AND v.body->'_provenance'->'verification'->>'verdict' = 'confirmed'
) AS auto_published;
