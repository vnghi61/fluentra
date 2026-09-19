-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS billing;

-- -------------------------------------------------- billing.orders
CREATE TABLE IF NOT EXISTS billing.orders (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    reference    text        NOT NULL UNIQUE,
    amount_vnd   bigint      NOT NULL CHECK (amount_vnd > 0),
    status       text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'expired', 'cancelled', 'refunded')),
    subject_kind text        NOT NULL DEFAULT 'course',
    subject_id   uuid        NOT NULL,
    expires_at   timestamptz NOT NULL,
    paid_at      timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user ON billing.orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_reference ON billing.orders(reference);
CREATE INDEX IF NOT EXISTS idx_orders_status ON billing.orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_expires_at ON billing.orders(expires_at) WHERE status = 'pending';

-- -------------------------------------------------- billing.sepay_transactions
CREATE TABLE IF NOT EXISTS billing.sepay_transactions (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    sepay_id         bigint      NOT NULL UNIQUE,
    gateway          text        NOT NULL DEFAULT '',
    transaction_date timestamptz NOT NULL,
    account_number   text        NOT NULL,
    sub_account      text        NOT NULL DEFAULT '',
    code             text        NOT NULL DEFAULT '',
    content          text        NOT NULL,
    transfer_type    text        NOT NULL,
    transfer_amount  bigint      NOT NULL,
    reference_code   text        NOT NULL DEFAULT '',
    accumulated      bigint      NOT NULL DEFAULT 0,
    order_id         uuid        REFERENCES billing.orders(id) ON DELETE SET NULL,
    matched_at       timestamptz,
    unmatched_reason text,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sepay_transactions_sepay_id ON billing.sepay_transactions(sepay_id);
CREATE INDEX IF NOT EXISTS idx_sepay_transactions_order_id ON billing.sepay_transactions(order_id);
CREATE INDEX IF NOT EXISTS idx_sepay_unmatched ON billing.sepay_transactions(matched_at) WHERE matched_at IS NULL;

-- -------------------------------------------------- billing.payment_webhooks
CREATE TABLE IF NOT EXISTS billing.payment_webhooks (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    provider          text        NOT NULL,
    provider_event_id text        NOT NULL UNIQUE,
    payload           jsonb       NOT NULL,
    signature_valid   bool        NOT NULL DEFAULT true,
    received_at       timestamptz NOT NULL DEFAULT now(),
    processed_at      timestamptz,
    error             text
);

CREATE INDEX IF NOT EXISTS idx_payment_webhooks_provider_event ON billing.payment_webhooks(provider, provider_event_id);

-- -------------------------------------------------- billing.refunds
CREATE TABLE IF NOT EXISTS billing.refunds (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   uuid        NOT NULL REFERENCES billing.orders(id) ON DELETE CASCADE,
    amount_vnd bigint      NOT NULL CHECK (amount_vnd > 0),
    reason     text        NOT NULL DEFAULT '',
    actor_id   uuid        NOT NULL REFERENCES core.users(id),
    status     text        NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'sent', 'failed')),
    sent_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- -------------------------------------------------- billing.payouts
CREATE TABLE IF NOT EXISTS billing.payouts (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id     uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    amount_vnd     bigint      NOT NULL CHECK (amount_vnd > 0),
    status         text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    bank_reference text,
    actor_id       uuid        NOT NULL REFERENCES core.users(id),
    sent_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

GRANT USAGE ON SCHEMA billing TO fluentra_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA billing TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS billing.payouts;
DROP TABLE IF EXISTS billing.refunds;
DROP TABLE IF EXISTS billing.payment_webhooks;
DROP TABLE IF EXISTS billing.sepay_transactions;
DROP TABLE IF EXISTS billing.orders;
DROP SCHEMA IF EXISTS billing;
-- +goose StatementEnd
