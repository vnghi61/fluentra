-- Development seed: bring one account's review cards forward so they are due.
--
-- Nothing here invents a card. The rows are the ones the learner earned by
-- finishing activities; the only thing that moves is the clock, which is what
-- "three days pass" means to a scheduler that compares due_at with now().
--
-- It exists because the review queue cannot be reached inside a test run any
-- other way. Measured against a live stack: a card answered correctly is
-- scheduled three days out, and a card answered wrong ten minutes out. Both are
-- correct FSRS behaviour and both are longer than any suite should wait, so the
-- P10.4 acceptance -- a full queue clears from the keyboard -- had no queue to
-- clear. The E2E journey drives this file through `make due-reviews`, the same
-- way the admin journey drives db/seeds/rbac.sql, so a change here breaks the
-- journey rather than silently diverging from it.
--
-- Idempotent, and safe to run before the account or its cards exist, in which
-- case it updates nothing and says so.
--
-- Usage:
--   psql "$DB_DSN" -v seed_due_email=you@example.com -f db/seeds/due_reviews.sql

\if :{?seed_due_email}
\else
\set seed_due_email 'learner@fluentra.dev'
\endif

-- The address goes through a session setting rather than being interpolated
-- into the statement, for the reason db/seeds/rbac.sql records: psql does not
-- substitute :'variables' inside a dollar-quoted body.
SELECT set_config('fluentra.seed_due_email', :'seed_due_email', false);

DO $$
DECLARE
  target_email citext := current_setting('fluentra.seed_due_email')::citext;
  target_id    uuid;
  moved        integer;
BEGIN
  SELECT id INTO target_id FROM core.users WHERE email = target_email;

  IF target_id IS NULL THEN
    RAISE NOTICE 'no account with address %, nothing to do', target_email;
    RETURN;
  END IF;

  -- A minute in the past, not now(): due_at = now() is a race against the
  -- request that reads it back.
  UPDATE learn.review_cards
  SET due_at = now() - interval '1 minute', updated_at = now()
  WHERE user_id = target_id
    AND suspended_at IS NULL
    AND due_at > now();

  GET DIAGNOSTICS moved = ROW_COUNT;
  RAISE NOTICE 'brought % review card(s) forward for %', moved, target_email;
END
$$;
