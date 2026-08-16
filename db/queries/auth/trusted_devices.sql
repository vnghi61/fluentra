-- TrustDevice records or refreshes a trusted device.
--
-- The conflict target is the partial unique index on live rows, so a second
-- sign-in from the same browser refreshes the existing trust rather than
-- accumulating duplicates the learner would have to untrust one at a time.
--
-- `absolute_expires_at` is excluded from the update on purpose. Signing in again
-- on a device you already trust moves the idle clock, not the cap — otherwise a
-- learner who uses a laptop daily would never reach the cap at all, which is
-- ADR-0022's rejected alternative C arriving through the back door.
--
-- name: TrustDevice :one
INSERT INTO core.trusted_devices (
    id, user_id, device_id_hash, label, idle_window, absolute_expires_at, trusted_at, last_seen_at
)
VALUES (
    $1, $2, $3, $4, sqlc.arg(idle_window)::interval,
    sqlc.arg(absolute_expires_at)::timestamptz, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT (user_id, device_id_hash) WHERE revoked_at IS NULL DO UPDATE
SET label = COALESCE(EXCLUDED.label, core.trusted_devices.label),
    idle_window = EXCLUDED.idle_window,
    last_seen_at = EXCLUDED.last_seen_at
RETURNING id, user_id, device_id_hash, label, idle_window, absolute_expires_at,
          trusted_at, last_seen_at, revoked_at;

-- name: ListTrustedDevices :many
SELECT id, user_id, device_id_hash, label, idle_window, absolute_expires_at,
       trusted_at, last_seen_at, revoked_at
FROM core.trusted_devices
WHERE user_id = $1 AND revoked_at IS NULL AND absolute_expires_at > sqlc.arg(now)::timestamptz
ORDER BY last_seen_at DESC;

-- GetOwnedTrustedDevice is the ownership check behind the 404.
--
-- Both the id and the owner are in the WHERE clause, so a device belonging to
-- somebody else and one that never existed produce the identical empty result.
--
-- name: GetOwnedTrustedDevice :one
SELECT id, user_id, device_id_hash, label, idle_window, absolute_expires_at,
       trusted_at, last_seen_at, revoked_at
FROM core.trusted_devices
WHERE id = $1 AND user_id = $2;

-- name: UntrustDevice :execrows
UPDATE core.trusted_devices
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE id = $1 AND revoked_at IS NULL;

-- name: UntrustAllDevicesForUser :execrows
UPDATE core.trusted_devices
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: TouchTrustedDevice :execrows
UPDATE core.trusted_devices
SET last_seen_at = sqlc.arg(now)::timestamptz
WHERE id = $1 AND revoked_at IS NULL;

-- name: DeleteTrustedDevicesForUser :exec
DELETE FROM core.trusted_devices
WHERE user_id = $1;

