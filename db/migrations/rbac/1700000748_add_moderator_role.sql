-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- core.roles
-- Phase 3 (WO-15): Content moderator role.
-- Content reviewer with permission to review and publish creator courses.
INSERT INTO core.roles (name, description) VALUES
    ('moderator', 'Content reviewer with permission to review and publish creator courses.')
ON CONFLICT (name) DO NOTHING;

-- Grant permissions to moderator:
-- content.review, content.publish, moderation.read, moderation.act
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
CROSS JOIN core.permissions p
WHERE r.name = 'moderator'
  AND p.name IN (
    'content.review',
    'content.publish',
    'moderation.read',
    'moderation.act'
  )
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM core.role_permissions
WHERE role_id IN (SELECT id FROM core.roles WHERE name = 'moderator');

DELETE FROM core.roles WHERE name = 'moderator';
-- +goose StatementEnd
