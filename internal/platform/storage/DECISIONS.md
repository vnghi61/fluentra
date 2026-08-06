---
module: storage
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry, job]
depended_on_by: [user, content, media, speaking, writing, analytics, audit]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# storage — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Proxy uploads through the API or presign? | Presign | Proxying makes API memory and bandwidth scale with file size, which would cap horizontal scaling on the exact operation that grows fastest |
| MinIO or S3? | MinIO, S3-compatible | Self-hosted in v1 with no code change required to move to S3 later — the SDK and the semantics are the same |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0018](../../../docs/adr/ADR-0018-media-presigned-upload.md) — Presigned direct-to-storage uploads
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
