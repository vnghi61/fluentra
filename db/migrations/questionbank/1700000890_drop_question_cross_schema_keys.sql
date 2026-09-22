-- +goose Up
-- +goose StatementBegin

-- DB4 (ADR-0004): the only cross-schema foreign key permitted is to
-- core.users(id). 1700000850 pointed assess.questions at content.content_items
-- and learn.activities; WO 19 F.1 specified both as bare uuids. Keep the
-- columns, drop the keys. questionbank now refuses to record a generated
-- question without resolving its item through content's contract, and only
-- sets activity_id from lesson's AppendActivity.
ALTER TABLE assess.questions
    DROP CONSTRAINT IF EXISTS questions_content_item_id_fkey,
    DROP CONSTRAINT IF EXISTS questions_activity_id_fkey;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE assess.questions
    ADD CONSTRAINT questions_content_item_id_fkey
        FOREIGN KEY (content_item_id) REFERENCES content.content_items (id) ON DELETE RESTRICT,
    ADD CONSTRAINT questions_activity_id_fkey
        FOREIGN KEY (activity_id) REFERENCES learn.activities (id) ON DELETE SET NULL;

-- +goose StatementEnd
