-- name: UpsertExtraction :one
INSERT INTO resource.extractions (
    resource_id,
    source,
    text,
    char_count,
    truncated,
    language,
    tool_version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (resource_id) DO UPDATE
SET source = EXCLUDED.source,
    text = EXCLUDED.text,
    char_count = EXCLUDED.char_count,
    truncated = EXCLUDED.truncated,
    language = EXCLUDED.language,
    tool_version = EXCLUDED.tool_version,
    created_at = now()
RETURNING resource_id, source, text, char_count, truncated, language, tool_version, created_at;

-- name: GetExtractionByResourceID :one
SELECT resource_id, source, text, char_count, truncated, language, tool_version, created_at
FROM resource.extractions
WHERE resource_id = $1;
