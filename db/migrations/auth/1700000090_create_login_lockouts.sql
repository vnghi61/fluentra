-- +goose Up
-- +goose StatementBegin

CREATE TABLE core.login_lockouts (
    scope          text        NOT NULL CHECK (scope IN ('account', 'ip')),
    subject_hash   bytea       NOT NULL,
    lockout_level  integer     NOT NULL DEFAULT 1 CHECK (lockout_level >= 1),
    locked_until   timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at     timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (scope, subject_hash)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE core.login_lockouts;

-- +goose StatementEnd
