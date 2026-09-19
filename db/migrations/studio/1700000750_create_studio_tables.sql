-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS studio;

-- -------------------------------------------------- studio.creator_profiles
CREATE TABLE IF NOT EXISTS studio.creator_profiles (
    user_id         uuid        PRIMARY KEY REFERENCES core.users(id) ON DELETE CASCADE,
    bio             text        NOT NULL DEFAULT '',
    headline        text        NOT NULL DEFAULT '',
    payout_eligible bool        NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- -------------------------------------------------- studio.payout_accounts
CREATE TABLE IF NOT EXISTS studio.payout_accounts (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id          uuid        NOT NULL REFERENCES studio.creator_profiles(user_id) ON DELETE CASCADE,
    bank_code           text        NOT NULL,
    account_number      text        NOT NULL,
    account_holder_name text        NOT NULL,
    is_default          bool        NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payout_accounts_creator ON studio.payout_accounts(creator_id);

-- -------------------------------------------------- studio.course_drafts
CREATE TABLE IF NOT EXISTS studio.course_drafts (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id          uuid        NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    title             text        NOT NULL,
    slug              text        NOT NULL,
    description       text        NOT NULL DEFAULT '',
    cefr_level        text        NOT NULL DEFAULT 'B1',
    topic_taxonomy_id uuid        REFERENCES content.taxonomies(id) ON DELETE SET NULL,
    price_vnd         bigint      NOT NULL DEFAULT 0,
    status            text        NOT NULL DEFAULT 'draft',
    structure         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_course_drafts_owner_slug UNIQUE (owner_id, slug),
    CONSTRAINT ck_course_drafts_price_non_negative CHECK (price_vnd >= 0),
    CONSTRAINT ck_course_drafts_status CHECK (
        status IN ('draft', 'submitted', 'verifying', 'in_review', 'published', 'rejected', 'changes_requested')
    )
);

CREATE INDEX IF NOT EXISTS idx_course_drafts_owner ON studio.course_drafts(owner_id);
CREATE INDEX IF NOT EXISTS idx_course_drafts_status ON studio.course_drafts(status);

-- -------------------------------------------------- studio.submissions
CREATE TABLE IF NOT EXISTS studio.submissions (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id            uuid        NOT NULL REFERENCES studio.course_drafts(id) ON DELETE CASCADE,
    version             int         NOT NULL DEFAULT 1,
    status              text        NOT NULL DEFAULT 'submitted',
    submitted_by        uuid        NOT NULL REFERENCES core.users(id),
    reviewer_id         uuid        REFERENCES core.users(id),
    feedback            text,
    verification_report jsonb,
    submitted_at        timestamptz NOT NULL DEFAULT now(),
    reviewed_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_submissions_reviewer_not_author CHECK (reviewer_id IS NULL OR reviewer_id <> submitted_by),
    CONSTRAINT ck_submissions_status CHECK (
        status IN ('submitted', 'verifying', 'in_review', 'approved', 'rejected', 'changes_requested')
    )
);

CREATE INDEX IF NOT EXISTS idx_submissions_draft ON studio.submissions(draft_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON studio.submissions(status);

GRANT USAGE ON SCHEMA studio TO fluentra_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA studio TO fluentra_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS studio.submissions;
DROP TABLE IF EXISTS studio.course_drafts;
DROP TABLE IF EXISTS studio.payout_accounts;
DROP TABLE IF EXISTS studio.creator_profiles;
DROP SCHEMA IF EXISTS studio;
-- +goose StatementEnd
