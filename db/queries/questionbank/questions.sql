-- name: CreateQuestion :one
INSERT INTO assess.questions (
    id,
    content_item_id,
    activity_id,
    exam_part_id,
    kind,
    skill,
    cefr_level,
    difficulty,
    question_count,
    fingerprint,
    provenance,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetQuestionByID :one
SELECT * FROM assess.questions
WHERE id = $1;

-- name: GetQuestionByFingerprint :one
SELECT * FROM assess.questions
WHERE fingerprint = $1;

-- name: GetQuestionByContentItemID :one
SELECT * FROM assess.questions
WHERE content_item_id = $1;

-- name: UpdateQuestionStatus :one
UPDATE assess.questions
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateQuestionActivityID :one
UPDATE assess.questions
SET activity_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListQuestions :many
SELECT * FROM assess.questions
WHERE (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('skill')::text IS NULL OR skill = sqlc.narg('skill'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('exam_part_id')::uuid IS NULL OR exam_part_id = sqlc.narg('exam_part_id'))
  AND (sqlc.narg('node_code')::text IS NULL OR EXISTS (
      SELECT 1 FROM content.content_tags ct
      JOIN content.taxonomies t ON t.id = ct.taxonomy_id
      WHERE ct.item_id = assess.questions.content_item_id
        AND t.code = sqlc.narg('node_code')
  ))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountQuestions :one
SELECT count(*) FROM assess.questions
WHERE (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('skill')::text IS NULL OR skill = sqlc.narg('skill'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('exam_part_id')::uuid IS NULL OR exam_part_id = sqlc.narg('exam_part_id'))
  AND (sqlc.narg('node_code')::text IS NULL OR EXISTS (
      SELECT 1 FROM content.content_tags ct
      JOIN content.taxonomies t ON t.id = ct.taxonomy_id
      WHERE ct.item_id = assess.questions.content_item_id
        AND t.code = sqlc.narg('node_code')
  ));

-- name: SamplePublishedQuestions :many
SELECT * FROM assess.questions
WHERE status = 'published'
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('skill')::text IS NULL OR skill = sqlc.narg('skill'))
  AND (sqlc.narg('cefr_level')::text IS NULL OR cefr_level = sqlc.narg('cefr_level'))
  AND (sqlc.narg('exam_part_id')::uuid IS NULL OR exam_part_id = sqlc.narg('exam_part_id'))
ORDER BY random()
LIMIT $1;

-- name: GetQuestionStats :one
SELECT * FROM assess.question_stats
WHERE question_id = $1;

-- name: UpsertQuestionStats :one
INSERT INTO assess.question_stats (
    question_id,
    attempts,
    p_value,
    discrimination,
    avg_time_ms,
    last_computed_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (question_id) DO UPDATE
SET attempts = EXCLUDED.attempts,
    p_value = EXCLUDED.p_value,
    discrimination = EXCLUDED.discrimination,
    avg_time_ms = EXCLUDED.avg_time_ms,
    last_computed_at = EXCLUDED.last_computed_at
RETURNING *;
