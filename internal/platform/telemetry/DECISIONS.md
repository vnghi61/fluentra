---
module: telemetry
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: []
depended_on_by: [ai, cache, storage, job, media, search, mailer]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# telemetry — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Direct exporters or a Collector? | Collector | One ingest point; backend changes, sampling and redaction become operational configuration instead of a deploy |
| Jaeger or Tempo? | Tempo in production, Jaeger only in the dev profile | Tempo shares MinIO for storage and integrates natively with Grafana; running both in production would double the operational surface for no benefit — see the plan review §6 |
| zap/zerolog or slog? | slog | Standard library, structured, and the performance difference is immaterial at our request volume |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0013](../../../docs/adr/ADR-0013-observability-otel.md) — OTel SDK + Collector; Tempo over Jaeger
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
