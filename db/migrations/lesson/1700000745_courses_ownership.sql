-- +goose Up
-- +goose StatementBegin

ALTER TABLE learn.courses
    ADD COLUMN IF NOT EXISTS owner_id          uuid REFERENCES core.users (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS visibility        text NOT NULL DEFAULT 'public',
    ADD COLUMN IF NOT EXISTS topic_taxonomy_id uuid REFERENCES content.taxonomies (id) ON DELETE SET NULL;

ALTER TABLE learn.courses
    DROP CONSTRAINT IF EXISTS ck_courses_origin;

ALTER TABLE learn.courses
    ADD CONSTRAINT ck_courses_origin CHECK (origin IN ('curriculum', 'generated', 'official', 'community'));

ALTER TABLE learn.courses
    DROP CONSTRAINT IF EXISTS ck_courses_visibility;

ALTER TABLE learn.courses
    ADD CONSTRAINT ck_courses_visibility CHECK (visibility IN ('public', 'unlisted'));

CREATE INDEX IF NOT EXISTS idx_courses_owner ON learn.courses (owner_id);
CREATE INDEX IF NOT EXISTS idx_courses_topic_taxonomy ON learn.courses (topic_taxonomy_id);
CREATE INDEX IF NOT EXISTS idx_courses_visibility_status ON learn.courses (visibility, status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS learn.idx_courses_visibility_status;
DROP INDEX IF EXISTS learn.idx_courses_topic_taxonomy;
DROP INDEX IF EXISTS learn.idx_courses_owner;

ALTER TABLE learn.courses DROP CONSTRAINT IF EXISTS ck_courses_visibility;
ALTER TABLE learn.courses DROP CONSTRAINT IF EXISTS ck_courses_origin;
ALTER TABLE learn.courses ADD CONSTRAINT ck_courses_origin CHECK (origin IN ('curriculum', 'generated'));

ALTER TABLE learn.courses
    DROP COLUMN IF EXISTS topic_taxonomy_id,
    DROP COLUMN IF EXISTS visibility,
    DROP COLUMN IF EXISTS owner_id;

-- +goose StatementEnd
