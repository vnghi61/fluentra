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

# telemetry — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 0

- [ ] Provider construction and graceful shutdown
- [ ] OTLP exporters with resource attributes including version and commit
- [ ] HTTP middleware: trace, request ID, recovery, structured access log
- [ ] slog handler with the OTLP bridge and redaction allowlist
- [ ] Standard instruments for HTTP, DB, cache, storage and jobs
- [ ] `/health`, `/ready`, `/version`
- [ ] Collector pipeline configuration including tail sampling
- [ ] The first Grafana dashboard, proving log→trace correlation works
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Continuous profiling with Pyroscope
- RUM metrics from the browser SDK
- Exemplars on all latency histograms
<!-- END GENERATED: todo-future -->
