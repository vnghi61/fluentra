---
module: cache
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry]
depended_on_by: [auth, rbac, user, content, lesson, learning, srs, gamification, ai, notification, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# cache — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/cache` |
| Schema | `none` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
A typed facade over Redis: key building, serialisation, TTL policy, single-flight, distributed locks, and rate limiting. Modules never touch the Redis client directly, so key conventions and degradation behaviour are enforced in one place.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Typed get/set/delete with a generic `Cache[T]`
- Key construction following the repository convention, including schema versioning
- Cache-aside and write-through helpers
- Single-flight to prevent stampedes
- Jittered TTLs
- Distributed locks (`SET NX PX`) with safe release
- Rate limiting (GCRA via `redis_rate`)
- Graceful degradation when Redis is unavailable
- Hit-ratio and latency metrics per module

**This module does NOT own:**

- Deciding what to cache — each module owns that decision
- Being a source of truth for anything except rate-limit counters and token denylists
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/cache/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/cache/contract/` | You are calling this module from another module |
| `internal/platform/cache/service/` | You are changing behaviour |
| `db/migrations/cache/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/cache/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `cache.Cache[T]` | `Get`, `Set`, `Delete`, `GetOrLoad` — typed, so a cached value cannot be deserialised into the wrong shape |
| interface | `cache.Locker` | `Acquire(ctx, key, ttl)` returning a release function |
| interface | `cache.Limiter` | `Allow(ctx, key, limit)` returning remaining and reset |
| func | `cache.Key` | Builds `fluentra:{env}:{module}:{entity}:{id}:v{n}` — the only sanctioned way to make a key |

### Events

_None yet._
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
This module owns no tables.
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `cache`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `DELETE` | `/api/v1/admin/cache/{pattern}` | `system.cache` | Operational invalidation by key prefix |
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
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | see its contract |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`rbac`](../../modules/rbac/AGENT.md) | ← used by | consumes this module's contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`lesson`](../../modules/lesson/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`srs`](../../modules/srs/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
| [`ai`](../../platform/ai/AGENT.md) | ← used by | consumes this module's contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`search`](../../platform/search/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-CACHE-01** — Every read path must work with Redis down: fall back to the source, log at `warn`, increment `cache_unavailable_total`. A cache outage degrades latency, never correctness.
2. **BR-CACHE-02** — Keys are built only via `cache.Key`. A hand-built key string fails review.
3. **BR-CACHE-03** — Every cached class carries a schema version; bumping it invalidates the whole class without scanning.
4. **BR-CACHE-04** — TTLs carry ±10 % jitter to avoid synchronised expiry.
5. **BR-CACHE-05** — `GetOrLoad` uses single-flight so a cold key produces one loader call, not a thundering herd.
6. **BR-CACHE-06** — Authorization decisions are never cached across users.
7. **BR-CACHE-07** — Personal data in the cache carries the same retention expectations as the database — deletion must bust the key.
8. **BR-CACHE-08** — Locks always have a TTL and are released with a token check, so a slow holder cannot release someone else's lock.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Cache a read path

1. Confirm it is worth it: measure the underlying query first. Caching a 3 ms query buys nothing and adds an invalidation bug.
2. Choose the key shape and register it in your module's AGENT.md §Cache strategy.
3. Decide the invalidation trigger before writing the read — an uninvalidated cache is a correctness bug waiting.
4. Use `GetOrLoad`, not manual get/set.
5. Add a test proving the fallback path works with Redis unavailable.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Single Redis instance — no cluster in v1. A failure degrades every module simultaneously, which is why degradation is mandatory rather than optional.
- No cache warming; cold starts hit the database.
- Prefix invalidation uses `SCAN`, which is fine at our key counts but is not free.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `RATE_LIMITED` | 429 | Rate limiter rejected the request |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/platform/cache/...                    # unit
go test -tags=integration ./internal/platform/cache/...  # integration (testcontainers)
```

**Focus areas**

- Every consumer's fallback path with Redis stopped
- Single-flight collapses concurrent loads into one
- TTL jitter is applied
- Lock release is token-checked
- Rate limiter boundaries: exactly at the limit, and just over
- Version bump invalidates a whole class
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not build a key by string concatenation.
- Do not treat the cache as a source of truth (except rate limits and denylists).
- Do not cache without deciding how it is invalidated.
- Do not cache a permission decision across users.
- Do not use `KEYS` — ever.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
