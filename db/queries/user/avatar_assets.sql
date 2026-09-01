-- name: InsertAvatarAsset :exec
INSERT INTO core.avatar_assets (asset_id, variant, user_id, object_key, mime_type, byte_size)
VALUES (@asset_id, @variant, @user_id, @object_key, @mime_type, @byte_size);

-- name: GetAvatarAsset :one
SELECT asset_id, variant, user_id, object_key, mime_type, byte_size, created_at
FROM core.avatar_assets
WHERE asset_id = @asset_id
  AND variant = @variant;

-- name: DeleteAvatarAssetsByAssetID :exec
DELETE FROM core.avatar_assets
WHERE asset_id = @asset_id;
