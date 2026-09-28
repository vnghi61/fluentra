-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage I: a reading part's passage length. The format table gave none,
-- and the generator's default (120-220 words) held: IELTS and VSTEP passages
-- came back at about 190 words with 10 to 14 questions on each.
--   IELTS   three passages of 2,150-2,750 words in all, so about 700-900 each.
--   VSTEP   four passages; the published format gives 400-500 words each
--           (the specification sign-off in the work order covers these).
-- TOEIC Part 7 passages are short notices and letters; its rows keep none.
UPDATE assess.exam_parts
SET constraints = constraints || '{"passage_min_words": 700}'::jsonb
WHERE version_id = '20000000-0000-0000-0000-000000000013' AND section = 'reading';

UPDATE assess.exam_parts
SET constraints = constraints || '{"passage_min_words": 400}'::jsonb
WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'reading';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE assess.exam_parts
SET constraints = constraints - 'passage_min_words'
WHERE version_id IN ('20000000-0000-0000-0000-000000000013', '20000000-0000-0000-0000-000000000002')
  AND section = 'reading';
-- +goose StatementEnd
