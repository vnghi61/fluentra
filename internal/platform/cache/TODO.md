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

# cache — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] Typed `Cache[T]` with JSON and msgpack codecs
- [ ] `cache.Key` builder with environment and version segments
- [ ] `GetOrLoad` with single-flight and jittered TTL
- [ ] Degradation path plus `cache_unavailable_total`
- [ ] Distributed locks with token-checked release
- [ ] Rate limiter wrapping `redis_rate`
- [ ] Per-module hit-ratio metrics
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Two-tier cache with an in-process L1
- Redis Cluster or Valkey if licensing or scale demands
- Cache warming for the daily review queue
<!-- END GENERATED: todo-future -->
