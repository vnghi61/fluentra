-- name: CountPlays :one
SELECT COUNT(*) FROM skill.listening_plays
WHERE user_id = $1 AND content_version_id = $2 AND context_id = $3;

-- name: RecordPlay :one
INSERT INTO skill.listening_plays (
    id,
    user_id,
    content_version_id,
    context_type,
    context_id,
    played_at
) VALUES (
    $1, $2, $3, $4, $5, clock_timestamp()
)
RETURNING id, user_id, content_version_id, context_type, context_id, played_at;
