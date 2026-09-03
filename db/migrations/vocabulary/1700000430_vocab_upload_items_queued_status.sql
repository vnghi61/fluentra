-- +goose Up
-- +goose StatementBegin

-- 1. Add 'queued' to allowed vocab_upload_items statuses
ALTER TABLE skill.vocab_upload_items DROP CONSTRAINT IF EXISTS ck_vocab_upload_items_status;
ALTER TABLE skill.vocab_upload_items ADD CONSTRAINT ck_vocab_upload_items_status
    CHECK (status IN ('pending', 'verified', 'rejected', 'failed', 'queued'));

CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_queued
    ON skill.vocab_upload_items (created_at) WHERE status = 'queued';

-- 2. Permit an unassigned CEFR level on this module's own words.
--
-- A word saved while every provider is out of quota has not been graded, and a
-- level nobody assigned must not be invented -- the previous version wrote "B1"
-- and stored a judgement no model made. The empty string is the honest value,
-- and `status = 'queued'` is what says why it is empty.
--
-- content.content_versions needs the same relaxation for the same reason, and
-- it is done in db/migrations/content/1700000440 rather than here: that table
-- belongs to the content module, and a migration that reaches across schemas
-- makes "who owns this table" unanswerable. It also fails outright wherever the
-- migration role does not own the table it is altering.
ALTER TABLE skill.words DROP CONSTRAINT IF EXISTS ck_words_cefr;
ALTER TABLE skill.words ADD CONSTRAINT ck_words_cefr
    CHECK (cefr_level = '' OR cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE skill.words DROP CONSTRAINT IF EXISTS ck_words_cefr;
ALTER TABLE skill.words ADD CONSTRAINT ck_words_cefr
    CHECK (cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

DROP INDEX IF EXISTS skill.idx_vocab_upload_items_queued;

ALTER TABLE skill.vocab_upload_items DROP CONSTRAINT IF EXISTS ck_vocab_upload_items_status;
ALTER TABLE skill.vocab_upload_items ADD CONSTRAINT ck_vocab_upload_items_status
    CHECK (status IN ('pending', 'verified', 'rejected', 'failed'));

-- +goose StatementEnd
