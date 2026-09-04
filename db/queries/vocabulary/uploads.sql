-- name: InsertUpload :one
INSERT INTO skill.vocab_uploads (user_id, raw_text, item_count)
VALUES ($1, $2, $3)
RETURNING *;

-- name: InsertUploadItem :one
-- The same word twice in one paste is one word (uq_vocab_upload_items), and
-- DO NOTHING is what makes a list with duplicates in it merely shorter rather
-- than an error the learner has to fix themselves.
INSERT INTO skill.vocab_upload_items (upload_id, user_id, term, provided_meaning)
VALUES ($1, $2, $3, $4)
ON CONFLICT (upload_id, term) DO NOTHING
RETURNING *;

-- name: GetUpload :one
SELECT * FROM skill.vocab_uploads WHERE id = $1 AND user_id = $2;

-- name: ListUploadsByUser :many
SELECT
    u.*,
    COUNT(*) FILTER (WHERE i.status = 'verified')::integer AS verified_count,
    COUNT(*) FILTER (WHERE i.status = 'rejected')::integer AS rejected_count,
    COUNT(*) FILTER (WHERE i.status = 'pending')::integer  AS pending_count,
    COUNT(*) FILTER (WHERE i.status = 'queued')::integer   AS queued_count
FROM skill.vocab_uploads u
LEFT JOIN skill.vocab_upload_items i ON i.upload_id = u.id
WHERE u.user_id = $1
GROUP BY u.id
ORDER BY u.created_at DESC
LIMIT $2;

-- name: ListUploadItems :many
SELECT * FROM skill.vocab_upload_items
WHERE upload_id = $1 AND user_id = $2
ORDER BY created_at, term;

-- name: ClaimPendingUploadItems :many
-- The verification job's input.
--
-- FOR UPDATE SKIP LOCKED so two workers running the job at once take different
-- items rather than both taking the first one and one of them wasting a
-- dictionary call and a model call on work already done. The advisory lock on
-- the cron job makes that unlikely; this makes it impossible.
--
-- `attempts < $1` retires an item that keeps failing. Without it a word the
-- dictionary and the model both choke on is retried every hour for ever, and
-- the log fills with the same failure.
SELECT * FROM skill.vocab_upload_items
WHERE status = 'pending' AND attempts < $1
ORDER BY created_at, id
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkUploadItemVerified :one
UPDATE skill.vocab_upload_items
SET status            = 'verified',
    word_sense_id     = $2,
    verified_by_model = $3,
    reason            = $4,
    verified_at       = now(),
    attempts          = attempts + 1
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkUploadItemRejected :one
UPDATE skill.vocab_upload_items
SET status      = 'rejected',
    reason      = $2,
    verified_at = now(),
    attempts    = attempts + 1
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: RecordUploadItemAttempt :exec
-- A transient failure — the dictionary was unreachable, the model timed out.
-- The item stays pending so the next run retries it; only the counter moves,
-- which is what eventually retires a word nothing can resolve.
UPDATE skill.vocab_upload_items
SET attempts = attempts + 1, reason = $2
WHERE id = $1 AND status = 'pending';

-- name: SetUploadDeck :exec
UPDATE skill.vocab_uploads SET deck_id = $2 WHERE id = $1;

-- name: CompleteFinishedUploads :many
-- Marks an upload completed once none of its items are still pending or queued.
--
-- Derived rather than counted down as items finish: a counter decremented by
-- the job is a counter that drifts the first time a run dies half way, and the
-- truth is already in the item rows.
UPDATE skill.vocab_uploads u
SET status = 'completed', completed_at = now()
WHERE u.status IN ('pending', 'processing')
  AND NOT EXISTS (
        SELECT 1 FROM skill.vocab_upload_items i
        WHERE i.upload_id = u.id AND i.status IN ('pending', 'queued')
      )
  AND EXISTS (SELECT 1 FROM skill.vocab_upload_items i WHERE i.upload_id = u.id)
RETURNING *;

-- name: CountVerifiedItemsForUser :one
SELECT COUNT(*)::bigint FROM skill.vocab_upload_items
WHERE user_id = $1 AND status = 'verified';

-- name: MarkUploadItemQueued :one
UPDATE skill.vocab_upload_items
SET status            = 'queued',
    word_sense_id     = $2,
    verified_by_model = '',
    reason            = $3,
    attempts          = attempts + 1
WHERE id = $1 AND status IN ('pending', 'queued')
RETURNING *;

-- name: ClaimQueuedUploadItems :many
SELECT * FROM skill.vocab_upload_items
WHERE status = 'queued'
  AND attempts < $1
ORDER BY created_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkQueuedUploadItemVerified :one
UPDATE skill.vocab_upload_items
SET status            = 'verified',
    verified_by_model = $2,
    verified_at       = now(),
    reason            = $3,
    attempts          = attempts + 1
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: MarkQueuedUploadItemRejected :one
UPDATE skill.vocab_upload_items
SET status      = 'rejected',
    reason      = $2,
    verified_at = now(),
    attempts    = attempts + 1
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: MarkQueuedUploadItemFailed :one
UPDATE skill.vocab_upload_items
SET status   = 'failed',
    reason   = $2,
    attempts = attempts + 1
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: ClaimPendingUploadItemsByUploadID :many
-- The immediate verification job's input: one upload's pending words.
--
-- Bounded like ClaimPendingUploadItems is, and for a sharper reason. An upload
-- carries up to MaxUploadEntries (300) words and each one is a model call with
-- its own timeout, so an unbounded claim would hold one of the `ai` queue's few
-- slots for hours while every other learner's words waited behind it. The job
-- takes a batch; the hourly sweep collects whatever is left.
SELECT * FROM skill.vocab_upload_items
WHERE upload_id = $1 AND status = 'pending' AND attempts < $2
ORDER BY created_at, id
LIMIT $3
FOR UPDATE SKIP LOCKED;
