-- +goose Up
-- +goose StatementBegin

-- --------------------------------------------- creator trust, and takedowns
-- Work order 15 §8: who a submission needs a human for, and what happens to a
-- course after it is published.
--
-- `studio/AGENT.md` has described `trusted_at`, `upheld_report_count` and
-- `gate2_required` since the module landed. None of them existed, so every
-- submission went to a moderator — the scaling argument the two-gate design
-- rests on was documented and not built. These are the columns it needs.

ALTER TABLE studio.creator_profiles
    -- When this creator earned the right to publish a free course on the
    -- automated gate alone. Null means every submission of theirs is reviewed.
    --
    -- A column rather than a computation over approved_course_count, so a
    -- moderator can grant trust to somebody obviously competent and revoke it
    -- from somebody who has been wrong once, without arguing with a formula.
    ADD COLUMN IF NOT EXISTS trusted_at            timestamptz,
    ADD COLUMN IF NOT EXISTS approved_course_count integer NOT NULL DEFAULT 0,
    -- Reports about this creator's courses that a moderator agreed with. One
    -- clears `trusted_at`; three on a single course take that course down.
    ADD COLUMN IF NOT EXISTS upheld_report_count   integer NOT NULL DEFAULT 0,
    -- A suspended creator cannot submit or sell. Their published courses stay
    -- readable for the learners who bought them (BR-STUDIO-04).
    ADD COLUMN IF NOT EXISTS suspended_at          timestamptz,
    ADD COLUMN IF NOT EXISTS suspended_reason      text;

COMMENT ON COLUMN studio.creator_profiles.trusted_at IS
    'When this creator stopped needing Gate 2 for free courses. Null means always reviewed.';
COMMENT ON COLUMN studio.creator_profiles.upheld_report_count IS
    'Reports a moderator agreed with. One clears trust.';

ALTER TABLE studio.submissions
    -- Decided when the submission is created and stored, not recomputed at
    -- review time: a creator who becomes trusted while their submission sits
    -- in the queue should not have it silently skip the human who was about to
    -- read it.
    ADD COLUMN IF NOT EXISTS gate2_required bool NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS gate2_reason   text;

COMMENT ON COLUMN studio.submissions.gate2_required IS
    'Whether this submission needs a human decision, decided when it was made.';
COMMENT ON COLUMN studio.submissions.gate2_reason IS
    'Why, so a moderator reading the queue knows what they are being asked to judge.';

-- Existing submissions keep `true`, which is what they were treated as.

-- ------------------------------------------------------------ takedowns
-- A course removed from sale, and why. Kept rather than only flipping the
-- listing's status, because "when, by whom and on what grounds" is what anyone
-- asks about a takedown afterwards, and a status column answers none of it.
CREATE TABLE IF NOT EXISTS studio.takedowns (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id     uuid        NOT NULL REFERENCES learn.courses(id) ON DELETE CASCADE,
    actor_id      uuid        NOT NULL REFERENCES core.users(id),
    reason        text        NOT NULL,
    -- Null while the course is down; set when a moderator puts it back.
    reinstated_at timestamptz,
    reinstated_by uuid        REFERENCES core.users(id),
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_takedowns_reason_not_empty CHECK (length(trim(reason)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_takedowns_course ON studio.takedowns(course_id);
CREATE INDEX IF NOT EXISTS idx_takedowns_open ON studio.takedowns(course_id) WHERE reinstated_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA studio TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS studio.takedowns;

ALTER TABLE studio.submissions
    DROP COLUMN IF EXISTS gate2_required,
    DROP COLUMN IF EXISTS gate2_reason;

ALTER TABLE studio.creator_profiles
    DROP COLUMN IF EXISTS trusted_at,
    DROP COLUMN IF EXISTS approved_course_count,
    DROP COLUMN IF EXISTS upheld_report_count,
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS suspended_reason;
-- +goose StatementEnd
