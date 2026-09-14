-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS assess;

-- --------------------------------------------------------------------- exams
CREATE TABLE IF NOT EXISTS assess.exams (
    id              uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            varchar(120)    NOT NULL UNIQUE,
    title_en        varchar(255)    NOT NULL,
    title_vi        varchar(255)    NOT NULL,
    description_en  text            NOT NULL DEFAULT '',
    description_vi  text            NOT NULL DEFAULT '',
    level           varchar(10)     NOT NULL,
    format          varchar(50)     NOT NULL,
    total_minutes   int             NOT NULL,
    created_at      timestamptz     NOT NULL DEFAULT now(),
    updated_at      timestamptz     NOT NULL DEFAULT now()
);

-- ------------------------------------------------------------- exam_sections
CREATE TABLE IF NOT EXISTS assess.exam_sections (
    id                      uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id                 uuid            NOT NULL REFERENCES assess.exams (id) ON DELETE CASCADE,
    position                int             NOT NULL,
    skill                   varchar(20)     NOT NULL,
    exam_duration_minutes   int             NOT NULL,
    item_count              int             NOT NULL,
    item_kinds              jsonb           NOT NULL DEFAULT '[]',
    created_at              timestamptz     NOT NULL DEFAULT now(),
    CONSTRAINT uq_exam_sections_exam_position UNIQUE (exam_id, position)
);

-- ------------------------------------------------------------- exam_attempts
CREATE TABLE IF NOT EXISTS assess.exam_attempts (
    id                      uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 uuid            NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    exam_id                 uuid            NOT NULL REFERENCES assess.exams (id) ON DELETE RESTRICT,
    mode                    varchar(20)     NOT NULL,
    chosen_duration_minutes int             NOT NULL,
    started_at              timestamptz     NOT NULL DEFAULT now(),
    deadline_at             timestamptz     NOT NULL,
    current_section         int             NOT NULL DEFAULT 1,
    status                  varchar(20)     NOT NULL DEFAULT 'in_progress',
    section_activities      jsonb           NOT NULL DEFAULT '[]',
    draft_answers           jsonb           NOT NULL DEFAULT '{}',
    submitted_at            timestamptz,
    submitted_by            varchar(20),
    created_at              timestamptz     NOT NULL DEFAULT now(),
    updated_at              timestamptz     NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_exam_attempts_user_status
    ON assess.exam_attempts (user_id, status);

CREATE INDEX IF NOT EXISTS idx_exam_attempts_status_deadline
    ON assess.exam_attempts (status, deadline_at);

-- ------------------------------------------------------------- score_reports
CREATE TABLE IF NOT EXISTS assess.score_reports (
    id                  uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id          uuid            NOT NULL UNIQUE REFERENCES assess.exam_attempts (id) ON DELETE CASCADE,
    user_id             uuid            NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    exam_id             uuid            NOT NULL REFERENCES assess.exams (id) ON DELETE RESTRICT,
    overall_score       numeric(5,2)    NOT NULL DEFAULT 0,
    overall_band        varchar(20)     NOT NULL DEFAULT '',
    status              varchar(20)     NOT NULL DEFAULT 'pending',
    per_section         jsonb           NOT NULL DEFAULT '[]',
    feedback            jsonb           NOT NULL DEFAULT '{}',
    integrity_signals   jsonb           NOT NULL DEFAULT '[]',
    created_at          timestamptz     NOT NULL DEFAULT now(),
    updated_at          timestamptz     NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_score_reports_user
    ON assess.score_reports (user_id);

-- ---------------------------------------------------------- integrity_events
CREATE TABLE IF NOT EXISTS assess.integrity_events (
    id          uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id  uuid            NOT NULL REFERENCES assess.exam_attempts (id) ON DELETE CASCADE,
    kind        varchar(50)     NOT NULL,
    occurred_at timestamptz     NOT NULL DEFAULT now(),
    metadata    jsonb           NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_integrity_events_attempt
    ON assess.integrity_events (attempt_id, occurred_at ASC);

-- Seed default exam templates
INSERT INTO assess.exams (id, slug, title_en, title_vi, description_en, description_vi, level, format, total_minutes)
VALUES
    ('10000000-0000-0000-0000-0000000000a2', 'mock-toeic-a2', 'TOEIC Mock Exam (A2)', 'Bài thi thử TOEIC (A2)', 'Full 4-skill mock examination at CEFR A2 level', 'Bài thi thử đầy đủ 4 kỹ năng ở trình độ CEFR A2', 'A2', 'mock_toeic', 75),
    ('10000000-0000-0000-0000-0000000000b1', 'mock-toeic-b1', 'TOEIC Mock Exam (B1)', 'Bài thi thử TOEIC (B1)', 'Full 4-skill mock examination at CEFR B1 level', 'Bài thi thử đầy đủ 4 kỹ năng ở trình độ CEFR B1', 'B1', 'mock_toeic', 75),
    ('10000000-0000-0000-0000-0000000000b2', 'mock-toeic-b2', 'TOEIC Mock Exam (B2)', 'Bài thi thử TOEIC (B2)', 'Full 4-skill mock examination at CEFR B2 level', 'Bài thi thử đầy đủ 4 kỹ năng ở trình độ CEFR B2', 'B2', 'mock_toeic', 75)
ON CONFLICT (slug) DO NOTHING;

-- Seed sections for A2
INSERT INTO assess.exam_sections (exam_id, position, skill, exam_duration_minutes, item_count, item_kinds)
VALUES
    ('10000000-0000-0000-0000-0000000000a2', 1, 'listening', 20, 3, '["listening_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000a2', 2, 'reading', 25, 2, '["reading_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000a2', 3, 'writing', 20, 4, '["writing_prompt", "grammar_sentence_transform"]'),
    ('10000000-0000-0000-0000-0000000000a2', 4, 'speaking', 10, 4, '["speaking_task"]')
ON CONFLICT (exam_id, position) DO NOTHING;

-- Seed sections for B1
INSERT INTO assess.exam_sections (exam_id, position, skill, exam_duration_minutes, item_count, item_kinds)
VALUES
    ('10000000-0000-0000-0000-0000000000b1', 1, 'listening', 20, 3, '["listening_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000b1', 2, 'reading', 25, 2, '["reading_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000b1', 3, 'writing', 20, 4, '["writing_prompt", "grammar_sentence_transform"]'),
    ('10000000-0000-0000-0000-0000000000b1', 4, 'speaking', 10, 4, '["speaking_task"]')
ON CONFLICT (exam_id, position) DO NOTHING;

-- Seed sections for B2
INSERT INTO assess.exam_sections (exam_id, position, skill, exam_duration_minutes, item_count, item_kinds)
VALUES
    ('10000000-0000-0000-0000-0000000000b2', 1, 'listening', 20, 3, '["listening_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000b2', 2, 'reading', 25, 2, '["reading_comprehension"]'),
    ('10000000-0000-0000-0000-0000000000b2', 3, 'writing', 20, 4, '["writing_prompt", "grammar_sentence_transform"]'),
    ('10000000-0000-0000-0000-0000000000b2', 4, 'speaking', 10, 4, '["speaking_task"]')
ON CONFLICT (exam_id, position) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS assess.integrity_events;
DROP TABLE IF EXISTS assess.score_reports;
DROP TABLE IF EXISTS assess.exam_attempts;
DROP TABLE IF EXISTS assess.exam_sections;
DROP TABLE IF EXISTS assess.exams;
-- +goose StatementEnd
