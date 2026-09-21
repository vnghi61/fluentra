-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS assess.mock_tests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    blueprint_id  uuid NOT NULL REFERENCES assess.blueprints (id) ON DELETE CASCADE,
    mode          text NOT NULL,     -- 'fixed' | 'random' | 'weak_topic' | 'full' | 'custom'
    seed          bigint NOT NULL,
    composition   jsonb NOT NULL,    -- [{part_id, activity_ids: [...]}], in order
    owner_id      uuid,              -- null for a fixed public test
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_mock_tests_mode CHECK (mode IN ('fixed', 'random', 'weak_topic', 'full', 'custom'))
);

CREATE INDEX IF NOT EXISTS idx_mock_tests_blueprint ON assess.mock_tests (blueprint_id);
CREATE INDEX IF NOT EXISTS idx_mock_tests_owner ON assess.mock_tests (owner_id);

ALTER TABLE assess.exam_attempts ADD COLUMN IF NOT EXISTS mock_test_id uuid REFERENCES assess.mock_tests (id) ON DELETE SET NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON assess.mock_tests TO fluentra_app;

INSERT INTO assess.exams (id, slug, title_en, title_vi, level, format, total_minutes, version_id)
VALUES
    ('20000000-0000-0000-0000-000000000010', 'toeic-lr-2026', 'TOEIC Listening & Reading', 'Bài thi TOEIC Listening & Reading', 'B2', 'toeic_lr', 120, '20000000-0000-0000-0000-000000000001'),
    ('20000000-0000-0000-0000-000000000020', 'vstep-3-5', 'VSTEP 3-5 Exam', 'Kỳ thi VSTEP 3-5', 'B2', 'vstep', 172, '20000000-0000-0000-0000-000000000002')
ON CONFLICT (slug) DO UPDATE SET version_id = EXCLUDED.version_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The two exams rows reference exam_versions, which 1700000860's Down deletes.
DELETE FROM assess.exams WHERE slug IN ('toeic-lr-2026', 'vstep-3-5');
ALTER TABLE assess.exam_attempts DROP COLUMN IF EXISTS mock_test_id;
DROP TABLE IF EXISTS assess.mock_tests;
-- +goose StatementEnd
