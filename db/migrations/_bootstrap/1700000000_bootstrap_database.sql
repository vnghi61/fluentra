-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS btree_gin;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fluentra_app') THEN
        CREATE ROLE fluentra_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fluentra_migrator') THEN
        CREATE ROLE fluentra_migrator NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;
    END IF;
    EXECUTE format('GRANT fluentra_migrator TO %I', session_user);
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO fluentra_app', current_database());
    -- The statements below run as fluentra_migrator, and creating a schema
    -- requires CREATE on the database. Without this grant the very first
    -- migration fails on a fresh database.
    EXECUTE format('GRANT CONNECT, CREATE ON DATABASE %I TO fluentra_migrator', current_database());

    -- Every later migration runs as fluentra_migrator (cmd/migrate does SET
    -- ROLE), and goose records each applied version in the same transaction.
    -- The bookkeeping table was created by the bootstrapping superuser, so
    -- hand it over now or every command after the first `up` is denied.
    IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version' AND relkind = 'r') THEN
        EXECUTE 'ALTER TABLE goose_db_version OWNER TO fluentra_migrator';
        IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'goose_db_version_id_seq' AND relkind = 'S') THEN
            EXECUTE 'ALTER SEQUENCE goose_db_version_id_seq OWNER TO fluentra_migrator';
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

DROP EXTENSION IF EXISTS btree_gin;
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS pg_stat_statements;
DROP EXTENSION IF EXISTS pgcrypto;
-- +goose StatementEnd
