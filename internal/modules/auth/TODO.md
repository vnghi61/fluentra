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

# auth — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1 — core

- [ ] Registration + email verification, with the outbox path proven end-to-end
- [ ] Argon2id credential storage with parameter upgrade on login
- [ ] Login with equalised timing and per-account/per-IP lockout
- [ ] Access token issuance and validating middleware, with `Actor` in the request context
- [ ] Refresh rotation with family revocation on reuse, covered by an integration test
- [ ] Session list and revoke, for the user and for an admin
- [ ] Password reset and change, revoking sessions
- [ ] Security event emission wired to `audit`

## Phase 2

- [ ] TOTP enrolment, verification, recovery codes; mandatory for admins
- [ ] Google and Apple OAuth with verified-email linking
- [ ] New-device notification email
- [ ] Offline breached-password Bloom filter as a fallback
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
- WebAuthn / passkeys — Phase 3 at the earliest
- SSO / SAML — only if the product ever serves institutions, which would also mean multi-tenancy
- Magic-link login — adds an email-delivery dependency to the critical path
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Risk-based authentication: step up to MFA on an unusual country or device
- Session anomaly detection feeding the security dashboard
- Per-device token binding once passkeys land
<!-- END GENERATED: todo-future -->
