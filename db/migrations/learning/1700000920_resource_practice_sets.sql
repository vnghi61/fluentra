-- +goose Up
-- +goose StatementBegin

-- One private practice set per resource, regenerable once a day (WO 21 D21-2).
--
-- The set is the ten activities generated from a learner's own upload. It is
-- private to them (BR-RESOURCE-12), so the row carries the owner and the read
-- path filters on it: another learner's resource id answers 404.
CREATE TABLE IF NOT EXISTS learn.resource_practice_sets (
    resource_id    uuid        PRIMARY KEY,
    user_id        uuid        NOT NULL REFERENCES core.users (id),
    lesson_id      uuid,
    activity_ids   uuid[]      NOT NULL DEFAULT '{}',
    status         text        NOT NULL DEFAULT 'generating',
    failure_reason text        NOT NULL DEFAULT '',
    generated_on   date        NOT NULL DEFAULT CURRENT_DATE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_resource_practice_sets_status
        CHECK (status IN ('generating', 'ready', 'failed'))
);

-- The generation sweep claims by status, oldest first.
CREATE INDEX IF NOT EXISTS idx_resource_practice_sets_generating
    ON learn.resource_practice_sets (created_at ASC)
    WHERE status = 'generating';

CREATE INDEX IF NOT EXISTS idx_resource_practice_sets_user
    ON learn.resource_practice_sets (user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON learn.resource_practice_sets TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.resource_practice_sets;
-- +goose StatementEnd
