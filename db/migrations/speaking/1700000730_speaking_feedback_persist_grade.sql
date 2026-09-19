-- +goose Up
-- +goose StatementBegin

-- The grade the learner was actually given, kept rather than reconstructed.
--
-- `speaking_feedback` stored the transcript, the criteria and the two measured
-- numbers, but not the score. The grader computes one — for a read-aloud task it
-- is `0.3 × model score + 0.7 × word accuracy` — hands it to the attempt, and
-- drops it. The submissions list then had nothing to show and re-derived a
-- replacement from the criteria, which produced a different number: the same
-- attempt read 85 in the runner and 92 in the history.
--
-- `writing_feedback` has carried `overall_band` and `score` since it was
-- written, which is why writing's list never had this problem. This makes
-- speaking match.
--
-- `task_type` is here for the same reason. The list inferred it from
-- `read_aloud_accuracy IS NOT NULL`, but the grader sets that number whenever a
-- reference text exists — so a `respond` task authored with one was labelled
-- read-aloud. The grader knows the real answer; it just never wrote it down.
ALTER TABLE skill.speaking_feedback
    ADD COLUMN IF NOT EXISTS overall_band numeric(3,1),
    ADD COLUMN IF NOT EXISTS score        integer,
    ADD COLUMN IF NOT EXISTS task_type    text;

COMMENT ON COLUMN skill.speaking_feedback.overall_band IS
    'Band the model gave the transcript. Null for rows graded before this column existed.';
COMMENT ON COLUMN skill.speaking_feedback.score IS
    'The 0-100 score the attempt was completed with, blended for read-aloud tasks.';
COMMENT ON COLUMN skill.speaking_feedback.task_type IS
    'read_aloud or respond, as authored. Null for rows graded before this column existed.';

-- Nullable on purpose, and not backfilled with a guess. A row written before
-- this migration has no record of the score it produced, and inventing one is
-- the thing this migration exists to stop. The read path reports such a row as
-- having no score rather than showing a number nobody computed.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skill.speaking_feedback
    DROP COLUMN IF EXISTS overall_band,
    DROP COLUMN IF EXISTS score,
    DROP COLUMN IF EXISTS task_type;
-- +goose StatementEnd
