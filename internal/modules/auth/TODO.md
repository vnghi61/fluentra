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
last_verified: 2026-08-11
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
| P2.2 | 2026-08-10 | `module.go` — `New(Deps)`, `Routes`, `Subscribe`, `CronJobs`, adapters for all service surfaces; `RegisterService.Register/VerifyEmail/Resend/PurgeUnverified`; three HTTP endpoints (`POST /auth/register`, `POST /auth/challenges/{id}/verify`, `POST /auth/challenges/{id}/resend`); `registration_attempt` email template (en + vi); outbox consumer for `auth.verification_requested` and `auth.registration_attempted`; `purge_unverified` cron job; OTP_HMAC_KEY plus MAIL_FROM and SMTP transport config wired in both `cmd/api` and `cmd/worker` |
| P2.3 | 2026-08-10 | `core.login_attempts` migration & table; `LoginService` handling Argon2id constant-time timing equalisation (`DummyVerify` for unknown emails), per-account and per-IP lockouts backed by persisted failures with 15-minute-to-24-hour exponential backoff, account status checks (suspended/unverified), transparent Argon2id parameter rehash on login, and `POST /auth/login` HTTP endpoint |
| P2.4 | 2026-08-11 | `TokenService` — HS256 access tokens carrying `sub`/`sid`/`role`/`jti`/`iat`/`exp`/`aud`/`iss` and no PII, two-key rotation, 60-second leeway, the signing method pinned so an `alg: none` token cannot verify, and the parser driven by the injected clock; `TokenDenylist` over Redis for explicit logout, failing open per ADR-0007; `Authenticate` middleware placing `httpx.Actor` in the request context; login and email verification both return an `AuthSession`; `EMAIL_NOT_VERIFIED` corrected to 403 and account suspension split out of `ACCOUNT_LOCKED` into `ACCOUNT_SUSPENDED` |

## Open after P2.4

- [ ] **Logout has no endpoint yet.** `TokenService.Revoke` and the denylist behind it are built
      and tested, but nothing calls them: `POST /auth/logout` is P2.6, which is also where the
      session row that makes `sid` mean something arrives. Until then a learner signs out by
      discarding the token client-side and waiting fifteen minutes.
- [ ] **`sid` identifies nothing yet.** It is a fresh identifier per login, minted so that the
      token format does not change when P2.6 adds `core.sessions`. Done when the claim matches a
      row and revoking that row invalidates the token.
- [ ] **The `/auth/login` 200 schema is declared inline** in `openapi.yaml` rather than in
      `components/auth.yaml` where every other module's schemas live. P2.4 replaced it with a
      `$ref` to `AuthSession`, so this is closed — but check the same has not happened again the
      next time an endpoint is added in a hurry.

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
- [x] **Wire the subsystem in.** `module.go` now reads `OTP_HMAC_KEY` and both binaries declare
      it as a required key. The six OTP tuning parameters (`OTP_TTL`, `OTP_MAX_ATTEMPTS`,
      `OTP_CODE_LENGTH`, `OTP_RESEND_COOLDOWN`, `OTP_ISSUE_PER_SUBJECT_PER_HOUR`,
      `OTP_ISSUE_PER_IP_PER_HOUR`) are still read from `service.DefaultConfig`; wiring them to
      the env block is a follow-up that belongs in P2.8 alongside the rate-limiter wiring.
- [ ] **A rotation story for `OTP_HMAC_KEY`.** Changing the key invalidates every live challenge —
      ten minutes of them, so the blast radius is small, but it is currently undocumented and
      there is no two-key path as `JWT_PREVIOUS_KEY` gives tokens. Done when the operational
      consequence is written down in `docs/deployment/configuration.md`.

## Open after P2.2

- [ ] **The code-in-plaintext outbox payload.** `ops.outbox_events` currently keeps published rows
      indefinitely, so `auth.verification_requested` payloads carry the OTP code after the
      challenge has expired and the code is worthless. The exposure is bounded by the ten-minute
      TTL — nobody can complete the challenge with a stale code — but the record outlives its
      usefulness. Done when the outbox pruner in `shared/outbox` deletes published rows older
      than some configurable window (tracked in `shared/outbox/TODO.md`).
- [ ] **OTP tuning parameters wired to config.** `OTP_TTL`, `OTP_MAX_ATTEMPTS`, `OTP_CODE_LENGTH`,
      `OTP_RESEND_COOLDOWN`, `OTP_ISSUE_PER_SUBJECT_PER_HOUR`, and `OTP_ISSUE_PER_IP_PER_HOUR`
      are all in `.env.example` and all read from `service.DefaultConfig` today. Done when
      `cmd/api` loads them and a test proves a non-default `OTP_CODE_LENGTH` takes effect (P2.8).

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
