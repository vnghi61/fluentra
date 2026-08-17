-- +goose Up
-- +goose StatementBegin

-- P4.1 splits the two admin operations that the base catalogue folded into
-- `user.suspend`. Reinstate and session revocation each get their own named
-- permission so an administrator can be granted one without the other, which
-- is what the OpenAPI spec declares (x-permission: user.reinstate /
-- user.manage_sessions) and what the admin handlers call Require() with.
INSERT INTO core.permissions (name, description) VALUES
    ('user.reinstate',       'Reinstate a suspended user account.'),
    ('user.manage_sessions', 'Revoke all sessions of a user account.')
ON CONFLICT (name) DO NOTHING;

-- `admin` holds every permission, by the same set-difference pattern the base
-- migration uses: a permission added later is granted by the migration that
-- adds it saying nothing else here.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
JOIN core.permissions p
  ON p.name IN ('user.reinstate', 'user.manage_sessions')
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.permissions WHERE name IN ('user.reinstate', 'user.manage_sessions');
-- +goose StatementEnd
