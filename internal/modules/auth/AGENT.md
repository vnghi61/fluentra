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

# auth — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `core` |
| Path | `internal/modules/auth` |
| Schema | `core` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Owns everything about proving who a caller is: registration, email verification, login, multi-factor authentication, token issuance and rotation, session lifecycle, password reset, and OAuth sign-in. It answers "who is this?" and nothing else.
<!-- END GENERATED: overview -->

**Context.** Authentication is deliberately separated from `user` (profile data) and `rbac` (what the caller may do). Keeping the three apart means a change to profile fields cannot break login, and a change to permissions cannot break token issuance.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Registration and email verification by one-time code (OTP)
- The generic challenge subsystem: issue, resend, verify and burn short-lived codes for any purpose
- Password hashing (Argon2id), verification, and policy enforcement including breached-password checks
- Access token (JWT, 15 min) issuance and validation
- Refresh token issuance, **sliding** rotation, reuse detection and family revocation
- Persistent sign-in: trusted devices, sliding idle window, absolute re-authentication cap
- Session records: creation, listing, revocation (by the user and by an admin)
- TOTP multi-factor enrolment, verification and recovery codes
- Password reset and change flows
- Google OAuth sign-in (authorization code + PKCE) and account linking by verified email only
- Brute-force protection: per-IP, per-account and per-challenge lockout
- Emitting security events for the audit trail

**This module does NOT own:**

- Profile data, preferences, avatars — that is `user`
- What a caller is allowed to do — that is `rbac`
- Storing the audit log — it emits events, `audit` persists them
- Sending the emails — it asks `platform/mailer` to
- Rate limiting infrastructure — it uses `platform/cache`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/auth/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/auth/contract/` | You are calling this module from another module |
| `internal/modules/auth/service/` | You are changing behaviour |
| `db/migrations/auth/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/auth/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `auth.TokenVerifier` | Validate an access token and return the actor — used by the HTTP middleware |
| interface | `auth.SessionRevoker` | Revoke sessions for a user — used by `admin` and by `user` on account deletion |
| struct | `auth.Actor` | `{UserID, SessionID, Role, TokenID}` — placed in the request context |
| event | `auth.UserRegistered` | Published after a successful registration |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `user.registered` | publishes | `{user_id, email_hash, locale, source, occurred_at}` |
| `auth.login_succeeded` | publishes | `{user_id, session_id, ip_country}` |
| `auth.security_event` | publishes | `{user_id, kind, severity, detail}` — consumed by `audit` |
| `auth.password_changed` | publishes | `{user_id}` — triggers a notification |
| `user.deletion_requested` | consumes | Revoke all sessions and credentials immediately |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `core` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/auth/` · Queries: `db/queries/auth/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `core.credentials` | Password hash per user | `user_id` UNIQUE, `password_hash`, `algo_params`, `updated_at`. Never selected in a list query. |
| `core.sessions` | One row per active sign-in | `user_id`, `device_label`, `ip_hash`, `user_agent_hash`, `last_seen_at`, `revoked_at` |
| `core.refresh_tokens` | Rotating refresh tokens | `token_hash` (SHA-256) UNIQUE, `family_id`, `session_id`, `used_at`, `expires_at`. Reuse of a used row revokes the whole family. |
| `core.mfa_secrets` | TOTP seeds and recovery codes | Seed encrypted with a KEK; recovery codes stored hashed and single-use |
| `core.auth_challenges` | Short-lived one-time codes for every purpose | `id` (the challenge handle returned to the client), `purpose` enum (`verify_email`, `login_otp`, `password_reset`, `link_oauth`), `subject_hash`, `code_hash` (HMAC-SHA256 with a server key), `attempts`, `max_attempts`, `expires_at`, `consumed_at`. 10-minute TTL, single use. |
| `core.trusted_devices` | Devices the learner chose to stay signed in on | `user_id`, `device_id` (client-generated, stored hashed), `label`, `idle_window`, `absolute_expires_at`, `last_seen_at`, `revoked_at` |
| `core.login_attempts` | Brute-force accounting and forensics | Partitioned monthly; `email_hash`, `ip_hash`, `result`, `created_at` |
| `core.oauth_identities` | Linked external identities | `provider`, `subject`, `email_hash`, `linked_at`, UNIQUE(provider, subject) |
| `core.oauth_states` | In-flight OAuth authorization requests | `state` UNIQUE, `nonce`, `pkce_verifier_hash`, `redirect_to`, `expires_at`. 10-minute TTL, single use — this is the CSRF defence for the OAuth flow. |

**Indexes of note**

- `uq_refresh_tokens_hash` — unique on `token_hash`, the lookup path on every refresh
- `idx_refresh_tokens_family` — used to revoke a whole family in one statement
- `idx_sessions_user_active` — partial index `WHERE revoked_at IS NULL`
- `idx_login_attempts_email_time` — the lockout query
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `auth`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | `public` | Create an account and issue a `verify_email` OTP challenge |
| `POST` | `/api/v1/auth/challenges/{id}/verify` | `public` | Submit the OTP code for a challenge |
| `POST` | `/api/v1/auth/challenges/{id}/resend` | `public` | Resend the code for a challenge |
| `POST` | `/api/v1/auth/login` | `public` | Exchange credentials for an access token plus a refresh cookie |
| `GET` | `/api/v1/auth/oauth/google/start` | `public` | Begin Google sign-in; returns the authorization URL |
| `POST` | `/api/v1/auth/mfa/verify` | `public` | Complete a login that required a second factor |
| `POST` | `/api/v1/auth/refresh` | `public` | Rotate the refresh token and issue a new access token |
| `POST` | `/api/v1/auth/logout` | `self` | Revoke the current session and refresh family |
| `POST` | `/api/v1/auth/forgot-password` | `public` | Send a reset link |
| `POST` | `/api/v1/auth/reset-password` | `public` | Consume a reset token and set a new password |
| `POST` | `/api/v1/auth/change-password` | `self` | Change the password while signed in |
| `GET` | `/api/v1/auth/sessions` | `self` | List the caller's active sessions |
| `DELETE` | `/api/v1/auth/sessions/{id}` | `self` | Revoke one session |
| `POST` | `/api/v1/auth/mfa/enroll` | `self` | Begin TOTP enrolment; returns a provisioning URI and recovery codes |
| `POST` | `/api/v1/auth/oauth/google/callback` | `public` | Complete Google sign-in: verify state, exchange the code, validate the ID token, link or create the account |
| `POST` | `/api/v1/auth/oauth/google/link` | `self` | Link a Google identity to the signed-in account |
| `DELETE` | `/api/v1/auth/oauth/google` | `self` | Unlink Google |
| `GET` | `/api/v1/auth/devices` | `self` | List trusted devices with their idle and absolute expiry |
| `DELETE` | `/api/v1/auth/devices/{id}` | `self` | Untrust a device, revoking its refresh family |
| `POST` | `/api/v1/admin/users/{id}/sessions/revoke` | `user.session.revoke` | Admin revokes all sessions for a user |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`user`](../../modules/user/AGENT.md) | → depends on | Create the user record on registration; read status (active/suspended) on login |
| [`rbac`](../../modules/rbac/AGENT.md) | → depends on | Attach the caller's role to the issued token |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Record authentication and security events |
| [`mailer`](../../platform/mailer/AGENT.md) | → depends on | Send verification, reset and new-device emails |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Lockout counters, rate limits, token denylist |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | Revoke sessions, force password reset, inspect login history |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-AUTH-01** — A password must be at least 12 characters and must not appear in the breached-password corpus.
2. **BR-AUTH-02** — Login responses for an unknown email and a wrong password are indistinguishable in content and comparable in timing.
3. **BR-AUTH-03** — An access token is valid for 15 minutes and carries no personal data beyond the user ID.
4. **BR-AUTH-04** — A refresh token is single-use. Presenting a used token revokes every token in its family and raises a `refresh_reuse` security event.
5. **BR-AUTH-05** — Changing or resetting a password revokes all sessions except, optionally, the current one.
6. **BR-AUTH-06** — Email verification is required before any AI-consuming feature may be used.
7. **BR-AUTH-07** — MFA is mandatory for accounts holding the `admin` role; a promotion to admin forces enrolment on next login.
8. **BR-AUTH-08** — Five failed attempts within 15 minutes lock the account for an exponentially increasing period, tracked per account and per IP independently.
9. **BR-AUTH-09** — A suspended user's tokens stop working within one access-token lifetime (15 min); their sessions are revoked immediately on suspension.
10. **BR-AUTH-10** — Every one-time code is 6 digits, single-use, expires in 10 minutes, allows at most 5 verification attempts, and is stored only as an HMAC — never in plaintext, never in a log.
11. **BR-AUTH-11** — A code is worthless without its `challenge_id`. The client receives the id; the code goes to the email. Guessing a code alone gets an attacker nothing.
12. **BR-AUTH-12** — Exceeding `max_attempts` burns the challenge permanently. A new one must be requested — the code is not simply retried.
13. **BR-AUTH-13** — Resend has a 60-second cooldown and a cap of 3 issuances per subject per hour. Resending replaces the code and resets attempts; it does not extend the absolute expiry.
14. **BR-AUTH-14** — Challenge issuance never reveals whether an account exists. `POST /auth/register` on an existing verified email returns the same shape as a fresh registration and sends a 'someone tried to register with your address' email instead.
15. **BR-AUTH-15** — Google sign-in links to an existing account **only** when Google asserts `email_verified: true` and the email matches a verified local account. An unverified Google email is refused outright.
16. **BR-AUTH-16** — If a Google email matches an existing account that is **not** yet verified, no automatic link occurs — the learner must complete an OTP challenge on that address first. Otherwise anyone who can register an unverified address could hijack it via a Google account.
17. **BR-AUTH-17** — The OAuth `state` is server-side, single-use and 10-minute-lived, and PKCE is mandatory even though we hold a client secret.
18. **BR-AUTH-18** — Google's ID token is verified against Google's JWKS: signature, `iss`, `aud`, `exp`, and the `nonce` we issued. A token that fails any check is rejected without a partial account being created.
19. **BR-AUTH-19** — An account signed in via Google skips email verification — Google has already performed it.
20. **BR-AUTH-20** — Unlinking the last sign-in method is refused; the account must always retain at least one way in.
21. **BR-AUTH-21** — Refresh tokens are **sliding**: each rotation issues a new token with a fresh idle window, so an active learner never signs in again.
22. **BR-AUTH-22** — Every session carries an **absolute** expiry independent of activity. When it passes, full re-authentication is required regardless of how recently the learner was active.
23. **BR-AUTH-23** — Idle windows: 30 days by default, 90 days on a device the learner explicitly chose to trust. Absolute cap: 180 days. Admin accounts get neither extension — 12-hour idle, 7-day absolute.
24. **BR-AUTH-24** — Trusting a device is an explicit opt-in, is listed in the account's device list, and can be revoked from any other device.
25. **BR-AUTH-25** — A password change, a password reset, or an admin suspension revokes every trusted device and every refresh family.
26. **BR-AUTH-26** — The system never reveals whether an email address is registered — `forgot-password` always returns 202.
<!-- END GENERATED: rules -->

### Validation rules

| Field / input | Rule | Error code |
|---|---|---|
| `email` | RFC 5322, normalised to lowercase, max 254 chars, MX-checkable domain | `VALIDATION_FAILED` |
| `password` | ≥ 12 chars, not in the breach corpus, not equal to the email local part | `PASSWORD_TOO_WEAK` |
| `totp_code` | 6 digits, ±1 time step tolerance, each code usable once | `MFA_INVALID` |
| `token` | Opaque, exists, unused, unexpired | `TOKEN_INVALID` |

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a new OAuth provider

1. Add the provider config keys to `shared/config` and `.env.example`.
2. Implement the provider adapter in `service/oauth/<provider>.go` behind the existing `oauthProvider` interface.
3. Register it in `module.go`'s provider map.
4. Add the callback path to `api/openapi/openapi.yaml` (it is parameterised, so usually no change).
5. Add a fixture-backed unit test and an integration test with a stubbed token endpoint.
6. Update §4 and §9 of this AGENT.md and `docs/security/authentication.md`.

### Change token lifetimes

1. Change `ACCESS_TTL` / `REFRESH_TTL` in config — never hard-code a duration.
2. Check the frontend's silent-refresh margin still fits (it refreshes at 80 % of the access TTL).
3. Update the denylist TTL, which must be ≥ the access TTL.
4. Update `docs/deployment/configuration.md` and this file §9.

### Add a new security event kind

1. Add the kind to `contract/events.go` with a severity.
2. Publish it from the relevant service method through the outbox.
3. Confirm `audit` persists it (it consumes the generic event, so usually no change there).
4. Add an alert rule if the event should page someone, plus a runbook.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- MFA is TOTP only. WebAuthn/passkeys are the obvious next step but are not in Phase 1.
- A 6-digit OTP is 10^6 of entropy. Five attempts per challenge plus per-subject and per-IP issuance limits keep the per-challenge guess probability at 5 in a million, but a distributed guessing campaign across many challenges still needs the global rate limiter to catch it — that limiter is load-bearing, not decorative.
- OTP delivery depends on email latency. A learner on a slow provider may wait; the resend cooldown is deliberately short (60 s) to compensate.
- Google is the only social provider. Apple is deferred until an iOS app exists, where the App Store requires it.
- Device identity is a client-generated `device_id` in local storage plus a coarse fingerprint. Clearing browser storage looks like a new device, which is the correct failure direction but does mean a re-login.
- The breached-password check requires an outbound call; the offline Bloom-filter fallback is not yet built.
- OAuth account linking requires a verified email match; providers that do not assert verification are not supported.
- Session geolocation is IP-country only, from a local database — it is indicative, not evidence.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
- Never log an email address. Log `user_id`, or `email_hash` when there is no user yet.
- Every timing-sensitive comparison uses `crypto/subtle` or the argon2id verifier.
- Token strings never appear in a struct that has a `String()` or is ever logged — wrap them in `shared/secret.Redacted[string]`.
- All token TTLs come from config; a literal duration in this module is a review failure.
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:auth:lockout:{email_hash}:v1` | 15 min sliding | Successful login |
| `fluentra:{env}:auth:lockout:ip:{ip_hash}:v1` | 15 min sliding | Natural expiry |
| `fluentra:{env}:auth:denylist:{jti}:v1` | remaining token TTL | Natural expiry |
| `fluentra:{env}:auth:session:{session_id}:v1` | 5 min | Session revoked |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `INVALID_CREDENTIALS` | 401 | Wrong email or password — never says which |
| `ACCOUNT_LOCKED` | 429 | Too many failed attempts |
| `EMAIL_NOT_VERIFIED` | 403 | Verification required for this action |
| `TOKEN_EXPIRED` | 401 | Access token expired; refresh |
| `TOKEN_INVALID` | 401 | Malformed, unknown, or bad signature |
| `SESSION_REVOKED` | 401 | Session revoked, including refresh-reuse detection |
| `MFA_REQUIRED` | 401 | Second factor needed to complete login |
| `MFA_INVALID` | 401 | Wrong or reused TOTP code |
| `EMAIL_ALREADY_REGISTERED` | 409 | Registration conflict |
| `PASSWORD_TOO_WEAK` | 422 | Fails policy or found in a breach corpus |

### Security considerations

- Access tokens live in browser memory only — never `localStorage`, never a non-HttpOnly cookie.
- Refresh tokens are `HttpOnly; Secure; SameSite=Lax; Path=/api/v1/auth`.
- Password comparison is constant-time; Argon2id parameters are stored in the hash so they can be raised without a migration.
- Every authentication failure is logged with `email_hash`, never the address.
- Admin session revocation is audited with the acting admin's ID.
- The JWT signing key supports two active keys so rotation does not sign everyone out.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **90% domain, 85% service (elevated: this is security-critical)**

```bash
go test ./internal/modules/auth/...                    # unit
go test -tags=integration ./internal/modules/auth/...  # integration (testcontainers)
```

**Focus areas**

- Refresh reuse detection: reusing a rotated token must revoke the whole family
- Login timing: unknown email and wrong password must not be distinguishable
- Lockout: counters per account and per IP behave independently and expire correctly
- Password change and reset revoke sessions
- MFA: replayed code rejected; recovery code single-use; clock-skew tolerance
- Suspended user cannot refresh, and existing access tokens stop working within one TTL
- OTP: single-use, 10-minute expiry, exactly 5 attempts then burned, constant-time comparison
- OTP: the code is never in the API response, never in a log, never in a span attribute
- OTP: resend cooldown and hourly cap; resend replaces the code but does not extend the absolute expiry
- OTP: a code from challenge A does not verify challenge B
- OAuth: forged, reused and expired `state` all rejected; each raises a security event
- OAuth: an ID token failing signature, `iss`, `aud`, `exp` or `nonce` creates no account and no partial state
- OAuth: unverified Google email refused; match against an unverified local account refused
- OAuth: unlinking the only sign-in method refused
- Sliding refresh: rotation moves the idle window forward but never past the absolute expiry
- Absolute expiry forces re-authentication even for a continuously active session
- Admin accounts do not receive the extended idle window
- Password change, reset and suspension all revoke every trusted device
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not put the access token in `localStorage` or a readable cookie.
- Do not add profile fields here — they belong to `user`.
- Do not check permissions here — that is `rbac`. This module only proves identity.
- Do not return different messages or noticeably different timings for unknown-email and wrong-password.
- Do not extend the access-token lifetime to avoid refresh complexity.
- Do not log tokens, password hashes, email addresses, TOTP seeds, or OTP codes — not even at debug level, not even in a test fixture.
- Do not implement 'stay signed in' as a token with no expiry. Use the sliding idle window plus the absolute cap; an immortal credential cannot be reasoned about or revoked reliably.
- Do not put the OTP code in the API response, in a URL, or in a push notification. It goes to the email channel and nowhere else — otherwise it verifies nothing.
- Do not compare an OTP with `==`. Use a constant-time comparison against the stored HMAC.
- Do not auto-link a Google identity to an unverified local account.
- Do not trust anything in Google's ID token before verifying its signature against the JWKS, plus `iss`, `aud`, `exp` and the `nonce` we issued.
- Do not reuse an OAuth `state` value, and do not accept one that did not come from our own store.
- Do not skip PKCE because we are a confidential client — it costs nothing and blocks code interception on mobile browsers.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
