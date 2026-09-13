-- +goose Up
-- +goose StatementBegin

ALTER TABLE skill.vocab_upload_items
    ADD COLUMN IF NOT EXISTS corrected_term text,
    ADD COLUMN IF NOT EXISTS suggested_term text,
    ADD COLUMN IF NOT EXISTS note_code text;

ALTER TABLE skill.vocab_upload_items DROP CONSTRAINT IF EXISTS ck_vocab_upload_items_note_code;
ALTER TABLE skill.vocab_upload_items ADD CONSTRAINT ck_vocab_upload_items_note_code
    CHECK (note_code IS NULL OR note_code IN (
        'spelling_corrected',
        'meaning_corrected',
        'spelling_suggestion',
        'meaning_mismatch',
        'already_in_your_words',
        'not_a_word',
        'proper_noun',
        'queued_for_enrichment'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE skill.vocab_upload_items DROP CONSTRAINT IF EXISTS ck_vocab_upload_items_note_code;
ALTER TABLE skill.vocab_upload_items
    DROP COLUMN IF EXISTS note_code,
    DROP COLUMN IF EXISTS suggested_term,
    DROP COLUMN IF EXISTS corrected_term;

-- +goose StatementEnd
