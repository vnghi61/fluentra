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
