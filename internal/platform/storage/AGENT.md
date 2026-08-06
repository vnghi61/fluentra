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

# storage — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/storage` |
| Schema | `none` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
A facade over MinIO (S3 API): bucket policy, presigned upload and download URLs, object verification, lifecycle rules, and orphan collection. Binary data never flows through the Go API — this module is how that rule is kept.
<!-- END GENERATED: overview -->


## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Bucket definitions, policies and lifecycle rules
- Presigned PUT (upload) and GET (download) with pinned content type, size and expiry
- Post-upload verification: existence, size, content type
- Object key conventions
- Copy, move and delete operations
- Orphan garbage collection
- Storage metrics per bucket

**This module does NOT own:**

- Processing file contents — that is `platform/media`
- Deciding who may access an object — the calling module authorises before requesting a URL
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/storage/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/storage/contract/` | You are calling this module from another module |
| `internal/platform/storage/service/` | You are changing behaviour |
| `db/migrations/storage/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/storage/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `storage.Store` | `PresignPut`, `PresignGet`, `Stat`, `Copy`, `Delete` |
| struct | `storage.UploadIntent` | `{URL, ObjectKey, ExpiresAt, MaxBytes, ContentType}` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `storage.object_uploaded` | publishes | `{bucket, key, size, content_type}` |
| `user.deleted` | consumes | Delete the user's objects |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
This module owns no tables.
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `storage`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/storage/stats` | `system.storage` | Object counts and sizes per bucket |
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
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`media`](../../platform/media/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`audit`](../../modules/audit/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-STORAGE-01** — Binary data never passes through the API process. Uploads and downloads use presigned URLs.
2. **BR-STORAGE-02** — A presigned PUT pins content type and maximum size and expires in 5 minutes.
3. **BR-STORAGE-03** — A presigned GET for private content expires in 15 minutes and is generated per request, never cached in a response the browser stores.
4. **BR-STORAGE-04** — After an upload the server verifies existence, size and sniffed content type before accepting the reference.
5. **BR-STORAGE-05** — Object keys are deterministic: `{bucket}/{owner_type}/{owner_id}/{yyyy}/{mm}/{asset_id}.{ext}` — so an orphan is identifiable and a re-run is idempotent.
6. **BR-STORAGE-06** — Public buckets contain only published content; user uploads are always private.
7. **BR-STORAGE-07** — Lifecycle rules enforce the retention table in ARCHITECTURE §8.5; retention is configuration, not a cron script.
8. **BR-STORAGE-08** — An object without a database reference for more than 24 hours is collected by the GC job.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a bucket

1. Define it in `deploy/minio/` with its policy and lifecycle rule.
2. Add the config key and the key-prefix convention.
3. Decide retention and add it to the retention table in ARCHITECTURE §8.5.
4. Include it in the GC job and in the storage metrics.
5. Document who may read it and how access is granted.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Single MinIO node in v1 — no erasure coding, so the offsite mirror is the durability story.
- There is no virus scanning in Phase 1; a ClamAV sidecar is planned if user uploads ever become publicly visible.
- Presigned URL expiry is sensitive to clock skew between the API host and MinIO.
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
| `UPLOAD_VERIFICATION_FAILED` | 422 | The uploaded object does not match the intent |
| `STORAGE_UNAVAILABLE` | 503 | Object store unreachable |

### Security considerations

- Buckets are private by default; making one public requires an explicit policy and a review.
- Presigned URLs are the only access path; the application never proxies object bytes.
- Access and secret keys are configuration only, scoped to the minimum bucket set.
- Export and report artefacts are single-use in practice: short expiry plus an unguessable key.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/platform/storage/...                    # unit
go test -tags=integration ./internal/platform/storage/...  # integration (testcontainers)
```

**Focus areas**

- Presigned PUT rejects an oversized or wrong-type upload
- Verification catches a missing or mismatched object
- Key generation is deterministic and idempotent
- GC deletes orphans and never deletes a referenced object
- Behaviour when MinIO is unavailable
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not stream file bytes through a Go handler.
- Do not make a bucket public without a review.
- Do not generate a long-lived presigned URL.
- Do not store a file reference before verifying the object exists.
- Do not put personal data in an object key.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
