-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------ core.banners
--
-- Announcement banners shown at the top of the learner dashboard, managed from
-- the admin screen. Owned exclusively by the admin module (Rule L3).
--
-- The CHECKs repeat the service's rules so that a row written by any other
-- path (a seed, a hand-run fix) cannot carry a link the client would follow
-- to a script: cta_url is an internal path ("/x", never "//x" or "/\x", which
-- browsers read as protocol-relative) or an https:// URL, and never contains
-- whitespace, control characters or a backslash.
--
-- created_by has no foreign key on purpose: erasing an administrator's
-- account must not be blocked by an announcement they once wrote.
CREATE TABLE IF NOT EXISTS core.banners (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    title        text        NOT NULL,
    body         text        NOT NULL DEFAULT '',
    cta_label    text,
    cta_url      text,
    tone         text        NOT NULL DEFAULT 'info',
    is_active    boolean     NOT NULL DEFAULT false,
    priority     integer     NOT NULL DEFAULT 0,
    dismissible  boolean     NOT NULL DEFAULT true,
    starts_at    timestamptz,
    ends_at      timestamptz,
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_banners_title_len CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    CONSTRAINT ck_banners_body_len CHECK (char_length(body) <= 500),
    CONSTRAINT ck_banners_cta_label_len CHECK (
        cta_label IS NULL OR char_length(btrim(cta_label)) BETWEEN 1 AND 40),
    CONSTRAINT ck_banners_cta_pair CHECK ((cta_label IS NULL) = (cta_url IS NULL)),
    CONSTRAINT ck_banners_cta_url CHECK (
        cta_url IS NULL OR (
            char_length(cta_url) <= 2048
            AND cta_url !~ '[[:space:][:cntrl:]\\]'
            AND (cta_url ~ '^/([^/]|$)' OR cta_url ~ '^https://[^/?#]+')
        )),
    CONSTRAINT ck_banners_tone CHECK (tone IN ('info', 'success', 'warning', 'promo')),
    CONSTRAINT ck_banners_priority CHECK (priority BETWEEN 0 AND 1000),
    CONSTRAINT ck_banners_window CHECK (
        starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)
);

-- The dashboard read: switched-on banners in display order. Partial, because
-- the switched-off rows are the ones nobody but the admin list ever reads.
CREATE INDEX idx_banners_active
    ON core.banners (priority DESC, created_at DESC)
    WHERE is_active;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.banners;
-- +goose StatementEnd
