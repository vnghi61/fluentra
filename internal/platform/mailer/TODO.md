---
module: mailer
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# mailer — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] Sender interface with SMTP implementation and Mailpit in dev
- [ ] MJML build step and the first templates: verification, reset, new device
- [ ] English and Vietnamese locales
- [ ] Delivery log and suppression list
- [ ] Bounce webhook with signature verification
- [ ] Async sending with retry classification
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Provider API implementation
- Deliverability dashboard
- Per-template engagement metrics
<!-- END GENERATED: todo-future -->
