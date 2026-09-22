---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources, renditions, extractions, classifications]
depends_on: [storage, job, user]
depended_on_by: [studio]
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/resource` |
| Schema | `resource` |
| Delivery phase | 4 |
| Status | **IMPLEMENTED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Intake for material a learner brings: an uploaded file or a submitted URL becomes an owned, validated resource with a status its owner can watch. Stops at validated; renditions, extraction and generation are later steps.
<!-- END GENERATED: overview -->

**Context.** Resource acts as the single entry point for external content (PDFs, audio, URLs) into the Fluentra learning engine. It enforces ownership isolation, validates byte streams against MIME spoofing, secures external URL fetching against SSRF, and schedules async validation via River jobs.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Upload intents: a presigned POST policy into fluentra-uploads that pins the content type and a 1 B to 50 MB size range, valid 5 minutes
- Confirming an upload and submitting a URL, each writing its row and enqueuing resource.validate in one transaction
- Lifecycle pending, uploaded, then validated, rejected or failed
- File validation by magic bytes against an allow-list: PDF, DOC, DOCX, PPT, PPTX, PNG, JPEG, WebP, MP3, WAV, M4A, MP4, WebM
- URL validation that stores the page title and never the body, through a client whose dialer refuses non-public addresses
- Per-user quotas: 50 resources and 250 MB, counting everything not rejected or failed
- Presigned GET for a validated file its owner requests
- Derived visual, audio, and video renditions in fluentra-derived for validated file resources
- Extracting text from PDF and Office document resources
- Transcribing audio and video resources via media.HTTPTranscriber
- Grounding and classifying extracted text into CEFR levels, skills, and spine taxonomy nodes
- Deleting a resource together with its stored object, derived renditions, extraction and classification
- A cron sweep that fails abandoned intents and deletes their objects, and fails uploads whose validation never finished
- Reading one resource for its owner and copying its original and ready renditions into a published course's own storage (WO 20)

**This module does NOT own:**

- Generating exercises from a resource (P6)
- Byte storage and serving (platform/storage)
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/resource/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/resource/contract/` | You are calling this module from another module |
| `internal/modules/resource/service/` | You are changing behaviour |
| `db/migrations/resource/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/resource/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `resource.ResourceReader` | `GetResource(ctx, id, userID)`: one resource, for its owner only. No consumer yet |
| struct | `resource.Resource` | `{ID, UserID, Kind, Title, ObjectKey, OriginalFilename, DeclaredMIME, DetectedMIME, ByteSize, Checksum, SourceURL, Status, FailureReason, DownloadURL, CreatedAt, UpdatedAt, ValidatedAt}` |
| interface | `resource.MaterialPublisher` | `MaterialForOwner(ctx, ownerID, resourceID)` and `CopyForPublication(ctx, resourceID, destPrefix)`: the read-and-copy surface `studio` uses to publish a material into a course |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `user.deleted` | consumes |  |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `resource` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/resource/` · Queries: `db/queries/resource/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `resource.resources` | One uploaded file or submitted URL | `user_id`, `kind` (file or url), `title`, `object_key`, `original_filename`, `declared_mime`, `detected_mime`, `byte_size`, `checksum`, `source_url`, `status`, `failure_reason`, `validated_at`. `ck_resources_shape`: a file has an object and no URL, a URL the reverse. `ck_resources_rejected_has_reason`. |
| `resource.renditions` | Derived visual, audio, and video renditions of validated file resources | `resource_id`, `kind`, `status`, `object_key`, `mime_type`, `width`, `height`, `duration_ms`, `byte_size`, `tool_version`, `attempts`, `failure_reason`. `uq_renditions_resource_kind`, `uq_renditions_object_key`. |
| `resource.extractions` | Extracted text from validated documents and audio/video transcripts | `resource_id`, `source` ('pdf_text', 'ocr', 'transcript'), `text`, `char_count` (max 400,000), `truncated`, `language`, `tool_version`. |
| `resource.classifications` | CEFR estimate, targeted skill, and grounded spine taxonomy node codes | `resource_id`, `cefr_estimate`, `skill`, `node_codes`, `prompt_version`, `model`, `ai_request_id`. |

**Indexes of note**

- `idx_resources_user` on `(user_id, created_at DESC)` for listing
- `idx_resources_pending` on `created_at` where status is pending or uploaded, for the sweeper
- `uq_resources_object_key`
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `resource`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/me/resources/upload-intent` | `self` | Presigned upload into fluentra-uploads; creates the row at pending |
| `POST` | `/api/v1/me/resources` | `self` | Confirm an uploaded file or submit a URL; queues validation |
| `GET` | `/api/v1/me/resources` | `self` | The caller's resources, newest first, filterable by status and kind |
| `GET` | `/api/v1/me/resources/{id}` | `self` | One resource, with a presigned GET for a validated file |
| `DELETE` | `/api/v1/me/resources/{id}` | `self` | Delete the resource and its stored object |
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
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Presigned PUT and GET, stat, read and delete in fluentra-uploads |
| [`job`](../../platform/job/AGENT.md) | → depends on | The resource.validate River worker and the resource.sweep_pending cron |
| [`user`](../../modules/user/AGENT.md) | → depends on | Account erasure event user.deleted to purge user resources |
| [`studio`](../../modules/studio/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-RESOURCE-01** — **BR-RESOURCE-01**: A resource belongs to one user and is visible only to them. Another user's id answers 404, never 403.
2. **BR-RESOURCE-02** — **BR-RESOURCE-02**: The stored original is never modified or replaced. Derived forms go to fluentra-derived (P3).
3. **BR-RESOURCE-03** — **BR-RESOURCE-03**: A URL resource keeps its title and metadata, never the fetched body.
4. **BR-RESOURCE-04** — **BR-RESOURCE-04**: A file is classified by its bytes. A declared kind that disagrees with the detected kind is a rejection.
5. **BR-RESOURCE-05** — **BR-RESOURCE-05**: rejected always carries a reason, and the database enforces it. Reasons are fixed learner-facing sentences; the detail goes to the log.
6. **BR-RESOURCE-06** — **BR-RESOURCE-15**: Erasing an account deletes that user's resources, originals and renditions.
7. **BR-RESOURCE-07** — **BR-RESOURCE-06**: Deleting a resource deletes its object. If storage cannot be reached the delete fails and the row stays.
8. **BR-RESOURCE-08** — **BR-RESOURCE-07**: An intent never confirmed within 15 minutes is swept to failed and its object removed. The sweeper marks the row before deleting, so a row confirmed meanwhile is left alone.
9. **BR-RESOURCE-09** — **BR-RESOURCE-08**: Quotas bound each user to 50 resources and 250 MB, counting everything not rejected or failed.
10. **BR-RESOURCE-10** — **BR-RESOURCE-09**: rejected is a verdict on the resource; failed is ours and retryable. A timeout or a 5xx never rejects a link.
11. **BR-RESOURCE-11** — **BR-RESOURCE-10**: The URL fetcher checks the address it is connecting to, in the dialer, on every hop, and never uses a proxy. Checking DNS first and connecting later is not a check.
12. **BR-RESOURCE-12** — Extracted text and anything generated from it is private to the resource's owner.
13. **BR-RESOURCE-13** — Classification stores only spine codes that exist; the rest are dropped.
14. **BR-RESOURCE-14** — A transcript is queued by cmd/media in the transaction that settles the audio_web rendition, ready or skipped. Video gets an audio_web rendition for its soundtrack.
15. **BR-RESOURCE-15** — An extraction and its classification job are written in one transaction.
16. **BR-RESOURCE-16** — **BR-RESOURCE-16**: A resource leaves its owner's private space only by being copied into a published course, and only its owner can start that copy.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
_No recipes recorded yet. Add one the first time you do something twice._
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
_None recorded. Add one the moment you take a shortcut._
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

_None yet._

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `RESOURCE_NOT_FOUND` | 404 | Resource does not exist or belongs to another user |
| `RESOURCE_TYPE_NOT_SUPPORTED` | 422 | File MIME type or kind is not supported |
| `RESOURCE_URL_NOT_ALLOWED` | 422 | Submitted URL points to private/internal IP or invalid scheme |
| `QUOTA_EXCEEDED` | 422 | User storage quota (100MB) exceeded |
| `INVALID_PAYLOAD` | 400 | Malformed JSON request body |
| `STORAGE_UNAVAILABLE` | 503 | Underlying storage service unreachable |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/resource/...                    # unit
go test -tags=integration ./internal/modules/resource/...  # integration (testcontainers)
```

**Focus areas**

_None yet._
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not return 403 for another user's resource. Return 404.
- Do not store a fetched URL body in the database or in storage.
- Do not validate a URL by resolving its host and then connecting separately. Check in the dialer.
- Do not give the URL fetcher a proxy.
- Do not write err.Error() into failure_reason. Learners read it.
- Do not delete an object before the guarded status update that owns it has matched.
- Do not delete a file resource without deleting its object.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
