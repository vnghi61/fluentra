-- +goose Up
-- +goose StatementBegin

-- Let a content version carry no CEFR level yet.
--
-- `vocabulary` materialises a content version for every word a learner adds. When
-- every AI provider is out of quota the word is still kept -- the learner gets a
-- flashcard to review, and `skill.vocab_upload_items.status = 'queued'` records
-- that it has not been graded. Nothing has assigned that word a level at that
-- point, and the previous version of the code wrote "B1" anyway: a judgement no
-- model made, stored where later code reads it as one.
--
-- The honest value is the empty string, and the queued status is what says why it
-- is empty. The background enrichment sweep fills it in when quota returns.
--
-- This lives in the content module's own migrations rather than in vocabulary's,
-- where it was first written. The table belongs to content, and a migration that
-- reaches into another module's schema makes "who owns this table" unanswerable
-- from the directory layout.
--
-- Moving it does not change who runs it, and that is worth stating plainly
-- because it looks like it might. ALTER TABLE requires *ownership*, not
-- privileges, so on any database where the migration role does not own
-- content_versions this fails either way:
--
--     ERROR: must be owner of table content_versions
--
-- and no GRANT fixes it. A database migrated from scratch by cmd/migrate is
-- fine, because everything after the bootstrap is created by the migration role
-- itself. One migrated by some other hand first is not.
ALTER TABLE content.content_versions
    DROP CONSTRAINT IF EXISTS ck_content_versions_cefr_level;

ALTER TABLE content.content_versions
    ADD CONSTRAINT ck_content_versions_cefr_level
    CHECK (cefr_level = '' OR cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE content.content_versions
    DROP CONSTRAINT IF EXISTS ck_content_versions_cefr_level;

ALTER TABLE content.content_versions
    ADD CONSTRAINT ck_content_versions_cefr_level
    CHECK (cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

-- +goose StatementEnd
