-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------- core.banners kinds
--
-- A banner is one of two things, chosen per banner:
--
--   announcement  a message shown once as a dialog on the dashboard; the
--                 learner closes it (for good when it is dismissible).
--   slide         an image-only promotion in the dashboard's carousel; always
--                 shown while live, never dismissed, optionally a link.
--
-- Every existing banner was written as an announcement.
ALTER TABLE core.banners
    ADD COLUMN kind text NOT NULL DEFAULT 'announcement',
    ADD CONSTRAINT ck_banners_kind CHECK (kind IN ('announcement', 'slide')),
    -- A slide is its picture.
    ADD CONSTRAINT ck_banners_slide_image CHECK (kind <> 'slide' OR image_key IS NOT NULL),
    -- A slide has no text to dismiss and no button, only an optional link.
    ADD CONSTRAINT ck_banners_slide_shape CHECK (
        kind <> 'slide' OR (body = '' AND cta_label IS NULL AND NOT dismissible));

-- An announcement's button needs both its text and its link; a slide's link
-- stands alone (the whole picture is the link).
ALTER TABLE core.banners
    DROP CONSTRAINT ck_banners_cta_pair,
    ADD CONSTRAINT ck_banners_cta_pair CHECK (
        kind = 'slide' OR (cta_label IS NULL) = (cta_url IS NULL));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Slides cannot satisfy the announcement-only pair rule; remove them first.
DELETE FROM core.banners WHERE kind = 'slide';
ALTER TABLE core.banners
    DROP CONSTRAINT IF EXISTS ck_banners_cta_pair,
    ADD CONSTRAINT ck_banners_cta_pair CHECK ((cta_label IS NULL) = (cta_url IS NULL)),
    DROP CONSTRAINT IF EXISTS ck_banners_slide_shape,
    DROP CONSTRAINT IF EXISTS ck_banners_slide_image,
    DROP CONSTRAINT IF EXISTS ck_banners_kind,
    DROP COLUMN IF EXISTS kind;
-- +goose StatementEnd
