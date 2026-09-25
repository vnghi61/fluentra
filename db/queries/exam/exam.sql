-- --------------------------------------------------------------------- exams
-- name: ListExams :many
SELECT *
FROM assess.exams
ORDER BY level ASC, slug ASC;

-- name: GetExamByID :one
SELECT *
FROM assess.exams
WHERE id = $1;

-- name: GetExamBySlug :one
SELECT *
FROM assess.exams
WHERE slug = $1;

-- name: ListExamSections :many
SELECT *
FROM assess.exam_sections
WHERE exam_id = $1
ORDER BY position ASC;

-- ------------------------------------------------------------- exam_attempts
-- name: CreateExamAttempt :one
INSERT INTO assess.exam_attempts (
    id, user_id, exam_id, mode, chosen_duration_minutes,
    started_at, deadline_at, current_section, status,
    section_activities, draft_answers, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $6, $6
) RETURNING *;

-- name: CreateMockTestAttempt :one
INSERT INTO assess.exam_attempts (
    id, user_id, exam_id, mode, chosen_duration_minutes,
    started_at, deadline_at, current_section, status,
    section_activities, draft_answers, mock_test_id, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12, $6, $6
) RETURNING *;

-- name: CountUserMockTestAttempts :one
SELECT COUNT(*)
FROM assess.exam_attempts
WHERE user_id = $1 AND mock_test_id = $2;

-- name: GetExamAttemptByID :one
SELECT *
FROM assess.exam_attempts
WHERE id = $1;

-- name: GetLatestUserMockTestAttempt :one
-- The caller's most recent sitting of one fixed test, with its report if any.
SELECT a.id, a.status, a.started_at, r.overall_score, r.overall_band
FROM assess.exam_attempts a
LEFT JOIN assess.score_reports r ON r.attempt_id = a.id
WHERE a.user_id = $1 AND a.mock_test_id = $2
ORDER BY a.started_at DESC
LIMIT 1;

-- name: GetExamAttemptForUser :one
SELECT *
FROM assess.exam_attempts
WHERE id = $1 AND user_id = $2;

-- name: ListUserExamAttempts :many
SELECT *
FROM assess.exam_attempts
WHERE user_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUserExamAttempts :one
SELECT COUNT(*)
FROM assess.exam_attempts
WHERE user_id = $1;

-- name: CountUserActiveAttempts :one
SELECT COUNT(*)
FROM assess.exam_attempts
WHERE user_id = $1 AND status = 'in_progress';

-- name: CountUserAttemptsToday :one
SELECT COUNT(*)
FROM assess.exam_attempts
WHERE user_id = $1
  AND started_at >= $2
  AND started_at < $3;

-- name: UpdateDraftAnswers :one
UPDATE assess.exam_attempts
SET draft_answers = $2,
    updated_at = $3
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: UpdateCurrentSection :one
UPDATE assess.exam_attempts
SET current_section = $2,
    updated_at = $3
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: MarkAttemptCompleted :one
UPDATE assess.exam_attempts
SET status = 'completed',
    submitted_at = $2,
    submitted_by = $3,
    updated_at = $2
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: MarkAttemptExpired :one
UPDATE assess.exam_attempts
SET status = 'expired',
    submitted_at = $2,
    submitted_by = 'expiry',
    updated_at = $2
WHERE id = $1 AND status = 'in_progress'
RETURNING *;

-- name: ListExpiredInProgressAttempts :many
SELECT *
FROM assess.exam_attempts
WHERE status = 'in_progress'
  AND deadline_at <= $1
ORDER BY deadline_at ASC
LIMIT 50;

-- ------------------------------------------------------------- score_reports
-- name: CreateScoreReport :one
INSERT INTO assess.score_reports (
    id, attempt_id, user_id, exam_id, overall_score,
    overall_band, status, per_section, feedback,
    integrity_signals, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $11
) RETURNING *;

-- name: GetScoreReportByAttemptID :one
SELECT *
FROM assess.score_reports
WHERE attempt_id = $1;

-- ListPendingScoreReports feeds the sweep that settles reports whose sittings
-- held asynchronously graded items.
-- name: ListPendingScoreReports :many
SELECT *
FROM assess.score_reports
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 50;

-- name: UpdateScoreReport :one
UPDATE assess.score_reports
SET overall_score = $2,
    overall_band = $3,
    status = $4,
    per_section = $5,
    feedback = $6,
    integrity_signals = $7,
    updated_at = $8
WHERE attempt_id = $1
RETURNING *;

-- ---------------------------------------------------------- integrity_events
-- name: RecordIntegrityEvent :exec
INSERT INTO assess.integrity_events (
    id, attempt_id, kind, occurred_at, metadata
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: ListIntegrityEvents :many
SELECT *
FROM assess.integrity_events
WHERE attempt_id = $1
ORDER BY occurred_at ASC;

-- name: GetUserBestVersionScore :one
-- The caller's best overall score on one exam version: full sittings in exam
-- mode with a ready report only, since a practice of one section is not
-- comparable (WO 22 Stage L, the exam card).
SELECT MAX(r.overall_score)::numeric AS best_score
FROM assess.exam_attempts a
JOIN assess.score_reports r ON r.attempt_id = a.id
JOIN assess.exams e ON e.id = a.exam_id
WHERE a.user_id = $1 AND e.version_id = $2 AND a.mode = 'exam' AND r.status = 'ready';
