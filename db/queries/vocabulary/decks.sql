-- name: InsertDeck :one
INSERT INTO skill.decks (
    owner_id, slug, name, description, is_public, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, now(), now()
) RETURNING *;

-- name: GetDeckByID :one
SELECT * FROM skill.decks
WHERE id = $1;

-- name: ListDecksByUser :many
SELECT d.*, COUNT(di.id)::bigint AS item_count
FROM skill.decks d
LEFT JOIN skill.deck_items di ON di.deck_id = d.id
WHERE d.owner_id = $1 OR (d.owner_id IS NULL AND d.is_public = true)
GROUP BY d.id
ORDER BY d.created_at DESC;

-- name: InsertDeckItem :one
INSERT INTO skill.deck_items (
    deck_id, word_sense_id, created_at
) VALUES (
    $1, $2, now()
)
ON CONFLICT (deck_id, word_sense_id) DO NOTHING
RETURNING *;

-- name: DeleteDeckItem :exec
DELETE FROM skill.deck_items
WHERE deck_id = $1 AND word_sense_id = $2;

-- name: ListDeckWords :many
SELECT di.created_at AS added_at, s.id AS sense_id, s.word_id, s.content_version_id, s.definition, s.definition_vi, s.register, s.domain, s.examples, w.lemma, w.pos, w.cefr_level, w.ipa, w.audio_asset_id
FROM skill.deck_items di
JOIN skill.word_senses s ON s.id = di.word_sense_id
JOIN skill.words w ON w.id = s.word_id
WHERE di.deck_id = $1
ORDER BY di.created_at DESC
LIMIT $2 OFFSET $3;
