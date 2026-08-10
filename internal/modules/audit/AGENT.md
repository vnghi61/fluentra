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
| interface | `audit.Recorder` | `Record(ctx, Entry)` — best effort from any module; it returns nothing, so it cannot fail the caller |
| interface | `audit.SecurityRecorder` | `RecordSecurityEvent(ctx, SecurityEvent)` — the same, for the security stream |
| struct | `audit.Entry` | `{Action, TargetType, TargetID, Before, After, ChangedFields, Meta}` — actor, client address and trace context come from `ctx` |
| struct | `audit.SecurityEvent` | `{Kind, Severity, UserID, Detail}` |
| const | `audit.PermissionRead / PermissionExport / PermissionManage` | The permission names the admin operations require, as plain strings — `audit` does not depend on `rbac` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `user.* and rbac.*` | consumes | Persisted to `audit_logs`; `service.SubscribedTopics()` is the exact list |
| `auth.security_event, rbac.access_denied` | consumes | Persisted to `security_events` instead |
<!-- END GENERATED: contract -->

### How this module reads other modules' events without importing them

Every arrow in [`MODULE_INDEX.md`](../../../MODULE_INDEX.md) §3 points **into** `audit`. It may
not import `user/contract` or `rbac/contract` to unmarshal what they published, so the consumer
reads the payload **structurally**, by field name:

```go
type envelope struct {
    OccurredAt    time.Time      `json:"occurred_at"`
    ActorID       *uuid.UUID     `json:"actor_id"`
    UserID        *uuid.UUID     `json:"user_id"`
    ChangedFields []string       `json:"changed_fields"`
    Before, After map[string]any `json:"before" / "after"`
    Severity      string         `json:"severity"`
}
```

That is the convention every emitting module follows, and it is why they write those field
names. A field a payload does not carry stays nil and is simply not recorded; a module `audit`
has never heard of still produces a usable entry.

The cost is that renaming an event in `user` does not break this at compile time. What catches
it is `TestOutboxEventBecomesAnAuditEntry`, which drives a real outbox row through the real
publisher and asserts a row appears.

**A module wiring `audit` in needs to know two things.** `Deps.Guard` is a
`Require(ctx, permission string) error` declared by this module — the composition root adapts
`rbac.Authorizer` to it, because `audit` cannot import `rbac`. And `Deps.IPHashKey` keys the
HMAC over the client address; leave it empty and no address is recorded at all, which is the
safe default rather than a broken one.

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `audit` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/audit/` · Queries: `db/queries/audit/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `audit.audit_logs` | Administrative and user action trail | Partitioned monthly on `created_at`, which the consumer takes from the emitting module's event so a redelivery lands in the same partition. `event_id` (the idempotency key, UNIQUE with `created_at`), `actor_id`, `actor_role`, `action`, `target_type`, `target_id`, `changed_fields` text[], `before`/`after`/`meta` jsonb, `ip_hash`, `trace_id`. No UPDATE or DELETE grant on this table, and none on its partitions. |
| `audit.security_events` | Security-relevant occurrences | Partitioned monthly on `created_at` like the trail. `event_id`, `kind`, `severity`, `user_id`, `detail` jsonb, `ip_hash`, `trace_id`, `resolved_at`/`resolved_by`/`resolution_note`. Takes UPDATE so an event can be triaged, and still no DELETE. |

**Indexes of note**

- `idx_audit_logs_target` — the "who touched this record" query
- `idx_audit_logs_actor_time` — the "what did this admin do" query
- `idx_audit_logs_action_time` — search by exact action name
- `idx_security_events_kind_time` — dashboard aggregation
- `idx_security_events_open` — partial, the open triage queue
- `idx_security_events_id` — resolving addresses an event by id alone, and the primary key leads with the partition key
- All are declared on the parent, so every partition inherits them.
<!-- END GENERATED: schema -->

### Append-only is a grant, not a habit

The bootstrap migration runs
`ALTER DEFAULT PRIVILEGES … IN SCHEMA audit GRANT SELECT, INSERT, UPDATE, DELETE`, so **every
table created in this schema starts out fully writable by `fluentra_app`**. BR-AUDIT-01 is
therefore a `REVOKE` in `db/migrations/audit/1700000030_*.sql`, not an omission:

```sql
REVOKE ALL ON audit.audit_logs FROM fluentra_app;
GRANT SELECT, INSERT ON audit.audit_logs TO fluentra_app;
```

If you add a table here, do the same, or it is writable by default.

### Partitions, and why a function creates them

Postgres checks privileges on the relation named in the statement, so the parent's grant does
not cover a partition somebody addresses directly. And a partition made next month is a _new_
table, which the default privileges above would hand `UPDATE` and `DELETE` — append-only would
have lasted exactly until the month rolled over.

Both are handled by `audit.ensure_partitions(months_ahead)`: `SECURITY DEFINER`, owned by
`fluentra_migrator`, `EXECUTE` granted only to `fluentra_app`, `REVOKE`d from `PUBLIC`. It
creates the partition and immediately re-applies the restricted grants to it. The application
role runs it without holding any DDL privilege of its own — which is the point, because a role
that can `CREATE TABLE` in this schema can also create one that shadows the trail.

`audit.detach_expired_partitions(retain)` is its counterpart, and it **detaches rather than
drops**: detaching is instant and reversible, dropping is neither, and archiving the detached
table to object storage needs `storage`. A detached partition keeps its rows and loses its
grants, so nothing can write to a table no search will read.

Partitions are created through dynamic SQL, which also keeps literal
`CREATE TABLE audit_logs_y2026m08` statements out of the migration file — `tools/docgen/check-drift.mjs`
regex-matches those and would demand every partition name in the `tables:` front-matter.

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

**As of P1.4, three of those four are implemented.** `GET /admin/audit-logs/export` is
specified above and is deliberately absent from `openapi.yaml` and from the router: an
asynchronous export has to put the artefact somewhere and hand back a signed URL, which needs
`platform/storage`, and this module depends only on `job`. Adding that arrow is a boundary
change, so it is a card of its own — see [`TODO.md`](TODO.md).

Two things about the three that do exist:

- **The search window is not optional.** Both list operations default `to` to now and `from` to
  90 days before it, and cap the span at 400 days. Both tables are partitioned monthly on
  `created_at`, so a query without a bound on it reads every partition that has ever been
  created. The default is a bounded scan rather than an unbounded one that happens to be fast
  while the table is young.
- **This module mounts no `/admin` group.** `rbac` already mounts one and chi allows a single
  handler per mount point, so `Routes` registers the three full paths and the composition root
  wraps them in `rbac`'s `AdminOnly`. Each handler still calls `Require` with its own
  permission: the middleware fences the route, the guard locks the operation.

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
- `GET /admin/audit-logs/export` is specified but not implemented: an async export needs `storage` for the signed URL, and `audit` does not depend on it. See TODO.md.
- Retention detaches an expired partition rather than archiving and dropping it, for the same reason. A detached partition keeps its rows and loses its grants.
- Search is bounded to a 400-day window and defaults to 90 days, so a query can always be pruned to a fixed number of partitions. There is no way to ask for the whole table.
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

- The application role genuinely cannot UPDATE or DELETE audit rows — on the parent and on each partition
- A partition created by the rotation job inherits the restricted grants rather than the schema defaults
- Audit failure does not roll back the business operation
- Duplicate event delivery produces exactly one row, including when the payload carries no occurred_at
- PII redaction in diffs, asserted against the stored columns and not only the function
- Partition rotation is idempotent, and retention detaches only fully expired months
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not use audit as application logging — that is Loki.
- Do not let an audit failure abort a business operation.
- Do not store raw IP addresses or full user agents.
- Do not grant UPDATE or DELETE on `audit_logs` to the application role, for any reason.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
