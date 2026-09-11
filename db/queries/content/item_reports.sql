-- name: InsertItemReport :one
INSERT INTO content.item_reports (
    content_version_id,
    user_id,
    reason,
    note
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (content_version_id, user_id) DO UPDATE
SET reason = EXCLUDED.reason,
    note = EXCLUDED.note,
    created_at = clock_timestamp()
RETURNING *;

-- name: ListItemReportsByVersion :many
SELECT * FROM content.item_reports
WHERE content_version_id = $1
ORDER BY created_at DESC;

-- name: ListReportedContentVersions :many
SELECT
    v.id AS content_version_id,
    v.item_id,
    i.slug,
    v.kind,
    v.cefr_level,
    i.status AS item_status,
    COUNT(DISTINCT r.user_id)::int AS report_count,
    MAX(r.created_at)::timestamptz AS last_reported_at
FROM content.item_reports r
JOIN content.content_versions v ON v.id = r.content_version_id
JOIN content.content_items i ON i.id = v.item_id
GROUP BY v.id, v.item_id, i.slug, v.kind, v.cefr_level, i.status
ORDER BY report_count DESC, last_reported_at DESC
LIMIT $1 OFFSET $2;

-- name: CountReportedContentVersions :one
SELECT COUNT(DISTINCT content_version_id)::int AS total
FROM content.item_reports;
