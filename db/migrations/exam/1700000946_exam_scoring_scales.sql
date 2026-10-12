-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage I.3.6 / D22-26: the report maps a sitting onto its exam's own
-- scale by scoring.type. VSTEP's row said "raw_with_estimate", TOEIC's type, so a
-- VSTEP sitting was reported on TOEIC's 10–990. VSTEP scores each skill 0–10.
UPDATE assess.exam_versions
SET scoring = scoring || '{"type": "vstep", "scale": [0, 10], "step": 0.5}'::jsonb
WHERE code = 'VSTEP_3_5';

-- The IELTS row claimed a published conversion, but the report maps 0–100 onto
-- the band linearly; no published raw-to-band table is applied. Until one is,
-- the band is an estimate and must not be labelled otherwise.
UPDATE assess.exam_versions
SET scoring = scoring - 'published_conversion'
WHERE code = 'IELTS_ACADEMIC_2026_R2';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE assess.exam_versions
SET scoring = (scoring - 'scale' - 'step') || '{"type": "raw_with_estimate"}'::jsonb
WHERE code = 'VSTEP_3_5';

UPDATE assess.exam_versions
SET scoring = scoring || '{"published_conversion": true}'::jsonb
WHERE code = 'IELTS_ACADEMIC_2026_R2';
-- +goose StatementEnd
