-- name: InsertWord :one
INSERT INTO skill.words (
    lemma, pos, cefr_level, frequency_rank, ipa, audio_asset_id, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, now(), now()
)
ON CONFLICT (lemma, pos) DO UPDATE SET
    cefr_level = EXCLUDED.cefr_level,
    frequency_rank = EXCLUDED.frequency_rank,
    ipa = EXCLUDED.ipa,
    audio_asset_id = EXCLUDED.audio_asset_id,
    updated_at = now()
RETURNING *;

-- name: GetWordByLemmaAndPOS :one
SELECT * FROM skill.words
WHERE lemma = $1 AND pos = $2;

-- name: GetWordByID :one
SELECT * FROM skill.words
WHERE id = $1;

-- name: ListWordsByLemma :many
SELECT * FROM skill.words
WHERE lemma = $1
ORDER BY frequency_rank ASC NULLS LAST, pos ASC;

-- name: SearchWords :many
SELECT * FROM skill.words
WHERE lemma ILIKE $1 || '%'
ORDER BY frequency_rank ASC NULLS LAST, lemma ASC
LIMIT $2 OFFSET $3;

-- CountSearchWords is the `total` the search response publishes. It repeats the
-- SearchWords predicate on purpose: a total that does not match the filter the
-- page was drawn from is a number that lies about how many pages there are.
-- name: CountSearchWords :one
SELECT COUNT(*)::bigint FROM skill.words
WHERE lemma ILIKE $1 || '%';

-- name: InsertWordSense :one
INSERT INTO skill.word_senses (
    word_id, content_version_id, definition, definition_vi, register, domain, examples, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, now(), now()
) RETURNING *;

-- name: ListSensesByWordID :many
SELECT * FROM skill.word_senses
WHERE word_id = $1
ORDER BY created_at ASC;

-- name: GetSenseByID :one
SELECT s.*, w.lemma, w.pos, w.cefr_level, w.ipa, w.audio_asset_id
FROM skill.word_senses s
JOIN skill.words w ON w.id = s.word_id
WHERE s.id = $1;

-- name: ListSensesByIDs :many
SELECT s.*, w.lemma, w.pos, w.cefr_level, w.ipa, w.audio_asset_id
FROM skill.word_senses s
JOIN skill.words w ON w.id = s.word_id
WHERE s.id = ANY($1::uuid[])
ORDER BY w.lemma ASC, s.created_at ASC;

-- name: InsertWordRelation :one
INSERT INTO skill.word_relations (
    from_word_id, to_word_id, relation, created_at
) VALUES (
    $1, $2, $3, now()
)
ON CONFLICT (from_word_id, to_word_id, relation) DO NOTHING
RETURNING *;

-- name: ListRelationsByWordID :many
SELECT r.*, w.lemma AS target_lemma, w.pos AS target_pos
FROM skill.word_relations r
JOIN skill.words w ON w.id = r.to_word_id
WHERE r.from_word_id = $1;

-- name: UpsertUserWordState :one
INSERT INTO skill.user_word_state (
    user_id, word_sense_id, status, first_seen_at, updated_at
) VALUES (
    $1, $2, $3, now(), now()
)
ON CONFLICT (user_id, word_sense_id) DO UPDATE SET
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: GetUserWordState :one
SELECT * FROM skill.user_word_state
WHERE user_id = $1 AND word_sense_id = $2;

-- name: GetSenseContentVersionByLemma :one
SELECT s.content_version_id
FROM skill.word_senses s
JOIN skill.words w ON w.id = s.word_id
WHERE w.lemma = $1
  AND s.content_version_id IS NOT NULL
ORDER BY w.frequency_rank ASC NULLS LAST, w.pos ASC
LIMIT 1;
