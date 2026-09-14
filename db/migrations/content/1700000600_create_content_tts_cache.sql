-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS content.tts_cache (
    text_hash       varchar(64) NOT NULL,
    voice           varchar(64) NOT NULL,
    engine          varchar(64) NOT NULL,
    engine_version  varchar(64) NOT NULL,
    object_key      text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT pk_content_tts_cache PRIMARY KEY (text_hash, voice)
);

CREATE INDEX IF NOT EXISTS idx_tts_cache_object_key ON content.tts_cache (object_key);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS content.tts_cache;

-- +goose StatementEnd
