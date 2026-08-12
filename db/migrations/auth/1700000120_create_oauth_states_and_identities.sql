-- +goose Up
-- +goose StatementBegin

-- --------------------------------------------------------- core.oauth_states
--
-- One in-flight authorization request. **This table is the CSRF defence for the
-- whole OAuth flow** (BR-AUTH-17): a callback carrying a `state` that has no row
-- here, or one whose row is already consumed, is not a callback from a flow this
-- learner started.
--
-- It is a table rather than a signed cookie because single use has to be
-- enforced somewhere that can say "already", and a cookie cannot. The row is the
-- only record that the exchange about to happen was asked for.
CREATE TABLE IF NOT EXISTS core.oauth_states (
    id                 uuid        PRIMARY KEY,

    -- The opaque value handed to Google and returned by the browser. Stored in
    -- the clear because it is the lookup key and is not a secret on its own --
    -- what makes it useful is that this server issued it and has not yet
    -- consumed it.
    state              text        NOT NULL,

    provider           text        NOT NULL CHECK (provider IN ('google')),

    -- The value bound into the ID token. Verifying it is what stops a token
    -- obtained for one authorization request being replayed into another
    -- (BR-AUTH-18).
    nonce              text        NOT NULL,

    -- SHA-256 of the PKCE verifier. The verifier itself never touches the
    -- database: the digest is all that is needed to hand the verifier back to
    -- Google at exchange time, and a dump of this table then contains nothing
    -- that completes a flow.
    pkce_verifier_hash bytea       NOT NULL,

    -- Where to send the learner afterwards. Kept here rather than round-tripped
    -- through Google, so it cannot be rewritten in flight into an open redirect.
    redirect_to        text,

    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL,

    -- Set the moment the state is spent. Single use is the point, and this is
    -- the record of it -- a second callback with the same state finds this
    -- non-null and is refused.
    consumed_at        timestamptz,

    CONSTRAINT uq_oauth_states_state UNIQUE (state),
    CONSTRAINT ck_oauth_states_pkce_length CHECK (octet_length(pkce_verifier_hash) = 32),
    CONSTRAINT ck_oauth_states_expiry_after_issue CHECK (expires_at > created_at)
);

-- The sweep of states nobody ever came back for. Most rows here are abandoned
-- flows -- a learner who closed the consent screen -- so the table needs a way
-- to shed them that does not scan.
CREATE INDEX IF NOT EXISTS idx_oauth_states_expires
    ON core.oauth_states (expires_at)
    WHERE consumed_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON core.oauth_states TO fluentra_app;

-- ----------------------------------------------------- core.oauth_identities
--
-- A provider account linked to a Fluentra account.
--
-- Keyed on (provider, subject) and not on the email, because an address can be
-- reassigned by a provider and a subject cannot. Linking by address would mean
-- whoever inherits a corporate mailbox inherits the Fluentra account with it.
CREATE TABLE IF NOT EXISTS core.oauth_identities (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    provider   text        NOT NULL CHECK (provider IN ('google')),

    -- Google's `sub`. Stable for the lifetime of the Google account and never
    -- reused, which is exactly what an identity key has to be.
    subject    text        NOT NULL,

    -- HMAC-SHA256 of the address Google asserted, for the same reason every
    -- other address digest here is keyed: an unkeyed hash of an email is
    -- reversible with a wordlist, which would make this table an address book.
    email_hash bytea       NOT NULL,

    linked_at  timestamptz NOT NULL DEFAULT now(),

    -- One Google account is one Fluentra account. Without this a single Google
    -- identity could be attached to several accounts, and "sign in with Google"
    -- would stop identifying anybody in particular.
    CONSTRAINT uq_oauth_identities_subject UNIQUE (provider, subject),

    -- And one link per provider per account, so unlinking has exactly one row
    -- to remove and the last-sign-in-method check has one thing to count.
    CONSTRAINT uq_oauth_identities_user_provider UNIQUE (user_id, provider),

    CONSTRAINT ck_oauth_identities_email_hash_length CHECK (octet_length(email_hash) = 32)
);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_user
    ON core.oauth_identities (user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON core.oauth_identities TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS core.oauth_identities;
DROP TABLE IF EXISTS core.oauth_states;

-- +goose StatementEnd
