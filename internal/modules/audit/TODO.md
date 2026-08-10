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
last_verified: 2026-08-10
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

## Progress

The list above is generated from `tools/docgen/data/core.json`, so its checkboxes cannot be
ticked by hand. Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P1.4 | 2026-08-10 | `audit_logs` and `security_events` partitioned monthly with the current month plus three created ahead; the `Recorder` and `SecurityRecorder` contracts; the outbox consumer, idempotent on `event_id`; append-only enforced by `REVOKE` and proven against the real role on the parent and on every partition; PII redaction in diffs; the two admin search operations and the resolve operation; the partition-rotation and retention cron jobs |

## Open after P1.4

- [x] **Wire the module in (P1.5).** Done 2026-08-10. `cmd/api` constructs it and adapts
      `rbac.Authorizer` to `audit.Deps.Guard`; `cmd/worker` calls `Subscribe(bus)` and registers
      `CronJobs()`. Proven by `cmd/api/wiring_integration_test.go`.
- [x] **Carry the request's trace into the entry.** Done 2026-08-10. `ops.outbox_events` now
      holds the producing transaction's `traceparent`, and the publisher restores it into the
      context it dispatches with — so BR-AUDIT-07 is kept in full: the id in the row is the one
      an operator pastes into Tempo to see the request. This module did not change; it already
      read the trace from the context it was handed. `TestTheTrailRecordsTheRequestsTraceID`
      in `cmd/api` is the end-to-end proof, and it drains in a deliberately different trace so
      it cannot pass by coincidence.
- [ ] **`GET /admin/audit-logs/export`.** Specified in AGENT.md §6, absent from `openapi.yaml`.
      An async export needs `platform/storage` for the signed URL and `audit` depends only on
      `job`, so it needs the dependency arrow added to `MODULE_INDEX.md` §3 and
      `.go-arch-lint.yml` — a card of its own, not a follow-up commit. Done when an admin can
      request an export and receive a short-lived link, and the artefact expires.
- [ ] **Archive detached partitions.** `audit.detach_expired_partitions` takes a partition out of
      the tree and removes its grants; nothing yet copies it to object storage and drops it, so
      expired partitions accumulate as detached relations. Same `storage` dependency as the
      export. Done when a detached partition is archived, verified, and then dropped.
- [ ] **Supply a real `IPHashKey`.** The module hashes the client address with an HMAC and
      records nothing when the key is empty, which is the current state — no address flows in
      before the auth middleware exists (P2.4) anyway. Done when the key is a documented entry in
      `.env.example` and `docs/deployment/configuration.md`, read by `cmd/api`, and rotatable.
- [ ] **A counter for dropped entries.** `Recorder.Record` swallows a write failure by design
      (BR-AUDIT-02) and logs at `error`; the consumer path is already visible through
      `outbox_lag_seconds` and `ops.job_failures`, but the best-effort path has no metric.
      `platform/telemetry.Instruments` has no module-level counter to hang it on, and adding one
      is a platform change. Done when a dropped entry increments something Prometheus scrapes.
- [ ] **Record the user agent.** §2 lists it among what an entry carries and no column holds one.
      Storing it needs a parsed family rather than the raw string (§14). Done when the family is
      stored and the raw header provably is not.

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
