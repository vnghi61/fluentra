-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage I: one format specification per exam version, sourced, that every
-- step reads.
--
-- Sources checked 2026-09-23 against the owners' own pages:
--   TOEIC   https://www.ets.org/toeic/test-takers/listening-reading/about.html
--           and the ETS Examinee Handbook (format only; no ETS wording is copied).
--   IELTS   https://ielts.org/take-a-test/test-types/ielts-academic-test and
--           https://ielts.org/take-a-test/test-types/ielts-academic-test/test-format
--   VSTEP   the Ministry of Education and Training's VSTEP.3-5 test format as
--           published; the version row no longer cites Circular 01/2014/TT-BGDT,
--           which is the six-level proficiency framework, not the test format.
-- A detail the official source does not publish is recorded as "not published"
-- and left to the blueprint, never guessed as if official.

-- 1. Listed. Three levels of the hub need to tell a sat exam from a retired
-- row: a version or an exam can stay for the attempts that point at it without
-- appearing to a learner choosing one.
ALTER TABLE assess.exam_versions ADD COLUMN IF NOT EXISTS listed boolean NOT NULL DEFAULT true;
ALTER TABLE assess.exams ADD COLUMN IF NOT EXISTS listed boolean NOT NULL DEFAULT true;

-- D22-16: Cambridge B2 First, THPT and TOEFL keep their rows, unlisted. The
-- three mixed-skill practice rows are unlisted too (D22-15); attempts point at
-- them, so they are never deleted.
UPDATE assess.exam_versions SET listed = false
WHERE code IN ('CAMBRIDGE_B2_FIRST', 'VN_THPT_2026', 'TOEFL_IBT_2026', 'IELTS_ACADEMIC_2026');
UPDATE assess.exams SET listed = false
WHERE slug IN ('mock-toeic-a2', 'mock-toeic-b1', 'mock-toeic-b2');

-- 2. The VSTEP version cites the test-format decision, not Circular 01/2014.
UPDATE assess.exam_versions
SET source_url = 'https://moet.gov.vn/',
    verified_at = '2026-09-23',
    notes = 'VSTEP.3-5 format: Listening 8/12/15, Reading 4x10, Writing a 120-word letter and a 250-word essay, Speaking 3 parts. Format per the Ministry test-format decision; a person signs the table off before Stage N.'
WHERE code = 'VSTEP_3_5';

-- 3. The one spec schema, written into constraints. Field names match
-- learning.ExamPartConstraints, which the verifier reads: the old rows wrote
-- options_count/sub_questions_per_group and the Go struct reads
-- option_count/questions_per_group, so even read they were dropped (§1).
--
-- Shape:
--   schema_version      int
--   answer_mode         "choice" | "typed"
--   option_count        int (0 when typed)
--   questions_per_group int
--   plays               int (listening recordings)
--   allowed_types       [string]  the question types this part may draw
--   type_mix            {type: share}  composition must follow this mix
--   min_words/max_words int (typed answers)
--   photo_required      bool (TOEIC Part 1)
--   sentence_insertion  bool (TOEIC Part 6)
--   spoken_only         bool (TOEIC Part 2: question and responses are audio only)
--   passage_sets        [single|double|triple] (TOEIC Part 7)
--   recording           {speakers, genre} (listening shape)
--   preparation_seconds/speaking_seconds int (speaking tasks)
--   source              text
--
-- TOEIC, checked against the ETS format page and examinee handbook.
UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 1, 'plays', 1, 'photo_required', true,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'source', 'ETS TOEIC L&R format; one photograph per question'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'listening' AND part_number = 1;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 3,
    'questions_per_group', 1, 'plays', 1, 'spoken_only', true,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'source', 'ETS TOEIC L&R format; a question and three responses, spoken only'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'listening' AND part_number = 2;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 3, 'plays', 1,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'recording', jsonb_build_object('speakers', 2, 'genre', 'conversation', 'graphic_some_sets', true),
    'source', 'ETS TOEIC L&R format; 13 conversations x 3, some with a graphic'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'listening' AND part_number = 3;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 3, 'plays', 1,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'recording', jsonb_build_object('speakers', 1, 'genre', 'talk', 'graphic_some_sets', true),
    'source', 'ETS TOEIC L&R format; 10 talks x 3, some with a graphic'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'listening' AND part_number = 4;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 1, 'plays', 0,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'source', 'ETS TOEIC L&R format'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'reading' AND part_number = 5;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 4, 'plays', 0, 'sentence_insertion', true,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'source', 'ETS TOEIC L&R format; 4 texts x 4, one sentence-insertion question each'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'reading' AND part_number = 6;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 1, 'plays', 0,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'passage_sets', jsonb_build_array('single', 'double', 'triple'),
    'source', 'ETS TOEIC L&R format; single, double and triple passage sets'
) WHERE version_id = '20000000-0000-0000-0000-000000000001' AND section = 'reading' AND part_number = 7;

-- VSTEP, checked against the Ministry's published format.
UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 1, 'plays', 1,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'recording', jsonb_build_object('speakers', 1, 'genre', 'announcement_or_message'),
    'source', 'VSTEP.3-5 format; 8 short announcements or messages, played once'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'listening' AND part_number = 1;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 4, 'plays', 1,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'recording', jsonb_build_object('speakers', 2, 'genre', 'conversation'),
    'source', 'VSTEP.3-5 format; 3 conversations x 4'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'listening' AND part_number = 2;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 5, 'plays', 1,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'recording', jsonb_build_object('speakers', 1, 'genre', 'talk'),
    'source', 'VSTEP.3-5 format; 3 talks x 5'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'listening' AND part_number = 3;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'choice', 'option_count', 4,
    'questions_per_group', 10, 'plays', 0,
    'allowed_types', jsonb_build_array('multiple_choice'),
    'type_mix', jsonb_build_object('multiple_choice', 1.0),
    'passage_sets', jsonb_build_array('single'),
    'source', 'VSTEP.3-5 format; 4 passages x 10'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'reading';

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'typed', 'option_count', 0,
    'questions_per_group', 1, 'plays', 0, 'min_words', 120,
    'allowed_types', jsonb_build_array('writing_prompt'),
    'type_mix', jsonb_build_object('writing_prompt', 1.0),
    'source', 'VSTEP.3-5 format; Task 1 a letter or email of at least 120 words'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'writing' AND part_number = 1;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'typed', 'option_count', 0,
    'questions_per_group', 1, 'plays', 0, 'min_words', 250,
    'allowed_types', jsonb_build_array('writing_prompt'),
    'type_mix', jsonb_build_object('writing_prompt', 1.0),
    'source', 'VSTEP.3-5 format; Task 2 an essay of at least 250 words'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'writing' AND part_number = 2;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'typed', 'option_count', 0,
    'questions_per_group', 1, 'plays', 0, 'speaking_seconds', 180,
    'allowed_types', jsonb_build_array('speaking_task'),
    'type_mix', jsonb_build_object('speaking_task', 1.0),
    'source', 'VSTEP.3-5 speaking part 1, social interaction'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'speaking' AND part_number = 1;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'typed', 'option_count', 0,
    'questions_per_group', 1, 'plays', 0, 'speaking_seconds', 240,
    'allowed_types', jsonb_build_array('speaking_task'),
    'type_mix', jsonb_build_object('speaking_task', 1.0),
    'source', 'VSTEP.3-5 speaking part 2, solution discussion'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'speaking' AND part_number = 2;

UPDATE assess.exam_parts SET constraints = jsonb_build_object(
    'schema_version', 1, 'answer_mode', 'typed', 'option_count', 0,
    'questions_per_group', 1, 'plays', 0, 'speaking_seconds', 300,
    'allowed_types', jsonb_build_array('speaking_task'),
    'type_mix', jsonb_build_object('speaking_task', 1.0),
    'source', 'VSTEP.3-5 speaking part 3, topic development'
) WHERE version_id = '20000000-0000-0000-0000-000000000002' AND section = 'speaking' AND part_number = 3;

-- 4. IELTS Academic, second revision: the coarse single-row sections cannot be
-- composed against, so a new version code carries the real parts. The old code
-- is not edited — parts are what tests and attempts point at (I.4.1).
UPDATE assess.exam_versions SET is_current = false
WHERE code = 'IELTS_ACADEMIC_2026';

INSERT INTO assess.exam_versions (
    id, exam_family, code, title, total_minutes, scoring, source_url, verified_at, is_current, listed, notes
) VALUES (
    '20000000-0000-0000-0000-000000000013',
    'ielts_academic',
    'IELTS_ACADEMIC_2026_R2',
    'IELTS Academic (2026 R2)',
    165,
    '{"type": "band", "scale": [0.0, 9.0], "step": 0.5, "published_conversion": true}'::jsonb,
    'https://ielts.org/take-a-test/test-types/ielts-academic-test/test-format',
    '2026-09-23',
    true,
    true,
    '4 skills, band 0-9 in halves. Listening 4 parts of 10; Reading 3 passages, 40 questions; Writing 2 tasks, 150 and 250 words; Speaking 3 parts.'
) ON CONFLICT (code) DO NOTHING;

-- IELTS Listening: four parts of ten, one recording each, completion and
-- choice-and-matching types. Part 1 social conversation, 2 social monologue,
-- 3 educational discussion, 4 academic lecture.
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0013-000000000001', '20000000-0000-0000-0000-000000000013', 'listening', 1, 'listening_comprehension', 10, 10, 8,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":10,"plays":1,"max_words":2,"allowed_types":["completion","multiple_choice"],"type_mix":{"completion":0.7,"multiple_choice":0.3},"recording":{"speakers":2,"genre":"social_conversation"},"source":"IELTS Academic test format; Part 1"}'::jsonb),
    ('20000000-0000-0000-0013-000000000002', '20000000-0000-0000-0000-000000000013', 'listening', 2, 'listening_comprehension', 10, 10, 8,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":10,"plays":1,"max_words":2,"allowed_types":["completion","matching"],"type_mix":{"completion":0.6,"matching":0.4},"recording":{"speakers":1,"genre":"social_monologue"},"source":"IELTS Academic test format; Part 2"}'::jsonb),
    ('20000000-0000-0000-0013-000000000003', '20000000-0000-0000-0000-000000000013', 'listening', 3, 'listening_comprehension', 10, 10, 7,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":10,"plays":1,"max_words":2,"allowed_types":["completion","multiple_choice","matching"],"type_mix":{"completion":0.5,"multiple_choice":0.3,"matching":0.2},"recording":{"speakers":2,"genre":"educational_discussion"},"source":"IELTS Academic test format; Part 3"}'::jsonb),
    ('20000000-0000-0000-0013-000000000004', '20000000-0000-0000-0000-000000000013', 'listening', 4, 'listening_comprehension', 10, 10, 7,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":10,"plays":1,"max_words":2,"allowed_types":["completion","multiple_choice"],"type_mix":{"completion":0.6,"multiple_choice":0.4},"recording":{"speakers":1,"genre":"academic_lecture"},"source":"IELTS Academic test format; Part 4"}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- IELTS Reading: three passages, forty questions.
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0013-000000000011', '20000000-0000-0000-0000-000000000013', 'reading', 1, 'reading_comprehension', 13, 13, 20,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":13,"plays":0,"max_words":2,"allowed_types":["true_false_not_given","matching_information","completion","multiple_choice"],"type_mix":{"completion":0.4,"true_false_not_given":0.3,"matching_information":0.2,"multiple_choice":0.1},"passage_sets":["single"],"source":"IELTS Academic test format; Reading Passage 1"}'::jsonb),
    ('20000000-0000-0000-0013-000000000012', '20000000-0000-0000-0000-000000000013', 'reading', 2, 'reading_comprehension', 13, 13, 20,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":13,"plays":0,"max_words":2,"allowed_types":["matching_headings","completion","multiple_choice"],"type_mix":{"completion":0.4,"matching_headings":0.3,"multiple_choice":0.3},"passage_sets":["single"],"source":"IELTS Academic test format; Reading Passage 2"}'::jsonb),
    ('20000000-0000-0000-0013-000000000013', '20000000-0000-0000-0000-000000000013', 'reading', 3, 'reading_comprehension', 14, 14, 20,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":14,"plays":0,"max_words":2,"allowed_types":["true_false_not_given","matching_information","completion"],"type_mix":{"completion":0.4,"true_false_not_given":0.3,"matching_information":0.3},"passage_sets":["single"],"source":"IELTS Academic test format; Reading Passage 3"}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- IELTS Writing: Task 1 describes a visual in at least 150 words; Task 2 an
-- essay of at least 250.
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0013-000000000021', '20000000-0000-0000-0000-000000000013', 'writing', 1, 'writing_prompt', 1, 1, 20,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":1,"plays":0,"min_words":150,"visual_required":true,"allowed_types":["writing_prompt"],"type_mix":{"writing_prompt":1.0},"source":"IELTS Academic test format; Writing Task 1"}'::jsonb),
    ('20000000-0000-0000-0013-000000000022', '20000000-0000-0000-0000-000000000013', 'writing', 2, 'writing_prompt', 1, 1, 40,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":1,"plays":0,"min_words":250,"allowed_types":["writing_prompt"],"type_mix":{"writing_prompt":1.0},"source":"IELTS Academic test format; Writing Task 2"}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- IELTS Speaking: three parts, with Part 2's one minute to prepare.
INSERT INTO assess.exam_parts (id, version_id, section, part_number, kind, question_count, group_size, duration_minutes, constraints)
VALUES
    ('20000000-0000-0000-0013-000000000031', '20000000-0000-0000-0000-000000000013', 'speaking', 1, 'speaking_task', 1, 1, 5,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":1,"plays":0,"speaking_seconds":300,"allowed_types":["speaking_task"],"type_mix":{"speaking_task":1.0},"source":"IELTS Academic test format; Speaking Part 1"}'::jsonb),
    ('20000000-0000-0000-0013-000000000032', '20000000-0000-0000-0000-000000000013', 'speaking', 2, 'speaking_task', 1, 1, 4,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":1,"plays":0,"preparation_seconds":60,"speaking_seconds":120,"allowed_types":["speaking_task"],"type_mix":{"speaking_task":1.0},"source":"IELTS Academic test format; Speaking Part 2, one minute to prepare"}'::jsonb),
    ('20000000-0000-0000-0013-000000000033', '20000000-0000-0000-0000-000000000013', 'speaking', 3, 'speaking_task', 1, 1, 4,
     '{"schema_version":1,"answer_mode":"typed","option_count":0,"questions_per_group":1,"plays":0,"speaking_seconds":300,"allowed_types":["speaking_task"],"type_mix":{"speaking_task":1.0},"source":"IELTS Academic test format; Speaking Part 3"}'::jsonb)
ON CONFLICT (version_id, section, part_number) DO NOTHING;

-- 5. The IELTS blueprint, with a CEFR mix like toeic_default.
INSERT INTO assess.blueprints (id, version_id, name, cefr_distribution, node_distribution)
VALUES (
    '20000000-0000-0000-0013-000000000001',
    '20000000-0000-0000-0000-000000000013',
    'ielts_default',
    '{"B1": 0.25, "B2": 0.45, "C1": 0.30}'::jsonb,
    '{}'::jsonb
) ON CONFLICT (version_id, name) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM assess.blueprints
WHERE version_id = '20000000-0000-0000-0000-000000000013';

DELETE FROM assess.exam_parts
WHERE version_id = '20000000-0000-0000-0000-000000000013';

DELETE FROM assess.exam_versions
WHERE code = 'IELTS_ACADEMIC_2026_R2';

UPDATE assess.exam_versions SET is_current = true, listed = true
WHERE code = 'IELTS_ACADEMIC_2026';

UPDATE assess.exam_versions SET listed = true
WHERE code IN ('CAMBRIDGE_B2_FIRST', 'VN_THPT_2026', 'TOEFL_IBT_2026');
UPDATE assess.exams SET listed = true
WHERE slug IN ('mock-toeic-a2', 'mock-toeic-b1', 'mock-toeic-b2');

ALTER TABLE assess.exams DROP COLUMN IF EXISTS listed;
ALTER TABLE assess.exam_versions DROP COLUMN IF EXISTS listed;

-- +goose StatementEnd
