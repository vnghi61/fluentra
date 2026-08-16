-- name: CreateChallenge :one
-- The id is supplied by the caller (UUIDv7 from shared/id) because the code
-- hash is bound to it: the id has to exist before the digest can be computed.
INSERT INTO core.auth_challenges (
    id, purpose, subject_hash, code_hash, max_attempts, expires_at, user_id, last_sent_at, created_at, updated_at
)
VALUES (@id, @purpose, @subject_hash, @code_hash, @max_attempts, @expires_at, @user_id, @now, @now, @now)
RETURNING id, purpose, subject_hash, code_hash, attempts, max_attempts,
          expires_at, consumed_at, last_sent_at, created_at, updated_at, user_id;

-- name: GetChallengeByID :one
SELECT id, purpose, subject_hash, code_hash, attempts, max_attempts,
       expires_at, consumed_at, last_sent_at, created_at, updated_at, user_id
FROM core.auth_challenges
WHERE id = @id;

-- name: ConsumeChallenge :one
-- Match-and-consume in one statement. Every condition that makes a challenge
-- unusable is in the WHERE, so a challenge cannot be consumed twice however many
-- requests arrive at once: the second finds no row.
--
-- The `code_hash = @code_hash` term is not the security comparison — that has
-- already happened in Go, in constant time, and this statement only runs when it
-- succeeded. It is here so that a resend landing between the read and this write
-- invalidates the consumption rather than letting a superseded code through.
UPDATE core.auth_challenges
-- The cast keeps @now a non-nullable timestamptz in the generated signature.
-- Without it sqlc infers the parameter's nullability from consumed_at, which is
-- nullable, and hands back a *time.Time for a value that is never absent.
SET consumed_at = @now::timestamptz, updated_at = @now
WHERE id = @id
  AND code_hash = @code_hash
  AND consumed_at IS NULL
  AND attempts < max_attempts
  AND expires_at > @now
RETURNING id, purpose, subject_hash, code_hash, attempts, max_attempts,
          expires_at, consumed_at, last_sent_at, created_at, updated_at, user_id;

-- name: RecordFailedAttempt :one
-- `attempts + 1` is evaluated against the row as the statement finds it, not
-- against a value the caller read earlier, so two wrong guesses arriving
-- together consume two attempts rather than one. The WHERE is what stops the
-- sixth: it matches no row, and the caller reads that as burned.
UPDATE core.auth_challenges
SET attempts = attempts + 1, updated_at = @now
WHERE id = @id
  AND consumed_at IS NULL
  AND attempts < max_attempts
  AND expires_at > @now
RETURNING id, purpose, subject_hash, code_hash, attempts, max_attempts,
          expires_at, consumed_at, last_sent_at, created_at, updated_at, user_id;

-- name: ResendChallenge :one
-- Replaces the code and clears the attempts, and deliberately does not touch
-- expires_at (BR-AUTH-13): resending gives a learner a fresh code, not a fresh
-- ten minutes.
--
-- `attempts < max_attempts` keeps a burned challenge burned — BR-AUTH-12 says a
-- new challenge must be requested, so resend must not be a way to un-burn one.
-- `last_sent_at <= @resend_allowed_from` is the cooldown, enforced here as well
-- as in Redis because the Redis limiter allows the request when it is
-- unreachable.
UPDATE core.auth_challenges
SET code_hash = @code_hash, attempts = 0, last_sent_at = @now, updated_at = @now
WHERE id = @id
  AND consumed_at IS NULL
  AND attempts < max_attempts
  AND expires_at > @now
  AND last_sent_at <= @resend_allowed_from
RETURNING id, purpose, subject_hash, code_hash, attempts, max_attempts,
          expires_at, consumed_at, last_sent_at, created_at, updated_at, user_id;

-- BurnLiveChallengesForSubject spends the attempt budget of every outstanding
-- challenge of one purpose for one subject.
--
-- It runs when a new challenge of the same purpose is issued, so that an older
-- email sitting in an inbox stops being a way in. Burning by attempts rather
-- than by consumed_at is the convention this table already uses -- there is no
-- burned_at column, and "burned" is derived from attempts >= max_attempts, so a
-- second stored record of the same fact cannot disagree with the first. It also
-- reads truthfully: the budget for presenting a code against this challenge is
-- spent, which is exactly what happened. Marking it consumed would claim
-- somebody used it, and nobody did.
--
-- name: BurnLiveChallengesForSubject :execrows
UPDATE core.auth_challenges
SET attempts = max_attempts, updated_at = sqlc.arg(now)::timestamptz
WHERE purpose = $1
  AND subject_hash = $2
  AND consumed_at IS NULL
  AND attempts < max_attempts
  AND expires_at > sqlc.arg(now)::timestamptz;

-- name: DeleteChallengesForUser :exec
DELETE FROM core.auth_challenges
WHERE user_id = $1;

