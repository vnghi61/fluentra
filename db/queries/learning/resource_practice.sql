-- Private practice generated from a learner's own resource (WO 21 Stage B).
--
-- The set is a row here plus real activities in a hidden per-learner course, so
-- the existing lesson runner can sit it. Regenerable once a day: the upsert
-- moves generated_on to today and clears the activity list.

-- name: UpsertResourcePracticeSet :one
INSERT INTO learn.resource_practice_sets (
    resource_id, user_id, status, failure_reason, generated_on, updated_at
) VALUES (
    $1, $2, 'generating', '', CURRENT_DATE, now()
) ON CONFLICT (resource_id) DO UPDATE
SET user_id        = EXCLUDED.user_id,
    lesson_id      = NULL,
    activity_ids   = '{}',
    status         = 'generating',
    failure_reason = '',
    generated_on   = EXCLUDED.generated_on,
    updated_at     = now()
RETURNING *;

-- name: GetResourcePracticeSet :one
SELECT *
FROM learn.resource_practice_sets
WHERE resource_id = $1 AND user_id = $2;

-- name: MarkResourcePracticeSetReady :one
UPDATE learn.resource_practice_sets
SET lesson_id    = $3,
    activity_ids = $4,
    status       = 'ready',
    updated_at   = now()
WHERE resource_id = $1 AND user_id = $2
RETURNING *;

-- name: MarkResourcePracticeSetFailed :one
UPDATE learn.resource_practice_sets
SET status         = 'failed',
    failure_reason = $3,
    updated_at     = now()
WHERE resource_id = $1 AND user_id = $2
RETURNING *;

-- name: ClaimGeneratingResourcePracticeSets :many
-- The generation sweep. Oldest first, bounded by the caller.
SELECT *
FROM learn.resource_practice_sets
WHERE status = 'generating'
ORDER BY created_at ASC
LIMIT $1;

-- name: DeleteResourcePracticeSetsForUser :exec
DELETE FROM learn.resource_practice_sets WHERE user_id = $1;
