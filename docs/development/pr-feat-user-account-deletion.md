# feat(user): GDPR account deletion with 30-day grace and event-driven purge

## Summary

Implements WP3 P3.3 Account Deletion (GDPR Article 17 / Right to Erasure):

- `DELETE /me`: Request deletion with 30-day grace period (202 Accepted)
- `POST /me/deletion/cancel`: Cancel pending deletion before deadline
- `GET /me/deletion/{id}`: Check deletion status
- **Event-driven architecture**: `user.deleted` event triggers purge across all modules
- **Immediate session revocation**: `user.deletion_requested` revokes all sessions before grace period
- **Cron job** (`user_deletion_executor`): Processes due deletions daily (advisory lock `1_700_000_050`)
- **Anonymisation, not deletion**: User row kept with `deleted-{id}@anonymised.invalid` to preserve aggregate statistics
- **Idempotent purge handlers**: Auth, RBAC modules subscribe to `user.deleted`

## Architecture & Design Decisions

### Event-Driven Deletion (NOT CASCADE)

**Decision:** Use event-based purge instead of database CASCADE.

**Rationale:**

- Rule L4: No transaction spans two modules
- Rule L3: Each module owns its own tables
- GDPR compliance: Need explicit verification that each module purged its data
- Future extensibility: New modules can subscribe without changing core deletion logic

**Alternative considered and rejected:**

```sql
-- REJECTED: CASCADE DELETE
ALTER TABLE core.users ADD ON DELETE CASCADE;
```

Why rejected: Violates module ownership, impossible to verify completeness, tight coupling.

### Anonymisation vs Hard Delete

**Decision:** Keep user row, anonymise email to `deleted-{uuid}@anonymised.invalid`.

**Rationale:**

- Aggregate statistics ("total users") remain valid
- Foreign keys from audit logs don't break
- Historical data integrity preserved
- GDPR compliant: PII removed, non-identifiable data retained

**Alternative considered:** Hard-delete user row.
Why rejected: Breaks audit trail, loses aggregate counts, violates referential integrity.

### Two-Event Pattern

**Decision:** Publish TWO events:

1. `user.deletion_requested` — immediate session revocation
2. `user.deleted` — 30 days later, cross-module purge

**Rationale:**

- Security: Account must be unusable immediately, not after 30 days
- Grace period: Learner has 30 days to cancel
- Clear separation: revocation vs purge are different concerns

## Cross-Module Integration

### Auth Module

**Subscribes to `user.deletion_requested`:**

```go
// auth/module.go
func (m *Module) handleDeletionRequested(ctx context.Context, msg eventbus.Message) error {
    // Revoke ALL sessions immediately
    _, err := m.revoker.RevokeAll(ctx, payload.UserID)
    return err
}
```

**Subscribes to `user.deleted`:**

```go
// auth/module.go
func (m *Module) handleUserDeleted(ctx context.Context, msg eventbus.Message) error {
    // Purge all auth data
    _, _ = m.revoker.RevokeAll(ctx, payload.UserID)
    return m.repo.PurgeUser(ctx, payload.UserID)
}
```

**Data purged:**

- Sessions (all devices)
- Refresh tokens
- Credentials (password hash)
- Trusted devices
- OAuth identities (Google links)
- OTP challenges

### RBAC Module

**Subscribes to `user.deleted`:**

```go
// rbac/module.go
func (m *Module) handleUserDeleted(ctx context.Context, msg eventbus.Message) error {
    return m.ForgetUser(ctx, payload.UserID)
}
```

**Data purged:**

- Role assignments (`DELETE FROM rbac.user_roles WHERE user_id = $1`)

### User Module

**Anonymisation performed by job:**

- Email → `deleted-{uuid}@anonymised.invalid`
- Display name → "Deleted User"
- Avatar → deleted from S3/MinIO storage
- Preferences → hard-deleted
- Learning profile → hard-deleted
- Status → `deleted`

## Database Schema

**Migration:** `db/migrations/user/1700000150_user_deletions.sql`

**Key design:**

- `UNIQUE (user_id) WHERE status IN ('pending', 'processing')` prevents duplicate requests
- Partial index on `(status, execute_at)` for efficient cron query
- NO CASCADE DELETE on foreign key

## Verification & Quality Gates

All checks passed:

```bash
# Tests
go test ./internal/modules/user/...
go test -tags=integration ./internal/modules/user/...

# Linting
golangci-lint run ./...
golangci-lint run --build-tags=integration ./...

# Architecture
go run github.com/fe3dback/go-arch-lint@latest check

# Documentation
node tools/docgen/check-drift.mjs
npx markdownlint-cli2 "**/*.md" "#node_modules"
```

## Acceptance Criteria Verification

### Cancellable before deadline, irreversible after

**Test:** `TestModule_DeletionLifecycle` in `internal/modules/user/deletion_integration_test.go`

```go
cancelRes := h.authReq(t, http.MethodPost, "/deletion/cancel", "")
// Returns 200 OK before execution, status restored to active
// After execution, returns 409 Conflict (cannot cancel)
```

### Sessions die immediately on request

**Test:** `TestModule_SessionsRevokedImmediatelyOnDeletionRequest`

- Verified `user.deletion_requested` triggers `RevokeAll` on active sessions and refresh tokens.

### Every module has idempotent purge handler

- Auth module: `DELETE FROM core.sessions`, `core.refresh_tokens`, `core.credentials`, `core.trusted_devices`, `core.oauth_identities` are all safe to run multiple times.
- RBAC module: `DELETE FROM rbac.user_roles WHERE user_id = $1` safe to run multiple times.

### Aggregate statistics survive anonymisation

**Test:** `TestModule_DeletionLifecycle`

- Row count in `core.users` remains identical before and after deletion execution.
- User record email updated to `deleted-{id}@anonymised.invalid` with status `deleted`.

## Configuration

No new config keys required. Reuses:

- `S3_BUCKET_AVATARS` (from P3.1)
- Database connection
- Cron scheduler (advisory lock: `1_700_000_050`)

## References

- Phase-1 Plan: `docs/development/phase-1-plan.md` §6 P3.3
- Handoff: `docs/development/HANDOFF-WP3.md` §5
- GDPR Article 17: Right to Erasure
- Architecture Rules: `/AGENT.md` L3, L4
