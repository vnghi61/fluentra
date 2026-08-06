---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Inbox tables, endpoints and unread-count caching
- [ ] Preferences with per-category channel settings and quiet hours
- [ ] Event subscriptions for grading, reviews, streaks, security
- [ ] Deduplication and per-user rate limits
- [ ] Email templates in English and Vietnamese
- [ ] Web push registration and delivery with token pruning

## Phase 3

- [ ] Digest scheduling
- [ ] Send-time heuristic
- [ ] Weekly progress email
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Learned send-time optimisation
- In-app notification centre with filters
- Native push when a mobile app exists
<!-- END GENERATED: todo-future -->
