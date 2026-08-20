---
doc_type: work_package
phase: 2
work_package: WP9
title: "srs and vocabulary — FSRS, the due queue, and the first grader"
tasks: 5
estimate: "~11 days"
blocked_by: "WP8 (P8.3)"
status: ready
last_verified: 2026-08-20
---

# WP9 — `srs` + `vocabulary`

This work package produces the thing that brings a learner back tomorrow, and the **first
implementation of `learning.ExerciseGrader`** — which is the proof that ADR-0015 works
before Phase 3 bets five more modules on it.

| Task | Branch |
|---|---|
| P9.1 | `feat/srs-vocabulary-contracts` |
| P9.2 | `feat/srs-fsrs-domain` |
| P9.3 | `feat/srs-schema-due-queue` |
| P9.4 | `feat/vocabulary-module` |
| P9.5 | `feat/vocabulary-grader` |

**Required reading:** `internal/modules/srs/AGENT.md`,
`internal/modules/vocabulary/AGENT.md`, `docs/adr/ADR-0016-srs-fsrs.md`,
`docs/knowledge/fsrs.md` if it exists.

---

## P9.1 — Contracts and OpenAPI, no implementation `S`

| | |
|---|---|
| **Depends on** | P8.3 |
| **Context** | `srs/AGENT.md` §4 and §6, `vocabulary/AGENT.md` §4 and §6 |
| **Files** | `internal/modules/srs/contract/`, `internal/modules/vocabulary/contract/`, `api/openapi/openapi.yaml` |
| **Do** | Contracts first, then the seven `srs` paths (`GET /reviews/session`, `GET /reviews/due-count`, `POST /reviews/{card_id}/answer`, `POST /reviews/session/complete`, `POST /reviews/{card_id}/suspend`, `POST /reviews/{card_id}/reset`, `GET /reviews/forecast`) and the eight `vocabulary` paths from that module's `AGENT.md` §6. The answer payload carries the four FSRS grades — name them in the schema as an enum (`again`, `hard`, `good`, `easy`), not as integers 1–4, so the API stays readable and the UI's keyboard map (1–4) is a UI concern rather than a wire format. |
| **Acceptance** | `make gen` and `pnpm gen:api` clean. A frontend agent can build the whole review session against generated types from this commit. |
| **Trap** | `GET /reviews/forecast` is a 30-day projection and is easy to over-build. In Phase 2 it exists in the spec because the module doc lists it; implementing it is optional and it is **not** on any Phase 2 screen. Do not build a screen for it. |

## P9.2 — FSRS in pure functions `M`

| | |
|---|---|
| **Depends on** | P9.1 |
| **Context** | ADR-0016 — *"Scheduling logic performing I/O fails review"* |
| **Files** | `internal/modules/srs/domain/` |
| **Do** | Implement FSRS with the four grades and the published parameter set, as pure functions over `(card state, grade, now) → next card state`. Stability and difficulty are modelled explicitly per item and learner. No database handle, no clock call inside the function — `now` is a parameter, which is what makes the property tests possible. Per-learner parameter optimisation is explicitly **out of scope**; it needs several hundred reviews per learner that do not exist yet. |
| **Acceptance** | Property-based tests assert the invariants: `easy` always schedules further out than `good`, which is further than `hard`, which is further than `again`; interval is monotonic in stability; `again` reduces stability; no grade ever produces a negative or zero interval; the function is total over the whole input domain. A grep proves `srs/domain/` imports no I/O package. |
| **Tests** | Property tests, not examples. The invariants above are the specification. |
| **Trap** | The FSRS parameters are not independent knobs — changing one without simulation is guessing. Take the published set, cite the source in a comment, and leave them alone. |

## P9.3 — `srs` schema, review cards, the due queue `L`

| | |
|---|---|
| **Depends on** | P9.2 |
| **Context** | `srs/AGENT.md` §5, the review §F1 |
| **Files** | `db/migrations/srs/`, `db/queries/srs/`, `internal/modules/srs/{service,repository,transport/http,job}/` |
| **Do** | Schema `learn`: `review_cards`, `review_logs`, `srs_params`, `review_daily_stats`. Cards are created from `GradeResult.ReviewItems`, which WP8 already emits. A card's `content_version_id` is a **plain `uuid` column with no foreign key** — `review_cards` lives in `learn` and `content_versions` in `content`, and **DB4 permits exactly one cross-schema foreign key, `→ core.users(id)`**. What keeps history intact is not a constraint but the authoring workflow: a published version is archived, never deleted (P7.3). Implement the due queue, the answer path (record a log, reschedule through the pure domain function), suspend, reset, and the daily stats rollup as a job. |
| **Acceptance** | **The due queue is timezone-aware.** A learner whose `user` preference is `Asia/Ho_Chi_Minh` gets a day boundary at their local midnight, not at 00:00 UTC — proven by a test with two learners in two timezones and the same card. Every answer writes a `review_logs` row; the logs are what makes per-learner tuning possible later, so nothing may be dropped. `GET /reviews/due-count` is cheap enough to call on every app open. |
| **Trap** | Computing "due today" in UTC is the bug this task exists to prevent. It looks correct in every test written by someone in UTC, and a learner in Vietnam gets their day rolling over at 07:00 local. The `user` module already stores a timezone — read it (through the contract) and use it. |

## P9.4 — `vocabulary`: words, senses, decks `M`

| | |
|---|---|
| **Depends on** | P7.3 |
| **Context** | `vocabulary/AGENT.md` |
| **Files** | `db/migrations/vocabulary/`, `internal/modules/vocabulary/**` |
| **Do** | Schema `skill`: `words`, `word_senses`, `word_relations`, `decks`, `deck_items`, `user_word_state`. A sense carries IPA, audio reference and examples — the flashcard in WP10 renders exactly these fields, so what is stored here is what the learner sees. Dictionary lookup and search; learner decks plus curated decks; mark known / ignored. |
| **Acceptance** | A word lookup returns senses with IPA and an audio URL resolved through `content.media_assets`. Deck membership is per learner. Marking a word known removes it from future scheduling. |
| **Trap** | `vocabulary` owns the `skill` schema, not `learn`. Its review cards live in `srs`, and it must not create its own — that is precisely the duplication ADR-0015 exists to prevent, and arch-lint plus review will reject it. |

## P9.5 — The first `ExerciseGrader` — the ADR-0015 proof `M`

| | |
|---|---|
| **Depends on** | P9.4, P8.3, P9.3 |
| **Context** | ADR-0015 §Consequences, `learning/AGENT.md` §4 |
| **Files** | `internal/modules/vocabulary/service/grader.go` |
| **Do** | Implement `learning.ExerciseGrader` for vocabulary activities, and register it. Grading a vocabulary activity returns a `GradeResult` whose `ReviewItems` create or update `srs` review cards. This closes the loop the whole phase is built around: complete a lesson → cards scheduled → review tomorrow. |
| **Acceptance** | **The end-to-end loop passes as an integration test**: an attempt on a vocabulary activity is graded, a review card appears, and its due date matches what the pure FSRS function returns for that grade. `vocabulary` defines **no** attempt table — ADR-0015 compliance, and it is a review-blocking condition. |
| **Trap** | This is where the shape of `ExerciseGrader` is proven or found wanting. If implementing it requires reaching into `learning`'s internals rather than through the contract, the interface is wrong — **say so and fix the interface now**, while there is one implementation and not six. That conversation is far cheaper in Phase 2 than in Phase 3, and having it is a success, not a delay. |

---

## Work-package gate

- FSRS property tests pass; `srs/domain/` performs no I/O
- `good` always schedules further out than `hard`, for every card state
- Two learners in two timezones get correct, different day boundaries for the same card
- Completing a vocabulary activity schedules a review card whose due date matches the pure
  function
- `vocabulary` has no attempt table
- Coverage ≥ 85 % on `srs`; `make check` green
