-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS content.item_reports (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    content_version_id  uuid NOT NULL REFERENCES content.content_versions (id) ON DELETE CASCADE,
    user_id             uuid NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    reason              text NOT NULL CHECK (reason IN ('wrong_answer', 'unclear', 'typo', 'my_answer_was_right', 'other')),
    note                varchar(500),
    created_at          timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT uq_item_reports_version_user UNIQUE (content_version_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_item_reports_version_id ON content.item_reports (content_version_id);
CREATE INDEX IF NOT EXISTS idx_item_reports_user_id ON content.item_reports (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS content.item_reports;

-- +goose StatementEnd
