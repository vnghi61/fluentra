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

# media

Everything that happens to audio and images: transcoding, waveform generation, speech recognition, pronunciation assessment, text-to-speech, and thumbnailing. It is the CPU-heavy part of the system and the first candidate for extraction to a separate service.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Listening and speaking are half the product. Neither works without a reliable pipeline that turns a learner's browser recording into something a scoring engine can read, and turns published text into natural audio.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Audio transcoding (ffmpeg) to canonical formats: 16 kHz mono for ASR, Opus for playback
- Waveform peak extraction for the player UI
- ASR adapter: transcript plus word-level timings
- Pronunciation assessment adapter: phoneme-level accuracy, fluency, completeness
- Text-to-speech adapter with voice selection and caching
- Image processing: resize, EXIF strip, re-encode, thumbnail
- Duration, format and loudness validation
- Orphaned-derivative garbage collection
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**PLANNED** — planned for delivery phase 3. See [/ROADMAP.md](../../../ROADMAP.md).
