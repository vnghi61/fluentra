---
module: listening
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [audio_items, transcripts, listening_attempts]
depends_on: [content, media, questionbank, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# listening — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/listening` |
| Schema | `skill` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Audio comprehension: audio items, transcripts, play-limit policy, dictation, note-taking, and listening graders.
<!-- END GENERATED: overview -->


## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Audio items with speaker metadata, accent, speed and transcript
- Play-limit policy enforcement (exam-realistic constraints)
- Transcript reveal policy
- Dictation grading with tolerance
- Segment-level replay for study mode
- Listening graders: MCQ, gap-fill, dictation, ordering

**This module does NOT own:**

- Audio processing — that is `platform/media`
- Question authoring — that is `questionbank`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/listening/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/listening/contract/` | You are calling this module from another module |
| `internal/modules/listening/service/` | You are changing behaviour |
| `db/migrations/listening/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/listening/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `listening.Grader` | Implements `learning.ExerciseGrader` for listening activity kinds |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `listening.attempt_completed` | publishes | `{user_id, item_id, score, plays_used}` |
| `media.processed` | consumes | Mark an audio item ready for publication |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `skill` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/listening/` · Queries: `db/queries/listening/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `skill.audio_items` | A listening resource | Content-versioned. `asset_id`, `duration_ms`, `accent`, `speech_rate`, `cefr_level` |
| `skill.transcripts` | Aligned transcript | `audio_item_id`, `segments` jsonb with timings and speaker labels |
| `skill.listening_attempts` | One attempt | `user_id`, `audio_item_id`, `plays_used`, `score`, `answers` jsonb |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `listening`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/listening/items/{id}` | `content.read.published` | Item with a presigned audio URL and the play policy |
| `POST` | `/api/v1/listening/attempts` | `self` | Start an attempt |
| `POST` | `/api/v1/listening/attempts/{id}/play` | `self` | Record a play (server-side counter) |
| `POST` | `/api/v1/listening/attempts/{id}/submit` | `self` | Submit answers |
| `GET` | `/api/v1/listening/attempts/{id}/transcript` | `self` | Transcript after submission |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`content`](../../modules/content/AGENT.md) | → depends on | see its contract |
| [`media`](../../platform/media/AGENT.md) | → depends on | see its contract |
| [`questionbank`](../../modules/questionbank/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-LISTENING-01** — The play counter is authoritative on the server. A client that hides the play button is a convenience; a client that lies must still be limited.
2. **BR-LISTENING-02** — Play limits are per activity configuration: exam mode typically allows one, study mode unlimited.
3. **BR-LISTENING-03** — The transcript is locked until the attempt is submitted, unless the activity explicitly permits it.
4. **BR-LISTENING-04** — Dictation grading normalises punctuation and case by default and accepts configured spelling variants; the tolerance is per activity.
5. **BR-LISTENING-05** — An audio item cannot be published until its media derivatives exist and its transcript is present.
6. **BR-LISTENING-06** — Presigned audio URLs are per attempt and short-lived, so a link cannot be shared to bypass limits.
7. **BR-LISTENING-07** — Segment replay is available only in study mode.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a listening exercise type

1. Add the activity kind and its play policy configuration.
2. Implement the grading branch, including the normalisation rules for dictation.
3. Add the player variant with the right controls for the mode.
4. Test that the server enforces the play limit independently of the UI.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Play counting is per attempt, not per second — a learner who replays within a single play is not detected.
- Accent coverage depends on the source material and the TTS voices available.
- No speed control in exam mode, by design.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->


### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `PLAY_LIMIT_REACHED` | 403 | No plays remaining for this attempt |
| `TRANSCRIPT_LOCKED` | 403 | Transcript not yet available |
| `AUDIO_NOT_READY` | 409 | Media still processing |


## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/listening/...                    # unit
go test -tags=integration ./internal/modules/listening/...  # integration (testcontainers)
```

**Focus areas**

- Server-side play limit cannot be bypassed by calling the endpoint directly
- Transcript lock and unlock timing
- Dictation normalisation and tolerance boundaries
- Presigned URL scoping and expiry
- Publication blocked without derivatives or transcript
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not enforce the play limit only in the UI.
- Do not reveal the transcript before submission unless the activity allows it.
- Do not issue a long-lived audio URL.
- Do not publish an item without a transcript.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
