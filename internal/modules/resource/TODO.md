---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources, renditions, extractions, classifications]
depends_on: [storage, job, user]
depended_on_by: [studio]
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Next, in order

- [ ] audio_web is transcoded for every audio upload; WO 18 asks for it only when the source is WAV or over 256 kbit/s. Needs an ffprobe bitrate check before the transcode
- [ ] Video renditions are measured against the 10-minute limit after the transcode, not before it: a long video spends the whole encode budget and is then skipped
- [ ] classifications.ai_request_id stays null until platform/ai returns the ai_requests row id
- [ ] OCR for text-heavy images (WO 19 B.1, optional; the first thing cut)
- [ ] Publish resource.validated and resource.rejected through the outbox once something consumes them
- [ ] A sweeper for `course-materials/{course_id}` copies no published version references (WO 20 Stage C; out of scope, the copies are cheap and correct to keep)
- [ ] A DOC/PPT material has no PDF rendition (WO 18 renders only thumbnail and preview), so its card opens the original download; a `pdf` rendition is a future step
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
- Retention of untouched resources. Needs a last-opened column, which P4 can add when it starts opening them.
- Sharing a resource with another learner. Resources are private; sharing is a visibility column, not a redesign.
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Multi-part uploads for large video files
- A per-user rate limit on intents. RATE_LIMIT_UPLOAD_PER_HOUR exists in .env.example and is read by nothing
<!-- END GENERATED: todo-future -->
