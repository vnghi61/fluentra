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

# audit — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `core` |
| Path | `internal/modules/audit` |
| Schema | `audit` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Records what happened, who did it, and what changed — immutably. Two streams: an administrative audit trail (every state change made by an admin, and every significant change made by a user to their own data) and a security event stream (authentication anomalies, permission denials, suspicious patterns).
<!-- END GENERATED: overview -->


## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Persisting audit entries with actor, action, target, before/after diff, IP, user agent, trace ID
- Persisting security events with a severity
- Enforcing append-only semantics
- Retention and partition management
- Query API for the admin UI, with filters and export

**This module does NOT own:**

- Deciding what is auditable — each module decides and emits
- Application logs — those go to Loki; audit is durable business record, not telemetry
- Alerting — Prometheus and Alertmanager do that
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/audit/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/audit/contract/` | You are calling this module from another module |
| `internal/modules/audit/service/` | You are changing behaviour |
| `db/migrations/audit/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/audit/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `audit.Recorder` | `Record(ctx, Entry)` — fire-and-forget from any module; never fails the caller |
| struct | `audit.Entry` | `{Action, TargetType, TargetID, Before, After, Meta}` — actor and trace context come from `ctx` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `auth.security_event` | consumes | Persist to `security_events` |
| `*.published / *.suspended / *.deleted` | consumes | Persist significant state changes |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `audit` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/audit/` · Queries: `db/queries/audit/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `audit.audit_logs` | Administrative and user action trail | Partitioned monthly. `actor_id`, `actor_role`, `action`, `target_type`, `target_id`, `before`/`after` jsonb, `ip_hash`, `trace_id`. No UPDATE or DELETE grant on this table. |
| `audit.security_events` | Security-relevant occurrences | `kind`, `severity`, `user_id`, `detail` jsonb, `resolved_at`. Feeds the security dashboard. |

**Indexes of note**

- `idx_audit_logs_target` — the "who touched this record" query
- `idx_audit_logs_actor_time` — the "what did this admin do" query
- `idx_security_events_kind_time` — dashboard aggregation
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `audit`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/audit-logs` | `audit.read` | Search the audit trail |
| `GET` | `/api/v1/admin/audit-logs/export` | `audit.export` | Async CSV export |
| `GET` | `/api/v1/admin/security-events` | `audit.read` | Security event feed |
| `POST` | `/api/v1/admin/security-events/{id}/resolve` | `audit.manage` | Mark an event triaged |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`job`](../../platform/job/AGENT.md) | → depends on | Retention, partition rotation, and export run as scheduled jobs |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
| [`rbac`](../../modules/rbac/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`questionbank`](../../modules/questionbank/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`payment`](../../modules/payment/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-AUDIT-01** — Audit entries are append-only. The application role holds no UPDATE or DELETE grant on `audit_logs`.
2. **BR-AUDIT-02** — Recording an audit entry must never fail the business operation: it is written through the outbox in the same transaction where correctness matters, and best-effort otherwise.
3. **BR-AUDIT-03** — Every admin action on another user's data is audited, including reads of personal data.
4. **BR-AUDIT-04** — The `before`/`after` diff stores changed fields only, and redacts anything on the PII deny-list.
5. **BR-AUDIT-05** — Retention is 2 years; partitions older than that are archived to object storage and detached.
6. **BR-AUDIT-06** — Audit data survives account deletion in anonymised form — the record of an action must outlive the actor's profile.
7. **BR-AUDIT-07** — Every entry carries the `trace_id`, so an audit row links directly to the full distributed trace.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Make a new action auditable

1. Decide whether it must be transactional (money, permissions, publishing) or best-effort (a read).
2. Transactional: write an outbox event in the same transaction. Best-effort: call `audit.Recorder` directly.
3. Name the action `<module>.<verb>_<object>` and add it to the action catalogue in this file.
4. Ensure the diff excludes PII-deny-listed fields.
5. Add a test asserting the entry exists with the right actor and target.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- There is no cryptographic chaining of entries; tamper-evidence relies on database grants and backups. Hash chaining is a candidate if a compliance requirement appears.
- Diffs are field-level, not semantic — a reordered array shows as a full change.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->



### Security considerations

- Reading the audit trail is itself audited.
- Exports are generated asynchronously and delivered by short-lived signed URL.
- IP addresses are stored hashed with a rotating salt; the raw value is never persisted.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/audit/...                    # unit
go test -tags=integration ./internal/modules/audit/...  # integration (testcontainers)
```

**Focus areas**

- The application role genuinely cannot UPDATE or DELETE audit rows
- Audit failure does not roll back the business operation
- Duplicate event delivery produces exactly one row
- PII redaction in diffs
- Partition rotation and retention job correctness
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not use audit as application logging — that is Loki.
- Do not let an audit failure abort a business operation.
- Do not store raw IP addresses or full user agents.
- Do not grant UPDATE or DELETE on `audit_logs` to the application role, for any reason.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
