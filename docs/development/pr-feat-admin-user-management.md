# feat(admin): add user management endpoints

## Summary

Implements WP4 P4.1 — Admin User Management (plus the P4.2 feature-flag CRUD it
shares a module with):

- `GET /api/v1/admin/users` — search users (cursor pagination, email prefix / display name / status / created range)
- `GET /api/v1/admin/users/{id}` — read one user (audited)
- `POST /api/v1/admin/users/{id}/suspend` — suspend + revoke every session
- `POST /api/v1/admin/users/{id}/reinstate` — reinstate a suspended account
- `POST /api/v1/admin/users/{id}/sessions/revoke` — revoke all sessions
- `GET/POST/PUT/DELETE /api/v1/admin/flags` — feature flag CRUD (P4.2)

Every admin action is audited with the acting admin and a required reason;
self-administration is refused with 403; every route is gated by the `admin`
role at the group and a named permission at the handler.

## Architecture & Design Decisions

### Contracts only, no cross-module transactions (Rules L3/L4/L12)

`admin` owns `core.admin_actions` and `core.feature_flags`, and nothing else. It
reaches user state through `user/contract.AdminReader`/`AdminManager`, session
revocation through `auth/contract.SessionRevoker`, and the trail through
`audit/contract.Recorder`. Each module writes its own transaction, so a
suspension is three separate transactions — admin action log, user status
change, session revocation — never one spanning them.

### Access-token revocation window — **Option A (accepted)**

`auth.SessionRevoker.RevokeAll` revokes sessions and refresh tokens, but **not**
already-issued access tokens; an access token stays valid for the remainder of
its ≤15-minute lifetime (ADR-0007's accepted trade). This PR therefore asserts
what is actually guaranteed rather than an instant lockout:

- suspension changes `core.users.status` to `suspended` (login and refresh are
  refused immediately by the existing `ACCOUNT_SUSPENDED` path),
- every session row is revoked immediately,
- the `admin_actions` log records the suspension.

Option B (denylisting every access token on the suspend path) was considered and
rejected: it would add a datastore read to the suspension path for a guarantee
ADR-0007 already decided the platform does not need, and no other module makes
that trade.

### Permission split (`user.reinstate`, `user.manage_sessions`)

The base rbac catalogue folded "suspend and reinstate" into one `user.suspend`
permission. The OpenAPI declares separate `user.reinstate` and
`user.manage_sessions` permissions, so a new migration
(`db/migrations/rbac/1700000021_admin_user_management_permissions.sql`) adds both
and grants them to `admin`, and `rbac/contract` gains the matching constants.
This lets an administrator be granted reinstate without suspend, or session
revocation without either.

## Database Schema

- `db/migrations/admin/1700000160_admin_actions.sql` — `core.admin_actions`, with
  `CHECK (actor_id != target_id)` and `CHECK (char_length(reason) >= 10)`.
- `db/migrations/admin/1700000170_feature_flags.sql` — `core.feature_flags`.
- `db/migrations/rbac/1700000021_admin_user_management_permissions.sql` — the two
  new permissions.

## Verification

```bash
# Unit tests
go test ./internal/modules/admin/... ./internal/modules/rbac/...

# Integration tests (real PostgreSQL)
TEST_DATABASE_URL=... go test -tags=integration ./internal/modules/admin/... -v

# Lint (both tag sets) + architecture
golangci-lint run ./internal/modules/admin/...
golangci-lint run --build-tags=integration ./internal/modules/admin/...
go-arch-lint check
```

All three integration tests pass against a real database:

- `TestModule_AdminSuspendUser_SuspendsAndRevokesSessions` — status → `suspended`,
  the session row is revoked, one `admin_actions` row is written.
- `TestModule_AdminCannotSuspendSelf` — self-suspension is 403, status stays `active`.
- `TestModule_NonAdminCannotAccessAdminEndpoints` — a learner is refused 403 on
  every `/admin/*` route.

## References

- Phase-1 plan: `docs/development/phase-1-plan.md` §7 P4.1
- Architecture rules: `/AGENT.md` L3, L4, L12; ADR-0007
