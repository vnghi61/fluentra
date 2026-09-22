-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------- core.permissions
-- Phase 3 (WO-19 Stage F): Question Bank permissions.
--
-- questionbank.read: View question bank items and difficulty statistics.
-- questionbank.create: Generate and author question bank items.
-- questionbank.review: Review and approve question bank drafts.
INSERT INTO core.permissions (name, description) VALUES
    ('questionbank.read',   'View question bank items and statistics.'),
    ('questionbank.create', 'Generate and author question bank items.'),
    ('questionbank.review', 'Review and approve question bank items.')
ON CONFLICT (name) DO NOTHING;

-- Admin holds all permissions.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
CROSS JOIN core.permissions p
WHERE r.name = 'admin'
  AND p.name IN ('questionbank.read', 'questionbank.create', 'questionbank.review')
ON CONFLICT DO NOTHING;

-- Moderator holds read and review for the question bank.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
CROSS JOIN core.permissions p
WHERE r.name = 'moderator'
  AND p.name IN ('questionbank.read', 'questionbank.review')
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.role_permissions
WHERE permission_id IN (
    SELECT id FROM core.permissions WHERE name IN ('questionbank.read', 'questionbank.create', 'questionbank.review')
);

DELETE FROM core.permissions WHERE name IN ('questionbank.read', 'questionbank.create', 'questionbank.review');
-- +goose StatementEnd
