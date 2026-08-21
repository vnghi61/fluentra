-- name: CreateContentReview :one
INSERT INTO content.content_reviews (id, version_id, reviewer_id, decision, comments)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, version_id, reviewer_id, decision, comments, created_at;

-- name: ListContentReviewsForVersion :many
SELECT id, version_id, reviewer_id, decision, comments, created_at
FROM content.content_reviews
WHERE version_id = $1
ORDER BY created_at DESC;

-- name: GetContentReviewByID :one
SELECT id, version_id, reviewer_id, decision, comments, created_at
FROM content.content_reviews
WHERE id = $1;
