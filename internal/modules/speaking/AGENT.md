---
module: speaking
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_tasks, speaking_attempts, pronunciation_scores, speaking_feedback]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# speaking — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/speaking` |
| Schema | `skill` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Spoken practice: prompts, browser recording, automatic speech recognition, phoneme-level pronunciation assessment, fluency measurement, and AI coaching built on top of those numbers.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Speaking tasks: read-aloud, describe-an-image, opinion, role-play
- Recording upload coordination and attempt lifecycle
- Orchestrating the media pipeline for transcription and pronunciation assessment
- Fluency metrics: speech rate, pauses, filler words
- AI coaching feedback derived from transcript plus scores
- Phoneme-level feedback rendering data (the heat map)

**This module does NOT own:**

- Transcoding or ASR — that is `platform/media`
- Storing audio — that is `platform/storage`
- Generating text feedback directly — it asks `platform/ai`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/speaking/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/speaking/contract/` | You are calling this module from another module |
| `internal/modules/speaking/service/` | You are changing behaviour |
| `db/migrations/speaking/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/speaking/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `speaking.Grader` | Implements `learning.ExerciseGrader`; always asynchronous |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `speaking.attempt_recorded` | publishes | `{user_id, attempt_id, asset_id}` |
| `speaking.scored` | publishes | `{user_id, attempt_id, accuracy, fluency}` |
| `media.transcribed` | consumes | Continue the pipeline once ASR is done |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `skill` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/speaking/` · Queries: `db/queries/speaking/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `skill.speaking_tasks` | Prompt definitions | Content-versioned. `type`, `prompt`, `reference_text` (read-aloud only), `prep_seconds`, `max_seconds` |
| `skill.speaking_attempts` | One recording | `user_id`, `task_id`, `asset_id`, `status`, `transcript_id`, `overall_score`, `duration_ms` |
| `skill.pronunciation_scores` | Per-attempt assessment | `attempt_id`, `accuracy`, `fluency`, `completeness`, `prosody`, `words` jsonb (per-word and per-phoneme) |
| `skill.speaking_feedback` | AI coaching | `attempt_id`, `summary`, `strengths`, `improvements`, `prompt_version` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `speaking`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/speaking/tasks` | `content.read.published` | Available tasks |
| `POST` | `/api/v1/speaking/upload-intent` | `self` | Presigned URL for the recording |
| `POST` | `/api/v1/speaking/attempts` | `self` | Create the attempt after upload |
| `GET` | `/api/v1/speaking/attempts/{id}` | `self` | Attempt with scores and feedback when ready |
| `GET` | `/api/v1/speaking/attempts` | `self` | History with score progression |
| `DELETE` | `/api/v1/speaking/attempts/{id}/recording` | `self` | Delete the audio while keeping the scores |
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
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`media`](../../platform/media/AGENT.md) | → depends on | see its contract |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | see its contract |
| [`storage`](../../platform/storage/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`content`](../../modules/content/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-SPEAKING-01** — Audio never passes through the API — presigned upload only.
2. **BR-SPEAKING-02** — Maximum recording length is enforced in the presigned URL **and** verified after upload; a client can lie about both.
3. **BR-SPEAKING-03** — Explicit consent is required before the first recording, and is recorded with a timestamp.
4. **BR-SPEAKING-04** — Recordings are retained 90 days unless the learner pins the attempt; scores and transcripts outlive the audio.
5. **BR-SPEAKING-05** — A learner may delete a recording at any time; the derived scores remain, clearly marked as having no audio.
6. **BR-SPEAKING-06** — If ASR confidence is below the threshold, the attempt is not scored — the learner is asked to re-record rather than being penalised for a bad microphone.
7. **BR-SPEAKING-07** — Read-aloud tasks score completeness against the reference text; open tasks do not.
8. **BR-SPEAKING-08** — AI coaching receives the transcript and the numeric scores, never the raw audio.
9. **BR-SPEAKING-09** — Scores are clamped to their defined ranges and cross-checked before display.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a speaking task type

1. Decide whether it has a reference text — that determines whether completeness is scored.
2. Add the content kind and its schema.
3. Extend the feedback prompt's input schema if the coaching shape differs (new prompt version).
4. Add the recorder UI variant with the right preparation and recording timers.
5. Add fixture audio to `test/fixtures/corpus/speech/` covering a strong and a weak speaker.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Pronunciation assessment quality varies by accent; this is a known and documented limitation of every current provider, and the UI wording avoids implying an authoritative judgement.
- No real-time feedback while speaking.
- Background noise degrades scores; there is no noise suppression stage in Phase 3.
- Prosody scoring depends on provider support and may be absent.
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
| `RECORDING_CONSENT_REQUIRED` | 403 | Voice consent not yet given |
| `AUDIO_TOO_LONG` | 422 | Exceeds the task maximum |
| `AUDIO_TOO_QUIET` | 422 | Signal level too low to assess |
| `TRANSCRIPTION_LOW_CONFIDENCE` | 422 | Ask the learner to re-record |
| `SCORING_FAILED` | 500 | Assessment pipeline failed after retries |

### Security considerations

- Voice is biometric-adjacent personal data: private bucket, presigned access only, explicit consent, short retention, learner-controlled deletion.
- Only speech providers with no-training-on-customer-data terms are used.
- Admin access to a recording requires a stated reason and is audited.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/speaking/...                    # unit
go test -tags=integration ./internal/modules/speaking/...  # integration (testcontainers)
```

**Focus areas**

- Consent gate before the first recording
- Duration and format enforced both in the presign and after upload
- Low-confidence transcripts are not scored and do not consume quota
- Recording deletion preserves scores
- Retention job deletes audio at 90 days and keeps pinned attempts
- Score clamping
- Completeness scored only for read-aloud tasks
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not stream audio through the API.
- Do not score an attempt whose transcription confidence is low.
- Do not send raw audio to an LLM.
- Do not record without explicit consent.
- Do not retain audio beyond the retention window.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
