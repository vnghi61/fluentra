-- +goose Up
CREATE SCHEMA IF NOT EXISTS resource;

CREATE TABLE IF NOT EXISTS resource.resources (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL,
    kind              text        NOT NULL,   -- 'file' | 'url'
    title             text        NOT NULL DEFAULT '',

    -- File resources. object_key is in fluentra-uploads and is never rewritten.
    object_key        text,
    original_filename text        NOT NULL DEFAULT '',
    declared_mime     text        NOT NULL DEFAULT '',  -- what the client said
    detected_mime     text        NOT NULL DEFAULT '',  -- what the bytes say
    byte_size         bigint,
    checksum          text,

    -- URL resources. Referenced, never copied (D17-2).
    source_url        text,

    status            text        NOT NULL DEFAULT 'pending',
    failure_reason    text        NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    validated_at      timestamptz,

    CONSTRAINT fk_resources_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_resources_kind CHECK (kind IN ('file', 'url')),
    CONSTRAINT ck_resources_status
        CHECK (status IN ('pending', 'uploaded', 'validated', 'rejected', 'failed')),
    -- A file resource has an object; a URL resource has a URL. Neither has both,
    -- and a row with neither is a row nothing downstream can act on.
    CONSTRAINT ck_resources_shape CHECK (
        (kind = 'file' AND object_key IS NOT NULL AND source_url IS NULL) OR
        (kind = 'url'  AND source_url IS NOT NULL AND object_key IS NULL)
    ),
    CONSTRAINT ck_resources_byte_size CHECK (byte_size IS NULL OR byte_size >= 0),
    CONSTRAINT ck_resources_rejected_has_reason CHECK (
        status <> 'rejected' OR length(btrim(failure_reason)) > 0
    ),
    CONSTRAINT uq_resources_object_key UNIQUE (object_key)
);

CREATE INDEX IF NOT EXISTS idx_resources_user ON resource.resources (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_resources_pending
    ON resource.resources (created_at) WHERE status IN ('pending', 'uploaded');

-- The application connects as fluentra_app, not as the owner. _bootstrap grants
-- it the schemas it creates; a schema created here gets nothing unless it is
-- granted here, and every test in the suite connects as the superuser, so a
-- missing grant passes CI and fails in production with "permission denied for
-- schema resource". Same pattern as studio's 1700000750.
GRANT USAGE ON SCHEMA resource TO fluentra_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA resource TO fluentra_app;

-- +goose Down
DROP TABLE IF EXISTS resource.resources;
DROP SCHEMA IF EXISTS resource CASCADE;
