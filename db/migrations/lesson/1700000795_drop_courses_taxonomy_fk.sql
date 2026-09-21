-- +goose Up
-- +goose StatementBegin

-- DB4 (ADR-0004): the only cross-schema foreign key permitted is to
-- core.users(id). 1700000745 pointed learn.courses.topic_taxonomy_id at
-- content.taxonomies; keep the column, drop the key — the fallback
-- phase-3-work-order-15 §5 names. A deleted taxonomy now leaves a dangling id,
-- which the topic filter simply stops matching.
ALTER TABLE learn.courses
    DROP CONSTRAINT IF EXISTS courses_topic_taxonomy_id_fkey;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE learn.courses
    ADD CONSTRAINT courses_topic_taxonomy_id_fkey
    FOREIGN KEY (topic_taxonomy_id) REFERENCES content.taxonomies (id) ON DELETE SET NULL;

-- +goose StatementEnd
