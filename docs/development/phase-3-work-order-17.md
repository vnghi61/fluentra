---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-21
---

# Phase 3 — work order 17

**Purpose.** Let a learner bring their own material in. A file they upload or a URL they submit becomes a
**resource**: stored, validated, owned, and visible to them with a status they can watch. This is step P2
of [the phase 4 plan](phase-4-plan.md).

**This work order stops at `validated`.** Turning a resource into learning material is three more steps:
media renditions are P3, extraction and transcription and classification are P4, generating exercises from
what was extracted is P6. WO 17 builds the intake — the part everything else queues behind — and it builds
it so that the later steps attach to a row that already exists rather than inventing their own.

The [plan's ordering table](phase-4-plan.md#9-ordering) originally gave P2 the gate "a PDF upload reaches
`extracted`". That was wrong: `extracted` is a P4 state. The gate is below in §11 and it is `validated`.

**Read first.** [ADR-0018](../adr/ADR-0018-media-presigned-upload.md), the phase 4 plan §§3 (D8, D9), 5
and 8, and the `AGENT.md` of `vocabulary`, `speaking`, `platform/storage` and `platform/job`.

**Do not start by designing an upload.** `vocabulary` and `speaking` have both already shipped one, and
§2 is mostly a description of copying what they did.

---

## 1. What exists today

Checked on 2026-09-21 against `feat/phase-3-work-order-15` at `30853ec`.

| Piece | State | Where |
|---|---|---|
| Presigned upload | **Built.** `MinIOStore.PresignPut(ctx, bucket, key, contentType, maxBytes, expiry)` builds an S3 POST policy that pins bucket, key, content type and a size range with a **one-byte floor**; falls back to a presigned PUT where POST policy is unsupported (R2) | `internal/platform/storage/presign.go:22` |
| Upload-intent endpoint precedent | **Built.** `POST /speaking/upload-intent`, `x-permission: self`, returns `upload_url`, `object_key`, `expires_at` and the caller's remaining daily quota | `openapi.yaml`, `speaking` module |
| Object key builder | **Built.** `storage.BuildKey(ownerType, ownerID, at, assetID, ext)` | `internal/platform/storage/keys.go:26` |
| Server-side copy | **Built.** `MinIOStore.Copy(ctx, srcBucket, srcKey, destBucket, destKey)` — what P3 will use to derive renditions without a round trip | `storage/store.go:203` |
| Buckets | **Three built**: `fluentra-avatars`, `fluentra-media`, `fluentra-exports`, as Go constants with `DefaultBuckets()` | `internal/platform/storage/buckets.go` |
| Async intake precedent | **Built and the closest model for this work.** `skill.vocab_uploads` (submission) + `vocab_upload_items` (unit of work), split because "one table would mean a single failed word marking the whole paste as failed". Status `pending`/`processing`/`completed`/`failed`, a partial index on pending for the claim query, and bounds on the raw text | `db/migrations/vocabulary/1700000270` |
| Job queue | **Built.** River args + worker per job (`vocabulary.verify_upload`), `InsertOpts` for queue and retry policy | `internal/modules/vocabulary/job/verify.go` |
| Transcription | **Built.** `media.HTTPTranscriber`, OpenAI-compatible, config `SPEECH_ASR_*` | `internal/platform/media/transcriber.go` |
| `resource` module | **Does not exist.** No directory, no `AGENT.md`, no entry in `MODULE_INDEX.md` | — |
| Highest migration | `1700000781` | `db/migrations/content/` |

### Two findings worth acting on

**The buckets this work order needs are already named, and nothing creates them.** `.env.example` declares
five `S3_BUCKET_*` keys:

```text
S3_BUCKET_AVATARS=fluentra-avatars      -> constant exists, created
S3_BUCKET_MEDIA=fluentra-media          -> constant exists, created
S3_BUCKET_EXPORTS=fluentra-exports      -> constant exists, created
S3_BUCKET_UPLOADS=fluentra-uploads      -> read by nothing, created by nothing
S3_BUCKET_DERIVED=fluentra-derived      -> read by nothing, created by nothing
```

So the names are decided and must not be re-invented: originals go to **`fluentra-uploads`**, and P3's
renditions to **`fluentra-derived`**. Both need adding to `buckets.go` and `DefaultBuckets()`, or the first
presign will hand a learner a URL to a bucket that does not exist and the failure will surface as a 403
from MinIO with nothing in our logs.

**Three upload-related config keys are documented and read by nobody**: `S3_BUCKET_UPLOADS`,
`S3_BUCKET_DERIVED`, `UPLOAD_MAX_MB` and `S3_PRESIGN_PUT_TTL` have zero references in `internal/` or
`cmd/`. Size ceilings in this codebase are module constants — `user/domain/avatar.go` has
`AvatarMaxBytes int64 = 5 * 1024 * 1024`. **Follow the constant convention**; do not wire `UPLOAD_MAX_MB`
as part of this work order. Reconciling `.env.example` against what is actually read is a documentation
cleanup of its own and it touches more than this module.

---

## 2. Scope

**In.**

1. A new module `resource`, schema `resource`, wired in `cmd/api` and `MODULE_INDEX.md`.
2. One migration: `resource.resources`, plus the two bucket constants.
3. Five endpoints, specified in `openapi.yaml` **before** any handler.
4. File intake: upload-intent → client PUTs to storage → register → validate.
5. URL intake: submit a URL → validate → store as a **reference** (D8), never a copy.
6. A sweeper job for intents that were never completed.
7. `resource/AGENT.md`, `API.md`, `DECISIONS.md`, `TODO.md`, docgen data, `MODULE_INDEX.md`.

**Out.**

- Extraction, transcription, classification — P4 / WO 19.
- Thumbnails, transcodes, any derived object — P3 / WO 18. `fluentra-derived` is created here and written
  to there.
- Generating exercises from a resource — P6.
- Any web UI.
- Sharing a resource with another learner. Resources are private to their owner in this work order, and
  making them shareable later is a visibility column, not a redesign.

---

## 3. Decisions

| # | Decision | Why |
|---|---|---|
| D17-1 | **One table, `resource.resources`**, not the two-table split `vocab_uploads` uses | That split exists because one paste holds many independently-verifiable words. One upload is one file: there is no per-item work until P4 finds some, and P4 can add its own child table then |
| D17-2 | A URL resource stores `source_url`, title and metadata, and **no fetched body** | Plan D8. It is also the difference between a study tool and a piracy tool, and it is much easier to hold now than to retrofit |
| D17-3 | The **original object is never modified or replaced** | Plan D9. P3 writes renditions to `fluentra-derived`; `fluentra-uploads` is append-only in practice |
| D17-4 | The **declared** content type is treated as a claim, not a fact: the validator sniffs the stored bytes | `content_type` comes from the client. An executable uploaded as `application/pdf` is the ordinary case this must refuse, not an exotic one |
| D17-5 | Size ceilings and the MIME allow-list are **Go constants in `resource/domain`** | Matches `AvatarMaxBytes`; see §1 on the unread config keys |
| D17-6 | Deleting a resource deletes its stored object in the same operation | Otherwise storage grows without bound and a learner who deleted something still has it in a bucket |
| D17-7 | Resources are **private to their owner**, enforced in the service, not only by a `WHERE user_id` in one query | The endpoints are `/me/...`, and an ownership check that lives in a single SQL clause is one refactor away from being dropped |

---

## 4. The migration

`db/migrations/resource/1700000790_create_resource_tables.sql`

```sql
CREATE SCHEMA IF NOT EXISTS resource;

CREATE TABLE IF NOT EXISTS resource.resources (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL,
    kind              text        NOT NULL,   -- 'file' | 'url'
    title             text        NOT NULL DEFAULT '',

    -- File resources. object_key is in fluentra-uploads and is never rewritten.
    object_key        text,
    original_filename text        NOT NULL DEFAULT '',
    declared_mime     text        NOT NULL DEFAULT '',  -- what the client said
    detected_mime     text        NOT NULL DEFAULT '',  -- what the bytes say
    byte_size         bigint,
    checksum          text,

    -- URL resources. Referenced, never copied (D17-2).
    source_url        text,

    status            text        NOT NULL DEFAULT 'pending',
    failure_reason    text        NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    validated_at      timestamptz,

    CONSTRAINT fk_resources_user FOREIGN KEY (user_id) REFERENCES core.users (id) ON DELETE CASCADE,
    CONSTRAINT ck_resources_kind CHECK (kind IN ('file', 'url')),
    CONSTRAINT ck_resources_status
        CHECK (status IN ('pending', 'uploaded', 'validated', 'rejected', 'failed')),
    -- A file resource has an object; a URL resource has a URL. Neither has both,
    -- and a row with neither is a row nothing downstream can act on.
    CONSTRAINT ck_resources_shape CHECK (
        (kind = 'file' AND object_key IS NOT NULL AND source_url IS NULL) OR
        (kind = 'url'  AND source_url IS NOT NULL AND object_key IS NULL)
    ),
    CONSTRAINT ck_resources_byte_size CHECK (byte_size IS NULL OR byte_size >= 0),
    CONSTRAINT ck_resources_rejected_has_reason CHECK (
        status <> 'rejected' OR length(btrim(failure_reason)) > 0
    ),
    CONSTRAINT uq_resources_object_key UNIQUE (object_key)
);

CREATE INDEX IF NOT EXISTS idx_resources_user ON resource.resources (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_resources_pending
    ON resource.resources (created_at) WHERE status IN ('pending', 'uploaded');
```

`ck_resources_shape` is the constraint to keep. Without it the table accepts a `file` row with no object
and a `url` row with no URL, and every consumer from P3 onward has to defend against both.

The bucket constants, in `internal/platform/storage/buckets.go`:

```go
BucketUploads = "fluentra-uploads"   // originals, as the learner sent them
BucketDerived = "fluentra-derived"   // renditions, regenerable, written from P3
```

Both go into `DefaultBuckets()` so the dev stack and CI create them.

---

## 5. The lifecycle

```text
                 POST /me/resources/upload-intent
                            |
                    (row: pending)
                            |
             client PUTs the bytes to fluentra-uploads
                            |
                 POST /me/resources  {resource_id}
                            |
                    (row: uploaded)  --> validation job
                            |
              +-------------+--------------+
              |                            |
        (row: validated)            (row: rejected)
        ready for P3/P4            reason shown to owner
```

A URL resource skips the first two states: `POST /me/resources {url}` creates the row at `uploaded` and
queues the same validation.

`failed` is separate from `rejected` on purpose. **`rejected` is about the resource** — wrong type, too
big, not what it claimed to be — and the reason is written for the learner to read. **`failed` is about
us** — storage was unreachable, the job died — and it is retryable. Collapsing them tells a learner their
file was bad when our sweeper was simply behind.

---

## 6. Validation

The "validate" step of the brief's §3, and the only place this work order does real work.

**For a file:**

1. The object exists in `fluentra-uploads` at the key we issued, and is owned by the caller's row.
2. `byte_size` is within `MaxResourceBytes` and above zero.
3. **Sniff the leading bytes** and set `detected_mime`. If it disagrees with `declared_mime` in kind
   (a PDF claiming to be a PNG), reject. `net/http.DetectContentType` covers the common cases;
   `golang.org/x/image` is already a dependency for image decoding.
4. `detected_mime` is in the allow-list: PDF, DOC/DOCX, PPT/PPTX, PNG/JPEG/WebP, MP3/WAV/M4A, MP4/WebM.
   Anything else is rejected by name so the learner knows what happened.
5. Record `checksum`.

**For a URL:**

1. Scheme is `http` or `https`. Nothing else — `file://`, `gopher://` and friends are rejected outright.
2. **The resolved host is a public address.** Resolve it and refuse loopback, private, link-local,
   unique-local and unspecified ranges. `netip.Addr.IsPrivate`, `IsLoopback` and `IsLinkLocalUnicast`
   are what `payment` already uses for its SePay IP allow-list — read `ipAllowed` there.
3. Fetch **metadata only**, with a timeout and a response size cap: status, final URL, title,
   `og:` tags. Per D17-2 the body is not stored.
4. Redirects are followed at most a few hops, and **every hop is re-checked against step 2**. A public
   host that redirects to `127.0.0.1` is the standard way around a one-time check.

---

## 7. The API

**Write `openapi.yaml` first**, tag every operation `resource`, and add the module to the drift check's
map. Spectral needs `x-permission`, a description and a response example on each.

| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/me/resources/upload-intent` | `self` | Presigned PUT into `fluentra-uploads`; creates the row at `pending` and returns its id with `upload_url`, `object_key`, `expires_at` |
| `POST` | `/api/v1/me/resources` | `self` | Confirm an uploaded file (`{resource_id}`) or submit a URL (`{url, title?}`). **202**, queues validation |
| `GET` | `/api/v1/me/resources` | `self` | The caller's resources, newest first, filterable by `status` and `kind`. Paginated |
| `GET` | `/api/v1/me/resources/{id}` | `self` | One resource, including `failure_reason` when rejected, and a presigned GET for a file the caller owns |
| `DELETE` | `/api/v1/me/resources/{id}` | `self` | Delete the row and its stored object (D17-6) |

Error codes owned by `resource`:

| Code | Status | Meaning |
|---|---|---|
| `RESOURCE_NOT_FOUND` | 404 | No such resource **for this caller** — the same answer someone else's id gets |
| `RESOURCE_QUOTA_EXCEEDED` | 429 | The caller is at their resource count or byte ceiling |
| `RESOURCE_TYPE_NOT_SUPPORTED` | 422 | `detected_mime` is not in the allow-list |
| `RESOURCE_URL_NOT_ALLOWED` | 422 | Bad scheme, or a host that resolves to a private address |
| `RESOURCE_NOT_UPLOADED` | 409 | Confirmation arrived for a key with no object behind it |

A resource belonging to someone else answers **404, not 403**. 403 confirms the id exists.

---

## 8. Steps

1. **Module skeleton** — `internal/modules/resource/{contract,domain,service,repository,transport/http,job}`,
   `module.go` with `New(deps)`, `doc.go` in every package, and the `AGENT.md` front matter the drift
   check validates (`schema: resource`, `tables: [resources]`).
2. **Migration** (§4) plus the two bucket constants. Up and down twice against a real database.
3. **OpenAPI** (§7), then `make gen` and `make gen-check`.
4. **Domain** — the status machine with its legal transitions, the MIME allow-list, the size ceiling, URL
   validation. Pure Go, no I/O: the sniffing takes a `[]byte`, the URL check takes a resolved address.
5. **Repository** — `db/queries/resource/resources.sql`, sqlc. Match physical column order in projections
   so sqlc reuses the table row type.
6. **Service** — intent, confirm, submit-URL, list, get, delete, validate. Ownership checked here (D17-7).
7. **Transport** — handlers, DTOs, routes under the authenticated router. All five are `self`.
8. **Job** — `resource.validate` River worker, plus a cron sweeper that marks rows `failed` when they have
   sat at `pending` past the presign TTL. Register both in `cmd/worker`; **verify the worker boots**.
9. **Docs** — docgen data, `make docs`, `MODULE_INDEX.md` §3, `.go-arch-lint.yml` edges.
10. **Verify** — `make check`, then `make lint`, then the integration suite.

---

## 9. Business rules

1. **BR-RESOURCE-01** — A resource belongs to exactly one user and is visible only to them. Another user's
   id is a 404.
2. **BR-RESOURCE-02** — The stored original is never modified or replaced. Every derived form is a new
   object in `fluentra-derived`.
3. **BR-RESOURCE-03** — A URL resource stores a reference and metadata, never the fetched body.
4. **BR-RESOURCE-04** — A file is classified by its bytes, not by what the client called it. Disagreement
   between declared and detected type is a rejection.
5. **BR-RESOURCE-05** — `rejected` always carries a reason written for the learner; the database enforces
   it.
6. **BR-RESOURCE-06** — Deleting a resource deletes its object. A delete that cannot reach storage fails
   and leaves the row, rather than orphaning the object silently.
7. **BR-RESOURCE-07** — An intent that is never confirmed is swept to `failed` after the presign TTL, and
   its object, if any, is removed.
8. **BR-RESOURCE-08** — Per-user quotas bound both the number of resources and their total bytes.

---

## 10. Traps

1. **The uploads bucket does not exist yet.** §1. Add both constants to `DefaultBuckets()` in the same
   change that first presigns into one, and check the dev stack actually creates them.
2. **SSRF on URL import.** §6.2 and §6.4. The redirect re-check is the half that gets forgotten, and it is
   the half that gets used.
3. **Trusting `content_type`.** It is a string the client chose. D17-4.
4. **The orphan pair.** An intent with no upload leaves a row; an upload with no confirmation leaves an
   object. BR-RESOURCE-07 covers the first. For the second, the sweeper must delete by key, not only mark
   the row.
5. **404 vs 403** on another user's resource. §7.
6. **Do not reuse `content.media_assets`.** It belongs to `content` (rule DB1), and a second writer is
   exactly the kind of cross-module table use `go-arch-lint` cannot see because it is SQL.
7. **`x-permission: self` still needs an ownership check.** `self` means "a signed-in caller acting on
   themselves"; it does not compare the id in the path to the actor. `speaking` shows the pattern.
8. **The drift check maps paths to modules by tag.** Tag these `resource` and add the module's `API.md`,
   or `check-drift.mjs` fails looking for a file that was never created.
9. **`make tts` is not `make seed`.** Unrelated, but the same shape of mistake: creating a row and
   rendering its media are different steps, and P3 is where the second one lives.

---

## 11. Testing

The gate for this work order:

```text
POST /me/resources/upload-intent  {filename: "grammar.pdf", content_type: "application/pdf"}
  -> 200, row at pending, upload_url into fluentra-uploads
PUT  <upload_url>                 <the bytes>
POST /me/resources                {resource_id}
  -> 202, row at uploaded
(validation job runs)
GET  /me/resources/{id}
  -> 200, status "validated", detected_mime "application/pdf"
```

Also required:

- **Unit.** The status machine: every illegal transition refused. The MIME allow-list, including a file
  whose declared type disagrees with its bytes. URL validation: each rejected scheme, and loopback,
  private, link-local and unspecified addresses.
- **Unit.** A redirect chain whose final hop is private is refused (§6.4).
- **Integration.** Migration up, down, up. `ck_resources_shape` refuses a `file` row with no object and a
  `url` row with no URL. `ck_resources_rejected_has_reason` refuses a reasoned-less rejection.
- **Integration.** Another user's resource id returns 404 from `GET` and from `DELETE`.
- **Integration.** `DELETE` removes the object from storage, not just the row.
- **Integration.** The sweeper marks an expired `pending` row `failed` and leaves a confirmed row alone.
- **Contract.** All five endpoints against the generated types.

---

## 12. Risks

| Risk | Mitigation |
|---|---|
| **Storage cost** grows with every upload and nothing reclaims it | BR-RESOURCE-07 and the quotas in BR-RESOURCE-08 ship in this work order, not after. A retention policy for resources nobody opened is a later decision, and it needs `created_at` plus a last-opened column that P4 can add |
| **The copyright line moves** once P4 starts extracting from uploaded PDFs | D17-2 holds the line for URLs. For uploaded files the plan's §4.5 is still open and this work order does not settle it — it only stores what the learner sent, which is the same posture as an email attachment |
| **A malicious upload** reaches an extractor in P4 | Out of scope here, deliberately. The allow-list and the sniff narrow what P4 will ever be handed, which is the part that belongs in intake |
| **The two new buckets** are created in dev and forgotten in production | `DefaultBuckets()` is what the deployment provisioning reads; adding them there is the whole fix, and the integration suite fails without them |

---

## 13. What this work order does not answer

- **Where extraction runs**, and whether document text extraction is a Go library or an external service.
  That is P4's first decision and it is the expensive one.
- **Whether a resource can become a shared course.** Today a resource is private. The studio path from
  work order 15 is how something becomes public, and connecting the two is neither P2 nor P4.
- **Retention.** How long an untouched resource is kept, and whether a learner's deletion of their account
  takes their resources with it — the FK says yes, but the objects in storage need a sweeper that this
  work order does not write.
