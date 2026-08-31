-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------- uploads
--
-- A learner's own vocabulary, pasted in and waiting to be checked.
--
-- Two tables rather than one because the two have different lifetimes and
-- different failure modes: the submission is what the learner did, and is
-- finished the moment it is stored; each word in it is a separate unit of work
-- that the verification job can succeed at, fail at, or retry independently.
-- One table would mean a single failed word marking the whole paste as failed.
CREATE TABLE IF NOT EXISTS skill.vocab_uploads (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL,
    -- The text exactly as pasted, kept so a learner can see what they sent and
    -- so a parser change can be re-run against the original rather than
    -- against what the old parser made of it.
    raw_text     text        NOT NULL,
    -- The deck the verified words land in. Nullable until the first word is
    -- verified: creating an empty deck for a paste that turns out to be
    -- nonsense leaves the learner a deck they never asked for.
    deck_id      uuid        REFERENCES skill.decks (id) ON DELETE SET NULL,
    status       text        NOT NULL DEFAULT 'pending',
    item_count   integer     NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,

    CONSTRAINT fk_vocab_uploads_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_vocab_uploads_status CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT ck_vocab_uploads_item_count CHECK (item_count >= 0),
    -- A paste with nothing in it is a mistake, not an upload. Bounded above so
    -- one request cannot hand the verification job a novel.
    CONSTRAINT ck_vocab_uploads_raw_text CHECK (length(raw_text) BETWEEN 1 AND 100000)
);

CREATE INDEX IF NOT EXISTS idx_vocab_uploads_user ON skill.vocab_uploads (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vocab_uploads_deck ON skill.vocab_uploads (deck_id);

-- ----------------------------------------------------------- upload_items
CREATE TABLE IF NOT EXISTS skill.vocab_upload_items (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id         uuid        NOT NULL,
    user_id           uuid        NOT NULL,
    -- The word as the learner wrote it, before any normalisation, so the
    -- rejection message can quote them rather than a cleaned-up version.
    term              text        NOT NULL,
    provided_meaning  text        NOT NULL DEFAULT '',
    status            text        NOT NULL DEFAULT 'pending',
    -- Why it was rejected, addressed to the learner. Empty while pending.
    reason            text        NOT NULL DEFAULT '',
    -- The dictionary entry the verified word became, and what a review card
    -- points at. Null while pending and for anything rejected.
    word_sense_id     uuid        REFERENCES skill.word_senses (id) ON DELETE SET NULL,
    -- Which model answered, so a verification done by the offline mock can be
    -- told apart from a real one long after the fact.
    verified_by_model text        NOT NULL DEFAULT '',
    attempts          integer     NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    verified_at       timestamptz,

    CONSTRAINT fk_vocab_upload_items_upload
        FOREIGN KEY (upload_id) REFERENCES skill.vocab_uploads (id) ON DELETE CASCADE,
    CONSTRAINT fk_vocab_upload_items_user
        FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    -- The same word twice in one paste is one word. Without this a learner who
    -- pasted a list with duplicates would be paid XP for each copy.
    CONSTRAINT uq_vocab_upload_items UNIQUE (upload_id, term),
    CONSTRAINT ck_vocab_upload_items_status
        CHECK (status IN ('pending', 'verified', 'rejected', 'failed')),
    CONSTRAINT ck_vocab_upload_items_term CHECK (length(btrim(term)) BETWEEN 1 AND 200),
    CONSTRAINT ck_vocab_upload_items_attempts CHECK (attempts >= 0)
);

-- The verification job's claim query: the oldest pending items that have not
-- already failed too many times.
CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_pending
    ON skill.vocab_upload_items (created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_upload
    ON skill.vocab_upload_items (upload_id, status);
CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_user
    ON skill.vocab_upload_items (user_id, status);
-- Nothing else covers the sense foreign key, so deleting a sense would
-- sequentially scan every learner's uploads.
CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_sense
    ON skill.vocab_upload_items (word_sense_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill.vocab_upload_items CASCADE;
DROP TABLE IF EXISTS skill.vocab_uploads CASCADE;
-- +goose StatementEnd
