-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------- speaking_feedback
-- Stores structured evaluation and transcript for graded speaking attempts.
-- Owned by the speaking module, schema is `skill`.
--
-- `attempt_id` is unique but carries no foreign key to `learn.attempts`
-- because that table is partitioned on (created_at, id); matching the
-- partition key is not available at write time in the background worker.
--
-- `user_id` carries a CASCADE so account deletion cleans up metadata rows;
-- audio objects stored in S3/MinIO are removed during account erasure.
CREATE TABLE IF NOT EXISTS skill.speaking_feedback (
    id                    uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id            uuid          NOT NULL UNIQUE,
    user_id               uuid          NOT NULL
                                        REFERENCES core.users (id) ON DELETE CASCADE,
    recording_key         text          NOT NULL,
    recording_deleted_at  timestamptz,
    transcript            text          NOT NULL,
    criteria              jsonb         NOT NULL DEFAULT '[]',
    read_aloud_accuracy   numeric(5,2),
    words_per_minute      int,
    feedback_en           text          NOT NULL,
    feedback_vi           text          NOT NULL,
    prompt_version        varchar(32)   NOT NULL,
    model                 varchar(64)   NOT NULL DEFAULT '',
    asr_model             varchar(64)   NOT NULL DEFAULT '',
    created_at            timestamptz   NOT NULL DEFAULT now(),
    updated_at            timestamptz   NOT NULL DEFAULT now()
);

-- Serves user-scoped queries and CASCADE cleanup.
CREATE INDEX IF NOT EXISTS idx_speaking_feedback_user_id
    ON skill.speaking_feedback (user_id);

-- Serves direct lookups by attempt ID.
CREATE INDEX IF NOT EXISTS idx_speaking_feedback_attempt_id
    ON skill.speaking_feedback (attempt_id);

-- Serves the 90-day retention purge sweep.
CREATE INDEX IF NOT EXISTS idx_speaking_feedback_purge
    ON skill.speaking_feedback (created_at)
    WHERE recording_deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill.speaking_feedback;
-- +goose StatementEnd
