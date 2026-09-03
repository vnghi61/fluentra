-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS learn.xp_activity_high_water (
    user_id     uuid        NOT NULL,
    activity_id text        NOT NULL,
    best_score  integer     NOT NULL DEFAULT 0,
    xp_granted  integer     NOT NULL DEFAULT 0,
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT pk_xp_activity_high_water PRIMARY KEY (user_id, activity_id),
    CONSTRAINT fk_xp_activity_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_xp_activity_best_score CHECK (best_score >= 0 AND best_score <= 100),
    CONSTRAINT ck_xp_activity_xp_granted CHECK (xp_granted >= 0)
);

CREATE INDEX IF NOT EXISTS idx_xp_activity_user ON learn.xp_activity_high_water (user_id);

-- Drop the global unique constraint so that activity retakes can receive incremental awards
ALTER TABLE learn.xp_events DROP CONSTRAINT IF EXISTS uq_xp_events_idempotency;

-- Preserve uniqueness for unscored sources so lesson, review_session, and upload_verified cannot be awarded twice
CREATE UNIQUE INDEX IF NOT EXISTS uq_xp_events_unscored
    ON learn.xp_events (user_id, source, source_id)
    WHERE source != 'activity';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS learn.uq_xp_events_unscored;
ALTER TABLE learn.xp_events ADD CONSTRAINT uq_xp_events_idempotency UNIQUE (user_id, source, source_id);
DROP TABLE IF EXISTS learn.xp_activity_high_water;
-- +goose StatementEnd
