-- +goose Up
-- +goose StatementBegin

-- Phase 2 (P7.x) content authoring and lesson administration. The four content
-- permissions back the OpenAPI x-permission declarations on the /admin/content/*
-- and /admin/courses, /admin/lessons/* endpoints: creating a draft, editing a
-- draft, reviewing a submitted version, and publishing/archiving. Splitting
-- create from publish lets an editor hold review without being able to ship.
INSERT INTO core.permissions (name, description) VALUES
    ('content.read.published', 'Read published courses, lessons and content versions.'),
    ('content.create',  'Create a draft content item or course.'),
    ('content.edit',    'Edit a working draft or reorder lesson activities.'),
    ('content.review',  'Approve or request changes on a submitted content version.'),
    ('content.publish', 'Publish or archive a content version.')
ON CONFLICT (name) DO NOTHING;

-- `admin` holds every permission, by the same set-difference pattern the base
-- migration uses: a permission added later is granted by the migration that
-- adds it saying nothing else here.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
JOIN core.permissions p
  ON p.name IN ('content.read.published', 'content.create', 'content.edit',
                'content.review', 'content.publish')
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- `user` gets the read permission and only the read permission: a signed-in
-- learner may see published material, and every create, edit, review, publish
-- and archive stays with `admin`.
--
-- This is the first named permission the learner role holds. 1700000020 said it
-- holds none, and that was right while every named permission was a back-office
-- one and `self` described a learner completely. Published learning material is
-- the first thing a learner reads that is not their own data, so `self` cannot
-- express it — and leaving the endpoints open instead would hand the whole
-- course to anyone with the URL and remove the surface Phase 4 attaches
-- entitlements to.
--
-- The split is the point: read is a grant, write is not. `content.read.published`
-- is the only permission that may ever appear on this role without a decision,
-- and TestAdminHoldsEverythingAndLearnerReadsOnly enforces exactly that.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
JOIN core.permissions p ON p.name = 'content.read.published'
WHERE r.name = 'user'
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- role_permissions cascades on the permission foreign key (1700000020), so
-- the grants above go with the rows they point at.
DELETE FROM core.permissions WHERE name IN ('content.read.published', 'content.create', 'content.edit', 'content.review', 'content.publish');
-- +goose StatementEnd
