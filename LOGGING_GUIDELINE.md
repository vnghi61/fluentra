---
doc_type: guideline
scope: logging
last_verified: 2026-08-06
---

# LOGGING_GUIDELINE.md

Logging is one third of observability. Read this together with
[OBSERVABILITY_GUIDELINE.md](OBSERVABILITY_GUIDELINE.md) (§5) — this file goes deeper on
*how to write a log line*.

---

## 1. Stack

`log/slog` (stdlib) → `otelslog` bridge → OTLP → Collector → Loki → Grafana.
JSON to stdout. The application never writes log files, never rotates, never ships logs itself.

## 2. The shape of a good log line

```
{"time":"2026-08-06T10:14:22.913Z","level":"INFO","msg":"submission graded",
 "trace_id":"4bf92f...","span_id":"00f067...","request_id":"01J8XQ7Z...",
 "module":"writing","service":"fluentra-worker","env":"production","version":"v1.4.2",
 "user_id":"0193a7c1-...","submission_id":"0193a7c2-...",
 "task":"writing.grade_essay","prompt_version":3,"provider":"anthropic",
 "band_overall":6.5,"duration_ms":18421}
```

| Property | Rule |
|---|---|
| `msg` | A short, constant, lowercase phrase. It is effectively an event name — you will group by it |
| Variables | Always attributes, never interpolated into `msg` |
| Naming | `snake_case` attribute keys, consistent across modules (`user_id`, never `userId` or `uid`) |
| Types | Numbers as numbers, booleans as booleans, durations as `*_ms` integers |
| Size | A record over 8 KB is truncated with `truncated: true` |

**Bad:** `slog.Info(fmt.Sprintf("graded submission %s for user %s in %v", id, uid, d))`
Un-groupable, un-queryable, and the user ID ends up in the message index.

## 3. Levels — the decision rule

| Ask | Level |
|---|---|
| Would an engineer act on this at 3 a.m.? | `error` |
| Did something degrade but the system handled it? | `warn` |
| Is this a meaningful state change someone might audit or debug later? | `info` |
| Is this only useful while developing? | `debug` |

Common mistakes: logging every function entry at `info`; logging user input validation
failures at `error`; using `warn` for things nobody will ever look at.

## 4. What to log — by situation

| Situation | Level | Required attributes |
|---|---|---|
| User registered | info | `user_id`, `source` |
| Login succeeded / failed | info | `user_id` (or `email_hash` on failure), `ip_country`, `reason` |
| Refresh reuse detected | warn | `user_id`, `family_id` |
| Permission denied | warn | `user_id`, `permission`, `route` |
| Rate limit hit | info | `limiter`, `subject_hash`, `route` |
| Content published | info | `content_id`, `version`, `actor_id` |
| Submission graded | info | `submission_id`, `task`, `prompt_version`, `provider`, `duration_ms` |
| AI provider fallback | warn | `task`, `from_provider`, `to_provider`, `reason` |
| AI schema violation | warn | `task`, `prompt_version`, `attempt` |
| Cache unavailable | warn | `module`, `op` |
| Job failed (will retry) | warn | `kind`, `attempt`, `max_attempts`, `error_code` |
| Job failed (final) | error | `kind`, `job_id`, `error_code` |
| Outbox publish failure | error | `event_id`, `attempt` |
| Migration applied | info | `version`, `duration_ms` |
| Startup / shutdown | info | `version`, `config_digest` |
| Panic recovered | error | `route`, stack trace |

## 5. What must never be logged

Passwords · access, refresh, verification, or reset tokens · session cookies · API keys ·
full JWTs · payment card data · full essay or transcript text · audio bytes · email addresses,
phone numbers, or full names · a learner's exact score history in aggregate exports.

**Enforcement:** a `slog.Handler` wrapper applies an **allowlist**. An attribute key not in the
allowlist is replaced with `"[redacted]"`. Adding a key to the allowlist is a reviewed change.
This means a new field is redacted by default — fail closed, not open.

For identity, log `user_id` (an opaque UUID). To debug an email-specific issue, log
`email_hash` (SHA-256, truncated), never the address.

## 6. Correlation

Every log record carries `trace_id`, `span_id`, and `request_id`. These are attached
automatically by the middleware and the `otelslog` bridge — **do not add them by hand**.

For background work, the enqueuing request's `trace_id` and `request_id` travel in the job
payload and are re-attached by the job middleware, so a user's click and the worker's log lines
share a trace.

Grafana is configured with a derived field: clicking `trace_id` in a Loki line opens the trace
in Tempo.

## 7. Volume control

| Technique | When |
|---|---|
| Log once per operation, at the boundary | Always |
| Sample repetitive `info` (e.g. cache miss) | Above 100/s per key, sample at 1 % |
| Aggregate loop results | Log a summary, not a line per item |
| `debug` disabled in production | Enabled per-module at runtime via a feature flag for time-boxed investigation |
| Rate-limit error logs | Deduplicate identical errors within a 1-minute window with a `count` attribute |

Budget: an API replica should produce well under 100 lines/second at steady state. If it does
not, the cause is almost always logging inside a loop or logging an error at every layer.

## 8. Retention and access

| Stream | Hot (Loki) | Archive |
|---|---|---|
| Application | 30 days | 90 days in MinIO |
| Security events | 90 days | 2 years |
| Audit (in Postgres, not Loki) | — | 2 years |
| Access logs | 14 days | 30 days |

Access to production logs is role-restricted and itself audited.

## 9. Frontend logging

| Rule | Detail |
|---|---|
| Console | `console.log` is banned in committed code (lint); `console.error` allowed in the error boundary |
| Telemetry | Errors and key interactions go through the OTel Web SDK to the Collector |
| PII | Never send form contents, essay text, or audio; send IDs and error codes |
| Sampling | 100 % of errors, 5 % of navigation traces |
| Consent | Product analytics respects the user's preference; error telemetry is essential-service and documented in the privacy notice |

## 10. Review checklist

- [ ] `msg` is constant and lowercase; variables are attributes
- [ ] Attribute keys match existing conventions
- [ ] Level matches the decision rule in §3
- [ ] No PII, no secret; new keys added to the allowlist deliberately
- [ ] Not inside a hot loop
- [ ] The error is logged exactly once across the whole call chain
- [ ] Enough context to debug without reproducing (IDs, not text)
