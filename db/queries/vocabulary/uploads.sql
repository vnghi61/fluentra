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
-- Keyset on (created_at, id), newest first. The caller asks for one row more
-- than the page to learn whether another page exists.
SELECT
    u.*,
    COUNT(i.id) FILTER (WHERE i.status = 'verified')::integer AS verified_count,
    COUNT(i.id) FILTER (WHERE i.status = 'rejected')::integer AS rejected_count,
    COUNT(i.id) FILTER (WHERE i.status = 'pending')::integer  AS pending_count,
    COUNT(i.id) FILTER (WHERE i.status = 'queued')::integer   AS queued_count,
    COUNT(i.id) FILTER (WHERE i.status = 'failed')::integer   AS failed_count
FROM skill.vocab_uploads u
LEFT JOIN skill.vocab_upload_items i ON i.upload_id = u.id
WHERE u.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (u.created_at, u.id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid))
GROUP BY u.id
ORDER BY u.created_at DESC, u.id DESC
LIMIT sqlc.arg(row_limit);

-- name: SearchUserUploadItems :many
-- Every word a learner has pasted, across uploads: what the search box and an
-- opened upload page through. Newest upload first, then alphabetical, keyset on
-- (created_at, term, id). `query` arrives LIKE-escaped from the service.
SELECT
    i.*,
    s.definition,
    s.definition_vi,
    s.domain AS topic,
    s.examples
FROM skill.vocab_upload_items i
LEFT JOIN skill.word_senses s ON s.id = i.word_sense_id
WHERE i.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(upload_id)::uuid IS NULL OR i.upload_id = sqlc.narg(upload_id)::uuid)
  AND (cardinality(sqlc.arg(statuses)::text[]) = 0 OR i.status = ANY(sqlc.arg(statuses)::text[]))
  AND (sqlc.arg(query)::text = ''
       OR i.term ILIKE '%' || sqlc.arg(query)::text || '%'
       OR i.corrected_term ILIKE '%' || sqlc.arg(query)::text || '%')
  AND (sqlc.narg(after_created_at)::timestamptz IS NULL
       OR i.created_at < sqlc.narg(after_created_at)::timestamptz
       OR (i.created_at = sqlc.narg(after_created_at)::timestamptz
           AND (i.term, i.id) > (sqlc.narg(after_term)::text, sqlc.narg(after_id)::uuid)))
ORDER BY i.created_at DESC, i.term, i.id
LIMIT sqlc.arg(row_limit);

-- name: LockUploadTerm :exec
-- Serialises verification of one term across every learner for the rest of
-- the caller's transaction: the second learner to paste a new word waits for
-- the first one's dictionary and model call, then finds the word stored.
SELECT pg_advisory_xact_lock(hashtextextended('vocab-upload-term:' || sqlc.arg(term)::text, 0));

-- name: FindRecentTermRejection :one
-- A verdict on the word itself, not on a learner's gloss, so only one given
-- without a meaning is reused, and only for a month.
SELECT reason, suggested_term, note_code
FROM skill.vocab_upload_items
WHERE lower(btrim(term)) = sqlc.arg(term)::text
  AND status = 'rejected'
  AND provided_meaning = ''
  AND note_code IN ('not_a_word', 'proper_noun', 'spelling_suggestion')
  AND verified_at > now() - interval '30 days'
ORDER BY verified_at DESC
LIMIT 1;

-- name: ListUploadItems :many
SELECT
    i.*,
    s.definition,
    s.definition_vi,
    s.domain AS topic,
    s.examples
FROM skill.vocab_upload_items i
LEFT JOIN skill.word_senses s ON s.id = i.word_sense_id
WHERE i.upload_id = $1 AND i.user_id = $2
ORDER BY i.created_at, i.term;

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
    corrected_term    = $5,
    note_code         = $6,
    verified_at       = now(),
    attempts          = attempts + 1
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkUploadItemRejected :one
UPDATE skill.vocab_upload_items
SET status         = 'rejected',
    reason         = $2,
    suggested_term = $3,
    note_code      = $4,
    verified_at    = now(),
    attempts       = attempts + 1
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: RecordUploadItemAttempt :exec
-- A transient failure — the dictionary was unreachable, the model timed out.
-- The item stays pending so the next run retries it, until the attempt that
-- reaches the limit: that one retires it as failed with the learner-facing
-- reason. Retiring by status, not only by the claim's `attempts < limit`
-- filter, is what keeps a word from showing "checking" for ever.
UPDATE skill.vocab_upload_items
SET attempts = attempts + 1,
    reason   = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts)::int
                    THEN sqlc.arg(exhausted_reason)::text ELSE sqlc.arg(reason)::text END,
    status   = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts)::int
                    THEN 'failed' ELSE status END
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: FailExhaustedUploadItems :execrows
-- Retires pending items that already used every attempt before
-- RecordUploadItemAttempt retired them itself; they were left pending, which
-- the learner saw as a word that never finished checking.
UPDATE skill.vocab_upload_items
SET status = 'failed', reason = sqlc.arg(reason)::text
WHERE status = 'pending' AND attempts >= sqlc.arg(max_attempts)::int;

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

-- name: MarkUploadItemQueued :one
UPDATE skill.vocab_upload_items
SET status            = 'queued',
    word_sense_id     = $2,
    verified_by_model = '',
    reason            = $3,
    note_code         = $4,
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
    corrected_term    = $4,
    note_code         = $5,
    attempts          = attempts + 1
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: MarkQueuedUploadItemRejected :one
UPDATE skill.vocab_upload_items
SET status         = 'rejected',
    reason         = $2,
    suggested_term = $3,
    note_code      = $4,
    verified_at    = now(),
    attempts       = attempts + 1
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
-- takes a batch at a time, within the job timeout; the hourly sweep collects
-- whatever is left.
--
-- `item_ids`, when given, narrows the claim to the words that job was created
-- for: Submit resolves the words the database already holds itself, and a job
-- that claimed those too would race it for the same rows.
SELECT * FROM skill.vocab_upload_items
WHERE upload_id = sqlc.arg(upload_id) AND status = 'pending' AND attempts < sqlc.arg(max_attempts)
  AND (sqlc.narg(item_ids)::uuid[] IS NULL OR id = ANY(sqlc.narg(item_ids)::uuid[]))
ORDER BY created_at, id
LIMIT sqlc.arg(row_limit)
FOR UPDATE SKIP LOCKED;

-- name: ListLearnerWordsQueueAdmin :many
SELECT
    i.id AS upload_item_id,
    i.upload_id,
    i.user_id,
    i.term,
    i.provided_meaning,
    i.status,
    i.verified_by_model,
    i.reason,
    i.corrected_term,
    i.suggested_term,
    i.note_code,
    i.attempts,
    i.verified_at,
    i.created_at,
    i.word_sense_id,
    s.word_id,
    s.content_version_id,
    s.definition,
    s.definition_vi,
    s.domain AS topic,
    s.examples,
    d.id AS deck_id,
    d.name AS deck_name
FROM skill.vocab_upload_items i
JOIN skill.vocab_uploads u ON u.id = i.upload_id
LEFT JOIN skill.decks d ON d.id = u.deck_id
LEFT JOIN skill.word_senses s ON s.id = i.word_sense_id
WHERE (sqlc.narg('status')::text IS NULL OR i.status = sqlc.narg('status'))
  AND (sqlc.narg('query')::text IS NULL OR i.term ILIKE sqlc.narg('query') || '%')
ORDER BY i.created_at DESC
LIMIT @result_limit OFFSET @result_offset;

-- name: CountLearnerWordsQueueAdmin :one
SELECT COUNT(*)::bigint
FROM skill.vocab_upload_items i
WHERE (sqlc.narg('status')::text IS NULL OR i.status = sqlc.narg('status'))
  AND (sqlc.narg('query')::text IS NULL OR i.term ILIKE sqlc.narg('query') || '%');
