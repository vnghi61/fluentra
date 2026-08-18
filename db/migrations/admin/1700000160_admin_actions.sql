-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------- core.admin_actions
--
-- Audit log of administrative operations performed on target accounts.
-- Owned exclusively by the admin module (Rule L3).
CREATE TABLE IF NOT EXISTS core.admin_actions (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      uuid        NOT NULL REFERENCES core.users(id),
    target_id     uuid        NOT NULL REFERENCES core.users(id),
    action        text        NOT NULL, -- suspend, reinstate, revoke_sessions
    reason        text        NOT NULL,
    occurred_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_admin_actions_not_self CHECK (actor_id != target_id),
    CONSTRAINT ck_admin_actions_reason_len CHECK (char_length(reason) >= 10)
);

CREATE INDEX idx_admin_actions_target ON core.admin_actions(target_id, occurred_at DESC);
CREATE INDEX idx_admin_actions_actor ON core.admin_actions(actor_id, occurred_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.admin_actions;
-- +goose StatementEnd
