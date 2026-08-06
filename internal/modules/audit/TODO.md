---
module: audit
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: audit
tables: [audit_logs, security_events]
depends_on: [job]
depended_on_by: [auth, user, rbac, admin, content, questionbank, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# audit — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] `audit_logs` and `security_events` with monthly partitions
- [ ] `audit.Recorder` contract and outbox consumer
- [ ] Append-only enforcement via database grants, proven by a test
- [ ] Admin search UI endpoints with filters
- [ ] Retention and partition-rotation jobs
- [ ] PII redaction in diffs
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Hash-chained entries for tamper evidence
- Anomaly detection over the security event stream
- Per-record access history shown to the user
<!-- END GENERATED: todo-future -->
