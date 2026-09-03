---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-03
---

# Phase 3 — work order 5

**Purpose.** The work after PR #71, with three decisions taken. It replaces §2 of
[phase-3-work-order-4.md](phase-3-work-order-4.md).

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps — the two codegen gates, the
migration numbering, the stale TODO checkboxes — and they are all still current.

---

## 1. Where this leaves off

`main` is green at `3224671` (PR #71): all five workflows passed.

| WP | State |
|---|---|
| WP12 — seed beyond vocabulary | not started |
| WP13 — dictionary autocomplete | done |
| WP14 — gamification | done |
| WP15 — `platform/ai` | done |
| WP16 — learner words | done: upload, verify, queue, enrich, My Words screen |
| WP17 — explanations | **not started — this order** |
| WP18 — quota, warnings, queue | done |
| WP19 — admin | users, feature flags and AI usage done; content, exams and vocabulary not |
| WP20 / WP21 — four skills | not started |

WP16 turned out to be further along than the earlier orders recorded: `/me/vocabulary/uploads`
exists, `MyWordsPage` renders the upload form and list, verification runs dictionary-first and
falls back to a queued flashcard when quota is out, and the enrichment sweep fills those in.

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| What comes next | WP17 — answer explanations |
| When an explanation is generated | **Lazily**, on the first learner who needs it, then stored |
| Vietnamese | Both languages from **one** call, stored together |
| Production | Migrations not run and no AI provider configured — see §4 and §5 |

---

## 3. WP17 — answer explanations

The original request, in the words it was asked in: when a learner picks an answer, show the
meaning of the word, what the right answer was, and *why* — with Vietnamese alongside.

### Where it attaches

Grading already returns the shape this belongs on. `SubmitAttemptResult` carries `correct`,
`correct_answer` and `feedback`, and the renderer already shows the last two through
`ExerciseFeedback`. An explanation is a fourth field on that response and a new block in that
component — not a new screen and not a second request.

`correct_answer` is revealed only after submission because the lesson body is redacted. The
explanation is the same kind of thing and must follow the same rule: it names the answer, so it
cannot reach the browser before the learner has answered.

### Lazy, and cached by question

**Generated on the first learner who reaches that question, then stored and reused.** The
alternative — a job that pre-generates everything — pays for questions nobody ever opens, and
on a free provider that is the whole daily budget spent on content no learner asked for.

The cost rule from §0 of the plan applies unchanged: an explanation that exists is never
regenerated. Key it by what actually determines the text, which is the question and the
learner's answer — a wrong answer deserves a different explanation from a right one, and that
is the entire point of "why".

Take migration `1700000450` or above; `1700000440` is used.

### One call, two languages

The template asks for English and Vietnamese in a single response and both are stored. Two
calls would cost twice and let the two drift into saying different things about the same
answer.

`platform/ai` already has the shape to copy: `TaskVerifyVocabulary` with
`prompts/vocab_verify.v1.md`, loaded by the versioned registry. A second task and a second
template is the same pattern again — and the cache key already includes the template version,
so revising the prompt invalidates the old answers rather than serving them for ever.

### What happens when there is no explanation yet

Quota is finite and the generation is lazy, so "not yet" is a normal state, not an error. The
verdict must render exactly as it does today when the explanation is missing — the learner sees
the feedback and the correct answer, and nothing about the screen suggests something broke.

**This is the failure mode to design for first.** Every AI feature on this project has met the
exhausted case in production before it met the happy one.

**Done means:** a learner answering a question sees an explanation with the word's meaning, the
correct answer and why, in English and Vietnamese; the second learner on the same question and
the same answer costs nothing; an exhausted quota shows today's verdict with no explanation and
no error; and the explanation never reaches the browser before the answer does.

---

## 4. Configuring a real provider

Still not done, and nothing is being validated anywhere until it is. The default is
`ai.provider = mock`, and **the mock accepts every word it is given**.

```bash
AI_PROVIDER=openai_compatible
AI_BASE_URL=<the provider's OpenAI-compatible endpoint>
AI_MODEL=<model id>
AI_API_KEY=<key>
AI_TIMEOUT=120s
```

`openai_compatible` is the only real adapter, deliberately: it covers Ollama, OpenRouter, Groq,
LM Studio and vLLM without one adapter each. A misconfigured `openai_compatible` — no base URL,
no model — is an error rather than a silent fall back to the mock.

**Seed `ai.ai_budgets` before the first real call.** A provider with no budget row is permitted
without limit: `CheckQuota` returns true when it finds no row. That is the right default for a
table nobody has filled in, and it means the ceiling does not exist until a row says so. One
row per `(provider, task)` — and WP17 adds a second task, so it needs its own rows.

The worker says so at start-up when it falls back:

```text
ai: using the mock provider; verification accepts every word it is given
```

If that line is in the production log, nothing is being checked by anything.

---

## 5. Production is behind `main`

Ten migrations pending, none run. There is no auto-migration on boot.

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
| `1700000430` | the `queued` upload status |
| `1700000440` | unassigned CEFR on content versions |

**Migrate first, then deploy.** Several are read by code already on `main` — `origin`,
`last_review_at`, `avatar_assets`, `xp_activity_high_water` — so code arriving first is a
"column does not exist" outage.

```bash
DB_DSN="<the production DSN from Render>" go run ./cmd/migrate up
```

**One of these can fail on a database not migrated from scratch by `cmd/migrate`.** `1700000440`
alters `content.content_versions`, and `ALTER TABLE` requires *ownership*, not privileges:

```text
ERROR: must be owner of table content_versions
```

No GRANT fixes it. Reproduced locally, where it stops the whole chain. If production hits it,
the owner of that table has to run the ALTER, or transfer ownership to the migration role.

`.env` points `DB_DSN` at the production Supabase pooler, so a command run **without** an
explicit `DB_DSN` targets production. `S3_REGION` must be `auto`; the boot guard refuses to
start on anything else against an R2 endpoint.

---

## 6. What keeps being found in review

Three rounds, the same three faults, each caught after it was written rather than before:

**Built and never wired.** The gamification widgets had no directory to render them, the avatar
route was advertised with nothing mounted at it, and `AdminAIUsage` was exported and rendered by
nothing. Before calling something done, open the screen or call the URL.

**A fabricated value is worse than a missing one.** A score invented as a perfect 100, a word
marked valid by nobody, a CEFR level of "B1" assigned by no model. All three compile, pass, and
are stored where later code trusts them. When a value is not known, leave it empty and let the
state say why.

**Raw SQL that no test executes is untested.** `DBCache`, `DBUsageRecorder`, `CheckQuota` and
`GetUsageOverview` all had thorough unit tests against fakes and not one line run against a real
schema. Add the integration test in the same commit as the query.

And one about the tests themselves: `go test` caches, so a schema change with no Go change is
verified by a stale result. Use `-count=1` after touching a migration, and run a new test twice
before believing it.
