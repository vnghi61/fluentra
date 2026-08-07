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

# auth — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Registration with OTP verification

The account, the credential and the challenge are created in one transaction; the email is dispatched through the outbox so a code can never be sent for a rolled-back registration. The learner stays on the same screen and types the code — no context switch to a second browser.

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant A as auth
    participant US as user (contract)
    participant D as PostgreSQL
    participant M as mailer

    U->>A: POST /auth/register { email, password }
    A->>A: validate email + password policy + breach corpus
    A->>D: BEGIN
    A->>US: CreateUser(email, locale)
    US->>D: INSERT core.users (status=pending_verification)
    A->>D: INSERT core.credentials (argon2id)
    A->>D: INSERT auth_challenges (purpose=verify_email, code_hash=HMAC(code), attempts=0, ttl=10m)
    A->>D: INSERT outbox(send verify_email code)
    A->>D: COMMIT
    A-->>U: 201 { challenge_id, expires_at, resend_after }
    Note over D,M: outbox → mailer job — never a direct call
    M->>U: email containing the 6-digit code

    loop up to 5 attempts
        U->>A: POST /auth/challenges/{id}/verify { code }
        A->>D: load challenge; check expiry and attempts
        A->>A: constant-time compare against code_hash
        alt wrong
            A->>D: attempts += 1
            A-->>U: 401 OTP_INVALID { attempts_remaining }
        else attempts exhausted
            A->>D: burn challenge
            A-->>U: 429 OTP_ATTEMPTS_EXCEEDED
        else correct
            A->>D: BEGIN; consume challenge; users.status = active
            A->>D: INSERT outbox(user.registered); COMMIT
            A-->>U: 200 { access_token } + refresh cookie
        end
    end
```

## Google sign-in with account linking

The dangerous branch is the one in the middle: a Google email that matches an **unverified** local account. Auto-linking there is the classic account-takeover path, so it is refused.

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant W as SPA
    participant A as auth
    participant G as Google
    participant D as PostgreSQL

    U->>W: tap 'Continue with Google'
    W->>A: GET /auth/oauth/google/start
    A->>D: INSERT oauth_states (state, nonce, pkce_verifier_hash, ttl=10m)
    A-->>W: { authorization_url }
    W->>G: redirect
    U->>G: consent
    G->>W: redirect back with code + state
    W->>A: POST /auth/oauth/google/callback { code, state }
    A->>D: load + consume state (single use)
    alt missing, reused or expired
        A-->>W: 400 OAUTH_STATE_INVALID + security event
    end
    A->>G: exchange code (with PKCE verifier)
    G-->>A: id_token + access_token
    A->>G: fetch JWKS; verify signature, iss, aud, exp, nonce
    alt email_verified is false
        A-->>W: 403 OAUTH_EMAIL_UNVERIFIED
    end
    A->>D: look up oauth_identities by (google, sub)
    alt identity known
        A->>A: sign in
    else email matches a VERIFIED local account
        A->>D: INSERT oauth_identities (link)
        A->>A: sign in
    else email matches an UNVERIFIED local account
        A-->>W: 409 OAUTH_ACCOUNT_CONFLICT — verify by OTP first
    else no local account
        A->>D: create user (status=active, email already verified by Google)
        A->>A: sign in
    end
    A-->>W: 200 { access_token } + refresh cookie
```

## Staying signed in — sliding window with an absolute cap

This is what 'log in once' actually means in a design that can still revoke a stolen credential. The idle window moves forward on every use; the absolute expiry does not.

```mermaid
flowchart TD
    A[Login with remember_device = true] --> B[create session:<br/>idle_window = 90d<br/>absolute_expires_at = now + 180d]
    B --> C[refresh token issued, expires = now + idle_window]
    C --> D{App opened}
    D --> E{now > absolute_expires_at?}
    E -->|yes| F[401 SESSION_ABSOLUTE_EXPIRED<br/>full re-authentication]
    E -->|no| G{refresh token expired?}
    G -->|yes| H[401 — idle too long, sign in again]
    G -->|no| I[rotate: new token, expires = now + idle_window]
    I --> J[access token issued<br/>learner never saw a login screen]
    J --> D

    K[Password change / reset /<br/>admin suspend / device untrusted] --> L[revoke every family<br/>+ every trusted device]
    L --> F
```

## Refresh rotation with reuse detection

The single most security-critical flow in the system. A stolen refresh token is detectable because the legitimate client will eventually present the same token the attacker already used.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant A as auth
    participant D as PostgreSQL
    participant AU as audit

    C->>A: POST /auth/refresh (cookie)
    A->>D: SELECT refresh_tokens WHERE token_hash = $1
    alt not found
        A-->>C: 401 TOKEN_INVALID
    else used_at IS NOT NULL
        A->>D: UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $f
        A->>D: UPDATE sessions SET revoked_at = now() WHERE id = $s
        A->>AU: security_event(refresh_reuse, severity=high)
        A-->>C: 401 SESSION_REVOKED
    else valid
        A->>D: BEGIN
        A->>D: UPDATE refresh_tokens SET used_at = now() WHERE id = $1
        A->>D: INSERT refresh_tokens (new hash, same family_id)
        A->>D: UPDATE sessions SET last_seen_at = now()
        A->>D: COMMIT
        A-->>C: 200 { access_token } + Set-Cookie
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
## State machine

Account lifecycle. Note that `suspended` is reversible and `deleted` is not — deletion runs through a 30-day grace period owned by `user`.

```mermaid
stateDiagram-v2
    [*] --> PendingVerification: register
    PendingVerification --> Active: verify email
    PendingVerification --> [*]: expire after 7 days
    Active --> Locked: too many failed attempts
    Locked --> Active: lockout expires or admin unlocks
    Active --> Suspended: admin suspends
    Suspended --> Active: admin reinstates
    Active --> PendingDeletion: user requests deletion
    Suspended --> PendingDeletion: user requests deletion
    PendingDeletion --> Active: user cancels within 30 days
    PendingDeletion --> [*]: anonymised after 30 days
```

<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
| Failure | Detected by | Behaviour |
|---|---|---|
| Mailer unavailable at registration | Outbox job retries | Account is created; the email is retried with backoff; the user can request a resend |
| Redis unavailable | `cache_unavailable_total` | Lockout counters fall back to a database query; login still works, with a warn log |
| Breached-password service unreachable | Timeout after 800 ms | Fail open with a warn log — a weak password is worse than a blocked registration, but an outage must not block signups |
| Clock skew between replicas | Token validation failures spike | 60-second leeway on `exp`/`nbf`; NTP is a host requirement |
<!-- END GENERATED: failures -->
