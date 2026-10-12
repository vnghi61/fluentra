-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage B resolves a word the database already holds and marks the item
-- with note_code 'known_word', which 1700000492's list never allowed. Every such
-- word failed the CHECK, was retried three times and was retired as "failed",
-- shown to the learner as the dictionary not responding.
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
        'queued_for_enrichment',
        'known_word'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- The old list cannot hold 'known_word'; the item stays verified, only the note goes.
UPDATE skill.vocab_upload_items SET note_code = NULL WHERE note_code = 'known_word';
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
