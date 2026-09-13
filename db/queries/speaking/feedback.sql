-- name: InsertSpeakingFeedback :one
INSERT INTO skill.speaking_feedback (
    id,
    attempt_id,
    user_id,
    recording_key,
    recording_deleted_at,
    transcript,
    criteria,
    read_aloud_accuracy,
    words_per_minute,
    feedback_en,
    feedback_vi,
    prompt_version,
    model,
    asr_model,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING *;

-- name: GetSpeakingFeedbackByAttemptID :one
SELECT * FROM skill.speaking_feedback
WHERE attempt_id = $1;

-- name: GetSpeakingFeedbackForUser :one
SELECT * FROM skill.speaking_feedback
WHERE attempt_id = $1 AND user_id = $2;

-- name: MarkRecordingDeleted :one
UPDATE skill.speaking_feedback
SET recording_deleted_at = now(), updated_at = now()
WHERE attempt_id = $1 AND user_id = $2 AND recording_deleted_at IS NULL
RETURNING recording_key;

-- name: ListRecordingsOlderThan :many
SELECT id, attempt_id, user_id, recording_key
FROM skill.speaking_feedback
WHERE recording_deleted_at IS NULL
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2;

-- name: MarkRecordingsDeletedBatch :exec
UPDATE skill.speaking_feedback
SET recording_deleted_at = now(), updated_at = now()
WHERE id = ANY($1::uuid[]);

-- name: ListUserRecordingKeys :many
SELECT recording_key
FROM skill.speaking_feedback
WHERE user_id = $1 AND recording_deleted_at IS NULL;
