---
doc_type: index
phase: 2
title: "Phase 2 work packages — handoff guide"
last_verified: 2026-08-24
---

# Phase 2 — work packages

Six files, one per work package, each sized to hand to a single agent in a single session.

| File | Tasks | Est. | Blocked by | Hand to |
|---|---|---|---|---|
| [WP6-design-system.md](WP6-design-system.md) | 6 | ~9 d | nothing | frontend agent |
| [WP7-content-lesson.md](WP7-content-lesson.md) | 5 | ~11 d | nothing | backend agent |
| [WP8-learning-engine.md](WP8-learning-engine.md) | 5 | ~12 d | WP7 | backend agent — **the careful one** |
| [WP9-srs-vocabulary.md](WP9-srs-vocabulary.md) | 5 | ~11 d | WP8 (P8.3) | backend agent |
| [WP10-learner-web.md](WP10-learner-web.md) | 5 | ~12 d | WP6 + contract tasks | frontend agent |
| [WP11-seed-e2e-ship.md](WP11-seed-e2e-ship.md) | 4 | ~8 d | all (except P11.1) | either |

The three work packages after WP8 each have an **implementation brief** beside the planning
file above: [P9-srs-vocabulary.md](P9-srs-vocabulary.md),
[P10-learner-web.md](P10-learner-web.md) and [P11-seed-e2e-ship.md](P11-seed-e2e-ship.md).
One brief per work package, each covering all of its tasks, each still implemented one branch
at a time in the order its table gives. The WP files keep the task rows and the estimates; the
briefs carry the traps, the current state of the tree, and the acceptance criteria.

Two of them run in parallel — P9 is backend, P10 is web against generated types — and P11.1,
the content authoring, starts before either of them finishes.

Context that applies to all of them:

- [../phase-2-plan.md](../phase-2-plan.md) — scope, dependency graph, Definition of Done,
  verification gates. **Every agent reads §2 before starting.**
- [../phase-2-ui-plan-review.md](../phase-2-ui-plan-review.md) — why the UI work is shaped
  the way it is, and what was moved to Phase 3.
- [REVIEW-CHECKLIST.md](REVIEW-CHECKLIST.md) — for the reviewing agent.

---

## The handoff loop

One work package at a time. Not two, and never all six.

```
1. IMPLEMENT   → give the implementing agent exactly one WP file
                 plus phase-2-plan.md §2
                 It produces one PR per task, not one PR per WP.

2. REVIEW      → give the reviewing agent the same WP file,
                 REVIEW-CHECKLIST.md, and the diff
                 It reports findings; it does not rewrite.

3. FIX         → back to the implementing agent with the findings

4. GATE        → run the work-package gate at the bottom of the WP file
                 Do not start the next WP until it passes.
```

**Why one at a time.** WP8 defines an interface that WP9 implements and Phase 3 implements
five more times. An agent given WP8 and WP9 together will design the interface around the
one caller it can see, which is the exact failure ADR-0015 exists to prevent.

## Starting order

Two agents can run from day one:

```
Frontend agent:  WP6  ────────────────►  WP10
Backend agent:   WP7  ──►  WP8  ──►  WP9
Content:              P11.1 (starts when P7.3 lands — see the note in WP11)
```

WP10 does not wait for WP8 and WP9 to finish. It waits for their **contract-only** tasks
(P7.1, P8.1, P9.1), which are small and land early. That is deliberate, and it is what makes
the two tracks genuinely parallel rather than nominally parallel.

## Handoff notes

State of the repository, for the agent picking up the next work package. Written when WP7
closed; verified against `main` @ `a1e0efc` plus the two P8.1 contract commits.

### What is done

| | Status | Coverage |
|---|---|---|
| WP7 — `content`, `lesson` | Gate passed | `content` 86.0 %, `lesson` 90.3 % |
| WP8 | P8.1 landed (contracts + spec, no implementation) | n/a — no statements to cover |

An author drives a content item draft → published through the API; a lesson is published
through `POST /admin/lessons/{id}/publish`; a locked lesson returns a human-readable reason
in `Problem.meta`; archiving does not break a learner mid-lesson.

### Six things WP8 needs and no WP file says

**1. `lesson` and `learning` share the `learn` schema.** Both `AGENT.md` front-matters say
`schema: learn`. DB1 gives a module exclusive ownership of *tables*, not of the schema:
`lesson` owns `courses`, `course_units`, `lessons`, `activities`, `lesson_prerequisites`;
`learning` owns the six in its own §5. P8.2's migrations go in `db/migrations/learning/`,
beside `db/migrations/lesson/`.

The consequence is a good one: `attempts.activity_id → learn.activities` is a *same-schema*
foreign key and is allowed. `activities.content_version_id` is a bare uuid only because DB4
forbids the cross-schema key into `content`. Do not copy that workaround where it is not
needed — P7.4 wrote a test asserting DB4 rather than assuming it, and the same reasoning
says an intra-schema key should be a real one.

**2. Two different modules publish events beginning `lesson.`.** `lesson.published` belongs
to `lesson` (outbox aggregate `lesson`); `lesson.completed` belongs to `learning` (aggregate
`learning`). They are unrelated, and a subscriber matched on a prefix would cross them.

**3. `UnlockChecker` is batched, and `lesson` has not adopted it.** P8.1 took the decision
`lesson/TODO.md` handed it: `IsUnlocked(ctx, userID, lessonIDs) (map[uuid.UUID]bool, error)`.
`lesson/service/service.go` still declares its own single-lesson copy, deliberately — there
was nothing to adopt while `learning` had no implementation. Whoever lands P8.4 changes both
sides in one diff.

**4. The `Idempotency-Key` parameter already exists.** `components/parameters/IdempotencyKey`
in `openapi.yaml`, required on `POST /attempts/{id}/submit`. P9 reuses the `$ref`; it does not
redeclare it. It is the spec's only request-header parameter, so it is also the worked
example if another one is needed.

**5. `content.publish` publishes both content and lessons.** There is no `lesson.publish`
permission and there should not be one — the same people publish material and the lessons
carrying it. `rbac` seeds it in migration `1700000180`.

**6. The caching pattern is established, including the part that is not obvious.**
`lesson` reads through `Cache[T].GetOrLoad` on three keys and invalidates synchronously after
commit, with an event consumer as the backstop. Two details worth copying rather than
rediscovering: a key whose value is a counter must be read with `Get`, never `GetOrLoad`,
because `GetOrLoad` writes its loaded value back on a goroutine and can land a stale write
after a concurrent bump; and a failed invalidation is logged, never returned, so a Redis
outage does not fail a write. `lesson/AGENT.md` §12 has the key table.

### Three things that each cost a round trip

**Generated documentation is generated.** The endpoint, contract, decision and TODO blocks in
every module's `AGENT.md`, `API.md`, `DECISIONS.md` and `TODO.md` come from
`tools/docgen/data/*.json` via `make docs`. Editing the Markdown by hand looks like it worked
and fails `make docs-check` in CI. P7.4 lost its TODO checkboxes to exactly this. Record
completed work in the hand-written **Progress** table below the generated block.

**`make check` is not the whole check.** CI additionally runs

```bash
golangci-lint run --build-tags=integration ./...   # files behind //go:build integration
make gen-check                                     # codegen vs git
```

`gen-check` compares generated output against what is committed, so codegen you ran but did
not commit reads as staleness.

**Ports.** `make dev-infra` publishes Postgres on 5432 and Redis on 6379. If something else
already holds them, it may belong to another project — do not stop it without asking.
Throwaway containers on other ports cost nothing:

```bash
docker run -d --name fluentra-test-pg -e POSTGRES_DB=fluentra -e POSTGRES_USER=fluentra \
  -e POSTGRES_PASSWORD=fluentra -p 55432:5432 postgres:17-alpine
docker run -d --name fluentra-test-redis -p 56379:6379 redis:7.4-alpine
export TEST_DATABASE_URL="postgres://fluentra:fluentra@localhost:55432/fluentra?sslmode=disable"
export TEST_REDIS_ADDR="localhost:56379"
```

`make cover-check` and the integration suite read both variables.

### If a failure looks unrelated to your change, it may be

- **`go test -race` aborting with "ThreadSanitizer failed to allocate"** is a Windows host
  limit, not a data race. CI runs the race detector on Linux; that is the run that counts.
- **A single E2E journey failing on one device project** was, once, `extractOtpCode` reading
  a six-digit display name instead of the code (fixed in `12a3a2e`). The lesson generalises:
  before writing a red journey off as a flake, read `error-context.md` in the Playwright
  artifact. It carries the page snapshot, and it named the cause in one line.

---

## What to give the implementing agent

The WP file is written to be self-contained. It still needs the repository's standing rules,
which are in `AGENT.md` and `CLAUDE.md` and are not repeated in each file. The prompt is
roughly:

```
Read /AGENT.md and /CLAUDE.md.
Read docs/development/phase-2-plan.md §2 (how to run this plan).
Then implement docs/development/phase-2/WP<N>-<name>.md, task <ID> only.

Test against the real stack: `make dev` brings it up. Do not mock the API.
If a port is taken, see "Ports" in the handoff notes — do not assume the
container holding it belongs to this project.

When done:
  1. make check
  2. show me the diff, grouped by file
  3. state which acceptance criteria you verified and how
  4. for web/ changes, state the bundle figure before and after
```

One task per session. A whole work package in one session produces a diff nobody reviews.

## What to give the reviewing agent

```
Read docs/development/phase-2/WP<N>-<name>.md and REVIEW-CHECKLIST.md.
Review this diff against the acceptance criteria of task <ID>.
Report findings; do not rewrite the code.
For each finding: file, line, what is wrong, why it matters, how to verify.
If an acceptance criterion cannot be verified from the diff, say so — do not assume it holds.
```

The last line matters. Phase 1's most useful review findings were the ones that said "this
was claimed but not demonstrated", not the ones that found typos.
