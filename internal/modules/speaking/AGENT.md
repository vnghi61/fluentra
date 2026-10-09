---
module: speaking
tier: learning
group: modules
status: DONE
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_feedback, speaking_consents]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-09-18
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
| Status | **DONE** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Spoken practice: prompts, browser recording, automatic speech recognition, read-aloud word accuracy, speaking rate, and AI coaching built on the transcript.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Speaking tasks: read-aloud and open response
- Recording upload coordination and attempt lifecycle
- Orchestrating the media pipeline for transcription
- Read-aloud word accuracy and speaking rate, computed in Go from the transcript
- AI coaching feedback derived from the transcript plus those numbers

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
| `user.deleted` | consumes | Delete the account's recordings from storage; the anonymised user row never cascades |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `skill` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/speaking/` · Queries: `db/queries/speaking/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `skill.speaking_feedback` | Per-attempt asynchronous feedback, metrics, and transcript | `attempt_id` UNIQUE FK learn.attempts, `user_id`, `recording_key`, `recording_deleted_at`, `transcript`, `criteria` jsonb, `read_aloud_accuracy`, `words_per_minute`, `feedback_en`, `feedback_vi`, `overall_band`, `score`, `task_type` |
| `skill.speaking_consents` | BR-SPEAKING-03: who agreed to be recorded, and when | `user_id` PK FK core.users ON DELETE CASCADE, `consented_at`. One row per learner; the upload-intent endpoint refuses without it. |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `speaking`). Per-endpoint detail is in §15.

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/speaking/upload-intent` | `self` | Presigned PUT URL for recording upload to storage |
| `DELETE` | `/api/v1/speaking/attempts/{id}/recording` | `self` | Purges the recording object while keeping scores and feedback |
| `GET` | `/api/v1/speaking/attempts/{id}/feedback` | `self` | Read feedback on a graded speaking attempt, with a presigned audio_url while the recording lives |
| `GET` | `/api/v1/speaking/submissions` | `self` | Paginated history of the learner's graded speaking attempts |
| `GET` | `/api/v1/speaking/consent` | `self` | Whether the learner has consented to voice recording, and when |
| `POST` | `/api/v1/speaking/consent` | `self` | Record the learner's consent to voice recording (BR-SPEAKING-03) |
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

Global rules: [`docs/CONVENTIONS.md#coding-standard`](../../../docs/CONVENTIONS.md#coding-standard). Deviations and additions
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

## 15. API detail

### Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
#### `POST /api/v1/speaking/upload-intent`

Presigned PUT URL for recording upload to storage

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `UNSUPPORTED_AUDIO_FORMAT`, `SPEECH_DAILY_LIMIT_REACHED` |

#### `DELETE /api/v1/speaking/attempts/{id}/recording`

Purges the recording object while keeping scores and feedback

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

#### `GET /api/v1/speaking/attempts/{id}/feedback`

Read feedback on a graded speaking attempt, with a presigned audio_url while the recording lives

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `FEEDBACK_NOT_FOUND` |

#### `GET /api/v1/speaking/submissions`

Paginated history of the learner's graded speaking attempts

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

#### `GET /api/v1/speaking/consent`

Whether the learner has consented to voice recording, and when

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

#### `POST /api/v1/speaking/consent`

Record the learner's consent to voice recording (BR-SPEAKING-03)

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

### Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `RECORDING_CONSENT_REQUIRED` | 403 | Voice consent not yet given |
| `AUDIO_TOO_LONG` | 422 | Exceeds the task maximum |
| `AUDIO_TOO_QUIET` | 422 | Signal level too low to assess |
| `TRANSCRIPTION_LOW_CONFIDENCE` | 422 | Ask the learner to re-record |
| `SCORING_FAILED` | 500 | Assessment pipeline failed after retries |
<!-- END GENERATED: api-errors -->

### Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/docs/CONVENTIONS.md](../../../docs/CONVENTIONS.md#rate-limiting).
<!-- END GENERATED: api-rate -->

## 16. Flows

### Record to score

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant S as speaking
    participant ST as storage
    participant M as media
    participant AI as platform/ai

    U->>S: POST /speaking/upload-intent
    S->>S: consent check, quota check
    S->>ST: PresignPut (5 min, type + size pinned)
    S-->>U: { upload_url, asset_id }
    U->>ST: PUT audio directly
    U->>S: POST /speaking/attempts { asset_id, task_id }
    S->>ST: Stat — verify size and type
    S->>S: BEGIN; INSERT attempt(status=processing); enqueue job; COMMIT
    S-->>U: 202 { attempt_id }

    M->>M: transcode → 16 kHz mono
    M->>M: ASR → transcript + word timings
    alt low confidence
        M->>S: fail with TRANSCRIPTION_LOW_CONFIDENCE
        S-->>U: ask to re-record; no quota charged
    else
        M->>M: pronunciation assessment
        M->>S: media.transcribed + scores
        S->>AI: Run(speaking.feedback, { transcript, scores, task, level })
        AI-->>S: structured coaching
        S->>S: clamp + persist; publish speaking.scored
        S-->>U: attempt ready (poll or SSE)
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

### Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->

## 17. Decisions

### Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Buy or build pronunciation assessment? | Buy, behind an interface | Phoneme-level scoring is a research problem; buying lets us ship and measure, and the interface keeps a self-hosted GOP option open (plan review Q2) |
| Keep recordings indefinitely? | 90 days, learner-deletable, pinnable | Storage cost and privacy exposure both grow with retention, and the scores — which are what progress is built on — do not need the audio |
| Send audio to the LLM? | No — transcript and scores only | Cheaper, faster, and it keeps voice data out of a second vendor's hands |
<!-- END GENERATED: decisions -->

### Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

### Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->

## 18. Backlog

### Backlog

_Empty. Add items with acceptance criteria as they are identified._
<!-- END GENERATED: todo -->

### Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

### Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Real-time streaming feedback
- Noise suppression before ASR
- Accent-aware scoring calibration
- Conversation practice with a model
<!-- END GENERATED: todo-future -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
