-- +goose Up
-- +goose StatementBegin

ALTER TABLE content.taxonomies
    ADD COLUMN IF NOT EXISTS description   text        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cefr_level    text,
    ADD COLUMN IF NOT EXISTS position      integer     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deprecated_at timestamptz;

-- Same enum as content_versions.cefr_level, and nullable: a strand root such as
-- TENSES has no single level, while PRESENT_PERFECT does.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_cefr_level
    CHECK (cefr_level IS NULL OR cefr_level ~ '^(A1|A2|B1|B2|C1|C2)$');

-- Namespaces this system knows. Additive: a new strand is a migration, not a typo.
--
-- `topic` is here because `content` already writes it: the tag resolver in
-- content/service/service.go looks content tags up as ('topic', <tag>). It
-- predates the spine and is not part of it, but a constraint that forbade it
-- would make every content tag unresolvable — the rows could never be created.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_namespace
    CHECK (namespace IN ('course_topic', 'topic', 'grammar', 'vocabulary',
                         'pattern', 'pronunciation', 'skill'));

-- Code format, per namespace.
--
-- The 20 course_topic rows seeded by 1700000746 are kebab-case
-- ('business-english') and content's own tags are lowercase words ('science');
-- the five spine namespaces are SCREAMING_SNAKE. One global pattern would
-- refuse rows that are already in the table and tags the module already writes.
ALTER TABLE content.taxonomies
    ADD CONSTRAINT ck_taxonomies_code_format
    CHECK (
        (namespace IN ('course_topic', 'topic') AND code ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
        OR
        (namespace NOT IN ('course_topic', 'topic') AND code ~ '^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$')
    );

-- ------------------------------------------------- taxonomy_prerequisites
--
-- "node_id requires requires_node_id first." A DAG, not a tree: PRESENT_PERFECT
-- requires both PAST_SIMPLE and PRESENT_SIMPLE, and REPORTED_SPEECH requires
-- PAST_SIMPLE through a different chain. parent_id cannot express either.
--
-- RESTRICT on requires_node_id, not CASCADE: silently dropping the edge that
-- says "learn this first" is how a learning path quietly starts teaching the
-- present perfect to somebody who has not met the past simple.
CREATE TABLE IF NOT EXISTS content.taxonomy_prerequisites (
    node_id          uuid        NOT NULL,
    requires_node_id uuid        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (node_id, requires_node_id),
    CONSTRAINT fk_taxonomy_prereq_node
        FOREIGN KEY (node_id) REFERENCES content.taxonomies (id) ON DELETE CASCADE,
    CONSTRAINT fk_taxonomy_prereq_requires
        FOREIGN KEY (requires_node_id) REFERENCES content.taxonomies (id) ON DELETE RESTRICT,
    CONSTRAINT ck_taxonomy_prereq_not_self CHECK (node_id <> requires_node_id)
);

CREATE INDEX IF NOT EXISTS idx_taxonomy_prereq_requires
    ON content.taxonomy_prerequisites (requires_node_id);

CREATE INDEX IF NOT EXISTS idx_taxonomies_namespace_position
    ON content.taxonomies (namespace, position) WHERE deprecated_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS content.taxonomy_prerequisites;

DROP INDEX IF EXISTS content.idx_taxonomies_namespace_position;

ALTER TABLE content.taxonomies
    DROP CONSTRAINT IF EXISTS ck_taxonomies_code_format,
    DROP CONSTRAINT IF EXISTS ck_taxonomies_namespace,
    DROP CONSTRAINT IF EXISTS ck_taxonomies_cefr_level,
    DROP COLUMN IF EXISTS deprecated_at,
    DROP COLUMN IF EXISTS position,
    DROP COLUMN IF EXISTS cefr_level,
    DROP COLUMN IF EXISTS description;

-- +goose StatementEnd
