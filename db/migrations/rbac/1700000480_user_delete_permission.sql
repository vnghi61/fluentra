-- +goose Up
-- +goose StatementBegin

-- Soft-deleting someone else's account is not suspension and must not ride on
-- `user.suspend`. Suspension is reversible by a second administrator in one
-- click; a soft delete starts a 30-day clock that ends in erasure, and the
-- deletion executor does not ask again before it runs. They are different
-- powers and they get different names.
INSERT INTO core.permissions (name, description) VALUES
    ('user.delete', 'Soft-delete a user account, starting the 30-day erasure grace period.')
ON CONFLICT (name) DO NOTHING;

-- `admin` holds every permission, by the set-difference pattern 1700000021 uses:
-- a permission added later is granted by the migration that adds it.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
JOIN core.permissions p ON p.name = 'user.delete'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.permissions WHERE name = 'user.delete';
-- +goose StatementEnd
