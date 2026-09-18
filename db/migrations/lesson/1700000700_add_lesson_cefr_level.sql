-- +goose Up
-- +goose StatementBegin
ALTER TABLE learn.lessons ADD COLUMN IF NOT EXISTS cefr_level core.cefr_level NULL;

CREATE INDEX IF NOT EXISTS idx_lessons_cefr_level ON learn.lessons (cefr_level) WHERE cefr_level IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS learn.idx_lessons_cefr_level;
ALTER TABLE learn.lessons DROP COLUMN IF EXISTS cefr_level;
-- +goose StatementEnd
