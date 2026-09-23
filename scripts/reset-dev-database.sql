-- Drop every schema this application owns, so `make migrate-up` rebuilds from nothing.
--
-- WHAT THIS DESTROYS
--
-- Every account, every credential, every piece of authored content, every
-- learner's progress, every review card, every generated draft, every uploaded
-- resource row, and the job queue. There is no undo and no backup taken here.
--
-- WHAT IT LEAVES ALONE
--
-- Only the thirteen schemas the bootstrap migration creates are dropped, named
-- one by one rather than discovered with a wildcard. On a managed Postgres —
-- Supabase, Neon, RDS — the platform keeps its own schemas (`auth`, `storage`,
-- `realtime`, `extensions`, `graphql`, `vault`, `public`…) in the same
-- database, and a `DROP SCHEMA` loop over everything would take the project
-- down with the app. `public` is kept for that reason; only goose's version
-- table is removed from it.
--
-- The two roles the bootstrap creates (fluentra_app, fluentra_migrator) are
-- cluster-wide and are deliberately kept: the bootstrap creates them with
-- IF NOT EXISTS, so leaving them costs nothing and dropping them would fail
-- while other databases in the cluster still grant to them.
--
-- USAGE
--
--   psql "$DB_DSN" -v i_understand=yes -f scripts/reset-dev-database.sql
--
-- The variable is required on purpose: without it the script stops before
-- touching anything. Running this against a production DSN is the mistake it
-- exists to make harder, not impossible — check the host in your DSN first.
--
-- AFTERWARDS
--
--   go run ./cmd/migrate up     -- rebuild the schema
--   go run ./cmd/seed           -- accounts, curriculum, vocabulary, gamification
--   go run ./cmd/seed -audio    -- recorded pronunciation (needs network)

\if :{?i_understand}
\else
\echo 'Refusing: re-run with  -v i_understand=yes  once you have checked the DSN.'
\quit
\endif

\set ON_ERROR_STOP on

-- Reported before the drop, so the output says what was lost rather than
-- leaving you to wonder whether the script found anything at all.
\echo 'Dropping the application schemas. Current contents:'
SELECT n.nspname AS schema, count(c.oid) AS tables
FROM pg_namespace n
LEFT JOIN pg_class c ON c.relnamespace = n.oid AND c.relkind = 'r'
WHERE n.nspname IN (
    'ai', 'analytics', 'assess', 'audit', 'billing', 'comm',
    'content', 'core', 'learn', 'ops', 'resource', 'skill', 'studio'
)
GROUP BY n.nspname
ORDER BY n.nspname;

BEGIN;

DROP SCHEMA IF EXISTS ai        CASCADE;
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS assess    CASCADE;
DROP SCHEMA IF EXISTS audit     CASCADE;
DROP SCHEMA IF EXISTS billing   CASCADE;
DROP SCHEMA IF EXISTS comm      CASCADE;
DROP SCHEMA IF EXISTS content   CASCADE;
DROP SCHEMA IF EXISTS core      CASCADE;
DROP SCHEMA IF EXISTS learn     CASCADE;
DROP SCHEMA IF EXISTS ops       CASCADE;
DROP SCHEMA IF EXISTS resource  CASCADE;
DROP SCHEMA IF EXISTS skill     CASCADE;
DROP SCHEMA IF EXISTS studio    CASCADE;

-- goose records which migrations ran. The migrate command registers no custom
-- table name or schema, so the table sits on the connection's search_path —
-- `public` on every managed Postgres. Leaving it behind is the failure mode
-- that looks like success: `migrate up` reads it, believes all 84 migrations
-- are applied, reports "no migrations to run", and hands you an empty database.
DROP TABLE IF EXISTS public.goose_db_version CASCADE;

COMMIT;

\echo 'Done. Next:  go run ./cmd/migrate up  &&  go run ./cmd/seed'
