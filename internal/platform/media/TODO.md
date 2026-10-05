---
module: media
tier: platform
group: platform
status: IMPLEMENTED
phase: 3
owner: "@platform-team"
schema: content
tables: [tts_cache]
depends_on: [storage, job, ai, telemetry]
depended_on_by: [speaking, listening, content, vocabulary, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# media — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] ffmpeg transcode stages with resource limits
- [ ] Waveform extraction
- [ ] ASR adapter with word timings
- [ ] Pronunciation assessment adapter
- [ ] TTS adapter with content-hash caching
- [ ] Image processing for avatars and content images
- [ ] Orphan GC job
- [ ] Speech benchmark corpus and a provider comparison document
<!-- END GENERATED: todo -->

## Completed local processing repair (2026-09-30)

- [x] Automatically process pending/new local resources in a dedicated, non-root Compose renderer with LibreOffice, Poppler and ffmpeg; `make dev-media` supports host-run API/worker setups.
- [x] Keep render output files until extraction and upload finish instead of deleting them when `processRendition` returns; cover output lifetime with a regression test.

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Self-hosted Whisper for cost control at volume
- Forced alignment fallback
- Per-learner voice preference
- Noise suppression before ASR
<!-- END GENERATED: todo-future -->
