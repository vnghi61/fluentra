-- +goose Up
-- +goose StatementBegin

-- Give `user` to every account that predates the code which grants it.
--
-- rbac.Service.GrantBaselineRole is called from user.CreateUser and from
-- `make seed`, and only from there. It grants on *creation*, so it fixed the
-- accounts made after it shipped and did nothing for the ones already in the
-- table — which is every real account on the deployed system. Those rows have
-- no entry in core.user_roles at all.
--
-- That was harmless while `user` held no permissions: an account with no roles
-- and an account with `user` could do exactly the same things. 1700000180 gave
-- the role its first permission, content.read.published, and from that point a
-- learner who registered earlier is refused the published catalogue —
-- GET /courses answers 403, and the Learn page is a wall. The access token is
-- no help: it claims `user` because HighestRole of an empty set is `user`, so
-- the token says one thing and the guard, which reads core.user_roles, reads
-- another.
--
-- granted_by NULL: the system made this grant, the same record
-- GrantBaselineRole and db/seeds/rbac.sql write for their own. BR-RBAC-04 holds
-- — nobody is granting themselves anything, and `user` is not a role anyone
-- could escalate to.
--
-- Accounts that already hold *any* role are left alone. An administrator holds
-- `admin`, and HighestRole already resolves that; adding `user` beside it would
-- change no permission and only make the audit picture noisier. Deleted
-- accounts are skipped because they cannot authenticate and the row would be
-- inert.
--
-- Idempotent by the NOT EXISTS and by the primary key: re-running writes
-- nothing.
INSERT INTO core.user_roles (user_id, role_id, granted_by)
SELECT u.id, r.id, NULL
FROM core.users u
CROSS JOIN core.roles r
WHERE r.name = 'user'
  AND u.status <> 'deleted'
  AND NOT EXISTS (
      SELECT 1 FROM core.user_roles ur WHERE ur.user_id = u.id
  )
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Deliberately not reversible.
--
-- The grants above are indistinguishable from the ones GrantBaselineRole makes
-- for every account created since — same role, same NULL actor. Deleting "the
-- ones this migration made" would delete those too and lock every learner out
-- of the catalogue a second time. Rolling this migration back leaves the rows
-- in place, which is the safe direction.
SELECT 1;

-- +goose StatementEnd
