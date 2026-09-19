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
