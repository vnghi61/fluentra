-- name: ListBanners :many
-- Every banner for the admin screen. Bounded: the set is small by design.
SELECT *
FROM core.banners
ORDER BY priority DESC, created_at DESC
LIMIT 200;

-- name: GetBannerByID :one
SELECT *
FROM core.banners
WHERE id = $1;

-- name: ListActiveBanners :many
-- The banners showing at the given instant: switched on and inside their window.
SELECT *
FROM core.banners
WHERE is_active
  AND (starts_at IS NULL OR starts_at <= sqlc.arg(at)::timestamptz)
  AND (ends_at IS NULL OR ends_at > sqlc.arg(at)::timestamptz)
ORDER BY priority DESC, created_at DESC
LIMIT sqlc.arg(max_rows);

-- name: CreateBanner :one
INSERT INTO core.banners (
    title,
    body,
    cta_label,
    cta_url,
    tone,
    is_active,
    priority,
    dismissible,
    starts_at,
    ends_at,
    created_by,
    image_key,
    image_alt,
    image_width,
    image_height,
    kind
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: UpdateBanner :one
UPDATE core.banners
SET
    title = $2,
    body = $3,
    cta_label = $4,
    cta_url = $5,
    tone = $6,
    is_active = $7,
    priority = $8,
    dismissible = $9,
    starts_at = $10,
    ends_at = $11,
    image_key = $12,
    image_alt = $13,
    image_width = $14,
    image_height = $15,
    kind = $16,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteBanner :execrows
DELETE FROM core.banners
WHERE id = $1;
