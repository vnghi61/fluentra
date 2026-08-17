-- name: ListFeatureFlags :many
SELECT *
FROM core.feature_flags
ORDER BY key ASC;

-- name: GetFeatureFlagByKey :one
SELECT *
FROM core.feature_flags
WHERE key = $1;

-- name: CreateFeatureFlag :one
INSERT INTO core.feature_flags (
    key,
    enabled,
    rollout_percent,
    owner,
    expires_on,
    description
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: UpdateFeatureFlag :one
UPDATE core.feature_flags
SET
    enabled = $2,
    rollout_percent = $3,
    expires_on = $4,
    description = $5,
    updated_at = now()
WHERE key = $1
RETURNING *;

-- name: DeleteFeatureFlag :exec
DELETE FROM core.feature_flags
WHERE key = $1;

-- name: GetFlagsExpiringWithin :many
SELECT *
FROM core.feature_flags
WHERE expires_on <= $1
ORDER BY expires_on ASC;
