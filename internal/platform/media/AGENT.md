---
module: media
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@platform-team"
schema: content
tables: [media_derivatives, transcripts, tts_cache]
depends_on: [storage, job, ai, telemetry]
depended_on_by: [speaking, listening, content, vocabulary, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# media — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/media` |
| Schema | `content` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Everything that happens to audio and images: transcoding, waveform generation, speech recognition, pronunciation assessment, text-to-speech, and thumbnailing. It is the CPU-heavy part of the system and the first candidate for extraction to a separate service.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Audio transcoding (ffmpeg) to canonical formats: 16 kHz mono for ASR, Opus for playback
- Waveform peak extraction for the player UI
- ASR adapter: transcript plus word-level timings
- Pronunciation assessment adapter: phoneme-level accuracy, fluency, completeness
- Text-to-speech adapter with voice selection and caching
- Image processing: resize, EXIF strip, re-encode, thumbnail
- Duration, format and loudness validation
- Orphaned-derivative garbage collection

**This module does NOT own:**

- Storing the files — that is `platform/storage`
- Deciding what a pronunciation score means for a learner — that is `speaking`
- Text generation — that is `platform/ai`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/media/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/media/contract/` | You are calling this module from another module |
| `internal/platform/media/service/` | You are changing behaviour |
| `db/migrations/media/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/media/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `media.Processor` | `Transcode`, `Waveform`, `Thumbnail` — enqueue-and-return, never synchronous |
| interface | `media.Recognizer` | `Transcribe(ctx, assetID, lang)` → transcript with word timings |
| interface | `media.Assessor` | `AssessPronunciation(ctx, assetID, referenceText)` → phoneme-level scores |
| interface | `media.Synthesizer` | `Speak(ctx, text, voice)` → an asset ID, cached by text and voice |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `media.processed` | publishes | `{asset_id, derivatives, duration_ms}` |
| `media.transcribed` | publishes | `{asset_id, transcript_id, confidence}` |
| `media.processing_failed` | publishes | `{asset_id, stage, reason}` |
| `content.published` | consumes | Pre-generate TTS for newly published text |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `content` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/media/` · Queries: `db/queries/media/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `content.media_derivatives` | Derived artefacts from a source asset | `source_asset_id`, `kind` (opus/wav16k/waveform/thumb), `object_key`, `status`, `duration_ms` |
| `content.transcripts` | ASR output | `asset_id`, `text`, `words` jsonb (with timings), `confidence`, `provider`, `language` |
| `content.tts_cache` | Synthesised audio keyed by text and voice | `text_hash` + `voice` UNIQUE, `object_key`, `hits` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `media`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/media/{asset_id}/derivatives` | `content.read` | Inspect the pipeline output for one asset |
| `POST` | `/api/v1/admin/media/{asset_id}/reprocess` | `content.manage` | Re-run the pipeline |
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
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Read sources, write derivatives |
| [`job`](../../platform/job/AGENT.md) | → depends on | Every stage runs as a job — none is fast enough for a request |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | Post-processing of transcripts into feedback is an AI task, invoked by the calling module |
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | Per-stage latency and failure metrics |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`listening`](../../modules/listening/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-MEDIA-01** — No media processing happens in the API process. Every stage is a job on the `media` queue.
2. **BR-MEDIA-02** — Source files are never mutated; every output is a new derivative with its own object key.
3. **BR-MEDIA-03** — ASR receives 16 kHz mono PCM — always transcode first, never send the browser's original.
4. **BR-MEDIA-04** — Uploads are validated for real content type by magic bytes, not by extension or the declared MIME type.
5. **BR-MEDIA-05** — Maximum recording length is 180 seconds; maximum upload 25 MB. Both are configuration, enforced server-side after upload as well as in the presigned URL.
6. **BR-MEDIA-06** — TTS output is cached by (text hash, voice, model); the same sentence is never synthesised twice.
7. **BR-MEDIA-07** — A failed stage retries up to 3 times, then records `media.processing_failed` with a user-safe reason.
8. **BR-MEDIA-08** — Raw user uploads are deleted once derivatives exist and are verified; only the normalised versions are retained.
9. **BR-MEDIA-09** — Every derivative records the exact tool version used, so a pipeline change can be correlated with a quality change.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a media stage

1. Add the job kind and register the worker in `job/`.
2. Make it idempotent — a re-run must produce the same derivative key.
3. Add per-stage duration and failure metrics.
4. Add resource limits (time, memory) to the external tool invocation.
5. Cover it with an integration test using a small real audio fixture, not a synthetic buffer.

### Swap the ASR provider

1. Implement `media.Recognizer` in a new adapter directory.
2. Run the benchmark suite in `test/fixtures/corpus/speech/` and compare word error rate and cost.
3. Record the comparison in `docs/knowledge/pronunciation-scoring.md`.
4. Switch by configuration; keep the previous adapter for one release.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- ffmpeg runs in-process in the worker container; a single very large file can occupy a worker slot for minutes.
- Pronunciation assessment quality varies by accent and is weakest for non-native reference speakers — documented for the product team.
- There is no forced-alignment fallback if the ASR provider does not return word timings.
- TTS voice selection is per-locale, not per-learner.
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
| `UNSUPPORTED_AUDIO_FORMAT` | 415 | Magic bytes do not match a supported container |
| `AUDIO_TOO_LONG` | 422 | Exceeds the configured maximum duration |
| `MEDIA_PROCESSING_FAILED` | 500 | A pipeline stage failed after retries |
| `TRANSCRIPTION_LOW_CONFIDENCE` | 422 | Audio too noisy or too quiet to score fairly |

### Security considerations

- Uploads land in a private quarantine bucket and are never publicly readable, even transiently.
- Content type is verified by sniffing before ffmpeg is invoked.
- ffmpeg runs with a wall-clock limit and a memory cap; a malformed file must not be able to exhaust the worker.
- Voice recordings are personal data: 90-day retention, deletable by the learner, and never sent to a provider without no-training terms.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service; integration tests carry the pipeline**

```bash
go test ./internal/platform/media/...                    # unit
go test -tags=integration ./internal/platform/media/...  # integration (testcontainers)
```

**Focus areas**

- Magic-byte sniffing rejects a renamed file
- Duration and size limits enforced after upload, not only in the presigned URL
- Idempotent reprocessing produces identical derivative keys
- Low-confidence transcripts are surfaced rather than scored
- Raw uploads are deleted only after derivatives are verified present
- TTS cache prevents duplicate synthesis
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not process media inside an HTTP handler.
- Do not trust the declared MIME type.
- Do not send the browser's original audio to an ASR provider — transcode first.
- Do not retain raw uploads once derivatives exist.
- Do not interpret scores here — return the numbers and let `speaking` decide what they mean.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
