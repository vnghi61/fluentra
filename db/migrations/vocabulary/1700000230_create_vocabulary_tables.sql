-- +goose Up
-- +goose StatementBegin

-- ----------------------------------------------------------------- words
CREATE TABLE IF NOT EXISTS skill.words (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    lemma          text        NOT NULL,
    pos            text        NOT NULL,
    cefr_level     text        NOT NULL DEFAULT 'A1',
    frequency_rank integer,
    ipa            text,
    audio_asset_id uuid,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_words_lemma_pos UNIQUE (lemma, pos),
    CONSTRAINT ck_words_cefr CHECK (cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$')
);

CREATE INDEX IF NOT EXISTS idx_words_lemma ON skill.words (lemma);
CREATE INDEX IF NOT EXISTS idx_words_cefr ON skill.words (cefr_level);

-- ----------------------------------------------------------- word_senses
CREATE TABLE IF NOT EXISTS skill.word_senses (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    word_id            uuid        NOT NULL REFERENCES skill.words (id) ON DELETE CASCADE,
    content_version_id uuid,
    definition         text        NOT NULL,
    definition_vi      text,
    register           text,
    domain             text,
    examples           jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_word_senses_word_id ON skill.word_senses (word_id);
CREATE INDEX IF NOT EXISTS idx_word_senses_content_version ON skill.word_senses (content_version_id);

-- -------------------------------------------------------- word_relations
CREATE TABLE IF NOT EXISTS skill.word_relations (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    from_word_id uuid        NOT NULL REFERENCES skill.words (id) ON DELETE CASCADE,
    to_word_id   uuid        NOT NULL REFERENCES skill.words (id) ON DELETE CASCADE,
    relation     text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_word_relations UNIQUE (from_word_id, to_word_id, relation),
    CONSTRAINT ck_word_relations_type CHECK (relation IN ('synonym', 'antonym', 'collocation', 'family', 'derivative'))
);

CREATE INDEX IF NOT EXISTS idx_word_relations_from ON skill.word_relations (from_word_id);
CREATE INDEX IF NOT EXISTS idx_word_relations_to ON skill.word_relations (to_word_id);

-- ----------------------------------------------------------------- decks
CREATE TABLE IF NOT EXISTS skill.decks (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        REFERENCES core.users (id) ON DELETE CASCADE,
    slug        text        NOT NULL,
    name        text        NOT NULL,
    description text,
    is_public   boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_decks_owner_slug UNIQUE NULLS NOT DISTINCT (owner_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_decks_owner ON skill.decks (owner_id);
CREATE INDEX IF NOT EXISTS idx_decks_public ON skill.decks (is_public);

-- ------------------------------------------------------------ deck_items
CREATE TABLE IF NOT EXISTS skill.deck_items (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    deck_id       uuid        NOT NULL REFERENCES skill.decks (id) ON DELETE CASCADE,
    word_sense_id uuid        NOT NULL REFERENCES skill.word_senses (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_deck_items UNIQUE (deck_id, word_sense_id)
);

CREATE INDEX IF NOT EXISTS idx_deck_items_deck ON skill.deck_items (deck_id);
CREATE INDEX IF NOT EXISTS idx_deck_items_sense ON skill.deck_items (word_sense_id);

-- ------------------------------------------------------- user_word_state
CREATE TABLE IF NOT EXISTS skill.user_word_state (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid        NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    word_sense_id uuid        NOT NULL REFERENCES skill.word_senses (id) ON DELETE CASCADE,
    status        text        NOT NULL DEFAULT 'new',
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_user_word_state UNIQUE (user_id, word_sense_id),
    CONSTRAINT ck_user_word_state_status CHECK (status IN ('new', 'learning', 'known', 'ignored'))
);

CREATE INDEX IF NOT EXISTS idx_user_word_state_user ON skill.user_word_state (user_id, status);
-- uq_user_word_state leads with user_id and idx_user_word_state_user leads with
-- user_id too, so neither covers the word_sense_id foreign key. Without this
-- index, deleting a sense sequentially scans every learner's word state.
CREATE INDEX IF NOT EXISTS idx_user_word_state_sense ON skill.user_word_state (word_sense_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill.user_word_state CASCADE;
DROP TABLE IF EXISTS skill.deck_items CASCADE;
DROP TABLE IF EXISTS skill.decks CASCADE;
DROP TABLE IF EXISTS skill.word_relations CASCADE;
DROP TABLE IF EXISTS skill.word_senses CASCADE;
DROP TABLE IF EXISTS skill.words CASCADE;
-- +goose StatementEnd
