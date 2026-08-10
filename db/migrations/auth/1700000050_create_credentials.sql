-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------- core.credentials
--
-- One password per account, or none: an account created through Google (P2.10)
-- never gets a row here, which is why this is a separate table rather than two
-- more columns on core.users. A NULL password_hash would otherwise have to mean
-- both "no password yet" and "password removed", and every read would have to
-- remember the difference.
--
-- The row is never returned by a list query and never joined into a projection
-- (auth/AGENT.md §5). It is read on exactly one path: verifying a login.
CREATE TABLE IF NOT EXISTS core.credentials (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid        NOT NULL,

    -- The full PHC string: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>. Salt and
    -- cost parameters travel with the digest, so raising the cost is a change to
    -- one constant plus rehash-on-login (BR-AUTH-01), not a migration.
    password_hash text        NOT NULL,

    -- Derived from the hash, never written by the application. The cost
    -- parameters are already in password_hash; storing them a second time as an
    -- ordinary column would let the two disagree, and the copy would be the one
    -- an operator trusted. GENERATED makes that impossible while still giving
    -- "how many accounts are still on the old parameters?" an indexable answer.
    --
    -- split_part is immutable, which is what a generated column requires.
    algo_params   text        GENERATED ALWAYS AS (split_part(password_hash, '$', 4)) STORED NOT NULL,

    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_credentials_user
        FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,

    -- 1:1 with users. This also indexes the foreign key, so a separate index on
    -- user_id would be dead weight — and user_id is the only way this table is
    -- ever looked up.
    CONSTRAINT uq_credentials_user UNIQUE (user_id),

    -- The cheapest possible guard against the worst possible bug. A plaintext
    -- password, a bcrypt digest, or an empty string reaching this column is a
    -- constraint violation rather than a silent compromise that verifies false
    -- for every learner and is discovered in production.
    CONSTRAINT ck_credentials_hash_is_argon2id CHECK (
        password_hash LIKE '$argon2id$v=%$m=%,t=%,p=%'
    )
);

-- The rehash campaign query: find the accounts still on superseded parameters.
-- Low cardinality, but the selection is the small side — once the parameters are
-- raised, the rows that have not been rehashed yet are the minority and shrink
-- with every login.
CREATE INDEX IF NOT EXISTS idx_credentials_algo_params ON core.credentials (algo_params);

GRANT SELECT, INSERT, UPDATE, DELETE ON core.credentials TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.credentials;
-- +goose StatementEnd
