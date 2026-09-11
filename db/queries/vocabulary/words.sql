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

-- name: UpdateWordSenseEnrichment :one
UPDATE skill.word_senses
SET content_version_id = $2,
    definition         = $3,
    examples           = $4,
    updated_at         = now()
WHERE id = $1
RETURNING *;

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
RETURNING *, (xmax = 0) AS inserted;

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

-- name: ListSensesForGeneration :many
-- The generator's input: every published sense with enough to build an exercise
-- from, joined to its word.
--
-- `content_version_id IS NOT NULL` because a generated exercise schedules a
-- review card at the word's own dictionary entry, and a sense with no entry has
-- nowhere to point one. `definition <> ''` because a definition is the question
-- in half the kinds.
--
-- Ordered by frequency rank so the first lessons the generator writes are about
-- the commonest words, and the ordering is stable across runs — which is what
-- keeps a lesson's position, and therefore its identity, from moving.
SELECT
    w.id            AS word_id,
    w.lemma,
    w.pos,
    w.cefr_level,
    w.ipa,
    w.frequency_rank,
    s.id            AS sense_id,
    s.definition,
    s.definition_vi,
    s.examples,
    s.content_version_id
FROM skill.word_senses s
JOIN skill.words w ON w.id = s.word_id
WHERE s.content_version_id IS NOT NULL
  AND btrim(s.definition) <> ''
ORDER BY w.frequency_rank NULLS LAST, w.lemma, s.id
LIMIT $1;

-- name: ListWordsAdmin :many
SELECT
    w.id,
    w.lemma,
    w.pos,
    w.cefr_level,
    w.frequency_rank,
    w.ipa,
    w.audio_asset_id,
    w.created_at,
    w.updated_at,
    (SELECT COUNT(*)::bigint FROM skill.word_senses ws WHERE ws.word_id = w.id) AS senses_count,
    EXISTS (
        SELECT 1
        FROM skill.vocab_upload_items ui
        JOIN skill.word_senses ws ON ws.id = ui.word_sense_id
        WHERE ws.word_id = w.id
    ) AS is_uploaded
FROM skill.words w
WHERE (sqlc.narg('query')::text IS NULL OR w.lemma ILIKE sqlc.narg('query') || '%')
  AND (
      sqlc.narg('source')::text IS NULL
      OR (sqlc.narg('source')::text = 'upload' AND EXISTS (
          SELECT 1
          FROM skill.vocab_upload_items ui
          JOIN skill.word_senses ws ON ws.id = ui.word_sense_id
          WHERE ws.word_id = w.id
      ))
      OR (sqlc.narg('source')::text = 'seed' AND NOT EXISTS (
          SELECT 1
          FROM skill.vocab_upload_items ui
          JOIN skill.word_senses ws ON ws.id = ui.word_sense_id
          WHERE ws.word_id = w.id
      ))
  )
ORDER BY w.updated_at DESC, w.lemma ASC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountWordsAdmin :one
SELECT COUNT(*)::bigint
FROM skill.words w
WHERE (sqlc.narg('query')::text IS NULL OR w.lemma ILIKE sqlc.narg('query') || '%')
  AND (
      sqlc.narg('source')::text IS NULL
      OR (sqlc.narg('source')::text = 'upload' AND EXISTS (
          SELECT 1
          FROM skill.vocab_upload_items ui
          JOIN skill.word_senses ws ON ws.id = ui.word_sense_id
          WHERE ws.word_id = w.id
      ))
      OR (sqlc.narg('source')::text = 'seed' AND NOT EXISTS (
          SELECT 1
          FROM skill.vocab_upload_items ui
          JOIN skill.word_senses ws ON ws.id = ui.word_sense_id
          WHERE ws.word_id = w.id
      ))
  );

-- name: UpdateWordSenseAdmin :one
UPDATE skill.word_senses
SET definition    = COALESCE(sqlc.narg('definition'), definition),
    definition_vi = COALESCE(sqlc.narg('definition_vi'), definition_vi),
    domain        = COALESCE(sqlc.narg('domain'), domain),
    examples      = COALESCE(sqlc.narg('examples'), examples),
    updated_at    = now()
WHERE id = @id
RETURNING *;

-- name: DeleteWordSense :exec
DELETE FROM skill.word_senses
WHERE id = $1;

-- name: DeleteWord :exec
DELETE FROM skill.words
WHERE id = $1;

-- ListSensesNeedingExampleEnrichment finds senses with room for more examples.
--
-- w.ipa and w.audio_asset_id are selected even though enrichment does not change
-- them: the job republishes the whole sense body, and a column it does not carry
-- is a field the new content version does not have. Leaving the IPA out silently
-- stripped the pronunciation from every word the sweep touched.
--
-- `examples IS NULL` is included deliberately. jsonb_typeof(NULL) is NULL, so the
-- typeof test alone excluded exactly the senses with no examples at all — the
-- ones that need this most.
-- name: ListSensesNeedingExampleEnrichment :many
SELECT s.id, s.word_id, s.content_version_id, s.definition, s.definition_vi, s.examples,
       w.lemma, w.pos, w.cefr_level, w.ipa, w.audio_asset_id
FROM skill.word_senses s
JOIN skill.words w ON w.id = s.word_id
WHERE s.examples IS NULL
   OR (jsonb_typeof(s.examples) = 'array' AND jsonb_array_length(s.examples) < 15)
ORDER BY s.updated_at ASC
LIMIT $1;

