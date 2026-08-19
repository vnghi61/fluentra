-- Development seed: give a local account the admin role.
--
-- The roles, the permission catalogue and the admin role-permission mapping are
-- NOT here — they are in db/migrations/rbac/1700000020_create_rbac_tables.sql,
-- because authorization is deny-by-default and a database with an empty
-- catalogue is one nobody can administer. Reference data the application cannot
-- run without belongs in a migration; this file is the part that is genuinely
-- development convenience.
--
-- Idempotent: safe to run repeatedly, and safe to run before the account
-- exists, in which case it does nothing and says so.
--
-- Usage:
--   psql "$DB_DSN" -v seed_admin_email=you@example.com -f db/seeds/rbac.sql
--
-- The email defaults to the address `make seed` uses for its development
-- learner. Pass -v to promote a different one.

\if :{?seed_admin_email}
\else
\set seed_admin_email 'admin@fluentra.local'
\endif

-- The address is handed to the block through a session setting rather than
-- interpolated into it. psql does not substitute :'variables' inside a
-- dollar-quoted body, so the original `citext := :'seed_admin_email'` was a
-- syntax error every time this file was run — which is to say it had never been
-- run. set_config is an ordinary statement, so the substitution happens here,
-- outside the quoting, and the block reads the value back.
SELECT set_config('fluentra.seed_admin_email', :'seed_admin_email', false);

DO $$
DECLARE
    target_email  citext := current_setting('fluentra.seed_admin_email')::citext;
    target_user   uuid;
    admin_role    uuid;
BEGIN
    SELECT id INTO target_user FROM core.users WHERE email = target_email;
    IF target_user IS NULL THEN
        RAISE NOTICE 'no account for %, nothing granted; register first, then re-run', target_email;
        RETURN;
    END IF;

    SELECT id INTO admin_role FROM core.roles WHERE name = 'admin';
    IF admin_role IS NULL THEN
        RAISE EXCEPTION 'the admin role is missing; run migrations before seeding';
    END IF;

    -- granted_by stays NULL: this grant was made by the system, not a person,
    -- and recording a person who did not do it would be worse than recording
    -- nobody.
    INSERT INTO core.user_roles (user_id, role_id, granted_by)
    VALUES (target_user, admin_role, NULL)
    ON CONFLICT (user_id, role_id) DO NOTHING;

    RAISE NOTICE 'granted admin to %', target_email;
END $$;
