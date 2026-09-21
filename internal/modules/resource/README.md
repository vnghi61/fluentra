---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources, renditions]
depends_on: [storage, job, user]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource

Intake and lifecycle management for uploaded documents, media and web URLs that serve as the foundation for learning items, extraction and generation.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Learners and educators bring external materials to study. Intake must accept files and links safely, securely and reliably before downstream extraction and exercise generation can begin.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Upload intents: a presigned POST policy into fluentra-uploads that pins the content type and a 1 B to 50 MB size range, valid 5 minutes
- Confirming an upload and submitting a URL, each writing its row and enqueuing resource.validate in one transaction
- Lifecycle pending, uploaded, then validated, rejected or failed
- File validation by magic bytes against an allow-list: PDF, DOC, DOCX, PPT, PPTX, PNG, JPEG, WebP, MP3, WAV, M4A, MP4, WebM
- URL validation that stores the page title and never the body, through a client whose dialer refuses non-public addresses
- Per-user quotas: 50 resources and 250 MB, counting everything not rejected or failed
- Presigned GET for a validated file its owner requests
- Derived visual, audio, and video renditions in fluentra-derived for validated file resources
- Deleting a resource together with its stored object and derived renditions
- A cron sweep that fails abandoned intents and deletes their objects, and fails uploads whose validation never finished
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**IMPLEMENTED** — planned for delivery phase 4. See [/ROADMAP.md](../../../ROADMAP.md).
