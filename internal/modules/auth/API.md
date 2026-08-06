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
| `POST` | `/api/v1/auth/register` | `public` | Create an account and send a verification email |
| `POST` | `/api/v1/auth/verify-email` | `public` | Consume a verification token |
| `POST` | `/api/v1/auth/login` | `public` | Exchange credentials for an access token plus a refresh cookie |
| `POST` | `/api/v1/auth/mfa/verify` | `public` | Complete a login that required a second factor |
| `POST` | `/api/v1/auth/refresh` | `public` | Rotate the refresh token and issue a new access token |
| `POST` | `/api/v1/auth/logout` | `self` | Revoke the current session and refresh family |
| `POST` | `/api/v1/auth/forgot-password` | `public` | Send a reset link |
| `POST` | `/api/v1/auth/reset-password` | `public` | Consume a reset token and set a new password |
| `POST` | `/api/v1/auth/change-password` | `self` | Change the password while signed in |
| `GET` | `/api/v1/auth/sessions` | `self` | List the caller's active sessions |
| `DELETE` | `/api/v1/auth/sessions/{id}` | `self` | Revoke one session |
| `POST` | `/api/v1/auth/mfa/enroll` | `self` | Begin TOTP enrolment; returns a provisioning URI and recovery codes |
| `POST` | `/api/v1/auth/oauth/{provider}/callback` | `public` | Complete an OAuth sign-in |
| `POST` | `/api/v1/admin/users/{id}/sessions/revoke` | `user.session.revoke` | Admin revokes all sessions for a user |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/auth/register`

Create an account and send a verification email

| | |
|---|---|
| Permission | `public` |
| Success | 201 |
| Errors | `EMAIL_ALREADY_REGISTERED`, `PASSWORD_TOO_WEAK`, `VALIDATION_FAILED`, `RATE_LIMITED` |


### `POST /api/v1/auth/verify-email`

Consume a verification token

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `TOKEN_INVALID`, `TOKEN_EXPIRED` |


### `POST /api/v1/auth/login`

Exchange credentials for an access token plus a refresh cookie

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `INVALID_CREDENTIALS`, `ACCOUNT_LOCKED`, `MFA_REQUIRED`, `EMAIL_NOT_VERIFIED` |
| Notes | Response timing is equalised between 'unknown email' and 'wrong password' |

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

Send a reset link

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


### `POST /api/v1/auth/oauth/{provider}/callback`

Complete an OAuth sign-in

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `TOKEN_INVALID` |


### `POST /api/v1/admin/users/{id}/sessions/revoke`

Admin revokes all sessions for a user

| | |
|---|---|
| Permission | `user.session.revoke` |
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
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
- `/auth/login`, `/auth/register`, `/auth/forgot-password`: 5/min per IP **and** per account, with exponential lockout
- `/auth/refresh`: 30/min per session
- `/auth/mfa/verify`: 5 attempts per login challenge
<!-- END GENERATED: api-rate -->
