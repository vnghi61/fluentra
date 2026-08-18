-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------- core.feature_flags
--
-- Runtime feature toggles with percentage rollouts.
-- Owned exclusively by the admin module (Rule L3).
CREATE TABLE IF NOT EXISTS core.feature_flags (
    key              text        PRIMARY KEY,
    enabled          boolean     NOT NULL DEFAULT false,
    rollout_percent  integer     NOT NULL DEFAULT 0 CHECK (rollout_percent >= 0 AND rollout_percent <= 100),
    owner            text        NOT NULL,
    expires_on       date        NOT NULL,
    description      text        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_feature_flags_owner_not_empty CHECK (char_length(owner) > 0),
    CONSTRAINT ck_feature_flags_expires_future CHECK (expires_on > current_date)
);

CREATE INDEX idx_feature_flags_enabled ON core.feature_flags(enabled) WHERE enabled = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.feature_flags;
-- +goose StatementEnd
