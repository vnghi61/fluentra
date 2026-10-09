-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    -- No extensions: gen_random_uuid() is core since PostgreSQL 13, the one GIN
    -- index is on jsonb (built-in operator class), and citext is created by the
    -- user module, which needs it.

    -- Roles are cluster-wide, and `IF NOT EXISTS` around CREATE ROLE is a check
    -- and an act with a gap between them. Two databases in one cluster migrating
    -- at the same time both see the role missing, both create it, and the loser
    -- fails the whole migration with
    --
    --     duplicate key value violates unique constraint "pg_authid_rolname_index"
    --
    -- That is not hypothetical: `go test -tags=integration ./...` runs packages
    -- in parallel and several of them create and migrate a database of their
    -- own, so CI failed on it intermittently with nothing to show for it in the
    -- diff. The window is small, which is what made it a flake rather than a
    -- bug somebody fixed.
    --
    -- Catching the conflict is the only race-free option: PostgreSQL has no
    -- CREATE ROLE IF NOT EXISTS. Both codes are caught because the two sessions
    -- can collide either on the name check (duplicate_object) or inside the
    -- catalogue index (unique_violation), depending on how they interleave.
    -- Extensions above are already written this way.
    BEGIN
        CREATE ROLE fluentra_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
    END;
    BEGIN
        CREATE ROLE fluentra_migrator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL;
    END;
    EXECUTE format('GRANT fluentra_migrator TO %I', session_user);
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO fluentra_app', current_database());
    -- The statements below run as fluentra_migrator, and creating a schema
    -- requires CREATE on the database. Without this grant the very first
    -- migration fails on a fresh database.
    EXECUTE format('GRANT CONNECT, CREATE ON DATABASE %I TO fluentra_migrator', current_database());

    -- Grant permissions on public schema (for goose version table and general access)
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'public') THEN
        BEGIN
            EXECUTE 'GRANT ALL ON SCHEMA public TO fluentra_migrator';
            EXECUTE 'GRANT USAGE ON SCHEMA public TO fluentra_app';
        EXCEPTION WHEN OTHERS THEN NULL;
        END;
    END IF;

    -- Grant permissions on extensions schema if present
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'extensions') THEN
        BEGIN
            EXECUTE 'GRANT USAGE ON SCHEMA extensions TO fluentra_migrator, fluentra_app';
        EXCEPTION WHEN OTHERS THEN NULL;
        END;
    END IF;

    -- Every later migration runs as fluentra_migrator (cmd/migrate does SET
    -- ROLE), and goose records each applied version in the same transaction.
    -- The bookkeeping table was created by the bootstrapping superuser, so
    -- hand it over now or every command after the first `up` is denied.
    IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version' AND relkind = 'r') THEN
        EXECUTE 'ALTER TABLE goose_db_version OWNER TO fluentra_migrator';
        IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version_id_seq' AND relkind = 'S') THEN
            EXECUTE 'ALTER SEQUENCE goose_db_version_id_seq OWNER TO fluentra_migrator';
        END IF;
        -- Supabase grants its Data API roles everything on new tables in
        -- public, which would let anyone holding the anon key rewrite migration
        -- history over REST. The application never uses the Data API.
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
            EXECUTE 'REVOKE ALL ON TABLE goose_db_version FROM anon, authenticated';
        END IF;
    END IF;
END $$;

SET LOCAL ROLE fluentra_migrator;

CREATE SCHEMA IF NOT EXISTS core AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS audit AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS content AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS learn AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS skill AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS assess AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS comm AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS billing AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS ai AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS ops AUTHORIZATION fluentra_migrator;
CREATE SCHEMA IF NOT EXISTS analytics AUTHORIZATION fluentra_migrator;

GRANT USAGE ON SCHEMA core, audit, content, learn, skill, assess, comm, billing, ai, ops, analytics TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA core GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA audit GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA content GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA learn GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA skill GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA assess GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA comm GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA billing GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA ai GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA ops GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA analytics GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA core GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA audit GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA content GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA learn GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA skill GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA assess GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA comm GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA billing GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA ai GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA ops GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;
ALTER DEFAULT PRIVILEGES FOR ROLE fluentra_migrator IN SCHEMA analytics GRANT USAGE, SELECT ON SEQUENCES TO fluentra_app;

-- SET LOCAL ROLE lasts until this transaction commits, and goose records the
-- applied version inside that same transaction. Without resetting, that write
-- happens as fluentra_migrator, which does not own goose_db_version.
RESET ROLE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS ops CASCADE;
DROP SCHEMA IF EXISTS ai CASCADE;
DROP SCHEMA IF EXISTS billing CASCADE;
DROP SCHEMA IF EXISTS comm CASCADE;
DROP SCHEMA IF EXISTS assess CASCADE;
DROP SCHEMA IF EXISTS skill CASCADE;
DROP SCHEMA IF EXISTS learn CASCADE;
DROP SCHEMA IF EXISTS content CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS core CASCADE;

RESET ROLE;
DO $$
BEGIN
    -- Hand the bookkeeping table back before dropping the role, or DROP OWNED
    -- takes goose's own version table with it.
    IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version' AND relkind = 'r') THEN
        EXECUTE format('ALTER TABLE goose_db_version OWNER TO %I', session_user);
        IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version_id_seq' AND relkind = 'S') THEN
            EXECUTE format('ALTER SEQUENCE goose_db_version_id_seq OWNER TO %I', session_user);
        END IF;
    END IF;
    EXECUTE format('REVOKE fluentra_migrator FROM %I', session_user);
END $$;
DROP OWNED BY fluentra_app;
DROP ROLE IF EXISTS fluentra_app;
DROP OWNED BY fluentra_migrator;
DROP ROLE IF EXISTS fluentra_migrator;

-- +goose StatementEnd
