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

**The trace id on an entry is the request's, not the worker's.** `audit` does nothing to
achieve that beyond reading `httpx.TraceID(ctx)`: `ops.outbox_events` carries the producing
transaction's `traceparent`, and `shared/outbox`'s publisher restores it into the context it
dispatches with, so the consumer is already inside the trace of the action. An event written
outside a span — a seed, a migration — records no trace rather than an invented one.

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
(tag: `audit`). Per-endpoint detail is in §15.

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
change, so it is a card of its own — see [`TODO.md`](#18-backlog).

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

Global rules: [`docs/CONVENTIONS.md#coding-standard`](../../../docs/CONVENTIONS.md#coding-standard). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Security considerations

- Reading the audit trail is itself audited.
- Exports are generated asynchronously and delivered by short-lived signed URL.
- IP addresses are stored hashed with a rotating salt; the raw value is never persisted.

## 13. Testing

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

## 15. API detail

### Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
#### `GET /api/v1/admin/audit-logs`

Search the audit trail

| | |
|---|---|
| Permission | `audit.read` |
| Success | 200 |
| Errors | standard set |

#### `GET /api/v1/admin/audit-logs/export`

Async CSV export

| | |
|---|---|
| Permission | `audit.export` |
| Success | 202 |
| Errors | standard set |

#### `GET /api/v1/admin/security-events`

Security event feed

| | |
|---|---|
| Permission | `audit.read` |
| Success | 200 |
| Errors | standard set |

#### `POST /api/v1/admin/security-events/{id}/resolve`

Mark an event triaged

| | |
|---|---|
| Permission | `audit.manage` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

### What is actually served (P1.4)

Three of the four above are in `openapi.yaml` and mounted. `GET /admin/audit-logs/export` is
not: an async export needs `platform/storage` for the signed URL, which is not a dependency of
this module. See [`TODO.md`](#18-backlog).

#### Search parameters

`GET /admin/audit-logs` and `GET /admin/security-events` share the window, cursor and limit
parameters and differ only in what they filter on.

| Parameter | Both | `audit-logs` | `security-events` |
|---|---|---|---|
| `from`, `to` | RFC 3339. `to` defaults to now, `from` to 90 days before it | | |
| `cursor` | Opaque, from the previous page's `next_cursor` | | |
| `limit` | 1–100, default 20 | | |
| filters | | `actor_id`, `action`, `target_type`, `target_id` | `kind`, `severity`, `resolved`, `user_id` |

**The window is the important one.** Both tables are partitioned monthly on `created_at`. A
query with no bound on it reads every partition ever created, so the server supplies a default
rather than letting a caller omit one, and trims any span over 400 days from the far end. A
client that wants everything gets the most recent 400 days of it.

Ordering is `created_at DESC, id DESC` — the primary key of both tables — so the keyset cursor
pages deep at the same cost as shallow. `next_cursor` is present only when another page exists.

#### Two things the responses do not carry

- **`ip_hash`.** The column exists; no response exposes it. It is a pseudonymous identifier that
  correlates one person's activity across sessions, and reading the trail does not need it.
- **Values, usually.** `changed_fields` names what moved; `before`/`after` are null unless the
  emitting module sent values, and anything on the PII deny-list is `[redacted]` when it did
  (BR-AUDIT-04). An audit log holding every old display name would be a second store of personal
  data with a longer retention period than the first.

### Error codes

<!-- BEGIN GENERATED: api-errors -->
_None yet._
<!-- END GENERATED: api-errors -->

| Code | Status | When |
|---|---|---|
| `UNAUTHENTICATED` | 401 | No actor on the request |
| `PERMISSION_DENIED` | 403 | The caller lacks `audit.read` or `audit.manage` |
| `NOT_FOUND` | 404 | No such security event — also returned for a malformed id, so probing the surface reveals nothing about its shape |
| `SECURITY_EVENT_ALREADY_RESOLVED` | 409 | A second resolution. The first administrator's explanation describes what was investigated; the second would replace it with a guess |
| `BAD_REQUEST` | 400 | The cursor did not decode. It is a token the server issued, not a field a person typed |
| `VALIDATION_FAILED` | 422 | A malformed filter, an out-of-range `limit`, `from` after `to`, or a missing/blank/overlong `note` |

### Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/docs/CONVENTIONS.md](../../../docs/CONVENTIONS.md#rate-limiting).
<!-- END GENERATED: api-rate -->

## 16. Flows

### Recording an admin action

```mermaid
sequenceDiagram
    autonumber
    participant AD as admin handler
    participant SV as content.Service
    participant DB as PostgreSQL
    participant OB as outbox publisher
    participant AU as audit

    AD->>SV: PublishContent(ctx, id)
    SV->>DB: BEGIN
    SV->>DB: UPDATE content_versions SET status='published'
    SV->>DB: INSERT outbox(content.published, before, after)
    SV->>DB: COMMIT
    OB->>AU: deliver event
    AU->>DB: INSERT audit.audit_logs (actor, action, diff, trace_id)
    Note over AU,DB: at-least-once; the handler dedupes on event_id
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

### Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->

## 17. Decisions

### Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Postgres or a log stream for audit? | Postgres | Audit is queryable business record with retention and access-control requirements; Loki is optimised for cheap, short-lived telemetry and is not a system of record |
| Synchronous or asynchronous recording? | Both, by criticality | Permission and money changes go through the outbox so they cannot be lost; reads are best-effort so an audit outage cannot break browsing |
<!-- END GENERATED: decisions -->

### Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

### Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->

## 18. Backlog

### Phase 1

- [ ] `audit_logs` and `security_events` with monthly partitions
- [ ] `audit.Recorder` contract and outbox consumer
- [ ] Append-only enforcement via database grants, proven by a test
- [ ] Admin search UI endpoints with filters
- [ ] Retention and partition-rotation jobs
- [ ] PII redaction in diffs
<!-- END GENERATED: todo -->

### Progress

The list above is generated from `tools/docgen/data/core.json`, so its checkboxes cannot be
ticked by hand. Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P1.4 | 2026-08-10 | `audit_logs` and `security_events` partitioned monthly with the current month plus three created ahead; the `Recorder` and `SecurityRecorder` contracts; the outbox consumer, idempotent on `event_id`; append-only enforced by `REVOKE` and proven against the real role on the parent and on every partition; PII redaction in diffs; the two admin search operations and the resolve operation; the partition-rotation and retention cron jobs |

### Open after P1.4

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
      `.env.example` and `.env.example`, read by `cmd/api`, and rotatable.
- [ ] **A counter for dropped entries.** `Recorder.Record` swallows a write failure by design
      (BR-AUDIT-02) and logs at `error`; the consumer path is already visible through
      `outbox_lag_seconds` and `ops.job_failures`, but the best-effort path has no metric.
      `platform/telemetry.Instruments` has no module-level counter to hang it on, and adding one
      is a platform change. Done when a dropped entry increments something Prometheus scrapes.
- [ ] **Record the user agent.** §2 lists it among what an entry carries and no column holds one.
      Storing it needs a parsed family rather than the raw string (§14). Done when the family is
      stored and the raw header provably is not.

### Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

### Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Hash-chained entries for tamper evidence
- Anomaly detection over the security event stream
- Per-record access history shown to the user
<!-- END GENERATED: todo-future -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
