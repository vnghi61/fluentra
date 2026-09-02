-- +goose Up
-- +goose StatementBegin

-- Remember where an avatar actually is.
--
-- `core.profiles.avatar_asset_id` has always been a uuid minted by
-- `s.newID(ctx)` in storeProcessedAvatar and written nowhere else. It is not a
-- foreign key: no row anywhere has that id. Meanwhile the object key -- the one
-- thing that can locate the bytes --
--
--     users/{userID}/{YYYY}/{MM}/{assetID}_{variant}.jpg
--
-- was computed, used for the PUT, handed to commitAvatarUpdate for its
-- cleanup-on-failure path, and then dropped.
--
-- So the upload worked, the bytes reached R2, and `GET /me` answered with
-- `avatar_url: /api/v1/storage/avatars/{assetID}` -- a URL nothing could ever
-- serve, because the `{YYYY}/{MM}` segment existed only in a local variable
-- during the request that wrote the file. `profiles.updated_at` is not a
-- substitute: changing a display name moves it.
--
-- The key is data. It lives in a table.
--
-- Three rows per avatar, because storeProcessedAvatar writes three variants
-- (64, 128 and 256 px) and each is a separate object. The asset id is shared;
-- the variant is what distinguishes them, so the key is composite rather than a
-- surrogate id that nothing would ever reference.
CREATE TABLE IF NOT EXISTS core.avatar_assets (
    asset_id   uuid        NOT NULL,
    variant    text        NOT NULL,
    user_id    uuid        NOT NULL REFERENCES core.users (id) ON DELETE CASCADE,
    object_key text        NOT NULL,
    mime_type  text        NOT NULL,
    byte_size  bigint      NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (asset_id, variant),

    -- The suffixes AvatarSizes declares. A fourth size is a migration, which is
    -- the point: the renderer must not be handed a variant it cannot ask for.
    CONSTRAINT ck_avatar_assets_variant CHECK (variant IN ('sm', 'md', 'lg')),
    CONSTRAINT ck_avatar_assets_byte_size CHECK (byte_size > 0),
    -- One object, one row. Re-confirming the same upload must collide rather
    -- than leave two rows disagreeing about who owns the file.
    CONSTRAINT uq_avatar_assets_object_key UNIQUE (object_key)
);

-- The serving endpoint reads by (asset_id, variant), which the primary key
-- covers. This one is for deletion and for finding what a user owns.
CREATE INDEX IF NOT EXISTS idx_avatar_assets_user ON core.avatar_assets (user_id);

-- Every avatar_asset_id that already exists is unreachable and always will be.
--
-- Its rows cannot be backfilled: the key needs the year and month of the
-- original upload, and that was never written down. Leaving the id in place
-- would mean `GET /me` keeps returning an avatar_url that 404s on every page
-- load, and the browser renders a broken image rather than the initials
-- fallback the UI already has.
--
-- So the dangling reference is cleared. The objects themselves stay in R2 --
-- this deletes a pointer, not a file -- and the learner re-uploads. That is a
-- worse outcome than a backfill and a better one than a permanently broken
-- image, and there is no third option available to a migration, which cannot
-- reach the bucket to go looking.
UPDATE core.profiles
SET avatar_asset_id = NULL,
    updated_at      = now()
WHERE avatar_asset_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS core.avatar_assets;

-- +goose StatementEnd
