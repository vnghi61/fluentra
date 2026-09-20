-- name: GetCreatorProfile :one
SELECT user_id, bio, headline, payout_eligible, created_at, updated_at
FROM studio.creator_profiles
WHERE user_id = $1;

-- name: UpsertCreatorProfile :one
INSERT INTO studio.creator_profiles (user_id, bio, headline, payout_eligible, updated_at)
VALUES ($1, $2, $3, false, now())
ON CONFLICT (user_id) DO UPDATE
SET bio = EXCLUDED.bio,
    headline = EXCLUDED.headline,
    updated_at = now()
RETURNING user_id, bio, headline, payout_eligible, created_at, updated_at;

-- name: SetPayoutEligible :one
UPDATE studio.creator_profiles
SET payout_eligible = $2, updated_at = now()
WHERE user_id = $1
RETURNING user_id, bio, headline, payout_eligible, created_at, updated_at;

-- name: GetPayoutAccountByCreatorID :one
SELECT id, creator_id, bank_code, account_number, account_holder_name, is_default, created_at, updated_at
FROM studio.payout_accounts
WHERE creator_id = $1 AND is_default = true
ORDER BY updated_at DESC
LIMIT 1;

-- name: UpsertPayoutAccount :one
INSERT INTO studio.payout_accounts (creator_id, bank_code, account_number, account_holder_name, is_default, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
RETURNING id, creator_id, bank_code, account_number, account_holder_name, is_default, created_at, updated_at;

-- name: CreateCourseDraft :one
INSERT INTO studio.course_drafts (owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', $8, now())
RETURNING id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at;

-- name: GetCourseDraftByID :one
SELECT id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at
FROM studio.course_drafts
WHERE id = $1;

-- name: GetCourseDraftByOwnerAndSlug :one
SELECT id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at
FROM studio.course_drafts
WHERE owner_id = $1 AND slug = $2;

-- name: ListCourseDraftsByOwner :many
SELECT id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at
FROM studio.course_drafts
WHERE owner_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCourseDraftsByOwner :one
SELECT COUNT(*)
FROM studio.course_drafts
WHERE owner_id = $1;

-- name: UpdateCourseDraft :one
UPDATE studio.course_drafts
SET title = $2,
    slug = $3,
    description = $4,
    cefr_level = $5,
    topic_taxonomy_id = $6,
    price_vnd = $7,
    structure = $8,
    updated_at = now()
WHERE id = $1
RETURNING id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at;

-- name: UpdateCourseDraftStatus :one
UPDATE studio.course_drafts
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, owner_id, title, slug, description, cefr_level, topic_taxonomy_id, price_vnd, status, structure, created_at, updated_at;

-- name: CreateSubmission :one
INSERT INTO studio.submissions (draft_id, version, status, submitted_by, submitted_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at;

-- name: GetSubmissionByID :one
SELECT id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at
FROM studio.submissions
WHERE id = $1;

-- name: GetLatestSubmissionByDraftID :one
SELECT id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at
FROM studio.submissions
WHERE draft_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: ListSubmissionsByStatus :many
SELECT id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at
FROM studio.submissions
WHERE status = $1
ORDER BY submitted_at ASC
LIMIT $2 OFFSET $3;

-- name: CountSubmissionsByStatus :one
SELECT COUNT(*)
FROM studio.submissions
WHERE status = $1;

-- name: UpdateSubmissionVerification :one
UPDATE studio.submissions
SET status = $2,
    verification_report = $3,
    feedback = $4,
    updated_at = now()
WHERE id = $1
RETURNING id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at;

-- name: UpdateSubmissionReview :one
UPDATE studio.submissions
SET status = $2,
    reviewer_id = $3,
    feedback = $4,
    reviewed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at;

-- name: ListPendingVerificationSubmissions :many
SELECT id, draft_id, version, status, submitted_by, reviewer_id, feedback, verification_report, submitted_at, reviewed_at, created_at, updated_at
FROM studio.submissions
WHERE status IN ('submitted', 'verifying')
ORDER BY submitted_at ASC
LIMIT $1;

-- -------------------------------------------------- listings

-- name: UpsertListing :one
INSERT INTO studio.listings (course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now())
ON CONFLICT (course_id) DO UPDATE
SET pricing_model = EXCLUDED.pricing_model,
    price_vnd = EXCLUDED.price_vnd,
    revenue_share_bps = EXCLUDED.revenue_share_bps,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at;

-- name: GetListingByCourseID :one
SELECT course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at
FROM studio.listings
WHERE course_id = $1;

-- name: ListListingsByCourseIDs :many
SELECT course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at
FROM studio.listings
WHERE course_id = ANY($1::uuid[]);

-- name: UpdateListingStatus :one
UPDATE studio.listings
SET status = $2, updated_at = now()
WHERE course_id = $1
RETURNING course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at;

-- name: UpdateListingPrice :one
UPDATE studio.listings
SET pricing_model = $2, price_vnd = $3, updated_at = now()
WHERE course_id = $1
RETURNING course_id, creator_id, pricing_model, price_vnd, revenue_share_bps, status, published_at, updated_at;

-- -------------------------------------------------- purchases

-- name: CreatePurchase :one
INSERT INTO studio.purchases (user_id, course_id, order_id, price_paid_vnd, granted_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at;

-- name: GetPurchaseByID :one
SELECT id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at
FROM studio.purchases
WHERE id = $1;

-- name: GetActivePurchase :one
SELECT id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at
FROM studio.purchases
WHERE user_id = $1 AND course_id = $2 AND revoked_at IS NULL
LIMIT 1;

-- name: ListPurchasesByUserID :many
SELECT id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at
FROM studio.purchases
WHERE user_id = $1
ORDER BY granted_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPurchasesByUserID :one
SELECT COUNT(*)
FROM studio.purchases
WHERE user_id = $1;

-- name: ListActivePurchasesByUserAndCourseIDs :many
SELECT id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at
FROM studio.purchases
WHERE user_id = $1 AND course_id = ANY($2::uuid[]) AND revoked_at IS NULL;

-- name: RevokePurchase :one
UPDATE studio.purchases
SET revoked_at = now(),
    revoke_reason = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, course_id, order_id, price_paid_vnd, granted_at, revoked_at, revoke_reason, created_at, updated_at;

-- -------------------------------------------------- creator_ledger

-- name: CreateLedgerEntry :one
INSERT INTO studio.creator_ledger (creator_id, kind, amount_vnd, gross_amount_vnd, fee_amount_vnd, purchase_id, payout_id, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, creator_id, kind, amount_vnd, gross_amount_vnd, fee_amount_vnd, purchase_id, payout_id, note, created_at;

-- GetSaleLedgerEntryByPurchaseID reads the credit a purchase created, so a
-- refund reverses the split that sale was recorded with rather than whatever
-- the listing charges today (BR-STUDIO-03).
-- name: GetSaleLedgerEntryByPurchaseID :one
SELECT id, creator_id, kind, amount_vnd, gross_amount_vnd, fee_amount_vnd,
       purchase_id, payout_id, note, created_at
FROM studio.creator_ledger
WHERE purchase_id = $1 AND kind = 'sale'
ORDER BY created_at ASC
LIMIT 1;

-- name: ListLedgerEntriesByCreatorID :many
SELECT id, creator_id, kind, amount_vnd, gross_amount_vnd, fee_amount_vnd, purchase_id, payout_id, note, created_at
FROM studio.creator_ledger
WHERE creator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCreatorBalance :one
SELECT COALESCE(SUM(amount_vnd), 0)::bigint AS balance_vnd
FROM studio.creator_ledger
WHERE creator_id = $1 AND kind IN ('sale', 'refund', 'payout', 'adjustment');

-- name: GetCreatorLifetimeEarnings :one
SELECT COALESCE(SUM(amount_vnd), 0)::bigint AS lifetime_earnings_vnd
FROM studio.creator_ledger
WHERE creator_id = $1 AND kind = 'sale';

-- name: GetCreatorTotalPaidOut :one
SELECT COALESCE(SUM(ABS(amount_vnd)), 0)::bigint AS total_paid_out_vnd
FROM studio.creator_ledger
WHERE creator_id = $1 AND kind = 'payout';


