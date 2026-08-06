---
module: auth
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [credentials, sessions, refresh_tokens, mfa_secrets, verification_tokens, login_attempts, oauth_identities]
depends_on: [user, rbac, audit, mailer, cache]
depended_on_by: [admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# auth — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| JWT or opaque access tokens? | JWT (HS256 now, EdDSA when services split) | Stateless validation keeps the middleware free of a database round trip on every request; the 15-minute lifetime bounds the revocation window, and the denylist covers explicit logout |
| Where does the refresh token live? | HttpOnly cookie scoped to `/api/v1/auth` | Not reachable by JavaScript, so an XSS cannot steal it; the narrow path limits CSRF surface to one endpoint, which also carries a double-submit token |
| Argon2id or bcrypt? | Argon2id | OWASP's first recommendation; memory-hardness resists GPU cracking; parameters embedded in the hash allow raising cost without a migration |
| Store sessions in Redis or Postgres? | Postgres, cached in Redis | Sessions are durable state a user can audit and an admin can revoke; Redis is a 5-minute read cache, not the source of truth |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0007](../../../docs/adr/ADR-0007-auth-jwt-refresh-rotation.md) — JWT access + rotating refresh tokens
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we support passkeys in Phase 2, and if so do they replace or supplement TOTP?
- Should admin MFA also require a hardware key, or is TOTP sufficient for our threat model?
<!-- END GENERATED: decisions-open -->
