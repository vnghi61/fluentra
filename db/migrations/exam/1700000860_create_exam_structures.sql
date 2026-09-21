-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS assess.exam_versions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_family    text NOT NULL,        -- 'vstep' | 'toeic_lr' | 'ielts_academic' | 'toefl_ibt' | 'cambridge_b2_first' | 'vn_thpt'
    code           text NOT NULL UNIQUE, -- 'VSTEP_3_5', 'TOEIC_LR_2026'
    title          text NOT NULL,
    total_minutes  integer NOT NULL,
    scoring        jsonb NOT NULL,
    source_url     text NOT NULL,
    verified_at    date NOT NULL,
    is_current     boolean NOT NULL DEFAULT false,
    notes          text NOT NULL DEFAULT '',
    CONSTRAINT ck_exam_versions_source CHECK (source_url ~ '^https://')
);

CREATE TABLE IF NOT EXISTS assess.exam_parts (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id       uuid NOT NULL REFERENCES assess.exam_versions (id) ON DELETE CASCADE,
    section          text NOT NULL,     -- 'listening' | 'reading' | 'writing' | 'speaking' | 'use_of_english'
    part_number      integer NOT NULL,
    kind             text NOT NULL,     -- the activity kind drawn for this part
    question_count   integer NOT NULL,  -- questions, not activities
    group_size       integer NOT NULL DEFAULT 1,
    duration_minutes integer,
    constraints      jsonb NOT NULL DEFAULT '{}',   -- option count, word limits: what Stage E's structure check reads
    UNIQUE (version_id, section, part_number),
    CONSTRAINT ck_exam_parts_groups CHECK (question_count % group_size = 0)
);

CREATE TABLE IF NOT EXISTS assess.blueprints (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id         uuid NOT NULL REFERENCES assess.exam_versions (id),
    name               text NOT NULL,
    cefr_distribution  jsonb NOT NULL,   -- {"B1": 0.4, "B2": 0.4, "C1": 0.2}
    node_distribution  jsonb NOT NULL DEFAULT '{}',
    UNIQUE (version_id, name)
);

-- Foreign key on questions table
ALTER TABLE assess.questions ADD CONSTRAINT fk_questions_part
    FOREIGN KEY (exam_part_id) REFERENCES assess.exam_parts (id);

-- Optional version reference on assess.exams
ALTER TABLE assess.exams ADD COLUMN IF NOT EXISTS version_id uuid REFERENCES assess.exam_versions (id);

-- Grants
GRANT SELECT, INSERT, UPDATE, DELETE ON assess.exam_versions, assess.exam_parts, assess.blueprints TO fluentra_app;

-- Update existing mock-toeic exams per WO 19 G.2
UPDATE assess.exams
SET version_id = NULL,
    title_en = 'Mixed-skill practice exam',
    title_vi = 'Bài thi luyện tập kỹ năng tổng hợp'
WHERE slug IN ('mock-toeic-a2', 'mock-toeic-b1', 'mock-toeic-b2');

-- Seed 6 Exam Families
-- 1. TOEIC Listening & Reading
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000001',
    'toeic_lr',
    'TOEIC_LR_2026',
    'TOEIC Listening & Reading (2026)',
    120,
    '{"type": "raw_with_estimate", "sections": {"listening": {"max_raw": 100, "scale": [5, 495]}, "reading": {"max_raw": 100, "scale": [5, 495]}}}'::jsonb,
    'https://www.etsglobal.org/dz/en/help-center/test-content/format-questions-toeic-listening-reading',
    '2026-09-20',
    true,
    '7 parts, 200 questions: Listening (100 questions, 45 min), Reading (100 questions, 75 min).'
) ON CONFLICT (code) DO NOTHING;

-- 2. VSTEP 3-5
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000002',
    'vstep',
    'VSTEP_3_5',
    'VSTEP (Level 3-5 / B1-C1)',
    172,
    '{"type": "raw_with_estimate", "sections": {"listening": {"max_raw": 35}, "reading": {"max_raw": 40}, "writing": {"tasks": 2}, "speaking": {"parts": 3}}}'::jsonb,
    'https://vstep.vnu.edu.vn/test-format/',
    '2026-09-20',
    true,
    'VSTEP 3-5 standard format under Circular 01/2014/TT-BGDDT. Listening 35 questions, Reading 40 questions.'
) ON CONFLICT (code) DO NOTHING;

-- 3. IELTS Academic
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000003',
    'ielts_academic',
    'IELTS_ACADEMIC_2026',
    'IELTS Academic',
    165,
    '{"type": "band", "scale": [0.0, 9.0], "step": 0.5}'::jsonb,
    'https://ielts.org/take-a-test/test-types/ielts-academic-test',
    '2026-09-20',
    true,
    '4 sections: Listening 40 questions (30m), Reading 40 questions (60m), Writing 2 tasks (60m), Speaking 3 parts (11-14m).'
) ON CONFLICT (code) DO NOTHING;

-- 4. TOEFL iBT (is_current = false per WO 19 G.2)
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000004',
    'toefl_ibt',
    'TOEFL_IBT_2026',
    'TOEFL iBT (January 2026)',
    90,
    '{"type": "scaled", "range": [0, 120]}'::jsonb,
    'https://www.ets.org/toefl/test-takers/ibt/about/content.html',
    '2026-09-20',
    false,
    'The January 2026 TOEFL iBT format is section-adaptive (Stage 1 to Stage 2 routing). Deferred from active mock generation until adaptive routing engine is built.'
) ON CONFLICT (code) DO NOTHING;

-- 5. Cambridge B2 First
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000005',
    'cambridge_b2_first',
    'CAMBRIDGE_B2_FIRST',
    'Cambridge B2 First (FCE)',
    209,
    '{"type": "cambridge_scale", "range": [140, 190]}'::jsonb,
    'https://www.cambridgeenglish.org/exams-and-tests/qualifications/first/format/',
    '2026-09-20',
    true,
    '4 papers: Reading & Use of English (52 questions, 75m), Writing (2 tasks, 80m), Listening (30 questions, 40m), Speaking (4 parts, 14m).'
) ON CONFLICT (code) DO NOTHING;

-- 6. Vietnam THPT English
INSERT INTO assess.exam_versions (id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, notes)
VALUES (
    '20000000-0000-0000-0000-000000000006',
    'vn_thpt',
    'VN_THPT_2026',
    'Kỳ thi Tốt nghiệp THPT môn Tiếng Anh (2026)',
    50,
    '{"type": "points_per_item", "points_per_item": 0.25, "max_score": 10.0}'::jsonb,
    'https://thuvienphapluat.vn/phap-luat/ho-tro-phap-luat/cau-truc-de-thi-tieng-anh-tot-nghiep-thpt-nam-2026-cap-nhat-moi-nhat-cach-tinh-diem-mon-tieng-anh-t-431248-267517.html',
    '2026-09-20',
    true,
    '40 multiple-choice questions, 50 min, 0.25 points each. Format under 2018 curriculum.'
) ON CONFLICT (code) DO NOTHING;

-- Seed TOEIC Parts in full (7 parts: 6/25/39/30 listening, 30/16/54 reading)
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0001-000000000001', '20000000-0000-0000-0000-000000000001', 'listening', 1, 'photo_description', 6, 1, 5, '{"options_count": 4}'::jsonb),
    ('20000000-0000-0000-0001-000000000002', '20000000-0000-0000-0000-000000000001', 'listening', 2, 'question_response', 25, 1, 10, '{"options_count": 3}'::jsonb),
    ('20000000-0000-0000-0001-000000000003', '20000000-0000-0000-0000-000000000001', 'listening', 3, 'listening_comprehension', 39, 3, 15, '{"options_count": 4, "sub_questions_per_group": 3}'::jsonb),
    ('20000000-0000-0000-0001-000000000004', '20000000-0000-0000-0000-000000000001', 'listening', 4, 'listening_comprehension', 30, 3, 15, '{"options_count": 4, "sub_questions_per_group": 3}'::jsonb),
    ('20000000-0000-0000-0001-000000000005', '20000000-0000-0000-0000-000000000001', 'reading', 5, 'mcq_gap', 30, 1, 20, '{"options_count": 4}'::jsonb),
    ('20000000-0000-0000-0001-000000000006', '20000000-0000-0000-0000-000000000001', 'reading', 6, 'text_completion', 16, 4, 15, '{"options_count": 4, "sub_questions_per_group": 4}'::jsonb),
    ('20000000-0000-0000-0001-000000000007', '20000000-0000-0000-0000-000000000001', 'reading', 7, 'reading_comprehension', 54, 1, 40, '{"options_count": 4}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed VSTEP Parts in full (Listening: 8/12/15 = 35; Reading: 10/10/10/10 = 40; Writing: 2; Speaking: 3)
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0002-000000000001', '20000000-0000-0000-0000-000000000002', 'listening', 1, 'listening_comprehension', 8, 1, 10, '{"options_count": 4}'::jsonb),
    ('20000000-0000-0000-0002-000000000002', '20000000-0000-0000-0000-000000000002', 'listening', 2, 'listening_comprehension', 12, 4, 15, '{"options_count": 4, "sub_questions_per_group": 4}'::jsonb),
    ('20000000-0000-0000-0002-000000000003', '20000000-0000-0000-0000-000000000002', 'listening', 3, 'listening_comprehension', 15, 5, 15, '{"options_count": 4, "sub_questions_per_group": 5}'::jsonb),
    ('20000000-0000-0000-0002-000000000004', '20000000-0000-0000-0000-000000000002', 'reading', 1, 'reading_comprehension', 10, 10, 15, '{"options_count": 4, "sub_questions_per_group": 10}'::jsonb),
    ('20000000-0000-0000-0002-000000000005', '20000000-0000-0000-0000-000000000002', 'reading', 2, 'reading_comprehension', 10, 10, 15, '{"options_count": 4, "sub_questions_per_group": 10}'::jsonb),
    ('20000000-0000-0000-0002-000000000006', '20000000-0000-0000-0000-000000000002', 'reading', 3, 'reading_comprehension', 10, 10, 15, '{"options_count": 4, "sub_questions_per_group": 10}'::jsonb),
    ('20000000-0000-0000-0002-000000000007', '20000000-0000-0000-0000-000000000002', 'reading', 4, 'reading_comprehension', 10, 10, 15, '{"options_count": 4, "sub_questions_per_group": 10}'::jsonb),
    ('20000000-0000-0000-0002-000000000008', '20000000-0000-0000-0000-000000000002', 'writing', 1, 'writing_prompt', 1, 1, 20, '{"min_words": 120}'::jsonb),
    ('20000000-0000-0000-0002-000000000009', '20000000-0000-0000-0000-000000000002', 'writing', 2, 'writing_prompt', 1, 1, 40, '{"min_words": 250}'::jsonb),
    ('20000000-0000-0000-0002-000000000010', '20000000-0000-0000-0000-000000000002', 'speaking', 1, 'speaking_task', 1, 1, 3, '{}'::jsonb),
    ('20000000-0000-0000-0002-000000000011', '20000000-0000-0000-0000-000000000002', 'speaking', 2, 'speaking_task', 1, 1, 4, '{}'::jsonb),
    ('20000000-0000-0000-0002-000000000012', '20000000-0000-0000-0000-000000000002', 'speaking', 3, 'speaking_task', 1, 1, 5, '{}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed section totals for IELTS Academic
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0003-000000000001', '20000000-0000-0000-0000-000000000003', 'listening', 1, 'listening_comprehension', 40, 1, 30, '{}'::jsonb),
    ('20000000-0000-0000-0003-000000000002', '20000000-0000-0000-0000-000000000003', 'reading', 1, 'reading_comprehension', 40, 1, 60, '{}'::jsonb),
    ('20000000-0000-0000-0003-000000000003', '20000000-0000-0000-0000-000000000003', 'writing', 1, 'writing_prompt', 2, 1, 60, '{}'::jsonb),
    ('20000000-0000-0000-0003-000000000004', '20000000-0000-0000-0000-000000000003', 'speaking', 1, 'speaking_task', 3, 1, 15, '{}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed section totals for TOEFL iBT
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0004-000000000001', '20000000-0000-0000-0000-000000000004', 'reading', 1, 'reading_comprehension', 50, 1, 30, '{}'::jsonb),
    ('20000000-0000-0000-0004-000000000002', '20000000-0000-0000-0000-000000000004', 'listening', 1, 'listening_comprehension', 47, 1, 29, '{}'::jsonb),
    ('20000000-0000-0000-0004-000000000003', '20000000-0000-0000-0000-000000000004', 'writing', 1, 'writing_prompt', 12, 1, 23, '{}'::jsonb),
    ('20000000-0000-0000-0004-000000000004', '20000000-0000-0000-0000-000000000004', 'speaking', 1, 'speaking_task', 11, 1, 8, '{}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed section totals for Cambridge B2 First
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0005-000000000001', '20000000-0000-0000-0000-000000000005', 'use_of_english', 1, 'mcq_gap', 52, 1, 75, '{}'::jsonb),
    ('20000000-0000-0000-0005-000000000002', '20000000-0000-0000-0000-000000000005', 'writing', 1, 'writing_prompt', 2, 1, 80, '{}'::jsonb),
    ('20000000-0000-0000-0005-000000000003', '20000000-0000-0000-0000-000000000005', 'listening', 1, 'listening_comprehension', 30, 1, 40, '{}'::jsonb),
    ('20000000-0000-0000-0005-000000000004', '20000000-0000-0000-0000-000000000005', 'speaking', 1, 'speaking_task', 4, 1, 14, '{}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed section totals for VN THPT English
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0006-000000000001', '20000000-0000-0000-0000-000000000006', 'reading', 1, 'mcq_gap', 40, 1, 50, '{}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- Seed Default Blueprints for TOEIC and VSTEP
INSERT INTO assess.blueprints (id, version_id, name, cefr_distribution, node_distribution)
VALUES
    ('20000000-0000-0000-0010-000000000001', '20000000-0000-0000-0000-000000000001', 'toeic_default', '{"A2": 0.2, "B1": 0.5, "B2": 0.3}'::jsonb, '{}'::jsonb),
    ('20000000-0000-0000-0010-000000000002', '20000000-0000-0000-0000-000000000002', 'vstep_default', '{"B1": 0.33, "B2": 0.34, "C1": 0.33}'::jsonb, '{}'::jsonb)
ON CONFLICT (version_id, name) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM assess.blueprints WHERE id IN ('20000000-0000-0000-0010-000000000001', '20000000-0000-0000-0010-000000000002');
DELETE FROM assess.exam_parts WHERE version_id IN (
    '20000000-0000-0000-0000-000000000001',
    '20000000-0000-0000-0000-000000000002',
    '20000000-0000-0000-0000-000000000003',
    '20000000-0000-0000-0000-000000000004',
    '20000000-0000-0000-0000-000000000005',
    '20000000-0000-0000-0000-000000000006'
);
DELETE FROM assess.exam_versions WHERE id IN (
    '20000000-0000-0000-0000-000000000001',
    '20000000-0000-0000-0000-000000000002',
    '20000000-0000-0000-0000-000000000003',
    '20000000-0000-0000-0000-000000000004',
    '20000000-0000-0000-0000-000000000005',
    '20000000-0000-0000-0000-000000000006'
);
ALTER TABLE assess.questions DROP CONSTRAINT IF EXISTS fk_questions_part;
ALTER TABLE assess.exams DROP COLUMN IF EXISTS version_id;
DROP TABLE IF EXISTS assess.blueprints;
DROP TABLE IF EXISTS assess.exam_parts;
DROP TABLE IF EXISTS assess.exam_versions;
-- +goose StatementEnd
