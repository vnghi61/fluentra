---
doc_type: third_party_register
project: fluentra
last_verified: 2026-08-06
---

# THIRD_PARTY.md

External **services** we integrate with (as opposed to libraries — those are in
[DEPENDENCIES.md](DEPENDENCIES.md)). Every entry records what data leaves our system, what
happens when the service fails, and how we would replace it.

---

## 1. Register

| Service | Purpose | Data sent | Criticality | Replaceable in | Contract |
|---|---|---|---|---|---|
| Anthropic API | Essay grading, grammar explanation, feedback | Learner-written text (PII-redacted), rubric, locale | High | days (adapter) | Commercial, no training on API data |
| OpenAI API | Fallback grading, embeddings, TTS, Whisper ASR | Same as above; audio for ASR | High | days | Commercial, no training on API data |
| Google Gemini API | High-volume, low-cost tasks | Content text | Medium | days | Commercial |
| OpenRouter | Model experimentation | Content text | Low | immediately | Pass-through — **not used for learner PII** |
| Speech provider (TBD: Azure Speech / self-hosted Whisper) | ASR + pronunciation assessment | Learner voice recordings | High (speaking only) | weeks | Pending — see plan review Q2 |
| Payment gateway (TBD: VNPay / MoMo / Stripe) | Subscriptions | Name, email, billing country. **No card data touches us** | High (Phase 4) | weeks | Pending — see plan review Q1 |
| Email provider (SMTP or API) | Verification, reminders, reports | Email address, name, learning summary | Medium | days | Standard DPA |
| Web push (VAPID / FCM) | Streak reminders | Device token | Low | days | — |
| Breached-password service (HIBP range API) | Password policy | First 5 chars of a SHA-1 hash — **never the password** | Low | immediately | Free, k-anonymity |
| GitHub | Source, CI, registry | Source code, build metadata | High (dev only) | weeks | Commercial |
| Error/uptime monitoring (optional) | External availability check | URL probes only | Low | immediately | — |

## 2. Data classification sent externally

| Class | Examples | Where it may go |
|---|---|---|
| **Public** | Published lesson content | Any provider |
| **Internal** | Aggregate statistics | Any provider under contract |
| **Personal** | Email, name, locale | Email provider, payment gateway only |
| **Sensitive personal** | Essay text, voice recordings, learning weaknesses | LLM/speech providers with no-training terms only; redacted where possible |
| **Secret** | Passwords, tokens, keys | **Never leaves the system** |

`OpenRouter` is explicitly restricted to non-personal content because it is a pass-through to
third parties whose terms we do not individually negotiate.

## 3. Failure handling per service

| Service down | User-visible behaviour | Internal behaviour |
|---|---|---|
| Primary LLM | None immediately — grading continues via fallback | `ai_fallback_total` increments; warn log |
| All LLMs | "We're still grading this — check back shortly" | Jobs retry with backoff; page after 5 min of total failure |
| Speech provider | Speaking submissions queue; recording still saved | Circuit breaker open; other skills unaffected |
| Payment gateway | Checkout unavailable; existing subscriptions unaffected | Webhooks replayed when it returns |
| Email provider | Signup verification delayed | Queued; support can resend manually |
| Push | Silent | Best-effort, no retry |
| GitHub | No deploys | Local development unaffected |

Every one of these is exercised by a chaos test in staging at least once per quarter.

## 4. Cost exposure

| Service | Cost driver | Control |
|---|---|---|
| LLM APIs | Tokens × model tier | Task routing, caching, per-user quota, global daily budget, 80 % alert |
| Speech | Audio minutes | Max recording length, per-user daily limit |
| Email | Messages sent | Digest batching, preference-respecting sends |
| Object storage egress | Media delivery | CDN in front, long cache TTLs on immutable assets |
| CI minutes | Pipeline duration | Path filters, caching, nightly scheduling of expensive suites |

The AI line is expected to dominate. It is metered per task, per provider, per user, and shown
on a dashboard — see `OBSERVABILITY_GUIDELINE.md` §4.2.

## 5. Adding a third-party service

Required before the first call in production:

- [ ] An ADR justifying the choice, with at least two alternatives
- [ ] An adapter behind an interface in `internal/platform/` — SDK types never leak
- [ ] Timeout, retry bounds, and a circuit breaker
- [ ] A `mock` implementation for tests
- [ ] Failure behaviour defined and user-facing copy written
- [ ] Data classification recorded in §2
- [ ] Contract review: data processing terms, training on data, retention, sub-processors, region
- [ ] Cost model and a budget alert
- [ ] Secrets stored properly with a rotation procedure
- [ ] A row in this table and in `DEPENDENCIES.md` if an SDK was added
- [ ] Privacy notice updated if personal data is involved

## 6. Licence posture (libraries)

| Licence | Policy |
|---|---|
| MIT, BSD, Apache-2.0, ISC | ✅ Allowed |
| MPL-2.0, LGPL | ⚠️ Allowed only as an unmodified dynamic dependency; reviewed case by case |
| GPL, AGPL | ❌ Not in anything we distribute or run as a service, without legal sign-off |
| Unlicensed / unclear | ❌ Not used |

`go-licenses` and `license-checker` run in CI and fail on a policy violation.
An SBOM (CycloneDX) is generated per release and attached to the GitHub Release.
