-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage I: an IELTS listening part's recording length. IELTS publishes
-- about 30 minutes of recordings in four parts, some seven minutes each, which
-- is 800-1,000 words of script at conversational speed; the generator's practice
-- default (80-160 words) held, and ten questions sat on 150 words. 600 is a
-- floor, not a target. VSTEP and TOEIC recordings are short announcements,
-- conversations and talks, and keep none.
UPDATE assess.exam_parts
SET constraints = constraints || '{"script_min_words": 600}'::jsonb
WHERE version_id = '20000000-0000-0000-0000-000000000013' AND section = 'listening';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE assess.exam_parts
SET constraints = constraints - 'script_min_words'
WHERE version_id = '20000000-0000-0000-0000-000000000013' AND section = 'listening';
-- +goose StatementEnd
