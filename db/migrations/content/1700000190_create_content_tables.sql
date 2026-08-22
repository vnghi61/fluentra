-- +goose Up
-- +goose StatementBegin

-- ----------------------------------------------------------------- content schema
--
-- Tables owned exclusively by the content authoring module (Rule DB1 / Rule L3).
-- Cross-schema references point only at core.users (Rule DB4).
-- The content_versions table enforces immutability after publication via trigger (BR-CONTENT-01).
--
-- Two consequences of that trigger, because they are not visible from the column list
-- and P7.3 and P7.5 both depend on them:
--
--   * `archived` is reachable on content_items.status and NOT on content_versions.status.
--     The trigger refuses every UPDATE to a published version, a status change included, so
--     archiving is an item-level action. The enum is shared between the two tables; that
--     value is simply unreachable on the version.
--
--   * A published version therefore stays `published` for ever. A learner-facing query
--     filtered only on content_versions.status = 'published' will return archived material.
--     Every such read joins content_items and filters there too — which is what
--     idx_content_items_status_kind exists to serve.

CREATE TYPE content.authoring_status AS ENUM ('draft', 'in_review', 'approved', 'published', 'archived');
CREATE TYPE content.review_decision AS ENUM ('approved', 'changes_requested');
CREATE TYPE content.media_status AS ENUM ('pending', 'ready', 'failed');

-- ----------------------------------------------------------- content_items
CREATE TABLE IF NOT EXISTS content.content_items (
    id                 uuid                     PRIMARY KEY DEFAULT gen_random_uuid(),
    kind               text                     NOT NULL,
    slug               text                     NOT NULL,
    current_version_id uuid,
    status             content.authoring_status NOT NULL DEFAULT 'draft',
    owner_id           uuid                     NOT NULL,
    created_at         timestamptz              NOT NULL DEFAULT now(),
    updated_at         timestamptz              NOT NULL DEFAULT now(),

    CONSTRAINT fk_content_items_owner FOREIGN KEY (owner_id) REFERENCES core.users (id) ON DELETE RESTRICT,
    CONSTRAINT uq_content_items_slug UNIQUE (slug),
    CONSTRAINT ck_content_items_kind_length CHECK (char_length(kind) BETWEEN 1 AND 50),
    CONSTRAINT ck_content_items_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE INDEX IF NOT EXISTS idx_content_items_owner_id ON content.content_items (owner_id);
CREATE INDEX IF NOT EXISTS idx_content_items_status_kind ON content.content_items (status, kind) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_content_items_created_at ON content.content_items (created_at DESC, id DESC);

-- -------------------------------------------------------- content_versions
CREATE TABLE IF NOT EXISTS content.content_versions (
    id           uuid                     PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id      uuid                     NOT NULL,
    version      integer                  NOT NULL DEFAULT 1,
    kind         text                     NOT NULL,
    body         jsonb                    NOT NULL DEFAULT '{}'::jsonb,
    cefr_level   text                     NOT NULL,
    status       content.authoring_status NOT NULL DEFAULT 'draft',
    media_refs   text[]                   NOT NULL DEFAULT ARRAY[]::text[],
    published_at timestamptz,
    created_at   timestamptz              NOT NULL DEFAULT now(),
    updated_at   timestamptz              NOT NULL DEFAULT now(),

    CONSTRAINT fk_content_versions_item FOREIGN KEY (item_id) REFERENCES content.content_items (id) ON DELETE CASCADE,
    CONSTRAINT uq_content_versions_item_version UNIQUE (item_id, version),
    CONSTRAINT ck_content_versions_version_positive CHECK (version > 0),
    CONSTRAINT ck_content_versions_kind_length CHECK (char_length(kind) BETWEEN 1 AND 50),
    -- Case-sensitive on purpose. The CEFRLevel schema in
    -- api/openapi/components/content.yaml is enum [A1, A2, B1, B2, C1, C2], so a row
    -- stored as 'b1' would serialise into a response that violates the published
    -- contract. `~*` would have let it in, and the constraint exists to be the line
    -- that holds when the service is wrong.
    CONSTRAINT ck_content_versions_cefr_level CHECK (cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$'),
    CONSTRAINT ck_content_versions_published_at CHECK (
        (status = 'published' AND published_at IS NOT NULL) OR
        (status != 'published')
    )
);

CREATE INDEX IF NOT EXISTS idx_content_versions_item_id ON content.content_versions (item_id);
CREATE INDEX IF NOT EXISTS idx_content_versions_status ON content.content_versions (status);
CREATE INDEX IF NOT EXISTS idx_content_versions_cefr_level ON content.content_versions (cefr_level);
CREATE INDEX IF NOT EXISTS idx_content_versions_body_gin ON content.content_versions USING gin (body);

-- Circular FK from content_items.current_version_id to content_versions(id)
ALTER TABLE content.content_items
    ADD CONSTRAINT fk_content_items_current_version
    FOREIGN KEY (current_version_id)
    REFERENCES content.content_versions (id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_items_current_version_id ON content.content_items (current_version_id);

-- ------------------------------------------------------- immutability trigger
CREATE OR REPLACE FUNCTION content.fn_prevent_published_version_update()
RETURNS TRIGGER AS $$
BEGIN
    -- BR-CONTENT-01: A published version is immutable.
    -- Once published, the version snapshot cannot be modified.
    IF OLD.status = 'published' THEN
        RAISE EXCEPTION 'cannot update a published content version'
            USING ERRCODE = '23514',
                  HINT = 'published content versions are immutable; create a new version instead';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_content_versions_immutable
    BEFORE UPDATE ON content.content_versions
    FOR EACH ROW
    EXECUTE FUNCTION content.fn_prevent_published_version_update();

-- ------------------------------------------------------------ media_assets
CREATE TABLE IF NOT EXISTS content.media_assets (
    id          uuid                 PRIMARY KEY DEFAULT gen_random_uuid(),
    object_key  text                 NOT NULL,
    kind        text                 NOT NULL,
    duration_ms integer,
    checksum    text,
    status      content.media_status NOT NULL DEFAULT 'pending',
    byte_size   bigint,
    mime_type   text,
    created_at  timestamptz          NOT NULL DEFAULT now(),
    updated_at  timestamptz          NOT NULL DEFAULT now(),

    CONSTRAINT uq_media_assets_object_key UNIQUE (object_key),
    CONSTRAINT ck_media_assets_kind_length CHECK (char_length(kind) BETWEEN 1 AND 50),
    CONSTRAINT ck_media_assets_duration_ms CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT ck_media_assets_byte_size CHECK (byte_size IS NULL OR byte_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_media_assets_status ON content.media_assets (status);
CREATE INDEX IF NOT EXISTS idx_media_assets_kind ON content.media_assets (kind);

-- ------------------------------------------------------------- taxonomies
CREATE TABLE IF NOT EXISTS content.taxonomies (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace   text        NOT NULL,
    code        text        NOT NULL,
    label       text        NOT NULL,
    parent_id   uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_taxonomies_parent FOREIGN KEY (parent_id) REFERENCES content.taxonomies (id) ON DELETE SET NULL,
    CONSTRAINT uq_taxonomies_namespace_code UNIQUE (namespace, code),
    CONSTRAINT ck_taxonomies_namespace_length CHECK (char_length(namespace) BETWEEN 1 AND 50),
    CONSTRAINT ck_taxonomies_code_length CHECK (char_length(code) BETWEEN 1 AND 100),
    CONSTRAINT ck_taxonomies_label_length CHECK (char_length(label) BETWEEN 1 AND 200)
);

CREATE INDEX IF NOT EXISTS idx_taxonomies_parent_id ON content.taxonomies (parent_id);
CREATE INDEX IF NOT EXISTS idx_taxonomies_namespace ON content.taxonomies (namespace);

-- ----------------------------------------------------------- content_tags
CREATE TABLE IF NOT EXISTS content.content_tags (
    item_id     uuid        NOT NULL,
    taxonomy_id uuid        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (item_id, taxonomy_id),
    CONSTRAINT fk_content_tags_item FOREIGN KEY (item_id) REFERENCES content.content_items (id) ON DELETE CASCADE,
    CONSTRAINT fk_content_tags_taxonomy FOREIGN KEY (taxonomy_id) REFERENCES content.taxonomies (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_content_tags_taxonomy_id ON content.content_tags (taxonomy_id, item_id);

-- -------------------------------------------------------- content_reviews
CREATE TABLE IF NOT EXISTS content.content_reviews (
    id          uuid                    PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id  uuid                    NOT NULL,
    reviewer_id uuid                    NOT NULL,
    decision    content.review_decision NOT NULL,
    comments    text,
    created_at  timestamptz             NOT NULL DEFAULT now(),

    CONSTRAINT fk_content_reviews_version FOREIGN KEY (version_id) REFERENCES content.content_versions (id) ON DELETE CASCADE,
    CONSTRAINT fk_content_reviews_reviewer FOREIGN KEY (reviewer_id) REFERENCES core.users (id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_content_reviews_version_id ON content.content_reviews (version_id);
CREATE INDEX IF NOT EXISTS idx_content_reviews_reviewer_id ON content.content_reviews (reviewer_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_content_versions_immutable ON content.content_versions;
DROP FUNCTION IF EXISTS content.fn_prevent_published_version_update();

DROP TABLE IF EXISTS content.content_reviews;
DROP TABLE IF EXISTS content.content_tags;
DROP TABLE IF EXISTS content.taxonomies;
DROP TABLE IF EXISTS content.media_assets;

ALTER TABLE IF EXISTS content.content_items DROP CONSTRAINT IF EXISTS fk_content_items_current_version;
DROP TABLE IF EXISTS content.content_versions;
DROP TABLE IF EXISTS content.content_items;

DROP TYPE IF EXISTS content.media_status;
DROP TYPE IF EXISTS content.review_decision;
DROP TYPE IF EXISTS content.authoring_status;
-- +goose StatementEnd
