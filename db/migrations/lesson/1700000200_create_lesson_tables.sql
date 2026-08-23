-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- courses
CREATE TABLE IF NOT EXISTS learn.courses (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text        NOT NULL,
    title           text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    cefr_from       text        NOT NULL,
    cefr_to         text        NOT NULL,
    status          text        NOT NULL DEFAULT 'draft',
    estimated_hours integer     NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_courses_slug UNIQUE (slug),
    CONSTRAINT ck_courses_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT ck_courses_title_length CHECK (char_length(title) BETWEEN 1 AND 255),
    CONSTRAINT ck_courses_cefr_from CHECK (cefr_from ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_courses_cefr_to CHECK (cefr_to ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_courses_status CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT ck_courses_estimated_hours CHECK (estimated_hours >= 0)
);

CREATE INDEX IF NOT EXISTS idx_courses_slug ON learn.courses (slug);
CREATE INDEX IF NOT EXISTS idx_courses_status ON learn.courses (status);
CREATE INDEX IF NOT EXISTS idx_courses_published_cefr ON learn.courses (cefr_from, title) WHERE status = 'published';

-- -------------------------------------------------------- course_units
CREATE TABLE IF NOT EXISTS learn.course_units (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   uuid        NOT NULL,
    position    integer     NOT NULL,
    title       text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_course_units_course FOREIGN KEY (course_id) REFERENCES learn.courses (id) ON DELETE CASCADE,
    CONSTRAINT uq_course_units_course_position UNIQUE (course_id, position),
    CONSTRAINT ck_course_units_position_positive CHECK (position > 0),
    CONSTRAINT ck_course_units_title_length CHECK (char_length(title) BETWEEN 1 AND 255)
);

CREATE INDEX IF NOT EXISTS idx_course_units_course_id ON learn.course_units (course_id);
CREATE INDEX IF NOT EXISTS idx_course_units_course_position ON learn.course_units (course_id, position);

-- ------------------------------------------------------------- lessons
CREATE TABLE IF NOT EXISTS learn.lessons (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id           uuid        NOT NULL,
    position          integer     NOT NULL,
    title             text        NOT NULL,
    skill_focus       text        NOT NULL,
    estimated_minutes integer     NOT NULL DEFAULT 0,
    status            text        NOT NULL DEFAULT 'draft',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_lessons_unit FOREIGN KEY (unit_id) REFERENCES learn.course_units (id) ON DELETE CASCADE,
    CONSTRAINT uq_lessons_unit_position UNIQUE (unit_id, position),
    CONSTRAINT ck_lessons_position_positive CHECK (position > 0),
    CONSTRAINT ck_lessons_title_length CHECK (char_length(title) BETWEEN 1 AND 255),
    CONSTRAINT ck_lessons_skill_focus CHECK (char_length(skill_focus) BETWEEN 1 AND 50),
    CONSTRAINT ck_lessons_estimated_minutes CHECK (estimated_minutes >= 0),
    CONSTRAINT ck_lessons_status CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_lessons_unit_id ON learn.lessons (unit_id);
CREATE INDEX IF NOT EXISTS idx_lessons_unit_position ON learn.lessons (unit_id, position);
CREATE INDEX IF NOT EXISTS idx_lessons_status ON learn.lessons (status);

-- ---------------------------------------------------------- activities
CREATE TABLE IF NOT EXISTS learn.activities (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id          uuid        NOT NULL,
    position           integer     NOT NULL,
    kind               text        NOT NULL,
    content_version_id uuid        NOT NULL,
    config             jsonb       NOT NULL DEFAULT '{}'::jsonb,
    weight             integer     NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_activities_lesson FOREIGN KEY (lesson_id) REFERENCES learn.lessons (id) ON DELETE CASCADE,
    CONSTRAINT uq_activities_lesson_position UNIQUE (lesson_id, position),
    CONSTRAINT ck_activities_position_positive CHECK (position > 0),
    CONSTRAINT ck_activities_kind_length CHECK (char_length(kind) BETWEEN 1 AND 50),
    CONSTRAINT ck_activities_weight CHECK (weight >= 0)
);

CREATE INDEX IF NOT EXISTS idx_activities_lesson_id ON learn.activities (lesson_id);
CREATE INDEX IF NOT EXISTS idx_activities_lesson_position ON learn.activities (lesson_id, position);
CREATE INDEX IF NOT EXISTS idx_activities_content_version_id ON learn.activities (content_version_id);

-- ------------------------------------------------ lesson_prerequisites
CREATE TABLE IF NOT EXISTS learn.lesson_prerequisites (
    lesson_id          uuid        NOT NULL,
    requires_lesson_id uuid        NOT NULL,
    min_score          integer     NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (lesson_id, requires_lesson_id),
    CONSTRAINT fk_lesson_prerequisites_lesson FOREIGN KEY (lesson_id) REFERENCES learn.lessons (id) ON DELETE CASCADE,
    CONSTRAINT fk_lesson_prerequisites_requires FOREIGN KEY (requires_lesson_id) REFERENCES learn.lessons (id) ON DELETE CASCADE,
    CONSTRAINT ck_lesson_prerequisites_no_self_ref CHECK (lesson_id <> requires_lesson_id),
    CONSTRAINT ck_lesson_prerequisites_min_score CHECK (min_score BETWEEN 0 AND 100)
);

CREATE INDEX IF NOT EXISTS idx_lesson_prerequisites_requires ON learn.lesson_prerequisites (requires_lesson_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS learn.lesson_prerequisites;
DROP TABLE IF EXISTS learn.activities;
DROP TABLE IF EXISTS learn.lessons;
DROP TABLE IF EXISTS learn.course_units;
DROP TABLE IF EXISTS learn.courses;
-- +goose StatementEnd
