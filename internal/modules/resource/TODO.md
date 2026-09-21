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
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Next, in order

- [ ] Purge an erased user's resources. Erasure anonymises the user row instead of deleting it, so the FK cascade never runs and both the rows and the files survive; subscribe to user.deleted as speaking does (work order 18, step 1). Done when erasure leaves no row and no object in either bucket
- [ ] Renditions into fluentra-derived by cmd/media in GitHub Actions (P3, work order 18); done when a validated image has a thumbnail and the original is byte-identical
- [x] Extraction and transcription of validated resources (P4, work order 19); done when a validated PDF yields text tagged to spine nodes
- [ ] Publish resource.validated and resource.rejected through the outbox once P4 exists to consume them
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
