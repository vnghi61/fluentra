---
module: subscription
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [plans, entitlements, subscriptions, subscription_events]
depends_on: [payment, user, notification, cache, job, audit]
depended_on_by: [writing, speaking, vocabulary, exam, ai, admin, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# subscription — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Plans and entitlements with a seeded free tier
- [ ] Subscription lifecycle with the full state machine
- [ ] Entitlement resolution with caching and eager invalidation
- [ ] Trials with explicit conversion consent
- [ ] Upgrade with proration, downgrade at period end
- [ ] Grace period handling on payment failure
- [ ] Cancellation and reactivation
- [ ] Admin search and complimentary grants
- [ ] Upgrade prompts wherever a feature is gated
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Metered top-ups for AI grading
- Annual plans with a discount
- Regional pricing
- Referral credits
<!-- END GENERATED: todo-future -->
