-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- core.sessions
--
-- One row per sign-in. Until now `sid` was a fresh identifier per login that
-- nothing stored: it went into the token from the start (P2.4) so that giving
-- it a row later would not change the token format and invalidate everything
-- issued before the deploy. This is that row.
--
-- The session is the unit of revocation a learner recognises -- "sign this
-- laptop out" -- and the refresh family below hangs off it, so revoking one
-- revokes the other's right to renew.
CREATE TABLE IF NOT EXISTS core.sessions (
    id              uuid        PRIMARY KEY,
    user_id         uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    -- A label for the device, derived from the user agent when P2.6 builds the
    -- session list. Nothing writes it yet.
    device_label    text,

    -- HMAC-SHA256 of the address, never the address. An IP is personal data
    -- under GDPR and this table would otherwise be a movement log; the digest
    -- still answers the only question the feature asks, which is "is this the
    -- same origin as last time". Keyed for the same reason the challenge
    -- subject is: an unkeyed digest of a v4 address is reversible by brute
    -- force in seconds, there being only four billion of them.
    ip_hash         bytea,
    user_agent_hash bytea,

    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),

    -- Set once, never cleared. A revoked session is not reinstated: the learner
    -- signs in again and gets a new row, so there is no state in which a
    -- session that was once taken away starts working a second time.
    revoked_at      timestamptz,

    CONSTRAINT ck_sessions_digest_lengths CHECK (
        (ip_hash IS NULL OR octet_length(ip_hash) = 32)
        AND (user_agent_hash IS NULL OR octet_length(user_agent_hash) = 32)
    )
);

-- Partial, because every query that reads this table reads live sessions: the
-- learner's own list, and the revoke-everything sweeps in P2.7. Revoked rows
-- are kept for forensics and are dead weight in the index.
CREATE INDEX IF NOT EXISTS idx_sessions_user_active
    ON core.sessions (user_id, last_seen_at DESC)
    WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON core.sessions TO fluentra_app;

-- ------------------------------------------------------- core.refresh_tokens
--
-- A chain of single-use tokens. Each rotation spends one row and writes the
-- next with the same `family_id`, so the family is the whole history of one
-- sign-in and revoking it takes one statement.
--
-- The reuse rule (BR-AUTH-04) is why the spent rows are kept rather than
-- deleted. A deleted row and a token that never existed are indistinguishable,
-- and that difference is the entire detection: presenting a token that was
-- already spent is evidence that two parties hold it, which can only mean one
-- of them stole it.
CREATE TABLE IF NOT EXISTS core.refresh_tokens (
    id         uuid        PRIMARY KEY,

    -- Plain SHA-256 of the token, not an HMAC and not a password hash. The
    -- token is 256 bits from crypto/rand, so there is no dictionary to attack
    -- and nothing for a keyed digest or a work factor to buy -- the argument
    -- that makes an unkeyed digest wrong for an email address does not apply to
    -- a value with full entropy. What the digest does buy is that a dump of
    -- this table contains no usable credential.
    token_hash bytea       NOT NULL,

    -- Constant across a rotation chain, so reuse revokes every descendant of
    -- the stolen token and not merely the one presented.
    family_id  uuid        NOT NULL,

    session_id uuid        NOT NULL REFERENCES core.sessions (id) ON DELETE CASCADE,

    issued_at  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,

    -- The single-use marker. Claiming a row is an UPDATE guarded on this being
    -- NULL, which is what makes two concurrent refreshes with the same token
    -- resolve to exactly one winner without a lock: the second UPDATE matches
    -- no row and takes the reuse branch.
    used_at    timestamptz,

    -- Set by family revocation. Distinct from `used_at`: a spent token was
    -- exchanged legitimately, a revoked one was taken away, and the audit trail
    -- needs to say which happened to each row in the family.
    revoked_at timestamptz,

    CONSTRAINT uq_refresh_tokens_hash UNIQUE (token_hash),
    CONSTRAINT ck_refresh_tokens_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT ck_refresh_tokens_expiry_after_issue CHECK (expires_at > issued_at)
);

-- The revocation path: one statement over a family, and only the rows that are
-- still worth revoking.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family
    ON core.refresh_tokens (family_id)
    WHERE revoked_at IS NULL;

-- Unpartial, and covering the foreign key rather than any query written yet.
-- PostgreSQL does not index a referencing column automatically, so without this
-- every delete of a session sequentially scans this table to enforce the
-- cascade -- and the schema invariant test in db/migrations/user asserts exactly
-- that, which is how the omission was caught.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_session
    ON core.refresh_tokens (session_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON core.refresh_tokens TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS core.refresh_tokens;
DROP TABLE IF EXISTS core.sessions;

-- +goose StatementEnd
