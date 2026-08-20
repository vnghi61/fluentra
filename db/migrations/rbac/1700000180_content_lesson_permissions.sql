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

-- `content.read.published` is deliberately granted to NOBODY here.
--
-- Six module AGENT.md files name it as the permission on published-content
-- reads, but 1700000020 gave the learner role no named permissions at all and
-- TestAdminHoldsEverythingAndLearnerHoldsNothing asserts that. Granting it to
-- `user` would silently overturn a tested design decision inside a
-- contract-only task.
--
-- So the permission exists, admin holds it, and *who else holds it* is an open
-- question for P7.5 — the task that writes the guard. Until it is answered the
-- five read endpoints are authenticated (no `security: []`) and un-enforced,
-- which is the honest state rather than a guess baked into the catalogue.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- role_permissions cascades on the permission foreign key (1700000020), so
-- the grants above go with the rows they point at.
DELETE FROM core.permissions WHERE name IN ('content.read.published', 'content.create', 'content.edit', 'content.review', 'content.publish');
-- +goose StatementEnd
