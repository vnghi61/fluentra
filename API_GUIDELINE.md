---
doc_type: guideline
scope: http_api
last_verified: 2026-08-06
---

# API_GUIDELINE.md

The contract is [`api/openapi/openapi.yaml`](api/openapi/openapi.yaml). This document explains
the rules that spec must follow. **Edit the spec before writing a handler** (rule L10).

---

## 1. Fundamentals

| Aspect | Rule |
|---|---|
| Style | REST, resource-oriented, JSON only |
| Base path | `/api/v1` |
| Versioning | Major version in the path. One major version supported at a time; the previous one gets 6 months of deprecation notice |
| Media type | `application/json`; errors use `application/problem+json` |
| Encoding | UTF-8 |
| Casing | Paths kebab-case, JSON fields `snake_case`, query params `snake_case` |
| Resource names | Plural nouns: `/decks`, `/submissions`, `/review-cards` |
| Verbs in paths | Only for non-CRUD actions, as a sub-resource: `POST /exams/{id}/submit` |
| Nesting depth | Maximum 2 levels: `/courses/{id}/lessons` is fine; `/courses/{id}/lessons/{id}/activities/{id}/attempts` is not — use `/attempts?activity_id=` |
| Trailing slash | Not accepted |
| Nulls | Omit absent optional fields rather than sending `null`, except when `null` is semantically meaningful |
| Unknown fields | Rejected with `422` (strict decoding) |

## 2. Methods and status codes

| Method | Use | Success | Idempotent |
|---|---|---|---|
| `GET` | Read | 200, 304 | ✅ |
| `POST` | Create, or an action | 201 (+`Location`), 202, 200 | ❌ (unless `Idempotency-Key`) |
| `PUT` | Full replace | 200, 204 | ✅ |
| `PATCH` | Partial update (JSON Merge Patch) | 200 | ❌ |
| `DELETE` | Remove | 204 | ✅ |

| Status | When |
|---|---|
| 200 | OK with a body |
| 201 | Created — `Location` header required |
| 202 | Accepted — long-running; body has a job/stream reference |
| 204 | No content |
| 304 | `If-None-Match` matched |
| 400 | Malformed request (bad JSON, bad cursor) |
| 401 | Missing/invalid/expired credentials |
| 403 | Authenticated but not permitted |
| 404 | Not found **or** not visible to this actor (never leak existence) |
| 409 | State conflict (already submitted, duplicate lemma) |
| 412 | `If-Match` failed |
| 413 | Payload too large |
| 415 | Unsupported media type |
| 422 | Semantically invalid (validation failures) |
| 429 | Rate limited — `Retry-After` required |
| 500 | Unexpected — never leaks internals |
| 503 | Dependency unavailable — `Retry-After` when known |

**404 vs 403:** if the actor may not know the resource exists, return 404. Use 403 only when
existence is already public knowledge to that actor.

## 3. Standard headers

### Request

| Header | Notes |
|---|---|
| `Authorization: Bearer <jwt>` | All authenticated endpoints |
| `X-Request-Id` | Optional client-supplied ULID; generated if absent; echoed in the response |
| `traceparent` | W3C trace context; propagated end-to-end |
| `Idempotency-Key` | Required on POSTs that create attempts, submissions, or charge money |
| `If-Match` / `If-None-Match` | Optimistic concurrency and caching on admin resources |
| `Accept-Language` | Drives message localisation |

### Response

| Header | Notes |
|---|---|
| `X-Request-Id` | Always |
| `ETag` | On cacheable and mutable resources |
| `Cache-Control` | `no-store` by default for authenticated responses |
| `RateLimit-Limit` / `RateLimit-Remaining` / `RateLimit-Reset` | On rate-limited endpoints |
| `Retry-After` | With 429 and 503 |
| `Deprecation` / `Sunset` | On deprecated operations |

## 4. Pagination

Cursor-based for everything user-facing.

```
GET /api/v1/vocabulary/decks?limit=20&cursor=eyJjIjoiMjAyNi0wOC0wNlQxMDowMDowMFoiLCJpIjoiMDE5M2E3In0

{
  "data": [ … ],
  "page": {
    "next_cursor": "eyJjIjoiMjAyNi0wOC0wNlQwOTowMDowMFoiLCJpIjoiMDE5M2E2In0",
    "has_more": true,
    "limit": 20
  }
}
```

| Rule | Detail |
|---|---|
| `limit` | default 20, max 100 |
| Cursor | Opaque base64 of `{sort_value, id}`; clients must not construct one |
| Ordering | Stable and explicit; ties broken by `id` |
| Total counts | Not returned by default (expensive). `?include_total=true` allowed only on admin lists |
| Offset pagination | Permitted only on admin tables with a bounded row count, and documented as such |

## 5. Filtering, sorting, sparse fields

| Feature | Syntax | Rule |
|---|---|---|
| Filter | `?status=published&level=B1` | Explicit, whitelisted params only. No generic query language |
| Range | `?created_after=…&created_before=…` | ISO-8601 |
| Search | `?q=` | Server decides the strategy |
| Sort | `?sort=-created_at,name` | Whitelisted fields only |
| Expand | `?expand=deck,author` | Whitelisted; max 2 |
| Fields | `?fields=id,name` | Optional; ignore unknown fields |

## 6. Errors

RFC 9457 Problem Details, always. Full catalogue in [ERROR_HANDLING.md](ERROR_HANDLING.md).

```json
{
  "type": "https://fluentra.dev/errors/deck-limit-reached",
  "title": "Deck limit reached",
  "status": 409,
  "detail": "Free plan allows up to 5 decks.",
  "instance": "/api/v1/vocabulary/decks",
  "code": "DECK_LIMIT_REACHED",
  "request_id": "01J8XQ…",
  "meta": { "limit": 5, "current": 5 }
}
```

| Rule | Detail |
|---|---|
| `code` | Stable, `SCREAMING_SNAKE_CASE`, never changes once released — clients branch on it |
| `title` / `detail` | Human-readable, safe to show, localised by `Accept-Language` |
| `errors[]` | Present on 422 with `field`, `code`, `message` per failure |
| Internals | Stack traces, SQL, provider messages **never** appear in a response; they go to logs with the same `request_id` |

## 7. Authentication & authorisation

| Aspect | Rule |
|---|---|
| Access token | `Authorization: Bearer`, JWT, 15 min |
| Refresh | `POST /auth/refresh`, cookie-based, rotating |
| Public endpoints | Explicitly marked in the spec with `security: []`; everything else is authenticated by default |
| Admin endpoints | Under `/admin/`; require the `admin` role at the route group |
| Permission | Every operation declares the permission it requires in an `x-permission` extension; CI checks the handler enforces it |
| Ownership | Enforced in the query, not the handler |

## 8. Idempotency

```
POST /api/v1/writing/submissions
Idempotency-Key: 01J8XQ7Z9K3M4N5P6Q7R8S9T0V
```

- The key is stored with a hash of the request body and the resulting response.
- A replay with the same key **and** same body returns the original response with
  `Idempotency-Replayed: true`.
- Same key, different body → `409 IDEMPOTENCY_KEY_REUSE`.
- Keys expire after 24 h.
- Required on: submissions, exam attempts, payments, any credit-consuming AI call.

## 9. Long-running operations

```
POST /writing/submissions        → 202 { submission_id, stream_url, poll_url }
GET  /writing/submissions/{id}   → 200 { status: queued|processing|graded|failed, … }
GET  /writing/submissions/{id}/stream  → text/event-stream
```

| Rule | Detail |
|---|---|
| Never block | An operation expected to exceed 2 s returns 202 |
| SSE | Events: `progress`, `chunk`, `done`, `error`; heartbeat every 15 s; client reconnects with `Last-Event-ID` |
| Terminal states | `graded` and `failed` are terminal; a failed job exposes a safe reason code |
| Polling | Allowed as a fallback with `Retry-After` guidance |

## 10. Webhooks (inbound, payment)

| Rule | Detail |
|---|---|
| Verification | Signature verified before parsing; raw body preserved |
| Idempotency | Provider event ID stored; duplicates acknowledged without reprocessing |
| Response | 2xx quickly; the work happens in a job |
| Replay | Admin can replay a stored webhook from the audit trail |

## 11. Rate limits (initial)

| Class | Limit |
|---|---|
| Anonymous per IP | 60 req/min |
| Authenticated per user | 600 req/min |
| `/auth/login`, `/auth/register`, `/auth/forgot-password` | 5/min per IP **and** per account, with exponential lockout |
| AI-consuming endpoints | Per-plan daily quota + 10/min burst |
| Upload intents | 30/hour |
| Admin bulk operations | 10/min |

## 12. Deprecation policy

1. Mark the operation `deprecated: true` in the spec with a `Sunset` date.
2. Emit `Deprecation` and `Sunset` response headers.
3. Log and meter usage per client.
4. Announce in `CHANGELOG.md`.
5. Remove no earlier than 6 months after announcement, and only when usage is zero for 30 days.

## 13. Spec conventions

| Rule | Detail |
|---|---|
| `operationId` | `camelCase`, unique, `<module><Action><Resource>` — e.g. `vocabularyCreateDeck`. It becomes the generated function name |
| Tags | One per module, matching the module name |
| Components | Shared schemas in `components/`, split per module file |
| Examples | Every operation has at least one request and response example — these generate MSW handlers |
| Descriptions | Every operation and every non-obvious field has one |
| `x-permission` | Required on every non-public operation |
| Lint | `spectral` with the repo ruleset runs in CI |
