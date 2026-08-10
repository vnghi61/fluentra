---
module: auth
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [credentials, sessions, refresh_tokens, mfa_secrets, auth_challenges, trusted_devices, login_attempts, oauth_identities, oauth_states]
depends_on: [user, rbac, audit, mailer, cache]
depended_on_by: [admin]
spec_version: 1.0.0
last_verified: 2026-08-10
---

# auth — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1 — core

- [ ] Argon2id credential storage with parameter upgrade on login
- [ ] Challenge subsystem: issue, resend, verify, burn — generic over `purpose`
- [ ] Registration + OTP email verification, with the outbox path proven end-to-end
- [ ] Login with equalised timing and per-account/per-IP lockout
- [ ] Access token issuance and validating middleware, with `Actor` in the request context
- [ ] Refresh rotation with family revocation on reuse, covered by an integration test
- [ ] Sliding idle window, absolute cap, trusted devices, device list and revoke
- [ ] Google OAuth: PKCE, state, nonce, JWKS verification, linking policy
- [ ] Session list and revoke, for the user and for an admin
- [ ] Password reset and change, revoking sessions and trusted devices
- [ ] Security event emission wired to `audit`

## Phase 2

- [ ] TOTP enrolment, verification, recovery codes; mandatory for admins
- [ ] OTP as a step-up factor on an untrusted device (reuses the `login_otp` purpose — already built)
- [ ] Apple sign-in (required if an iOS app ships)
- [ ] New-device notification email
- [ ] Offline breached-password Bloom filter as a fallback
<!-- END GENERATED: todo -->

## Progress

The list above is generated from `tools/docgen/data/core.json`, so its checkboxes cannot be ticked
by hand. Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P2.1 | 2026-08-10 | `core.credentials`, with the cost parameters as a generated column and a CHECK that only a PHC-encoded Argon2id hash may be stored; `domain.Hasher` at m=64 MiB/t=3/p=2, parameters embedded in the hash, a rehash decision returned by every verify; `domain.Policy` — length, not equal to the email local part, and the breach corpus, failing open; `domain.PasswordRange` and the HIBP k-anonymity adapter over `net/http` |
| P2.1b | 2026-08-10 | `core.auth_challenges` with the `purpose` enum, the attempt cap as a CHECK and no `burned_at`; `domain.Keyring` — subject and code HMACs, the code hash bound to the challenge id; uniform code draw from `crypto/rand`; `ChallengeService.Issue`/`Verify`/`Resend` with the two issuance limiters and the resend cooldown; every state transition a guarded single-statement UPDATE, proven single-use under ten concurrent consumers |

## Open after P2.1

- [ ] **Wire the policy to configuration.** `ARGON2_MEMORY_KIB`, `ARGON2_ITERATIONS`,
      `ARGON2_PARALLELISM`, `PASSWORD_MIN_LENGTH` and `BREACHED_PASSWORD_CHECK` are all in
      `.env.example` and none is read yet — P2.1 ships no service and no `module.go`, so there is
      nothing to read them into. `domain.DefaultHashParams` carries the same numbers, so the
      defaults are already the documented ones. Done when `cmd/api` loads them and a test proves
      `BREACHED_PASSWORD_CHECK=false` leaves `Policy.Breaches` nil.
- [ ] **A metric for the breach check.** The fail-open path logs at warn and nothing counts it, so
      "how often did a password go unchecked?" is a log query rather than a number.
      `platform/telemetry.Instruments` has no module-level counter to hang it on, which makes this
      a platform change rather than a follow-up commit — the same blocker
      `internal/modules/audit/TODO.md` records for dropped entries. Done when the fail-open path
      increments something Prometheus scrapes.
- [ ] **The offline fallback.** Listed under Phase 2 above, but worth stating why it matters
      sooner than that: failing open means a sustained HIBP outage silently disables BR-AUTH-01
      entirely, and nothing currently makes that visible. A local Bloom filter satisfies
      `domain.BreachChecker` and needs no new port. Done when the corpus check works with no
      network.

## Open after P2.1b

- [ ] **Sweep expired challenges.** Nothing deletes them, so the table grows without bound and
      keeps subject hashes long past any use for them. It needs a cron job in `job/` and an index
      on `expires_at` to go with it — the only access path this card created is the primary key,
      and an index nothing queries would be dead weight until the sweeper exists. Done when a
      challenge past its expiry is removed on a schedule and the retention window is documented.
- [ ] **Emit `auth.security_event` for a burned challenge.** Five wrong codes against one
      challenge is a signal `audit` should hold; today it is a warn log and nothing more. The
      payload follows the field convention `audit` reads structurally (`occurred_at`, `user_id`,
      `severity`). Blocked on nothing but scope. Done when burning a challenge produces a row in
      `security_events`.
- [ ] **Wire the subsystem in.** There is no `module.go` yet, so nothing reads `OTP_TTL`,
      `OTP_MAX_ATTEMPTS`, `OTP_CODE_LENGTH`, `OTP_RESEND_COOLDOWN`, `OTP_ISSUE_PER_SUBJECT_PER_HOUR`,
      `OTP_ISSUE_PER_IP_PER_HOUR` or `OTP_HMAC_KEY`. `service.DefaultConfig` carries the same
      values, so the defaults are the documented ones. P2.2 does this as part of registration.
- [ ] **A rotation story for `OTP_HMAC_KEY`.** Changing the key invalidates every live challenge —
      ten minutes of them, so the blast radius is small, but it is currently undocumented and
      there is no two-key path as `JWT_PREVIOUS_KEY` gives tokens. Done when the operational
      consequence is written down in `docs/deployment/configuration.md`.

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
