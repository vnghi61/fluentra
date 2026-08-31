-- name: ListRoleNamesByUserID :many
-- The caller's roles. Small, bounded by the number of roles in the product,
-- and read on the way to almost every authorization decision.
SELECT r.name
FROM core.user_roles ur
JOIN core.roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.name
LIMIT 100;

-- name: ListPermissionNamesByUserID :many
-- The caller's effective permissions, resolved through their roles and
-- flattened.
--
-- DISTINCT matters: two roles granting the same permission is a normal state,
-- and a duplicate here would show up twice in GET /me/permissions and be
-- counted twice by anything that measures the set.
SELECT DISTINCT p.name
FROM core.user_roles ur
JOIN core.role_permissions rp ON rp.role_id = ur.role_id
JOIN core.permissions p ON p.id = rp.permission_id
WHERE ur.user_id = $1
ORDER BY p.name
LIMIT 500;

-- name: ListRolesWithPermissions :many
-- The role catalogue for the admin screen. The permissions come back as an
-- array per role rather than as a row per pair, so the caller does not have to
-- regroup them — and so this stays one query.
SELECT r.name,
       r.description,
       COALESCE(
           array_agg(p.name ORDER BY p.name) FILTER (WHERE p.name IS NOT NULL),
           ARRAY[]::text[]
       )::text[] AS permissions
FROM core.roles r
LEFT JOIN core.role_permissions rp ON rp.role_id = r.id
LEFT JOIN core.permissions p ON p.id = rp.permission_id
GROUP BY r.name, r.description
ORDER BY r.name
LIMIT 100;

-- name: GetRoleByName :one
SELECT id, name, description, created_at, updated_at
FROM core.roles
WHERE name = $1;

-- name: AssignRole :execrows
-- Idempotent by design: granting a role the user already holds is not an
-- error, it is a no-op. The row count tells the caller which happened, so the
-- service can skip the event and the cache bust when nothing changed.
INSERT INTO core.user_roles (user_id, role_id, granted_by)
SELECT $1, r.id, sqlc.narg(granted_by)
FROM core.roles r
WHERE r.name = $2
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: RevokeRole :execrows
-- Idempotent in the same way, and for the same reason.
DELETE FROM core.user_roles ur
USING core.roles r
WHERE ur.role_id = r.id
  AND ur.user_id = $1
  AND r.name = $2;

-- name: CountUsersWithRole :one
-- The last-administrator guard (BR-RBAC-05). It is read inside the same
-- transaction as the revocation; what stops two concurrent revocations from
-- each seeing two admins is that the transaction runs at SERIALIZABLE and the
-- loser is retried against the committed state.
SELECT count(*)
FROM core.user_roles ur
JOIN core.roles r ON r.id = ur.role_id
WHERE r.name = $1;

-- name: LockRoleAssignments :many
-- Takes a row lock on every assignment of a role, so the count above and the
-- delete that follows are decided against a state nobody else can change until
-- the transaction ends.
--
-- This is a performance measure, not the correctness one: SERIALIZABLE already
-- makes the concurrent case safe by aborting and retrying the loser. The lock
-- turns that abort into a wait, which is cheaper under contention.
SELECT ur.user_id
FROM core.user_roles ur
JOIN core.roles r ON r.id = ur.role_id
WHERE r.name = $1
ORDER BY ur.user_id
LIMIT 10000
FOR UPDATE OF ur;

-- name: DeleteRoleAssignmentsForUser :execrows
-- The `user.deleted` reaction: an account that no longer exists holds no roles.
DELETE FROM core.user_roles WHERE user_id = $1;

-- name: FirstHolderOfRole :one
-- The longest-standing holder of a role.
--
-- Exists for one caller: the practice generator needs a stable owner for the
-- content it authors, `content_items.owner_id` is not nullable, and
-- unattributed content is content nobody can be asked about. Ordered by
-- assignment time so the answer does not move when a second administrator is
-- added, which would otherwise reassign every generated item.
SELECT ur.user_id
FROM core.user_roles ur
JOIN core.roles r ON r.id = ur.role_id
WHERE r.name = $1
ORDER BY ur.granted_at, ur.user_id
LIMIT 1;
