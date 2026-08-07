---
doc_type: guideline
scope: errors
last_verified: 2026-08-06
---

# ERROR_HANDLING.md

One error model, from the domain to the browser.

---

## 1. The model

```
domain error  →  apperr.Error  →  Problem Details JSON  →  typed frontend error  →  user message
   (sentinel)      (typed, coded)     (RFC 9457)              (discriminated union)   (i18n catalogue)
```

`shared/apperr.Error` carries:

| Field | Purpose | Exposed to client? |
|---|---|---|
| `Kind` | Category → HTTP status (see §2) | as status |
| `Code` | Stable machine code (`DECK_NOT_FOUND`) | ✅ |
| `Message` | Safe, human-readable, localisable | ✅ |
| `Fields` | Per-field validation failures | ✅ |
| `Meta` | Safe structured context (`{limit: 5}`) | ✅ |
| `Cause` | Wrapped underlying error | ❌ logs only |
| `Internal` | Developer detail, SQL state, provider message | ❌ logs only |
| `Retryable` | Hint for the caller and for job retry logic | via `Retry-After` |

## 2. Kinds → status

| Kind | HTTP | Meaning | Logged at |
|---|---|---|---|
| `Validation` | 422 | Input is well-formed but semantically invalid | debug |
| `BadRequest` | 400 | Malformed request | debug |
| `Unauthenticated` | 401 | No or invalid credentials | info |
| `Forbidden` | 403 | Authenticated, not permitted | warn |
| `NotFound` | 404 | Absent, or not visible to this actor | debug |
| `Conflict` | 409 | State conflict | info |
| `PreconditionFailed` | 412 | `If-Match` mismatch | info |
| `TooLarge` | 413 | Payload exceeds limit | info |
| `RateLimited` | 429 | Quota or rate limit | info |
| `Unavailable` | 503 | Dependency down | error |
| `Timeout` | 504 | Upstream timeout | error |
| `Internal` | 500 | Bug or unexpected condition | error |

Anything not an `apperr.Error` at the HTTP boundary is treated as `Internal`, logged with a
stack trace, and rendered as a generic 500. **The client never sees an unclassified message.**

## 3. Layer responsibilities

| Layer | Does |
|---|---|
| `domain` | Returns sentinel errors expressing invariants (`ErrDeckLimitReached`) |
| `repository` | Translates driver errors: `pgx.ErrNoRows` → `apperr.NotFound`, unique violation `23505` → `apperr.Conflict`, FK violation `23503` → `apperr.Conflict`; wraps everything else with query context |
| `service` | Maps domain sentinels to `apperr` with a code and a user-safe message; adds `Meta` |
| `transport/http` | Renders Problem Details; never invents an error; never adds business meaning |
| `job` | Decides retry from `Retryable`; on final failure records a safe reason and emits a failure event |
| frontend | Maps `code` → message via the i18n catalogue; falls back to `title` |

## 4. Wrapping

```go
// repository: add query context
return fmt.Errorf("select deck %s: %w", id, err)

// service: classify at the module boundary
if errors.Is(err, domain.ErrDeckLimitReached) {
    return apperr.Conflict("DECK_LIMIT_REACHED", "You have reached the deck limit for your plan.").
        WithMeta("limit", limit).
        WithCause(err)
}
```

Rules: wrap with `%w` exactly once per layer · never wrap an `apperr` in another `apperr` ·
never `errors.New(fmt.Sprintf(...))` · never log **and** return the same error (the boundary
logs it once).

## 5. Error code catalogue

Codes are permanent public API. Adding one is a normal change; changing or removing one is a
breaking change.

### Common

| Code | Kind | Meaning |
|---|---|---|
| `VALIDATION_FAILED` | 422 | One or more fields invalid; see `errors[]` |
| `MALFORMED_REQUEST` | 400 | Body is not valid JSON or has unknown fields |
| `INVALID_CURSOR` | 400 | Pagination cursor is not decodable |
| `RESOURCE_NOT_FOUND` | 404 | Generic not-found |
| `PERMISSION_DENIED` | 403 | Missing permission |
| `RATE_LIMITED` | 429 | Too many requests |
| `IDEMPOTENCY_KEY_REUSE` | 409 | Same key, different payload |
| `PRECONDITION_FAILED` | 412 | ETag mismatch |
| `DEPENDENCY_UNAVAILABLE` | 503 | Downstream unavailable |
| `INTERNAL_ERROR` | 500 | Unexpected |

### auth

| Code | Kind | Meaning |
|---|---|---|
| `INVALID_CREDENTIALS` | 401 | Wrong email or password (never say which) |
| `ACCOUNT_LOCKED` | 429 | Too many failed attempts |
| `EMAIL_NOT_VERIFIED` | 403 | Verification required for this action |
| `TOKEN_EXPIRED` | 401 | Access token expired — refresh |
| `TOKEN_INVALID` | 401 | Malformed or bad signature |
| `SESSION_REVOKED` | 401 | Session revoked (includes refresh-reuse detection) |
| `MFA_REQUIRED` | 401 | Second factor needed |
| `MFA_INVALID` | 401 | Wrong TOTP code |
| `EMAIL_ALREADY_REGISTERED` | 409 | Registration conflict |
| `PASSWORD_TOO_WEAK` | 422 | Fails policy or appears in a breach list |
| `SESSION_ABSOLUTE_EXPIRED` | 401 | Absolute session lifetime reached; full re-authentication required |
| `DEVICE_LIMIT_REACHED` | 409 | Too many trusted devices |

### auth — one-time codes (ADR-0021)

| Code | Kind | Meaning |
|---|---|---|
| `OTP_INVALID` | 401 | Wrong code; response carries `attempts_remaining` |
| `OTP_EXPIRED` | 401 | Code older than its 10-minute TTL |
| `OTP_ATTEMPTS_EXCEEDED` | 429 | Challenge burned after 5 attempts; a new one must be requested |
| `OTP_RESEND_TOO_SOON` | 429 | Within the 60-second cooldown; `Retry-After` set |
| `CHALLENGE_NOT_FOUND` | 404 | Unknown, consumed or expired challenge |

### auth — OAuth (ADR-0023)

| Code | Kind | Meaning |
|---|---|---|
| `OAUTH_STATE_INVALID` | 400 | Missing, reused or expired state — possible CSRF; raises a security event |
| `OAUTH_EMAIL_UNVERIFIED` | 403 | The provider did not assert a verified email |
| `OAUTH_ACCOUNT_CONFLICT` | 409 | The address belongs to an unverified local account; verify by OTP first |
| `OAUTH_EMAIL_MISMATCH` | 409 | Linking attempted with an address other than the account's |
| `OAUTH_ALREADY_LINKED` | 409 | That provider identity is linked to another account |
| `LAST_SIGN_IN_METHOD` | 409 | Unlinking would leave the account with no way to sign in |

### learning / skills

| Code | Kind | Meaning |
|---|---|---|
| `LESSON_LOCKED` | 403 | Prerequisites not met |
| `ACTIVITY_ALREADY_COMPLETED` | 409 | Re-submission not allowed |
| `ATTEMPT_EXPIRED` | 409 | Time limit exceeded |
| `DECK_LIMIT_REACHED` | 409 | Plan limit |
| `WORD_ALREADY_IN_DECK` | 409 | Duplicate |
| `REVIEW_CARD_SUSPENDED` | 409 | Card is suspended |
| `SUBMISSION_TOO_SHORT` / `_TOO_LONG` | 422 | Outside task bounds |
| `AUDIO_TOO_LONG` | 422 | Recording exceeds the limit |
| `UNSUPPORTED_AUDIO_FORMAT` | 415 | Bad mime type |
| `PLAY_LIMIT_REACHED` | 403 | Listening replay policy |
| `EXAM_ALREADY_SUBMITTED` | 409 | Terminal state |
| `EXAM_WINDOW_CLOSED` | 403 | Outside the availability window |

### AI

| Code | Kind | Meaning |
|---|---|---|
| `AI_QUOTA_EXCEEDED` | 429 | Per-user daily quota |
| `AI_BUDGET_EXCEEDED` | 503 | Global budget cap reached |
| `AI_UNAVAILABLE` | 503 | All providers in the chain failed |
| `AI_TIMEOUT` | 504 | Provider timed out |
| `AI_OUTPUT_INVALID` | 500 | Output failed schema validation after repair |
| `AI_CONTENT_FLAGGED` | 422 | Moderation blocked the input |
| `AI_OPTED_OUT` | 403 | User disabled AI processing |

### content / admin

| Code | Kind | Meaning |
|---|---|---|
| `CONTENT_NOT_PUBLISHED` | 404 | Draft not visible to learners |
| `INVALID_STATE_TRANSITION` | 409 | e.g. archived → published |
| `CONTENT_IN_USE` | 409 | Referenced by a published lesson |
| `SELF_APPROVAL_FORBIDDEN` | 403 | An author cannot approve their own content |

### billing

| Code | Kind | Meaning |
|---|---|---|
| `PAYMENT_FAILED` | 402 | Gateway declined |
| `SUBSCRIPTION_ALREADY_ACTIVE` | 409 | Duplicate subscribe |
| `PLAN_NOT_AVAILABLE` | 409 | Plan withdrawn or region-restricted |
| `REFUND_WINDOW_CLOSED` | 409 | Outside the refund period |

## 6. Frontend handling

| Situation | UI behaviour |
|---|---|
| 422 with `errors[]` | Map each `field` to the form field; focus the first |
| 401 `TOKEN_EXPIRED` | Silent refresh (single-flight), retry once; on failure redirect to login preserving the route |
| 401 `SESSION_REVOKED` | Clear state, redirect to login with an explanatory toast |
| 403 | Inline "you don't have access" panel — never a blank page |
| 404 | Route-level not-found view |
| 409 | Contextual toast plus a refetch of the affected query |
| 429 | Disable the action, show a countdown from `Retry-After` |
| 5xx | Error boundary with a retry button and the `request_id` shown for support |
| Network offline | Offline banner; queue the mutation where the feature supports drafts |

Every `code` has an entry in `web/src/lib/errors/catalogue.ts`; a missing entry falls back to
`title` and logs a warning so the gap is visible.

## 7. Logging policy for errors

| Kind | Level | Includes |
|---|---|---|
| 4xx caused by the client | `debug`/`info` | code, path, user_id, request_id |
| 403 | `warn` | plus the permission that was required |
| 429 | `info` | plus the limiter that fired |
| 5xx | `error` | plus cause chain and stack, never PII |

An error is logged **once**, at the boundary that renders or retries it. Logging at every layer
produces noise and makes the real failure harder to find.

## 8. Retry semantics

| Context | Rule |
|---|---|
| HTTP client → API | Retry only idempotent methods, or POSTs carrying an `Idempotency-Key`, on 429/503/504 |
| Job → external service | Retry on `Retryable` errors with exponential backoff + jitter, capped attempts |
| Job → business rejection | Do **not** retry; fail the job, record a reason, emit a failure event |
| Database serialisation failure | Retry the whole transaction up to 3 times |
| Provider chain | On a non-retryable provider error, move to the next provider rather than retrying the same one |

## 9. Anti-patterns

| Anti-pattern | Why it is wrong |
|---|---|
| `if err != nil { return nil }` | Silently produces wrong data |
| Returning `500` for a validation failure | Pages the on-call for a user typo |
| Leaking `pq: duplicate key value violates unique constraint "uq_decks_user_slug"` | Discloses schema and confuses the user |
| A generic `"something went wrong"` with no code | Un-debuggable, un-branchable |
| Different codes for the same condition across modules | Clients cannot handle it uniformly |
| Retrying a 422 | Wastes budget and never succeeds |
| Using an error to control normal flow | Errors are for exceptional conditions |
