-- name: CreateProfile :one
INSERT INTO core.profiles (id, user_id, display_name, country, timezone, date_of_birth)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, display_name, avatar_asset_id, country, timezone, date_of_birth,
          created_at, updated_at;

-- name: GetProfileByUserID :one
SELECT id, user_id, display_name, avatar_asset_id, country, timezone, date_of_birth,
       created_at, updated_at
FROM core.profiles
WHERE user_id = $1;

-- name: ListProfilesByUserIDs :many
-- The profile half of contract.Reader.GetManyByIDs — Summary needs the display
-- name and the avatar, so batching users without batching profiles would move
-- the N+1 rather than remove it.
SELECT id, user_id, display_name, avatar_asset_id, country, timezone, date_of_birth,
       created_at, updated_at
FROM core.profiles
WHERE user_id = ANY (@user_ids::uuid[])
ORDER BY user_id
LIMIT 1000;

-- name: UpdateProfile :one
-- Partial update: sqlc.narg means "not supplied" is NULL, and COALESCE keeps
-- the stored value. Clearing a nullable field is a separate query, so that
-- "absent" and "explicitly null" cannot collapse into the same request.
UPDATE core.profiles
SET display_name  = COALESCE(sqlc.narg(display_name), display_name),
    country       = COALESCE(sqlc.narg(country), country),
    timezone      = COALESCE(sqlc.narg(timezone), timezone),
    date_of_birth = COALESCE(sqlc.narg(date_of_birth), date_of_birth),
    updated_at    = now()
WHERE user_id = @user_id
RETURNING id, user_id, display_name, avatar_asset_id, country, timezone, date_of_birth,
          created_at, updated_at;

-- name: UpdateProfileAvatar :one
UPDATE core.profiles
SET avatar_asset_id = @avatar_asset_id,
    updated_at      = now()
WHERE user_id = @user_id
RETURNING id, user_id, display_name, avatar_asset_id, country, timezone, date_of_birth,
          created_at, updated_at;

