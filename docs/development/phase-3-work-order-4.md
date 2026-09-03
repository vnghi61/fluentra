---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-03
---

# Phase 3 — work order 4

**Purpose.** The work after PR #70, with two more decisions taken. It replaces §2 of
[phase-3-work-order-3.md](phase-3-work-order-3.md).

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps — the two codegen gates, the
migration numbering, the stale TODO checkboxes — and they are all still current.

---

## 1. Where this leaves off

`main` is green at `96424c4` (PR #70): all five workflows passed.

| WP | State |
|---|---|
| WP12 — seed beyond vocabulary | not started |
| WP13 — dictionary autocomplete | done |
| WP14 — gamification | done, XP now follows the recorded rule |
| WP15 — `platform/ai` | done: router, DB cache, usage, pruner, all wired |
| WP18 — quota, warnings, queue | budget and queue done; **two pieces missing** |
| WP16 — learner words | tables only |
| WP17 — explanations | not started |
| WP19 — admin | needs the quota view below |
| WP20 / WP21 — four skills | not started |

### What WP18 actually has

Budgets are per `(provider, task)`, the router reports `ErrQuotaExhausted` only when every
provider is out, a check that errors fails closed with a structured alert, and
`vocabulary/service/upload.go` degrades an unverifiable word into a reviewable flashcard
rather than failing the upload. That is most of the work order and it holds up.

### What it is missing, and why it matters

**Nothing performs the enrichment the learner was promised.** The queued path writes this note:

> Queued for background enrichment. Your flashcard is ready to review.

No job selects those rows. No code anywhere reads the `queued` marker. The word stays exactly
as it was written for ever, and the note is a promise with nothing behind it.

**Nothing shows an admin the quota.** The budget check logs at warn and error, and that is
all. "The admin sees it before the learner does" was the acceptance and there is no surface
that could.

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| What comes next | Close WP18 before WP16/WP17 |
| A word saved while quota is out | Mark it unverified, do not assert what nothing checked |
| Production migrations | The owner runs them; see §5 |
| AI provider | Not configured yet; see §4 |

---

## 3. The work, in order

### 1. Give `queued` a state of its own

Today the quota path calls `MarkUploadItemVerified`, which sets `status = 'verified'`, and
then passes the string `queued` as `verified_by_model`. So the row says verified and the model
column says otherwise, and the only way to find these words is to look for a sentinel in a
column that means something else.

`skill.vocab_upload_items.status` allows `pending, verified, rejected, failed`. Add `queued`
to that CHECK and set it, so the state is a state.

**And stop asserting what nothing verified.** The fallback currently writes:

```go
Valid: true, MeaningMatches: true, CEFRLevel: "B1",
PartOfSpeech: firstNonEmpty(entry.PartOfSpeech, "noun"),
```

`Valid` and `MeaningMatches` are the two questions the model exists to answer, and here they
are answered without asking it. `B1` is invented outright, and `noun` is a guess when the
dictionary had nothing. This is the same fault as the XP rule's fabricated perfect score,
caught in review last round: a value nobody produced, stored where later code will trust it.

Keep the word — the learner should not lose it — but record what is actually known: the term,
whatever the dictionary supplied, and the learner's own meaning. Leave the model's fields
empty, and let the state say why they are empty.

**Done means:** a word saved while quota is out has `status = 'queued'`, no CEFR level nobody
assigned, and is still reviewable as a flashcard.

### 2. The enrichment job

A cron job that selects `status = 'queued'` and re-runs verification. Four retention and
maintenance jobs already exist to copy the shape from — `ai.prune_expired_cache` is the most
recent and the closest.

**It must not become the thing that empties the budget.** A job that wakes up and re-verifies
every queued word will exhaust the day's quota in one sweep and starve the learners who are
uploading right now. Take a bounded batch, and stop the sweep as soon as `CheckQuota` says no
rather than grinding through the rest of the batch to be refused each time.

**A word that fails re-verification is not a word to retry for ever.** Decide what happens on
the third or fourth attempt — reject it, or leave it queued and stop counting — and record the
choice in `DECISIONS.md`. An unbounded retry against a paid API is a bill with no ceiling.

**Done means:** quota returns, the job enriches queued words in bounded batches, a word that
is genuinely invalid ends up rejected rather than retried for ever, and the sweep yields to
live traffic instead of consuming the budget.

### 3. The admin quota view

`GET /api/v1/admin/ai/usage` or similar: per provider and task, today's request and token
counts against the configured limits, and which budgets are exhausted.

Everything it needs is already recorded — `ai.ai_usage` aggregates by
`(provider, model, task, usage_date)` and `ai.ai_budgets` holds the limits. This is a read.

**OpenAPI first, then the handler.** And regenerate both sides: `make gen-api` *and*
`make gen-web`, then `make gen-check` *and* `make gen-check-web`, and commit the result. That
sequence has cost two CI rounds already.

**Done means:** an admin can see, before any learner is affected, that a provider is close to
its ceiling.

### 4. Then WP16, then WP17

Learner-contributed words, then answer explanations. This is what the project is for, and the
ceiling now exists to make it safe.

---

## 4. Configuring a real provider

The default is `ai.provider = mock`, and the mock **accepts every word it is given**. Nothing
is being validated in any environment today.

The worker reads five keys, all overridable by environment variable:

```bash
AI_PROVIDER=openai_compatible
AI_BASE_URL=<the provider's OpenAI-compatible endpoint>
AI_MODEL=<model id>
AI_API_KEY=<key>
AI_TIMEOUT=120s
```

`openai_compatible` is the only real adapter, which is deliberate: it covers Ollama,
OpenRouter, Groq, LM Studio and vLLM without an adapter each. A misconfigured
`openai_compatible` — no base URL, no model — is an error rather than a silent fall back to
the mock, so a broken deployment says so instead of quietly accepting every word.

`ai.New` also takes fallback provider settings, so a second free provider can be configured as
the fallback the router already knows how to use.

**Seed `ai.ai_budgets` before the first real call.** A provider with no budget row is
permitted without limit — `CheckQuota` returns true when it finds no row. That is the right
default for a table nobody has filled in, and it means the ceiling does not exist until a row
says so. One row per `(provider, task)`.

### Verify the mock is gone

The worker logs a warning at start-up when it falls back:

```text
ai: using the mock provider; verification accepts every word it is given
```

If that line is in the production log, no word is being checked by anything.

---

## 5. Production is behind `main`

There is no auto-migration on boot; migrations are a manual step. Pending:

| Version | Adds |
|---|---|
| `1700000260` | gamification tables |
| `1700000270` | vocabulary upload tables |
| `1700000280` | `learn.courses.origin` |
| `1700000290` | `learn.review_cards.last_review_at` |
| `1700000300` | badge and quest catalogue |
| `1700000310` | the `ai` schema |
| `1700000400` | `core.avatar_assets` |
| `1700000410` | provider dimension on budgets and cache |
| `1700000420` | `learn.xp_activity_high_water` |

**Migrate first, then deploy.** Several of these are read by code already on `main` —
`origin`, `last_review_at`, `avatar_assets`, `xp_activity_high_water` — so code arriving first
is a "column does not exist" outage.

```bash
DB_DSN="<the production DSN from Render>" go run ./cmd/migrate up
```

`.env` points `DB_DSN` at the production Supabase pooler, so a command run **without** an
explicit `DB_DSN` targets production. That cuts both ways: it is why the command above is
written with one, and why an absent-minded local `go run ./cmd/seed` is dangerous.

`S3_REGION` must be `auto`. The boot guard refuses to start on anything else against an R2
endpoint.

---

## 6. Two review habits that keep paying

**Ask the database, not the code.** Raw SQL that no test executes is untested however green
the suite looks. Every raw statement added this round was run against the real schema before
it was believed; the partial unique index on `xp_events` was checked by inserting a duplicate
and watching it be refused.

**A fabricated value is worse than a missing one.** Twice now the same fault has shipped and
been caught in review: a score invented as a perfect 100, and a word marked valid by nobody.
Both compile, both pass, and both are stored where later code trusts them. When a value is not
known, leave it empty and let the state say why.
