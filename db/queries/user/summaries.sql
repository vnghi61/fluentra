-- name: ListUserSummariesByIDs :many
-- The read behind contract.Reader.GetManyByIDs.
--
-- It is one statement rather than three batched reads because the acceptance
-- criterion is a query count, and because the join is free: all three tables
-- live in `core` and belong to this module, so this is not the cross-module
-- JOIN that rule L2 forbids.
--
-- The joins are LEFT because a user row can exist for a moment before its
-- profile and preference rows do — registration writes all three in one
-- transaction, but a partially migrated account from an earlier release would
-- otherwise vanish from every list that renders it.
SELECT u.id,
       u.status,
       p.display_name,
       p.avatar_asset_id,
       p.timezone,
       pr.locale
FROM core.users u
LEFT JOIN core.profiles p ON p.user_id = u.id
LEFT JOIN core.user_preferences pr ON pr.user_id = u.id
WHERE u.id = ANY (@ids::uuid[])
ORDER BY u.id
LIMIT 1000;

-- name: GetUserSummaryByID :one
SELECT u.id,
       u.status,
       p.display_name,
       p.avatar_asset_id,
       p.timezone,
       pr.locale
FROM core.users u
LEFT JOIN core.profiles p ON p.user_id = u.id
LEFT JOIN core.user_preferences pr ON pr.user_id = u.id
WHERE u.id = $1;
