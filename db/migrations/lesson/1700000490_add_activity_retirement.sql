-- +goose Up
-- +goose StatementBegin
ALTER TABLE learn.activities ADD COLUMN IF NOT EXISTS retired_at timestamptz NULL;

ALTER TABLE learn.activities DROP CONSTRAINT IF EXISTS uq_activities_lesson_position;
DROP INDEX IF EXISTS learn.uq_activities_lesson_position;
CREATE UNIQUE INDEX IF NOT EXISTS uq_activities_lesson_position ON learn.activities (lesson_id, position) WHERE retired_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS learn.uq_activities_lesson_position;
ALTER TABLE learn.activities DROP COLUMN IF EXISTS retired_at;
ALTER TABLE learn.activities ADD CONSTRAINT uq_activities_lesson_position UNIQUE (lesson_id, position);
-- +goose StatementEnd
