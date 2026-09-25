-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage I.4: a sitting is recorded under an assess.exams row, and a mock
-- test finds it through its version (resolveExamForVersion). TOEIC and VSTEP got
-- theirs in 1700000870; IELTS_ACADEMIC_2026_R2 had none, so every IELTS test
-- refused to start with EXAM_VERSION_HAS_NO_EXAM.
INSERT INTO assess.exams (id, slug, title_en, title_vi, level, format, total_minutes, version_id)
VALUES (
    '20000000-0000-0000-0000-000000000030',
    'ielts-academic-2026',
    'IELTS Academic',
    'Bài thi IELTS Academic',
    'B2',
    'ielts_academic',
    165,
    '20000000-0000-0000-0000-000000000013'
)
ON CONFLICT (slug) DO UPDATE SET version_id = EXCLUDED.version_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM assess.exams WHERE slug = 'ielts-academic-2026';
-- +goose StatementEnd
