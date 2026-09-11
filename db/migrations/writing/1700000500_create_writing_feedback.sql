-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------- writing_feedback
-- Stores structured IELTS-style feedback for graded writing attempts.
-- Owned by the writing module, schema is `skill`.
--
-- `attempt_id` is the primary key but carries no FK to `learn.attempts`
-- because that table is partitioned on (created_at, id); a regular FK
-- would require matching the partition key, which is not available at
-- write time in the worker.
--
-- `user_id` carries a CASCADE so account deletion removes all feedback.
-- Annotations quote the learner's essay, making this learner data.
CREATE TABLE IF NOT EXISTS skill.writing_feedback (
    attempt_id      uuid          PRIMARY KEY,
    user_id         uuid          NOT NULL
                                  REFERENCES core.users (id) ON DELETE CASCADE,
    overall_band    numeric(3,1)  NOT NULL,
    score           int           NOT NULL,
    criteria        jsonb         NOT NULL,
    annotations     jsonb         NOT NULL DEFAULT '[]',
    feedback_en     text          NOT NULL,
    feedback_vi     text          NOT NULL,
    prompt_version  text          NOT NULL,
    model           text          NOT NULL DEFAULT '',
    created_at      timestamptz   NOT NULL DEFAULT now()
);

-- Serves cascade deletes and user-scoped queries (e.g. "my writing" page).
CREATE INDEX IF NOT EXISTS idx_writing_feedback_user_id
    ON skill.writing_feedback (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill.writing_feedback;
-- +goose StatementEnd
