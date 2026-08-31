-- +goose Up
-- +goose StatementBegin

-- Give a review card its own memory of when it was last reviewed.
--
-- It never had one. `learn.review_cards` has no `last_review_at`, so the
-- scheduler substituted `updated_at`:
--
--     LastReviewAt: cardRow.UpdatedAt      -- srs/service/service.go
--
-- That is wrong in two ways, and both matter.
--
-- **It is written by the wrong clock.** `updated_at` is set by `now()` inside
-- the database, while the service takes an injected clock. The whole point of
-- injecting a clock is defeated at the one place FSRS needs it, which is why
-- TestIntegration_AttemptToReviewLoop passed on 2026-08-28 and failed on
-- 2026-08-31 with nobody touching the code: once real time passed the fixture's
-- fake due date, `updated_at` landed *after* it, elapsed days became zero, and
-- the assertion "a good answer must raise stability" stopped holding.
--
-- **It is written by things that are not reviews.** Five queries set
-- `updated_at = now()` — suspending a card, resetting it, and the rest. Each one
-- silently moves the baseline FSRS measures elapsed time from. In the success
-- formula
--
--     S' = S · (1 + e^w8 · (11 − D) · S^(−w9) · (e^(w10·(1 − R)) − 1) · h)
--
-- the term `(e^(w10·(1−R)) − 1)` is zero when R = 1, and R is 1 when no time has
-- elapsed. So a learner who suspends and unsuspends a card gets S' = S exactly:
-- their intervals stop growing, quietly and for ever.
--
-- The column is nullable because "never reviewed" is a real state and is not the
-- same as "reviewed at the zero time". elapsedDays already treats a zero
-- LastReviewAt as no elapsed time, which is the correct reading for a card that
-- has only ever been scheduled.
ALTER TABLE learn.review_cards
    ADD COLUMN IF NOT EXISTS last_review_at timestamptz;

-- Backfill from `updated_at`, which is what the scheduler has been reading all
-- along. It is the best estimate available and no worse than today's behaviour;
-- a card touched by a suspend carries that timestamp either way. From here on
-- the column only moves when a review actually happens.
--
-- Only cards that have been reviewed. `reps = 0` means scheduled and never
-- answered, and inventing a review date for one would tell FSRS a review
-- happened that did not.
UPDATE learn.review_cards
SET last_review_at = updated_at
WHERE last_review_at IS NULL
  AND reps > 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE learn.review_cards DROP COLUMN IF EXISTS last_review_at;

-- +goose StatementEnd
