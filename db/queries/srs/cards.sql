-- name: UpsertReviewCard :one
--
-- `last_review_at` is set on creation, because creation *is* a review: the card
-- is born from a graded answer, and Schedule gives it reps = 1. Leaving it null
-- meant the first real review measured zero elapsed time, and R = 1 makes the
-- success term `(e^(w10·(1−R)) − 1)` exactly zero — so the first answer after a
-- lesson never raised stability, and the interval never grew.
INSERT INTO learn.review_cards (
    user_id, content_version_id, skill, stability, difficulty, due_at, reps, lapses, state,
    last_review_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, sqlc.arg('last_review_at'), now()
)
-- The conflict path deliberately leaves the schedule alone. A learner who redoes
-- a lesson activity already has a card carrying weeks of FSRS history; copying
-- the initial stability, difficulty, reps and lapses over it would silently reset
-- that history and re-teach a word they know. Rescheduling happens through
-- UpdateReviewCardSchedule, on the answer path, and nowhere else.
ON CONFLICT (user_id, content_version_id) DO UPDATE SET
    skill = EXCLUDED.skill,
    updated_at = now()
RETURNING *;

-- name: GetReviewCardByID :one
SELECT * FROM learn.review_cards
WHERE id = $1 AND user_id = $2;

-- name: GetReviewCardByUserAndContent :one
SELECT * FROM learn.review_cards
WHERE user_id = $1 AND content_version_id = $2;

-- name: ListDueCards :many
SELECT * FROM learn.review_cards
WHERE user_id = $1
  AND suspended_at IS NULL
  AND due_at <= $2
ORDER BY due_at ASC, id ASC
LIMIT $3;

-- name: ListDueCardsByDeck :many
SELECT c.* FROM learn.review_cards c
JOIN skill.word_senses ws ON ws.content_version_id = c.content_version_id
JOIN skill.deck_items di ON di.word_sense_id = ws.id
WHERE c.user_id = $1
  AND di.deck_id = $2
  AND c.suspended_at IS NULL
  AND c.due_at <= $3
ORDER BY c.due_at ASC, c.id ASC
LIMIT $4;

-- name: CountDueCards :one
SELECT COUNT(*)::bigint FROM learn.review_cards
WHERE user_id = $1
  AND suspended_at IS NULL
  AND due_at <= $2;

-- name: UpdateReviewCardSchedule :one
--
-- `last_review_at` comes from the caller, not from now(). It is the moment the
-- learner answered, measured by the injected clock, and it is the baseline FSRS
-- measures elapsed time from. Reading it off `updated_at` — which the database
-- writes, and which suspend and reset also write — is what made a suspended card
-- stop growing its interval, and what made an integration test flip from green
-- to red on a date change with nobody touching the code.
UPDATE learn.review_cards SET
    stability = $3,
    difficulty = $4,
    due_at = $5,
    reps = $6,
    lapses = $7,
    state = $8,
    last_review_at = $9,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: SuspendReviewCard :one
UPDATE learn.review_cards SET
    suspended_at = now(),
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- SetReviewCardsSuspended is how another module takes content out of, or puts it
-- back into, a learner's rotation without owning learn.review_cards itself.
-- vocabulary calls it when a learner marks a word known or ignored.
-- name: SetReviewCardsSuspended :execrows
UPDATE learn.review_cards SET
    suspended_at = CASE WHEN @suspended::boolean THEN now() ELSE NULL END,
    updated_at = now()
WHERE user_id = @user_id
  AND content_version_id = ANY(@content_version_ids::uuid[]);

-- name: ResetReviewCard :one
UPDATE learn.review_cards SET
    stability = $3,
    difficulty = $4,
    due_at = $5,
    reps = 0,
    lapses = 0,
    state = 'new',
    suspended_at = NULL,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- ForecastDueCards buckets the learner's upcoming workload by local calendar day.
-- The date is computed in the learner's own timezone for the same reason the due
-- queue is: a projection bucketed in UTC shows a learner in Asia/Ho_Chi_Minh the
-- wrong day's workload for every card scheduled between 17:00 and midnight.
-- name: ForecastDueCards :many
SELECT (due_at AT TIME ZONE @timezone::text)::date AS due_date,
       COUNT(*)::bigint AS due_count
FROM learn.review_cards
WHERE user_id = @user_id
  AND suspended_at IS NULL
  AND due_at < @until
GROUP BY 1
ORDER BY 1;

-- name: EnsureSRSPartitions :one
SELECT learn.ensure_srs_partitions($1::integer) AS created_count;

-- SuspendReviewCardsByContentVersion suspends a card for EVERY learner holding
-- it, which is what withdrawing shared material requires: skill.word_senses is
-- one row shared across everyone who added the word, so taking it out of the
-- dictionary has to take it out of every queue, not one.
--
-- It returns the affected user_ids because the due-count cache is keyed per
-- learner and a global write cannot otherwise name whose entry went stale.
-- name: SuspendReviewCardsByContentVersion :many
UPDATE learn.review_cards SET
    suspended_at = now(),
    updated_at = now()
WHERE content_version_id = ANY(@content_version_ids::uuid[])
  AND suspended_at IS NULL
RETURNING user_id;
