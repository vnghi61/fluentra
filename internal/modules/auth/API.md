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
last_verified: 2026-08-06
---

# auth — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `auth`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | `public` | Create an account and issue a `verify_email` OTP challenge |
| `POST` | `/api/v1/auth/challenges/{id}/verify` | `public` | Submit the OTP code for a challenge |
| `POST` | `/api/v1/auth/challenges/{id}/resend` | `public` | Resend the code for a challenge |
| `POST` | `/api/v1/auth/login` | `public` | Validate credentials and enforce lockout |
| `GET` | `/api/v1/auth/oauth/google/start` | `public` | Begin Google sign-in; returns the authorization URL |
| `POST` | `/api/v1/auth/mfa/verify` | `public` | Complete a login that required a second factor |
| `POST` | `/api/v1/auth/refresh` | `public` | Rotate the refresh token and issue a new access token |
| `POST` | `/api/v1/auth/logout` | `self` | Revoke the current session and refresh family |
| `POST` | `/api/v1/auth/forgot-password` | `public` | Send a reset code |
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
| `POST` | `/api/v1/admin/users/{id}/sessions/revoke` | `user.manage_sessions` | Admin revokes all sessions for a user |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/auth/register`

Create an account and issue a `verify_email` OTP challenge

| | |
|---|---|
| Permission | `public` |
| Success | 201 |
| Errors | `EMAIL_ALREADY_REGISTERED`, `PASSWORD_TOO_WEAK`, `VALIDATION_FAILED`, `RATE_LIMITED` |
| Notes | Returns a `challenge_id`; the 6-digit code goes to the email. The account cannot sign in until verified. |

### `POST /api/v1/auth/challenges/{id}/verify`

Submit the OTP code for a challenge

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `OTP_INVALID`, `OTP_EXPIRED`, `OTP_ATTEMPTS_EXCEEDED`, `CHALLENGE_NOT_FOUND` |
| Notes | Generic across every purpose. Consumes the challenge on success; burns it after `max_attempts`. |

### `POST /api/v1/auth/challenges/{id}/resend`

Resend the code for a challenge

| | |
|---|---|
| Permission | `public` |
| Success | 202 |
| Errors | `OTP_RESEND_TOO_SOON`, `RATE_LIMITED`, `CHALLENGE_NOT_FOUND` |
| Notes | 60-second cooldown, 3 issuances per subject per hour. Issues a new code and resets attempts; the challenge id is unchanged. |

### `POST /api/v1/auth/login`

Validate credentials and enforce lockout

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `INVALID_CREDENTIALS`, `ACCOUNT_LOCKED`, `MFA_REQUIRED`, `EMAIL_NOT_VERIFIED` |
| Notes | Accepts `remember_device` and an optional client `device_id`. Response timing is equalised between 'unknown email' and 'wrong password'; token issuance arrives in P2.4. |

### `GET /api/v1/auth/oauth/google/start`

Begin Google sign-in; returns the authorization URL

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |
| Notes | Generates `state`, `nonce` and a PKCE challenge, stored server-side with a 10-minute TTL |

### `POST /api/v1/auth/mfa/verify`

Complete a login that required a second factor

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `MFA_INVALID`, `TOKEN_EXPIRED` |

### `POST /api/v1/auth/refresh`

Rotate the refresh token and issue a new access token

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `TOKEN_INVALID`, `SESSION_REVOKED` |
| Notes | Reuse of an already-used token revokes the entire family and raises a security event |

### `POST /api/v1/auth/logout`

Revoke the current session and refresh family

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

### `POST /api/v1/auth/forgot-password`

Send a reset code

| | |
|---|---|
| Permission | `public` |
| Success | 202 |
| Errors | `RATE_LIMITED` |
| Notes | Always returns 202 regardless of whether the email exists |

### `POST /api/v1/auth/reset-password`

Consume a reset token and set a new password

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `TOKEN_INVALID`, `TOKEN_EXPIRED`, `PASSWORD_TOO_WEAK` |
| Notes | Revokes all sessions on success |

### `POST /api/v1/auth/change-password`

Change the password while signed in

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `INVALID_CREDENTIALS`, `PASSWORD_TOO_WEAK` |

### `GET /api/v1/auth/sessions`

List the caller's active sessions

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `DELETE /api/v1/auth/sessions/{id}`

Revoke one session

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | `RESOURCE_NOT_FOUND` |

### `POST /api/v1/auth/mfa/enroll`

Begin TOTP enrolment; returns a provisioning URI and recovery codes

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/auth/oauth/google/callback`

Complete Google sign-in: verify state, exchange the code, validate the ID token, link or create the account

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `OAUTH_STATE_INVALID`, `OAUTH_EMAIL_UNVERIFIED`, `OAUTH_ACCOUNT_CONFLICT`, `TOKEN_INVALID` |

### `POST /api/v1/auth/oauth/google/link`

Link a Google identity to the signed-in account

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `OAUTH_ALREADY_LINKED`, `OAUTH_EMAIL_MISMATCH` |

### `DELETE /api/v1/auth/oauth/google`

Unlink Google

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | `LAST_SIGN_IN_METHOD` |
| Notes | Refused if it would leave the account with no way to sign in |

### `GET /api/v1/auth/devices`

List trusted devices with their idle and absolute expiry

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `DELETE /api/v1/auth/devices/{id}`

Untrust a device, revoking its refresh family

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

### `POST /api/v1/admin/users/{id}/sessions/revoke`

Admin revokes all sessions for a user

| | |
|---|---|
| Permission | `user.manage_sessions` |
| Success | 204 |
| Errors | `PERMISSION_DENIED`, `RESOURCE_NOT_FOUND` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
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
| `OTP_INVALID` | 401 | Wrong code; the response includes `attempts_remaining` |
| `OTP_EXPIRED` | 401 | Code older than 10 minutes |
| `OTP_ATTEMPTS_EXCEEDED` | 429 | Challenge burned; request a new one |
| `OTP_RESEND_TOO_SOON` | 429 | Within the 60-second cooldown; `Retry-After` set |
| `CHALLENGE_NOT_FOUND` | 404 | Unknown, consumed or expired challenge |
| `OAUTH_STATE_INVALID` | 400 | Missing, reused or expired state — a possible CSRF attempt |
| `OAUTH_EMAIL_UNVERIFIED` | 403 | The provider did not assert a verified email |
| `OAUTH_ACCOUNT_CONFLICT` | 409 | The email belongs to an unverified local account; verify it first |
| `OAUTH_EMAIL_MISMATCH` | 409 | Linking attempted with an address other than the account's |
| `OAUTH_ALREADY_LINKED` | 409 | That Google identity is already linked to another account |
| `LAST_SIGN_IN_METHOD` | 409 | Unlinking would leave no way to sign in |
| `SESSION_ABSOLUTE_EXPIRED` | 401 | Absolute session lifetime reached; full re-authentication required |
| `DEVICE_LIMIT_REACHED` | 409 | Too many trusted devices; untrust one first |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
- `/auth/login`, `/auth/register`, `/auth/forgot-password`: 5/min per IP **and** per account, with exponential lockout
- `/auth/refresh`: 30/min per session
- `/auth/mfa/verify`: 5 attempts per login challenge
<!-- END GENERATED: api-rate -->
