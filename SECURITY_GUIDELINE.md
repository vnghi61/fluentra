---
doc_type: guideline
scope: security
last_verified: 2026-08-06
---

# SECURITY_GUIDELINE.md

Full design in [ARCHITECTURE.md §14](ARCHITECTURE.md#14-security-architecture) and
`docs/security/`. This document is the working rulebook.

---

## 1. Non-negotiables

| # | Rule |
|---|---|
| S1 | No secret in code, tests, fixtures, seeds, docs, logs, or commit history. `gitleaks` runs pre-commit and in CI |
| S2 | Every endpoint is authenticated unless explicitly marked public in the OpenAPI spec |
| S3 | Authorization is deny-by-default and checked in the service layer, not only in middleware |
| S4 | Every query touching user-owned data filters by the actor's user ID |
| S5 | All SQL is parameterised via sqlc. String-built SQL fails lint |
| S6 | All external input is validated at the edge and again as a domain invariant |
| S7 | No user-controlled value is ever interpolated into an LLM instruction block |
| S8 | Passwords are Argon2id; tokens are stored hashed; nothing sensitive is reversible without a key |
| S9 | Binary uploads go directly to object storage with a presigned, size- and type-pinned URL |
| S10 | Errors returned to clients never contain SQL, stack traces, provider messages, or internal IDs |
| S11 | Any change to auth, RBAC, payments, or upload handling requires a second reviewer |
| S12 | Dependencies are pinned and scanned; a known-exploitable CVE blocks release |

## 2. Authentication rules

| Aspect | Rule |
|---|---|
| Password storage | Argon2id, m=64 MiB, t=3, p=2; parameters stored in the hash; rehash on login if outdated |
| Password policy | ≥ 12 characters; rejected if found in a breach corpus (k-anonymity range query, or a local Bloom filter offline) |
| Login response | Identical message and comparable timing for "unknown email" and "wrong password" |
| Access token | JWT, 15 min, `sub`, `sid`, `role`, `jti`, `iat`, `exp`, `aud`, `iss`; no PII in claims |
| Refresh token | Opaque 256-bit random, stored as a SHA-256 hash, 30 days, single-use, rotating, grouped by `family_id` |
| Reuse detection | A used refresh token presented again revokes the whole family and raises a security event |
| Transport | Refresh in `HttpOnly; Secure; SameSite=Lax; Path=/api/v1/auth`; access token in memory only — **never** `localStorage` |
| Logout | Deletes the session, revokes the family, adds the `jti` to a short-lived denylist in Redis |
| MFA | TOTP with 10 single-use recovery codes; mandatory for `admin` |
| Verification & reset tokens | Single-use, 30-minute TTL, stored hashed, invalidated on use and on password change |
| Brute force | Per-IP and per-account counters in Redis with exponential backoff; CAPTCHA after 5 failures |
| Session listing | Users can see and revoke their sessions (device, IP city, last seen) |

## 3. Authorization rules

```
1. Route group     : /admin/* requires role=admin
2. Service guard   : authz.Require(ctx, "content.publish")
3. Ownership       : WHERE user_id = $actor   ← the one that actually stops IDOR
```

| Rule | Detail |
|---|---|
| Deny by default | An operation with no declared permission is rejected |
| Permission naming | `<resource>.<action>[.<qualifier>]` — `content.publish`, `content.read.published` |
| Admin ≠ omnipotent | Admin actions on user data are audited and, for sensitive reads, require a stated reason |
| Impersonation | Allowed for support, time-boxed, banner shown to the impersonating admin, fully audited, never for payment actions |
| Testing | Every user-owned resource has an integration test asserting that user B receives 404 for user A's resource |

## 4. Input handling

| Input | Controls |
|---|---|
| JSON body | Strict decoding (unknown fields rejected), size limit (1 MB default), depth limit |
| Query params | Whitelisted names and values; enums validated |
| Path params | UUID format validated before any lookup |
| File upload | Presigned URL pins content type and max size; server verifies the object after upload (`HEAD`, magic-byte sniff, duration probe) |
| Rich text | Sanitised server-side with a strict allowlist; rendered through DOMPurify client-side |
| Learner free text | Length-bounded; passed to LLMs only inside the untrusted-content wrapper |
| Webhooks | Signature verified on the raw body before parsing |
| CSV/bulk import (admin) | Row-count and size limits, streaming parse, per-row validation, dry-run mode |

## 5. Output and headers

| Header | Value |
|---|---|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'nonce-…'; img-src 'self' data: <cdn>; media-src 'self' <cdn>; connect-src 'self' <otlp>; frame-ancestors 'none'; base-uri 'none'; object-src 'none'` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), geolocation=(), microphone=(self)` — microphone is needed for speaking |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cache-Control` | `no-store` on authenticated responses |

CORS: explicit origin allowlist, `credentials: true` only on the refresh path, no wildcard.

## 6. Secrets

| Rule | Detail |
|---|---|
| Source | Environment variables, injected from Docker secrets or the platform's secret store |
| Never | In `.env` committed, in images, in CI logs, in error messages, in the frontend bundle |
| Rotation | Documented per secret in `docs/security/secrets.md`; JWT signing key rotation supports two active keys |
| Scanning | `gitleaks` pre-commit + CI; a hit fails the build and triggers rotation of whatever leaked |
| Access | Least privilege; production secrets accessible to two named people plus the deployment role |

## 7. AI-specific security

| Threat | Control |
|---|---|
| Prompt injection via learner content | Content wrapped in delimiters; system preamble declares wrapped content to be data; instructions inside it must be ignored and reported |
| Score manipulation | Output schema validation, rubric-bound clamping, cross-field consistency check, anomaly detection against the learner's history |
| Data exfiltration through model output | Output is schema-constrained; free-form fields are length-limited and sanitised before rendering |
| PII leaking to a provider | Redaction pass before send; only providers contractually barred from training on API data are used for learner content |
| Cost abuse | Per-user quota, global budget, rate limits, and per-request max-token caps |
| Model supply chain | Exact model IDs pinned; nightly evals detect silent behaviour changes |
| Jailbreak / abuse of the feedback channel | Moderation pass on input; flagged content routed to admin review, not graded |

## 8. Privacy operations

| Right | Implementation |
|---|---|
| Access / portability | `GET /me/export` → async job → signed link, 24 h expiry |
| Erasure | `DELETE /me` → 30-day grace → PII hard-deleted, learning statistics anonymised |
| Rectification | Profile editing |
| Objection to AI processing | Preference flag; disables AI grading only, with a clear explanation of what is lost |
| Voice data | Explicit consent before the first recording; 90-day retention; per-recording deletion |
| Minors | Age gate; under-16 requires a guardian email; no marketing communications; reduced retention |
| Breach | 72-hour notification process in `docs/operations/runbooks/security-incident.md` |

## 9. Secure development lifecycle

| Stage | Activity |
|---|---|
| Design | Threat model updated for any new external interface or data class |
| Code | Guidelines above; `gosec` in golangci-lint |
| Review | Security checklist in the PR template; second reviewer for S11 areas |
| CI | gitleaks, govulncheck, npm audit, CodeQL, Trivy, SBOM |
| Pre-release | Dependency review; ASVS checklist for the changed area |
| Production | Alerting on auth anomalies, admin actions, and rate-limit spikes |
| Periodic | Quarterly dependency audit; annual external penetration test |

## 10. PR security checklist

- [ ] No new secret, and no secret in a test fixture
- [ ] New endpoints declare and enforce a permission
- [ ] User-owned queries filter by actor
- [ ] Input validated at the edge and in the domain
- [ ] Errors leak nothing internal
- [ ] New external calls have timeout, retry bounds, and a circuit breaker
- [ ] New uploads use presigned URLs with pinned type and size
- [ ] New LLM input uses the untrusted-content wrapper
- [ ] Audit log entries added for admin or state-changing actions
- [ ] Rate limits considered for anything expensive
- [ ] Dependencies added are in `DEPENDENCIES.md` and scanned
