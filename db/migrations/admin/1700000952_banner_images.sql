-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------ core.banners images
--
-- An optional image per banner. image_key names a processed JPEG the admin
-- module wrote to fluentra-media under banners/images/: never a URL (those
-- are signed per read and expire) and never the creator's raw upload. Width
-- and height are recorded so the dashboard reserves the image's space before
-- it loads instead of shifting the page.
ALTER TABLE core.banners
    ADD COLUMN image_key    text,
    ADD COLUMN image_alt    text    NOT NULL DEFAULT '',
    ADD COLUMN image_width  integer,
    ADD COLUMN image_height integer,
    ADD CONSTRAINT ck_banners_image_key CHECK (
        image_key IS NULL OR image_key ~ '^banners/images/[0-9a-f-]{36}[.]jpg$'),
    ADD CONSTRAINT ck_banners_image_alt_len CHECK (char_length(image_alt) <= 160),
    ADD CONSTRAINT ck_banners_image_size CHECK (
        (image_key IS NULL AND image_width IS NULL AND image_height IS NULL)
        OR (image_key IS NOT NULL AND image_width BETWEEN 1 AND 1600
            AND image_height BETWEEN 1 AND 900));

-- One banner per image: replacing or deleting a banner deletes its image, so
-- a second banner pointing at the same object would lose its picture.
CREATE UNIQUE INDEX uq_banners_image_key ON core.banners (image_key)
    WHERE image_key IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS core.uq_banners_image_key;
ALTER TABLE core.banners
    DROP CONSTRAINT IF EXISTS ck_banners_image_size,
    DROP CONSTRAINT IF EXISTS ck_banners_image_alt_len,
    DROP CONSTRAINT IF EXISTS ck_banners_image_key,
    DROP COLUMN IF EXISTS image_height,
    DROP COLUMN IF EXISTS image_width,
    DROP COLUMN IF EXISTS image_alt,
    DROP COLUMN IF EXISTS image_key;
-- +goose StatementEnd
