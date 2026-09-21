-- +goose Up
CREATE TABLE IF NOT EXISTS resource.renditions (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id    uuid        NOT NULL REFERENCES resource.resources (id) ON DELETE CASCADE,
    kind           text        NOT NULL,
    status         text        NOT NULL DEFAULT 'pending',
    object_key     text,
    mime_type      text        NOT NULL DEFAULT '',
    width          integer,
    height         integer,
    duration_ms    integer,
    byte_size      bigint,
    tool_version   text        NOT NULL DEFAULT '',  -- 'ffmpeg 6.1.1', so a bad encoder can be found later
    attempts       integer     NOT NULL DEFAULT 0,
    failure_reason text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_renditions_resource_kind UNIQUE (resource_id, kind),
    CONSTRAINT uq_renditions_object_key UNIQUE (object_key),
    CONSTRAINT ck_renditions_kind CHECK (kind IN (
        'thumbnail', 'display', 'preview', 'audio_web', 'poster', 'video_360p', 'video_720p')),
    CONSTRAINT ck_renditions_status CHECK (status IN ('pending', 'ready', 'failed', 'skipped')),
    CONSTRAINT ck_renditions_ready_has_object CHECK (status <> 'ready' OR object_key IS NOT NULL),
    CONSTRAINT ck_renditions_attempts CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS idx_renditions_pending
    ON resource.renditions (created_at) WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_renditions_resource_status
    ON resource.renditions (resource_id, status);

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA resource TO fluentra_app;

-- +goose Down
DROP TABLE IF EXISTS resource.renditions;
