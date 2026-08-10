-- +goose Up
-- +goose StatementBegin

-- P2.1b built core.auth_challenges with no user_id, on the argument that a
-- challenge should say nothing about which account it belongs to. That argument
-- does not survive contact with the operation the table exists for.
--
-- Verification has to mark an address proved, which needs an account, and the
-- only other thing on the row is a keyed HMAC of the subject — irreversible by
-- design, so it can identify nothing. Recovering the account without this column
-- means storing the address in plaintext somewhere else, which is worse.
--
-- The privacy the omission bought was close to nil in any case: subject_hash is
-- keyed, so the row already reveals no address, and anybody who can read this
-- table can read core.users beside it.
--
-- Nullable, because a challenge can legitimately precede an account: link_oauth
-- runs before a Google sign-up has one (P2.10).
ALTER TABLE core.auth_challenges
    ADD COLUMN IF NOT EXISTS user_id uuid;

ALTER TABLE core.auth_challenges
    ADD CONSTRAINT fk_auth_challenges_user
    FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE;

-- The foreign key needs an index of its own: without one, deleting an account
-- scans this table, and the seven-day sweep of unverified accounts deletes in
-- batches. Partial, because a challenge with no account is the uncommon case and
-- nothing ever queries for those.
CREATE INDEX IF NOT EXISTS idx_auth_challenges_user
    ON core.auth_challenges (user_id)
    WHERE user_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS core.idx_auth_challenges_user;
ALTER TABLE core.auth_challenges DROP CONSTRAINT IF EXISTS fk_auth_challenges_user;
ALTER TABLE core.auth_challenges DROP COLUMN IF EXISTS user_id;
-- +goose StatementEnd
