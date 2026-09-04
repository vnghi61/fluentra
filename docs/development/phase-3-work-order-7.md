---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-04
---

# Phase 3 — work order 7

**Purpose.** Three proven defects in the AI provider path, and the change that lets a learner's
word be checked in seconds instead of within the hour. It runs **alongside**
[phase-3-work-order-6.md](phase-3-work-order-6.md), which is WP12 and does not supersede it —
see §5 for the one place the two collide.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps, and they are all still
current.

---

## 1. Why this exists

Three findings, each reproduced by running the code rather than reading it.

### The provider registry collapses every real provider into one

`ProviderRegistry.providers` is a map keyed by `Provider.Name()`, and
`OpenAICompatibleProvider.Name()` returns the constant `"openai_compatible"` whatever base URL
it was built with. So every provider registered after the first **overwrites** the one before
it. Building a Cerebras, a Groq and a Gemini provider and registering them in that order:

```text
configured primary  = cerebras  (0x...1d0)
configured fallback = groq      (0x...220)
configured third    = gemini    (0x...270)
Primary()  returns  -> 0x...270      <- gemini
Fallback() returns  -> 0x...270      <- gemini
```

All three collapse into the last one, and `Primary()` and `Fallback()` return the same object.
The two-provider case documented in `.env.example` fails the same way: the provider configured
as primary is never called, and the router's "fallback" retries the endpoint that just failed.

The one arrangement that works today is `openai_compatible` primary with `mock` fallback,
because `mock` has a different name. The fallback path is tested only in the configuration
nobody deploys.

### `cmd/api` cannot be given a fallback at all

`initAIClient` in `cmd/api/main.go` passes five fields. The five `Fallback*` fields exist on
`ai.Config` and are passed by `cmd/worker`, but not here — so the client that generates answer
explanations has no second provider even in principle.

### Every cron job skips its first run

```go
ticker := time.NewTicker(job.Interval)
for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C: s.executeWithLock(ctx, job)
    }
}
```

Nothing runs before the loop, so a worker that has just started waits a full interval before
doing anything. For `vocabulary.verify_uploads` that is an hour of a fresh deployment
verifying nothing, which reads in development as "the AI is broken".

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| Scope | Provider registry, cold start, and verify-on-submit. Per-user quota waits for real data |
| Provider configuration | Numbered slots — `AI_PROVIDER_1_*` through `AI_PROVIDER_4_*` |
| Existing `ai_budgets` / `ai_usage` rows | Discard and re-seed under the new provider names |
| Who | Gravity |

Per-user quota is deliberately **not** in this order. `ai.Request` carries no user id and
`ai.ai_requests.user_id` has been NULL since the table was created, so there is no data to
choose a limit from. Pick the number after watching real usage, not before.

---

## 3. The work

### 1. Providers get names, and fallbacks become a chain

Add a `Name` to `OpenAICompatibleConfig`, defaulting to `openai_compatible` when empty so
nothing already configured changes meaning. `Name()` returns it.

Then `ProviderRegistry` holds `fallbacks []string` in order rather than one `fallback string`,
and `Router.executeFallback` walks that list instead of trying a single alternate. A provider
whose quota is exhausted or whose call fails moves to the next one; the request fails only when
the chain is spent.

This is what makes the intended deployment expressible:

```text
Cerebras  ->  Groq  ->  Gemini  ->  Ollama
```

**And it is what makes `ai.ai_budgets` mean anything.** The table has a `provider` column and a
`uq_ai_budgets_provider_task` constraint, but with one name for every deployment there is one
row for all of them, their usage sums together, and "Groq is exhausted" and "OpenRouter is
exhausted" are the same fact. Named providers give each its own row, which is what
`.env.example` already tells the operator to create.

### 2. Configuration: four numbered slots

```bash
AI_PROVIDER_1_NAME=cerebras
AI_PROVIDER_1_BASE_URL=...
AI_PROVIDER_1_MODEL=...
AI_PROVIDER_1_API_KEY=...
AI_PROVIDER_1_TIMEOUT=120s
# ... _2_, _3_, _4_
```

Slot 1 is the primary; 2 to 4 are the chain in order. A slot with an empty name is skipped, so
a one-provider deployment fills one slot and leaves the rest blank.

**Every key must be declared.** `config.Load` drops an environment variable whose key appears
in no `Defaults`, `Required` or `EnvSections` entry — silently, with no warning. `EnvKey`
splits on the *first* underscore, so `AI_PROVIDER_1_NAME` becomes `ai.provider_1_name`: the
section is `ai` and the rest is one flat key. That works, but it means the slots are a fixed
set declared in code, not a list koanf can grow on its own. Four slots, twenty keys, all in
`.env.example` — `TestConfigOptions_EveryDeclaredKeyIsDocumented` fails otherwise, which is the
guard doing its job.

Replace the existing ten `AI_*` / `AI_FALLBACK_*` keys rather than layering the new ones beside
them. Two ways to say the same thing is how a deployment ends up configured twice and running
neither.

**Do both binaries.** `cmd/worker` and `cmd/api` each build their own client, and the API's is
missing the fallback fields today. A chain configured on one and not the other is worse than no
chain, because explanations and word verification would then disagree about which providers
exist.

### 3. Run each cron job once at startup

Call `executeWithLock` once before entering the ticker loop. Two lines, and it fixes all four
cron jobs, not just vocabulary's.

Guard the obvious hazard: this runs on every worker start, so a crash-loop becomes a call-loop.
The advisory lock already prevents two instances colliding; what it does not prevent is one
instance restarting repeatedly. Rate-limit or record the last run if that is a real risk on
Render's restart behaviour — and say in `DECISIONS.md` which you chose.

### 4. Verify a word when it is submitted, not within the hour

Today `Submit` writes rows and returns. `vocabulary.verify_uploads` picks them up on its hourly
tick, 50 at a time. The learner watches a word sit in "pending" for up to an hour with nothing
telling them why.

Enqueue a River job in the same transaction that writes the upload:

| Piece | Where | Note |
|---|---|---|
| `VerifyUploadArgs{UploadID}` | new `internal/modules/vocabulary/job/` | copy `user/job/export.go`; queue `ai` |
| Transaction around `Submit` | `service/upload.go` | it is currently a sequence of pool calls |
| `EnqueueTx` inside it | same | `EnqueueTx` **requires** a tx (BR-JOB-01) — that is the point: no orphan job, no lost one |
| `Enqueuer` in vocabulary's deps | `module.go`, `cmd/api/modules.go` | only `user` receives one today |
| `registerJobKinds` 1 → 2 | `cmd/worker/main.go` | it also logs the count |
| `m_vocabulary_job` component | `.go-arch-lint.yml` | **a boundary change** — update MODULE_INDEX.md §3 |

Copy `m_user_job`'s shape for the new component: it depends on domain, repository and platform
packages with `anyVendorDeps: true`, and **not** on the service. The export worker declares its
own narrow `ExportRepository` interface rather than reaching into `user/service`. Do the same
here — the job declares what it needs and the module root supplies it.

**Keep the cron job.** It stops being the delivery mechanism and becomes the sweeper: items
whose River job exhausted its attempts, items submitted while the worker was down, anything the
queue lost. An hourly interval is right for that role. Deleting it would mean a single failed
job strands a word for ever.

### 5. Discard the old usage and budget rows

No real provider has ever run, so every row in `ai.ai_usage` and `ai.ai_budgets` describes the
mock or nothing at all. Keeping them under a renamed provider would attribute history to a
service that never produced it.

Delete them and seed fresh rows per `(provider, task)` under the new names. The task strings
are `vocab_verify` and `explain_answer`, from `internal/platform/ai/ai.go` — not the Go
constant names, which are `TaskVerifyVocabulary` and `TaskExplainAnswer`.

**And pick limits that can actually bind.** The cron schedule caps `vocab_verify` at
50/hour + 20/hour = **1,680 per day** before any budget row is consulted. A limit above that
number is a control that can never fire. Verify-on-submit changes this ceiling — that is the
point of §4 — so set the budget deliberately rather than inheriting a number from a document.

---

## 4. What must not change

**The explanation still yields silently.** WP17 decided that "no explanation yet" is a normal
state and not an error: quota exhaustion returns the grade with the explanation absent, and
nothing on the screen suggests a failure. Do not turn this into a 429 or an error toast. The
learner loses their result to protect a feature that is optional by design.

**A word saved while quota is out is still kept.** `status = 'queued'`, no CEFR level nobody
assigned, still reviewable as a flashcard. Verify-on-submit must degrade exactly the way the
cron path does when the chain is spent.

**The API contract does not change.** No new endpoint, no new field. `make gen-api` and
`make gen-web` have nothing to regenerate here — and if that turns out to be false, both gates
have to run and the result has to be committed.

---

## 5. Migration numbers, and the collision with work order 6

Work order 6 tells its implementer to take `1700000460`. **This order takes `1700000470` or
above.** Two branches are open at once and either may land first; a duplicate number breaks the
goose run for whoever merges second, and goose applies in lexical order across every module
directory.

If §5 above is done as a data migration rather than an operator's SQL, that is the number to
use.

---

## 6. Done means

- Three real providers configured in three slots are three distinct providers: `Primary()`
  returns slot 1, and a provider that fails or is out of quota hands off to the next in order,
  proved by a test that builds three and asserts the chain — the test that would have caught
  this the first time.
- `ai.ai_budgets` has one row per real provider per task, and exhausting one does not read as
  exhausting the others.
- `cmd/api` and `cmd/worker` build the same chain from the same keys.
- A worker that has just started verifies pending words without waiting for a tick.
- A learner submitting a word sees it verified in seconds; with the worker stopped, the same
  word is still verified when the worker returns, by the cron sweep.
- Quota exhausted still produces a queued flashcard and a graded answer with no explanation —
  not an error, on either path.

---

## 7. What keeps being found in review

**Built and never wired.** The gamification widgets, the avatar route, `AdminAIUsage`. This
order's version is the fallback chain itself: it has always been "supported" and has never
worked in the configuration the documentation recommends.

**A test that agrees with itself proves nothing.** `TestRouter_HasQuota` and the router's
fallback tests all pass, and all of them use providers with distinct names — so none of them
could see the collision. When a test needs a fixture, ask whether the fixture is the case that
actually ships.

**Ask the database, and run the binary.** Every claim in §1 came from executing the code, not
from reading it. The registry defect is four lines of Go and it survived three work orders of
review because everyone who looked at it, including the reviews, read `Fallback()` and believed
it.
