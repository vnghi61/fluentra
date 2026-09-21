---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources]
depends_on: [storage, job]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Next, in order

- [ ] Purge a deleted user's objects from fluentra-uploads. The FK cascades the rows away and leaves the files; done when account deletion leaves no object under that user's prefix
- [ ] Renditions into fluentra-derived (P3, work order 18); done when a validated image has a thumbnail and the original is byte-identical
- [ ] Extraction and transcription of validated resources (P4, work order 19); done when a validated PDF yields text tagged to spine nodes
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
