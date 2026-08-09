-- +goose Up
-- +goose StatementBegin

-- ------------------------------------------------------------- core.roles
--
-- Exactly two rows, forever, until an ADR says otherwise (BR-RBAC-02). The
-- table exists anyway because named permissions are what make a third role a
-- data change rather than a hunt through the codebase for `if role == "admin"`
-- (ADR-0008).
CREATE TABLE IF NOT EXISTS core.roles (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL,
    description text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_roles_name UNIQUE (name),
    CONSTRAINT ck_roles_name_format CHECK (name ~ '^[a-z][a-z0-9_]*$')
);

-- ------------------------------------------------------- core.permissions
--
-- Named capabilities, `<resource>.<action>[.<qualifier>]` (BR-RBAC-03). There
-- are no negative permissions: a grant is additive and its absence is the
-- denial, which is what makes deny-by-default expressible at all.
CREATE TABLE IF NOT EXISTS core.permissions (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL,
    description text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_permissions_name UNIQUE (name),
    CONSTRAINT ck_permissions_name_format CHECK (
        name ~ '^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)?$'
    )
);

-- -------------------------------------------------- core.role_permissions
CREATE TABLE IF NOT EXISTS core.role_permissions (
    role_id       uuid        NOT NULL,
    permission_id uuid        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_role_permissions PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_role_permissions_role
        FOREIGN KEY (role_id) REFERENCES core.roles (id) ON DELETE CASCADE,
    CONSTRAINT fk_role_permissions_permission
        FOREIGN KEY (permission_id) REFERENCES core.permissions (id) ON DELETE CASCADE
);

-- The composite primary key indexes (role_id, permission_id), which serves the
-- role_id foreign key as a prefix. permission_id has no such prefix, so it
-- gets its own index: without it, deleting a permission scans the table.
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission
    ON core.role_permissions (permission_id);

-- ------------------------------------------------------- core.user_roles
CREATE TABLE IF NOT EXISTS core.user_roles (
    user_id    uuid        NOT NULL,
    role_id    uuid        NOT NULL,
    -- Nullable because the seeded first administrator is granted by the
    -- system, not by a person, and because an admin who is later deleted must
    -- not take the record of what they did with them.
    granted_by uuid,
    granted_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_user_roles PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user_roles_user
        FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_roles_role
        FOREIGN KEY (role_id) REFERENCES core.roles (id) ON DELETE RESTRICT,
    CONSTRAINT fk_user_roles_granted_by
        FOREIGN KEY (granted_by) REFERENCES core.users (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_user_roles_role ON core.user_roles (role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_granted_by ON core.user_roles (granted_by);

-- ------------------------------------------------------- reference data
--
-- Roles and permissions are seeded here, in the migration, and not in
-- db/seeds/rbac.sql. They are not sample data: authorization is deny-by-default
-- (BR-RBAC-01), so a database with an empty permission catalogue is one where
-- every administrative operation is refused and nobody can grant themselves the
-- ability to fix it. A production deployment that ran migrations but not seeds
-- would be unadministrable.
--
-- db/seeds/rbac.sql does the part that really is development data: giving a
-- local account the admin role.
--
-- ON CONFLICT DO NOTHING throughout, so re-running is harmless and so a
-- permission added by a later migration is not clobbered by this one.
INSERT INTO core.roles (name, description) VALUES
    ('admin', 'Full administrative access to every back-office operation.'),
    ('user',  'A learner. Holds no named permissions; access to their own data is what /me means.')
ON CONFLICT (name) DO NOTHING;

-- The Phase 1 permission set, taken from the `Permission` column of every
-- module AGENT.md §6 that ships in this phase. Permissions for operations that
-- do not exist yet are included deliberately: the catalogue is data, and a
-- module arriving in a later card should be able to call Require() without a
-- migration of its own.
INSERT INTO core.permissions (name, description) VALUES
    ('rbac.read',         'List roles and the permissions they grant.'),
    ('rbac.assign',       'Grant and revoke roles.'),
    ('user.list',         'Search and list user accounts.'),
    ('user.read',         'Read one user account, including personal data.'),
    ('user.suspend',      'Suspend and reinstate a user account.'),
    ('user.impersonate',  'Act as another user for support purposes.'),
    ('audit.read',        'Search the audit log.'),
    ('audit.export',      'Export audit records.'),
    ('audit.manage',      'Manage audit retention and partitions.'),
    ('admin.dashboard',   'View the administrative dashboard.'),
    ('moderation.read',   'View the moderation queue.'),
    ('moderation.act',    'Act on a moderation item.'),
    ('system.flags',      'Read and change feature flags.'),
    ('system.jobs',       'Inspect and retry background jobs.')
ON CONFLICT (name) DO NOTHING;

-- `admin` holds every permission. This is a set difference rather than a list,
-- so a permission added later is granted by the migration that adds it having
-- to say nothing here — and so this statement cannot drift out of step with
-- the catalogue above.
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM core.roles r
CROSS JOIN core.permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- `user` deliberately holds none. Reading your own profile is not a named
-- permission; it is the meaning of the /me routes, which take no user id.

GRANT SELECT, INSERT, UPDATE, DELETE
    ON core.roles, core.permissions, core.role_permissions, core.user_roles
    TO fluentra_app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS core.user_roles;
DROP TABLE IF EXISTS core.role_permissions;
DROP TABLE IF EXISTS core.permissions;
DROP TABLE IF EXISTS core.roles;
-- +goose StatementEnd
