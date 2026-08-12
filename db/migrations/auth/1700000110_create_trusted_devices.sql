-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------ core.trusted_devices
--
-- A device the learner explicitly chose to stay signed in on (BR-AUTH-24).
--
-- Trusting buys a longer idle window and nothing else. It does not extend the
-- absolute expiry and it skips no check, because the cap is what bounds a theft
-- and a learner cannot consent their way out of it on a device an attacker may
-- already be holding.
CREATE TABLE IF NOT EXISTS core.trusted_devices (
    id                  uuid        PRIMARY KEY,
    user_id             uuid        NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,

    -- HMAC-SHA256 of the client-generated identifier, keyed with the server
    -- key. Keyed rather than plain for the reason every other digest here is:
    -- a client id is low-entropy and attacker-chosen, so an unkeyed hash of one
    -- is reversible by whoever chose it.
    --
    -- This is not a fingerprint and not evidence. A learner who clears their
    -- browser storage looks like a new device and pays one sign-in, which is
    -- the correct failure direction.
    device_id_hash      bytea       NOT NULL,

    -- The coarse label the session list shows. Null when the sign-in that
    -- trusted the device had no readable user agent.
    label               text,

    -- Stored rather than recomputed, so that changing SESSION_IDLE_WINDOW_TRUSTED
    -- does not silently move the expiry of a device already trusted under the
    -- old value. A learner told "this laptop is trusted until March" should not
    -- find that date moved by a deploy.
    idle_window         interval    NOT NULL,

    -- Set once at trust time and never extended (BR-AUTH-22). There is no
    -- statement anywhere in this module that raises it, and that absence is the
    -- security argument for the idle window being ninety days.
    absolute_expires_at timestamptz NOT NULL,

    trusted_at          timestamptz NOT NULL DEFAULT now(),
    last_seen_at        timestamptz NOT NULL DEFAULT now(),
    revoked_at          timestamptz,

    CONSTRAINT ck_trusted_devices_digest_length CHECK (octet_length(device_id_hash) = 32),
    CONSTRAINT ck_trusted_devices_idle_window CHECK (idle_window > interval '0'),
    CONSTRAINT ck_trusted_devices_absolute_after_trust CHECK (absolute_expires_at > trusted_at)
);

-- One live trust per device per account. A second sign-in from the same browser
-- refreshes the existing row rather than accumulating duplicates the learner
-- would then have to untrust one at a time.
CREATE UNIQUE INDEX IF NOT EXISTS uq_trusted_devices_live
    ON core.trusted_devices (user_id, device_id_hash)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_trusted_devices_user_active
    ON core.trusted_devices (user_id, last_seen_at DESC)
    WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON core.trusted_devices TO fluentra_app;

-- ----------------------------------------------- core.sessions gains two columns
--
-- absolute_expires_at is the cap rotation may never move past. It is NOT NULL
-- because a session without one is a session that renews itself forever, which
-- is the exact state ADR-0022 rejected -- so the schema refuses to hold one
-- rather than trusting every future INSERT to remember.
ALTER TABLE core.sessions ADD COLUMN IF NOT EXISTS absolute_expires_at timestamptz;

-- Rows that predate this migration were created before there was a cap. They
-- get one measured from when they were created, so an old session is not
-- suddenly immortal and not suddenly dead either.
UPDATE core.sessions
SET absolute_expires_at = created_at + interval '180 days'
WHERE absolute_expires_at IS NULL;

ALTER TABLE core.sessions ALTER COLUMN absolute_expires_at SET NOT NULL;

-- The idle window this session renews with, stored rather than recomputed on
-- each rotation.
--
-- Recomputing would mean rotation had to know the role and the trust decision
-- again, which is a lookup per refresh for a value that was settled at sign-in.
-- Deriving it from the previous token's own lifetime would be free but wrong at
-- the edge: a token clamped to the cap would shrink the window for the next one.
-- Storing it also means changing SESSION_IDLE_WINDOW does not move the expiry of
-- a session already running under the old value.
ALTER TABLE core.sessions ADD COLUMN IF NOT EXISTS idle_window interval;
UPDATE core.sessions SET idle_window = interval '30 days' WHERE idle_window IS NULL;
ALTER TABLE core.sessions ALTER COLUMN idle_window SET NOT NULL;

-- The device a session belongs to, when it belongs to one. Null is the ordinary
-- case: most sign-ins are not on a trusted device. It exists so that untrusting
-- can revoke the sessions and refresh families of that device alone, which is
-- what a learner means by "sign this laptop out".
ALTER TABLE core.sessions ADD COLUMN IF NOT EXISTS trusted_device_id uuid
    REFERENCES core.trusted_devices (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_trusted_device
    ON core.sessions (trusted_device_id)
    WHERE trusted_device_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS core.idx_sessions_trusted_device;
ALTER TABLE core.sessions DROP COLUMN IF EXISTS trusted_device_id;
ALTER TABLE core.sessions DROP COLUMN IF EXISTS idle_window;
ALTER TABLE core.sessions DROP COLUMN IF EXISTS absolute_expires_at;
DROP TABLE IF EXISTS core.trusted_devices;

-- +goose StatementEnd
