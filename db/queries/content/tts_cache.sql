-- name: GetTTSCache :one
SELECT text_hash, voice, engine, engine_version, object_key, created_at
FROM content.tts_cache
WHERE text_hash = $1 AND voice = $2
LIMIT 1;

-- name: UpsertTTSCache :one
INSERT INTO content.tts_cache (
    text_hash,
    voice,
    engine,
    engine_version,
    object_key,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, clock_timestamp()
)
ON CONFLICT (text_hash, voice) DO UPDATE
SET engine = EXCLUDED.engine,
    engine_version = EXCLUDED.engine_version,
    object_key = EXCLUDED.object_key,
    created_at = clock_timestamp()
RETURNING text_hash, voice, engine, engine_version, object_key, created_at;
