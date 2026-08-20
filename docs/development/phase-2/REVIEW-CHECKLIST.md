---
doc_type: review_checklist
phase: 2
title: "What the reviewing agent checks"
last_verified: 2026-08-20
---

# Phase 2 review checklist

For the agent reviewing a Phase 2 pull request. Report findings; do not rewrite the code.

For every finding give: **file and line · what is wrong · why it matters · how to verify.**
If an acceptance criterion cannot be verified from the diff, **say so explicitly**. In
Phase 1 the most valuable review findings were the ones that said "claimed but not
demonstrated", not the ones that found typos.

---

## 0. The five questions that matter most in this phase

Check these before anything else. Each is a mistake that Phase 3 cannot route around.

1. **Did a Phase 3 concept leak into Phase 2?** Search the diff for `streak`, `xp`,
   `badge`, `achievement`, `daily_goal`, `leaderboard`. All of these belong to
   `gamification`, which is a Phase 3 module. A field returning a plausible zero is worse
   than an absent field — the frontend builds a card around it and ships something untrue.
2. **Did a frontend PR hand-write a response type?** `grep` the diff for `interface .*Response`
   and `type .*Response` under `web/src/`. Types come from `pnpm gen:api`. Phase 1 shipped
   two schema mismatches into the E2E suite exactly this way.
3. **Did a handler land before the spec?** If the HTTP surface changed,
   `api/openapi/openapi.yaml` changed in the same commit. `CLAUDE.md` rule 2.
4. **Does a module touch another module's schema or internals?** Only `contract/` packages
   cross module boundaries. `learn`, `content` and `skill` schemas have exactly one owner
   each. `go-arch-lint` catches imports; it does **not** catch a migration writing to
   someone else's schema. Read the migrations.
5. **Is a table, endpoint, config key or metric invented?** If it is not in the module's
   `AGENT.md`, in `openapi.yaml`, in a migration, or in
   `docs/deployment/configuration.md`, it does not exist yet. `CLAUDE.md` rule 3.

---

## 1. Every PR

- [ ] `make check` green — and the PR **states** it, rather than implying it
- [ ] Acceptance criteria from the task are addressed one by one, each with how it was
      verified
- [ ] Tests are new or changed, and would fail without the change. A test that passes
      before and after tests nothing
- [ ] Errors use `shared/apperr` with documented codes; no bare `errors.New` reaching a
      handler
- [ ] Logs are structured and carry no PII — no email, no display name, no answer text
- [ ] The module's `AGENT.md` matches the code; `last_verified` bumped
- [ ] Diff is under ~400 lines excluding generated code. If not, the task was too big and
      splitting it is the finding

## 2. Backend

- [ ] Migration is reversible, and the down was actually run
- [ ] Every foreign key has an index
- [ ] No `SELECT *`; no filtering in Go what SQL should have filtered — this is the version
      of the bug that passes tests and leaks in production
- [ ] Query count is bounded on read paths that a screen calls on every open. Assert it in
      a test, do not eyeball it
- [ ] Transactions wrap multi-table writes; rollback path is tested
- [ ] Spans cover the new I/O; span names contain no ids
- [ ] `srs/domain/` performs no I/O — ADR-0016 makes this review-blocking
- [ ] `now` is a parameter or comes from `shared/clock`, never `time.Now()` inside domain
      logic

## 3. Frontend

- [ ] Types from `web/src/types/api.ts`, generated. No hand-written DTOs anywhere
- [ ] MSW handlers typed as `components["schemas"][...]`
- [ ] Empty, loading and error states exist and are tested — all three, in this PR
- [ ] Bundle figure stated, before and after. Budget 200 kB gzipped; the figure was
      166.4 kB on 2026-08-20
- [ ] New strings exist in **both** `en.json` and `vi.json`
- [ ] Keyboard path works; focus is visible; status is never conveyed by colour alone
- [ ] Touch targets ≥ 44 × 44 px (the `mobile-baseline` ESLint rules enforce this — check
      they were not disabled)
- [ ] No new runtime dependency without the bundle cost stated in the PR body

## 4. Per work package

### WP6 — design system

- [ ] No `tailwind.config.js` was created. Tailwind v4 uses `@theme` in CSS; a v3 config
      file is ignored **silently** and the colours look right by accident
- [ ] `grep -rE "indigo-|slate-[0-9]" web/src/components/ui/` returns nothing
- [ ] Contrast is asserted by a test, in both themes, for every button variant — light mode
      was broken before this WP and "unchanged" is not the criterion
- [ ] The dark mechanism is still the class on `<html>` set before first paint, not a media
      query
- [ ] The shell grew as little as possible. Anything imported by `AppShell` is downloaded
      by a learner who only visits `/login`

### WP7 — content, lesson

- [ ] A published `content_versions` row cannot be updated — enforced by the database, not
      by a comment
- [ ] Learner responses are modelled around the **version**, never the mutable item
- [ ] Archiving an item does not break a learner who is mid-lesson, and there is a test
- [ ] `GetManyVersions` issues one query for N ids — the query count is asserted
- [ ] `lesson` does not import `learning`; the arrow goes the other way
- [ ] A locked lesson returns a human-readable reason, not just `locked: true`

### WP8 — learning engine

- [ ] The grader registry is validated **at startup** and names the offending kind
- [ ] Double submit with the same idempotency key grades once — tested **concurrently**,
      not sequentially
- [ ] The idempotency key is in the HTTP contract, not only in the service layer
- [ ] `GradeRequest` passes ids, not a whole content version. Coupling every future grader
      to the content shape is the one mistake here that Phase 3 cannot route around
- [ ] `GradeResult.Async` is honoured even though no Phase 2 grader uses it
- [ ] `attempts` partitioning ships **with** the partition-creation job. Monthly
      partitioning without it is an outage on the 1st
- [ ] The dashboard response contains no Phase 3 field
- [ ] Next-activity has an explicit answer for a brand-new learner — not `null`, not a 404

### WP9 — srs, vocabulary

- [ ] FSRS invariants are **property**-tested, not example-tested
- [ ] `easy` > `good` > `hard` > `again` in resulting interval, for every card state
- [ ] The due queue uses the learner's timezone preference. Two learners in two timezones,
      one card, different day boundaries — there is a test, or this is a finding
- [ ] Every answer writes a `review_logs` row; nothing is dropped
- [ ] `vocabulary` defines **no** attempt table (ADR-0015, review-blocking)
- [ ] The loop closes: a graded vocabulary activity produces a review card whose due date
      matches the pure FSRS function

### WP10 — learner web

- [ ] Dashboard is three cards. Not six, not four
- [ ] Learn renders a unit list, not a node map
- [ ] The lesson runner sends the idempotency key; two tabs produce one attempt
- [ ] The review grade goes on the wire as the enum, not as the keyboard digit
- [ ] 320 px passes in `vi`, not only `en`
- [ ] No chart library was added for the progress screen

### WP11 — seed, E2E, ship

- [ ] Seed content went through the authoring workflow, not around it via raw SQL
- [ ] E2E runs against the real stack; no mocked API in E2E
- [ ] E2E assertions match the **real** rendered labels — open the screen and read them.
      Phase 1's entire suite had to be rewritten for want of this
- [ ] D1 retention is readable from a dashboard by a person, not merely present as events
      in Loki
- [ ] No module still declares `status: PLANNED` while shipping a service

---

## 5. What is out of scope, and saying so is a valid finding

If a PR builds any of these, that is a finding, regardless of quality:

speaking · audio recording · microphone permission · any AI call · XP · streaks · badges ·
quests · achievements · leaderboard · league · retention forecast · radar analytics ·
placement test · the vertical node-map learning path

All of them are Phase 3 or later. See [../phase-2-plan.md](../phase-2-plan.md) §6.
