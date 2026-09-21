-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------- studio.listings
CREATE TABLE IF NOT EXISTS studio.listings (
    course_id          uuid        PRIMARY KEY REFERENCES learn.courses(id) ON DELETE CASCADE,
    creator_id         uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    pricing_model      text        NOT NULL DEFAULT 'free' CHECK (pricing_model IN ('free', 'one_time')),
    price_vnd          bigint      NOT NULL DEFAULT 0 CHECK (price_vnd >= 0),
    revenue_share_bps  int         NOT NULL DEFAULT 7000 CHECK (revenue_share_bps >= 0 AND revenue_share_bps <= 10000),
    status             text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'unlisted', 'taken_down')),
    published_at       timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_listings_creator ON studio.listings(creator_id);
CREATE INDEX IF NOT EXISTS idx_listings_status ON studio.listings(status);

-- -------------------------------------------------- studio.purchases
CREATE TABLE IF NOT EXISTS studio.purchases (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    course_id          uuid        NOT NULL REFERENCES learn.courses(id) ON DELETE CASCADE,
    order_id           uuid        REFERENCES billing.orders(id) ON DELETE SET NULL,
    price_paid_vnd     bigint      NOT NULL DEFAULT 0 CHECK (price_paid_vnd >= 0),
    granted_at         timestamptz NOT NULL DEFAULT now(),
    revoked_at         timestamptz,
    revoke_reason      text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_purchases_user ON studio.purchases(user_id);
CREATE INDEX IF NOT EXISTS idx_purchases_course ON studio.purchases(course_id);
CREATE INDEX IF NOT EXISTS idx_purchases_order ON studio.purchases(order_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_purchases_user_course_active ON studio.purchases(user_id, course_id) WHERE revoked_at IS NULL;

-- -------------------------------------------------- studio.creator_ledger
CREATE TABLE IF NOT EXISTS studio.creator_ledger (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id         uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    kind               text        NOT NULL CHECK (kind IN ('sale', 'refund', 'payout', 'adjustment', 'platform_share')),
    amount_vnd         bigint      NOT NULL,
    gross_amount_vnd   bigint      NOT NULL DEFAULT 0,
    fee_amount_vnd     bigint      NOT NULL DEFAULT 0,
    purchase_id        uuid        REFERENCES studio.purchases(id) ON DELETE SET NULL,
    payout_id          uuid        REFERENCES billing.payouts(id) ON DELETE SET NULL,
    note               text        NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_creator_ledger_creator ON studio.creator_ledger(creator_id);
CREATE INDEX IF NOT EXISTS idx_creator_ledger_purchase ON studio.creator_ledger(purchase_id);
CREATE INDEX IF NOT EXISTS idx_creator_ledger_payout ON studio.creator_ledger(payout_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA studio TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS studio.creator_ledger;
DROP TABLE IF EXISTS studio.purchases;
DROP TABLE IF EXISTS studio.listings;
-- +goose StatementEnd
