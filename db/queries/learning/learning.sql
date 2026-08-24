-- name: GetEnrollmentByUserCourse :one
SELECT id, user_id, course_id, status, started_at, completed_at, created_at, updated_at
FROM learn.enrollments
WHERE user_id = $1 AND course_id = $2;

-- name: ListEnrollmentsByUser :many
SELECT id, user_id, course_id, status, started_at, completed_at, created_at, updated_at
FROM learn.enrollments
WHERE user_id = $1
ORDER BY started_at DESC
LIMIT $2;

-- name: CreateEnrollment :one
INSERT INTO learn.enrollments (user_id, course_id, status, started_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, course_id, status, started_at, completed_at, created_at, updated_at;

-- name: UpdateEnrollmentStatus :one
UPDATE learn.enrollments
SET status = $3,
    completed_at = $4,
    updated_at = now()
WHERE user_id = $1 AND course_id = $2
RETURNING id, user_id, course_id, status, started_at, completed_at, created_at, updated_at;

-- name: GetProgressByUserScope :one
SELECT id, user_id, scope, scope_id, status, score, completed_at, created_at, updated_at
FROM learn.progress
WHERE user_id = $1 AND scope = $2 AND scope_id = $3;

-- name: ListProgressByUser :many
SELECT id, user_id, scope, scope_id, status, score, completed_at, created_at, updated_at
FROM learn.progress
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListProgressByUserAndScope :many
SELECT id, user_id, scope, scope_id, status, score, completed_at, created_at, updated_at
FROM learn.progress
WHERE user_id = $1 AND scope = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: CreateProgress :one
INSERT INTO learn.progress (user_id, scope, scope_id, status, score, completed_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, scope, scope_id, status, score, completed_at, created_at, updated_at;

-- name: UpdateProgress :one
UPDATE learn.progress
SET status = $4,
    score = $5,
    completed_at = $6,
    updated_at = now()
WHERE user_id = $1 AND scope = $2 AND scope_id = $3
RETURNING id, user_id, scope, scope_id, status, score, completed_at, created_at, updated_at;

-- name: CreateAttempt :one
INSERT INTO learn.attempts (
    id, created_at, user_id, activity_id, idempotency_key,
    response, score, max_score, grader, duration_ms, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, created_at, updated_at, user_id, activity_id, idempotency_key,
          response, score, max_score, grader, duration_ms, status;

-- name: GetAttemptByID :one
SELECT id, created_at, updated_at, user_id, activity_id, idempotency_key,
       response, score, max_score, grader, duration_ms, status
FROM learn.attempts
WHERE id = $1;

-- name: ListAttemptsByUserAndActivity :many
SELECT id, created_at, updated_at, user_id, activity_id, idempotency_key,
       response, score, max_score, grader, duration_ms, status
FROM learn.attempts
WHERE user_id = $1 AND activity_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- ClaimAttemptForGrading is what makes submission idempotent, and it is the
-- reason `attempts` needs no unique index on idempotency_key.
--
-- A unique constraint on a partitioned table has to include the partition key,
-- so `UNIQUE (idempotency_key)` is not available and `UNIQUE (idempotency_key,
-- created_at)` would only hold within one calendar month. The invariant that
-- actually matters is narrower than global key uniqueness: **one attempt row is
-- graded at most once**. That is a per-row invariant, and `AND status =
-- 'in_progress'` makes the database the arbiter of it — of two concurrent
-- submissions, one updates the row and the other returns no rows, because the
-- loser re-evaluates the WHERE clause after the winner commits.
--
-- The caller that gets no rows reads the attempt back: a matching
-- idempotency_key means its own retry already succeeded and the stored result is
-- the answer; a different one means a second client is submitting the same
-- attempt, which is a conflict, not a retry.
-- name: ClaimAttemptForGrading :one
UPDATE learn.attempts
SET status          = 'grading',
    idempotency_key = $3,
    response        = $4,
    updated_at      = now()
WHERE id = $1
  AND created_at = $2
  AND status = 'in_progress'
RETURNING id, created_at, updated_at, user_id, activity_id, idempotency_key,
          response, score, max_score, grader, duration_ms, status;

-- name: UpdateAttemptStatus :one
UPDATE learn.attempts
SET status = $3,
    score = $4,
    grader = $5,
    duration_ms = $6,
    updated_at = now()
WHERE id = $1 AND created_at = $2
RETURNING id, created_at, updated_at, user_id, activity_id, idempotency_key,
          response, score, max_score, grader, duration_ms, status;

-- name: CreateLearningSession :one
INSERT INTO learn.learning_sessions (user_id, started_at, metadata)
VALUES ($1, $2, $3)
RETURNING id, user_id, started_at, ended_at, activities_completed, minutes, metadata, created_at, updated_at;

-- name: CompleteLearningSession :one
UPDATE learn.learning_sessions
SET ended_at = $2,
    activities_completed = $3,
    minutes = $4,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, started_at, ended_at, activities_completed, minutes, metadata, created_at, updated_at;

-- name: GetSkillMastery :one
SELECT id, user_id, skill, level, confidence, updated_at, created_at
FROM learn.skill_mastery
WHERE user_id = $1 AND skill = $2;

-- name: ListSkillMasteryByUser :many
SELECT id, user_id, skill, level, confidence, updated_at, created_at
FROM learn.skill_mastery
WHERE user_id = $1
ORDER BY skill ASC;

-- name: UpsertSkillMastery :one
INSERT INTO learn.skill_mastery (user_id, skill, level, confidence, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (user_id, skill)
DO UPDATE SET
    level = EXCLUDED.level,
    confidence = EXCLUDED.confidence,
    updated_at = now()
RETURNING id, user_id, skill, level, confidence, updated_at, created_at;

-- name: CreatePlacementResult :one
INSERT INTO learn.placement_results (user_id, estimated_level, per_skill, taken_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, estimated_level, per_skill, taken_at, created_at, updated_at;

-- name: GetLatestPlacementResult :one
SELECT id, user_id, estimated_level, per_skill, taken_at, created_at, updated_at
FROM learn.placement_results
WHERE user_id = $1
ORDER BY taken_at DESC
LIMIT 1;

-- name: EnsurePartitions :one
SELECT learn.ensure_partitions($1::integer) AS created_count;
