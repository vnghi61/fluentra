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
| Verification by emailed link or by OTP code? | OTP code | A majority of learners open email on a phone. A link launches a second browser context with no session, which breaks the flow and loses the registration. A 6-digit code is typed into the tab already open. See ADR-0021 |
| One challenge table or a table per purpose? | One, with a `purpose` enum | Rate limiting, attempt capping, hashing, expiry and burning are identical across purposes. Three copies would mean three places to get constant-time comparison wrong |
| How is 'stay signed in' implemented? | Sliding idle window + absolute cap, not a long-lived token | An active learner is never signed out, which is the actual requirement, while every credential still has a bounded lifetime and a revocation path. See ADR-0022 |
| Which social providers in Phase 1? | Google only | It covers the overwhelming majority of the target market. Apple can wait until there is an iOS app, where it becomes a store requirement |
| What happens when a Google email matches an unverified local account? | Refuse and require OTP verification of that address first | Auto-linking would let anyone who registers an address they do not own capture it later via Google. This is the subtle account-takeover path in every social login integration |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0007](../../../docs/adr/ADR-0007-auth-jwt-refresh-rotation.md) — JWT access + rotating refresh tokens
- [ADR-0021](../../../docs/adr/ADR-0021-email-otp-challenges.md) — Email OTP challenges instead of verification links
- [ADR-0022](../../../docs/adr/ADR-0022-persistent-sessions.md) — Persistent sign-in: sliding window with an absolute cap
- [ADR-0023](../../../docs/adr/ADR-0023-google-oauth-linking.md) — Google OAuth and the account-linking policy
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we support passkeys in Phase 2, and if so do they replace or supplement TOTP?
- Should admin MFA also require a hardware key, or is TOTP sufficient for our threat model?
<!-- END GENERATED: decisions-open -->
