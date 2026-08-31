-- +goose Up
-- +goose StatementBegin

-- Separate the curriculum from the drills generated out of it.
--
-- The practice generator upserts a course of its own — "Vocabulary Practice",
-- built from every word in the learner's dictionary — and the catalogue listed
-- it beside the authored curriculum. It sorts by `cefr_from ASC, title ASC`,
-- the generated course is A1 and the authored one is A2, so the generated
-- course came first and /learn opened on it. A learner arriving at the
-- curriculum was shown a machine-made drill set instead.
--
-- That surfaced the moment cmd/worker stopped panicking on boot: until then the
-- generator had never run, so nothing but the seeded course existed.
--
-- The distinction is what each course *is*, not how it happens to sort, so it
-- becomes a column. `curriculum` is the default because everything that exists
-- today was authored, and because a new course should have to say it is
-- generated rather than have to say it is not.
ALTER TABLE learn.courses
    ADD COLUMN IF NOT EXISTS origin text NOT NULL DEFAULT 'curriculum';

ALTER TABLE learn.courses
    DROP CONSTRAINT IF EXISTS ck_courses_origin;

ALTER TABLE learn.courses
    ADD CONSTRAINT ck_courses_origin CHECK (origin IN ('curriculum', 'generated'));

-- The one course that already exists and is not curriculum. Keyed on the slug
-- the generator uses, which is the same key that makes its upsert idempotent.
UPDATE learn.courses
SET origin = 'generated', updated_at = now()
WHERE slug = 'generated-vocabulary-practice'
  AND origin <> 'generated';

-- The catalogue reads this column on every request.
CREATE INDEX IF NOT EXISTS idx_courses_origin_status
    ON learn.courses (origin, status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS learn.idx_courses_origin_status;
ALTER TABLE learn.courses DROP CONSTRAINT IF EXISTS ck_courses_origin;
ALTER TABLE learn.courses DROP COLUMN IF EXISTS origin;

-- +goose StatementEnd
