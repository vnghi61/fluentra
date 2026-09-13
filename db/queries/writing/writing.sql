-- name: InsertWritingFeedback :exec
INSERT INTO skill.writing_feedback (
    attempt_id,
    user_id,
    overall_band,
    score,
    criteria,
    annotations,
    feedback_en,
    feedback_vi,
    prompt_version,
    model
) VALUES (
    @attempt_id,
    @user_id,
    @overall_band,
    @score,
    @criteria,
    @annotations,
    @feedback_en,
    @feedback_vi,
    @prompt_version,
    @model
)
ON CONFLICT (attempt_id) DO UPDATE SET
    overall_band   = EXCLUDED.overall_band,
    score          = EXCLUDED.score,
    criteria       = EXCLUDED.criteria,
    annotations    = EXCLUDED.annotations,
    feedback_en    = EXCLUDED.feedback_en,
    feedback_vi    = EXCLUDED.feedback_vi,
    prompt_version = EXCLUDED.prompt_version,
    model          = EXCLUDED.model;

-- name: GetWritingFeedbackByAttemptAndUser :one
SELECT
    attempt_id,
    user_id,
    overall_band,
    score,
    criteria,
    annotations,
    feedback_en,
    feedback_vi,
    prompt_version,
    model,
    created_at
FROM skill.writing_feedback
WHERE attempt_id = @attempt_id
  AND user_id    = @user_id;

-- name: ListWritingSubmissionsByUser :many
SELECT
    attempt_id,
    user_id,
    overall_band,
    score,
    feedback_en,
    feedback_vi,
    prompt_version,
    created_at
FROM skill.writing_feedback
WHERE user_id = @user_id
ORDER BY created_at DESC
LIMIT @query_limit OFFSET @query_offset;

-- name: CountWritingSubmissionsByUser :one
SELECT count(*)
FROM skill.writing_feedback
WHERE user_id = @user_id;

