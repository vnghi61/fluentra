-- +goose Up
-- +goose StatementBegin

-- Written as a bare CREATE TYPE rather than wrapped in a DO block with an
-- existence guard, for the reason the user migration gives: sqlc parses this
-- file as its schema source and cannot see inside a DO block, so a guarded enum
-- generates `interface{}` instead of typed Go constants.
--
-- The four purposes are the closed set from ADR-0021. Adding a fifth is a
-- migration, which is the point — a `purpose` that is free text would let a
-- caller invent one and silently bypass whatever limits that purpose deserves.
CREATE TYPE core.challenge_purpose AS ENUM ('verify_email', 'login_otp', 'password_reset', 'link_oauth');

-- ------------------------------------------------------ core.auth_challenges
--
-- One short-lived one-time code, for any purpose. Built once and generically
-- (ADR-0021 alternative C): three purpose-specific tables would mean three
-- implementations of constant-time comparison and attempt capping, and a bug in
-- one of them would be silent in the other two.
--
-- There is no user_id and no foreign key. The row identifies its subject only by
-- an HMAC, so the table says nothing about which account a challenge belongs to,
-- and a challenge can exist for an address that has no account yet.
CREATE TABLE IF NOT EXISTS core.auth_challenges (
    id           uuid                   PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose      core.challenge_purpose NOT NULL,

    -- HMAC-SHA256 of the normalised subject, keyed with the server key. An
    -- unkeyed digest of an email address is reversible with a wordlist, which
    -- would make this table an address book for anyone who could read it.
    subject_hash bytea                  NOT NULL,

    -- HMAC-SHA256 over the challenge id and the code together, keyed with the
    -- server key. Binding the id in is what makes "a code from challenge A does
    -- not verify challenge B" structural rather than probabilistic: with the
    -- code alone, two live challenges that happen to draw the same six digits
    -- would accept each other's code.
    code_hash    bytea                  NOT NULL,

    attempts     integer                NOT NULL DEFAULT 0,
    max_attempts integer                NOT NULL DEFAULT 5,

    -- Set once at issuance and never moved. Resend replaces the code and clears
    -- the attempts, but a learner who keeps pressing resend does not get an
    -- indefinitely valid challenge (BR-AUTH-13).
    expires_at   timestamptz            NOT NULL,
    consumed_at  timestamptz,

    -- The 60-second resend cooldown is enforced in Redis, which gives the client
    -- its `resend_after`. This column is the backstop: `cache.Limiter` allows
    -- the request when Redis is unreachable, by design, and without a second
    -- check a Redis outage would turn resend into an email flood aimed at
    -- whichever address the attacker names.
    last_sent_at timestamptz            NOT NULL DEFAULT now(),

    created_at   timestamptz            NOT NULL DEFAULT now(),
    updated_at   timestamptz            NOT NULL DEFAULT now(),

    -- The cap is a constraint, not only a WHERE clause. BR-AUTH-12 is the rule
    -- an attacker attacks; making the database refuse a sixth attempt means a
    -- missing guard in application code is a failed statement rather than an
    -- unbounded guessing budget.
    CONSTRAINT ck_auth_challenges_attempts CHECK (attempts >= 0 AND attempts <= max_attempts),
    CONSTRAINT ck_auth_challenges_max_attempts CHECK (max_attempts BETWEEN 1 AND 10),

    -- Both are SHA-256 outputs. A short value here means something wrote a
    -- truncated digest, or a plaintext code.
    CONSTRAINT ck_auth_challenges_digest_lengths CHECK (
        octet_length(code_hash) = 32 AND octet_length(subject_hash) = 32
    ),
    CONSTRAINT ck_auth_challenges_expiry_after_issue CHECK (expires_at > created_at)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON core.auth_challenges TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.auth_challenges;
DROP TYPE IF EXISTS core.challenge_purpose;
-- +goose StatementEnd
