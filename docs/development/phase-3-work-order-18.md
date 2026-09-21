---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-21
---

# Phase 3 — work order 18

> **This is Stage A of [work order 19](phase-3-work-order-19.md),** which builds the rest of phase 4 in one
> staged run. Build it exactly as written here, then continue with Stage B there.

**Purpose.** Turn a validated resource into something a learner can comfortably open: a thumbnail, a
screen-sized image that keeps its text readable, a first-page preview of a document, a web-playable audio
file, and a small set of video sizes. The original is never touched. This is step P3 of
[the phase 4 plan](phase-4-plan.md), and it includes one fix to P2 that cannot wait.

**Read first.** The phase 4 plan §§3 (D9), 8 and 10; [work order 17](phase-3-work-order-17.md);
`internal/modules/resource/AGENT.md`; [`ai-voice-render-workflow.md`](ai-voice-render-workflow.md); and
`.github/workflows/tts-render.yml`, which this work order copies the shape of.

**The one fact that decides this work order.** Production runs on Render's free tier. The repo's own
render-workflow document says that tier cannot render text-to-speech, and it sleeps after about fifteen
minutes idle. Transcoding video on that worker is not a plan. The codebase already solved this once:
`cmd/tts -all` is a batch command that finds what has not been rendered and renders it, and a GitHub
Actions workflow runs it hourly and on demand against production storage. **Renditions are produced the
same way.** The worker never runs ffmpeg.

---

## 1. What exists today

Checked on 2026-09-21 against `feat/phase-3-work-order-15` at `f5445ee`.

| Piece | State | Where |
|---|---|---|
| Resource intake | **Built (WO 17).** `resource.resources` with `pending` / `uploaded` / `validated` / `rejected` / `failed`; `detected_mime` set from the bytes; originals in `fluentra-uploads` | `internal/modules/resource/` |
| `fluentra-derived` bucket | **Created, unused.** A constant in `storage/buckets.go` and made by the dev stack; nothing writes to it | `internal/platform/storage/buckets.go` |
| Out-of-process media | **Built, for TTS only.** `cmd/tts -all` renders every clip missing from `content.tts_cache`; `tts-render.yml` runs it hourly and on `workflow_dispatch`, against production R2 with secrets from the `tts-production` environment | `cmd/tts/`, `.github/workflows/tts-render.yml` |
| Dispatch from the worker | **Built.** `media.GitHubWorkflowDispatcher.RequestRender(ctx)` triggers a workflow, so a new item does not wait for the hourly schedule | `internal/platform/media/dispatch.go` |
| ffmpeg, poppler, LibreOffice | **Nowhere.** No Go reference, no Dockerfile installs them. `platform/media/AGENT.md` promises "ffmpeg transcode, waveform" and none of it exists | — |
| Image code | `golang.org/x/image` is a dependency: decoding and `draw.CatmullRom` resampling are available. **There is no WebP encoder** in the standard library or `x/image`; JPEG and PNG encoding are stdlib | `go.mod` |
| Production storage | Cloudflare R2 (`S3_REGION=auto` in the workflow). R2 does not support S3 POST policy; the presign code already falls back to presigned PUT | `storage/presign.go` |
| Account erasure | **Anonymises** the user row (`UPDATE core.users`); it does not delete it. So `ON DELETE CASCADE` on `resource.resources` **never fires**: a deleted learner's resource rows **and** their files survive. `speaking` handles the same problem by subscribing to `user.deleted` | `user/service`, `speaking/module.go` `Subscribe` |
| Highest migration | `1700000790` | `db/migrations/resource/` |

### Correction to what the resource docs said

`resource/TODO.md` (written in the WO 17 review) says "the FK cascades the rows away and leaves the
files". That is wrong in the direction that matters: erasure is an anonymising `UPDATE`, the cascade never
runs, and **both** the rows and the objects outlive the account. Step 1 below fixes it, and this change
corrects the TODO text.

---

## 2. Scope

**In.**

1. **Erasure purge** — `resource` subscribes to `user.deleted` and removes that user's rows and objects.
2. One migration: `resource.renditions`.
3. `cmd/media -all`: a batch renderer that finds validated resources lacking renditions and produces them.
4. `.github/workflows/media-render.yml`, hourly plus `workflow_dispatch`, modelled on `tts-render.yml`.
5. The worker dispatches that workflow when a resource becomes `validated`.
6. `GET /api/v1/me/resources/{id}` returns the resource's ready renditions, each with a presigned GET.
7. `make media` for local runs.

**Rendition set, in the order to build it:**

| Source | Renditions | Tool |
|---|---|---|
| Image (PNG, JPEG, WebP) | `thumbnail` (long edge 320), `display` (long edge 2048, never upscaled) | Go: `x/image/draw` |
| PDF | `thumbnail` and `preview` of page 1 | `pdftoppm` (poppler-utils) |
| DOC, DOCX, PPT, PPTX | the same, via a PDF conversion | LibreOffice headless, then `pdftoppm` |
| Audio (WAV, MP3, M4A) | `audio_web`: AAC 128 kbit/s in M4A, only when the source is WAV or over 256 kbit/s | ffmpeg |
| Video (MP4, WebM) | `poster` (frame at 1 s), `video_360p`, `video_720p` (never upscaled) | ffmpeg |

**Out.**

- Transcripts. The brief lists them under media, but transcription is extraction — P4, work order 19 —
  and it uses the `HTTPTranscriber` that already exists.
- HLS or DASH. See D18-2.
- Any UI beyond the API field.
- Running any of this on the Render worker.

---

## 3. Decisions

| # | Decision | Why |
|---|---|---|
| D18-1 | Renditions are made by **`cmd/media -all` in GitHub Actions**, never by the worker | §1. The precedent is `cmd/tts`, it already works against production R2, and it keeps ffmpeg, poppler and LibreOffice off a machine that cannot afford them |
| D18-2 | Video ships as **progressive MP4 at two sizes**, not an adaptive HLS ladder | Originals are in a private bucket reached by presigned URLs. An HLS playlist names dozens of segment files, and each would need signing, or the playlist rewritten per request with signed segment URLs. Two MP4 sizes with HTTP range requests satisfy "multiple resolutions" and play everywhere. HLS is a later decision, taken when there is demand for long video |
| D18-3 | **Readable text beats small files.** A PNG stays PNG. The `display` rendition never goes below a 2048 px long edge, and a source smaller than that is copied, not resampled. JPEG is re-encoded at quality 85 | The brief: "without making normal learning content visibly blurry, unreadable". Screenshots and scanned worksheets are PNG-heavy and lossy compression smears their text |
| D18-4 | Renditions are rows in **`resource.renditions`**, one per `(resource_id, kind)` | A rendition has its own status and can fail on its own. A JSON column on `resources` cannot be claimed, retried or queried |
| D18-5 | Every rendition is **regenerable and disposable**. Deleting one never touches the original (BR-RESOURCE-02) | Plan D9 |
| D18-6 | **Video is the last step and the first cut.** Only video up to 10 minutes and 200 MB of source is rendered; anything larger keeps its original and a poster only | Plan §10 names P3 video as the first thing to cut. GitHub-hosted minutes are finite, and a 30-minute workflow timeout bounds one run |
| D18-7 | Every external tool runs on **untrusted input with hard limits**: a per-file timeout, output-size caps, and ffmpeg restricted to local files | See Trap 1. These are learner uploads, and all three tools have a history of parser bugs |

---

## 4. Step 1 — the erasure purge

Do this first; it is a privacy defect in shipped code.

- `resource/module.go` gains `Subscribe(bus)`, exactly as `speaking/module.go` does, handling
  `usercontract.EventDeleted`.
- `service.DeleteUserResources(ctx, userID)`: list the user's resources, delete every object in
  `fluentra-uploads` **and every rendition in `fluentra-derived`**, then delete the rows. A storage failure
  returns an error so the event is redelivered; deleting an object that is already gone is not a failure.
- Register it in `cmd/worker/main.go` next to `speakingModule.Subscribe(bus)`.
- `resource/AGENT.md` §4 gains the consumed event; the docgen entry's `events.consumes` lists it.

**Done when** an integration test erases a user with two file resources and finds no row and no object
under that user's key prefix in either bucket.

---

## 5. The migration

`db/migrations/resource/1700000800_create_resource_renditions.sql`

```sql
CREATE TABLE IF NOT EXISTS resource.renditions (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id   uuid        NOT NULL REFERENCES resource.resources (id) ON DELETE CASCADE,
    kind          text        NOT NULL,
    status        text        NOT NULL DEFAULT 'pending',
    object_key    text,
    mime_type     text        NOT NULL DEFAULT '',
    width         integer,
    height        integer,
    duration_ms   integer,
    byte_size     bigint,
    tool_version  text        NOT NULL DEFAULT '',  -- 'ffmpeg 6.1.1', so a bad encoder can be found later
    attempts      integer     NOT NULL DEFAULT 0,
    failure_reason text       NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_renditions_resource_kind UNIQUE (resource_id, kind),
    CONSTRAINT uq_renditions_object_key UNIQUE (object_key),
    CONSTRAINT ck_renditions_kind CHECK (kind IN (
        'thumbnail', 'display', 'preview', 'audio_web', 'poster', 'video_360p', 'video_720p')),
    CONSTRAINT ck_renditions_status CHECK (status IN ('pending', 'ready', 'failed', 'skipped')),
    CONSTRAINT ck_renditions_ready_has_object CHECK (status <> 'ready' OR object_key IS NOT NULL),
    CONSTRAINT ck_renditions_attempts CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS idx_renditions_pending
    ON resource.renditions (created_at) WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA resource TO fluentra_app;
```

**The `GRANT` is not optional.** WO 17's migration shipped without one and every test still passed,
because every test connects as the owner. `db/migrations/resource/schema_integration_test.go` already has
`TestResourceSchema_AppRoleCanUseIt`; extend it to cover `resource.renditions`.

`skipped` is a real outcome, not a failure: a 200×150 image needs no `display` rendition, a 128 kbit/s MP3
needs no `audio_web`, and a 40-minute video gets a poster and nothing else (D18-6). Recording the skip is
what stops the renderer from trying again every hour.

---

## 6. `cmd/media`

Shaped like `cmd/tts`. Read `cmd/tts/main.go` before writing a line of it.

```text
go run ./cmd/media -all [-limit 50] [-kinds image,pdf,office,audio,video] [-dry-run]
```

1. **Plan.** For each `validated` file resource, compute the renditions its `detected_mime` calls for,
   and insert any missing ones as `pending` (`ON CONFLICT (resource_id, kind) DO NOTHING`).
2. **Claim.** `SELECT ... WHERE status = 'pending' AND attempts < 3 ORDER BY created_at LIMIT n
   FOR UPDATE SKIP LOCKED`, incrementing `attempts` in the same statement. Two overlapping runs must not
   render the same rendition, and the workflow's `concurrency` group is a second guard, not the only one.
3. **Render.** Download the original to a temp directory, run the tool for its kind (§7), inspect the
   output (dimensions, duration, size), upload to `fluentra-derived`.
4. **Record.** `ready` with the object key and measurements, `skipped` with a reason, or `failed` with a
   reason once `attempts` reaches 3.
5. Temp files are removed on every path, including a panic.

Object keys: `renditions/<resource_id>/<kind>.<ext>`. Deterministic keys mean a re-run overwrites
instead of accumulating orphans.

The tools are found from flags with a `PATH` fallback (`-ffmpeg`, `-pdftoppm`, `-soffice`). A missing tool
**skips its kinds with a logged warning** rather than failing the run: a laptop without LibreOffice should
still render images.

---

## 7. The tools, with their limits

| Kind | Command, in outline | Limits |
|---|---|---|
| Image | Go: decode, `draw.CatmullRom.Scale`, encode PNG or JPEG q85 by source format | Refuse to decode beyond 50 megapixels: check `image.DecodeConfig` **before** `image.Decode`, or a small PNG that claims to be 60000×60000 allocates gigabytes |
| PDF | `pdftoppm -f 1 -l 1 -png -scale-to 1600 in.pdf out` | 60 s timeout |
| Office | `soffice --headless --convert-to pdf --outdir tmp in.docx`, then as PDF | 120 s timeout; a fresh `-env:UserInstallation=file:///tmp/lo-<id>` profile per run, or concurrent conversions collide |
| Audio | `ffmpeg -nostdin -protocol_whitelist file -i in -vn -c:a aac -b:a 128k out.m4a` | 120 s timeout; output capped with `-fs` |
| Poster | `ffmpeg ... -ss 1 -i in -frames:v 1 -vf scale='min(1280,iw)':-2 poster.jpg` | 30 s timeout |
| Video | `ffmpeg ... -vf scale=-2:'min(720,ih)' -c:v libx264 -preset veryfast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 20 min timeout per file, and only within D18-6's bounds |

`-movflags +faststart` moves the index to the front of the file. Without it a browser must download the
whole MP4 before it can play it, which defeats the point of a smaller rendition.

---

## 8. The workflow

`.github/workflows/media-render.yml`: copy `tts-render.yml` and change what differs.

- Triggers: `schedule` hourly (offset from TTS, e.g. `47 * * * *`), and `workflow_dispatch`.
- `concurrency: media-render`, `cancel-in-progress: false`.
- Environment `media-production` with the same `DB_DSN` and `S3_*` secrets the TTS environment holds.
- `apt-get install -y --no-install-recommends ffmpeg poppler-utils`; LibreOffice behind a cache or a
  separate job, because it is the slow install.
- `timeout-minutes: 45`, and `-limit` sized so one run fits inside it.
- Actions pinned by commit SHA, as `tts-render.yml` does.

The worker calls `RequestRender` when a resource reaches `validated`, through a **second** dispatcher
configured with the new workflow file. Read `dispatch.go` for how the TTS one is configured and do not add
a config key that is not in `.env.example` — add it there in the same change.

---

## 9. The API

Spec first, in `api/openapi/components/resource.yaml`: `Resource` gains an optional `renditions` array.

```json
"renditions": [
  { "kind": "thumbnail", "mime_type": "image/png", "width": 320, "height": 240,
    "url": "https://...presigned...", "expires_at": "2026-09-21T10:15:00Z" }
]
```

Only `ready` renditions are listed, only on `GET /me/resources/{id}`, and only for the owner. The list
endpoint does not sign URLs: signing fifty thumbnails for a page nobody scrolls is wasted work. If the list
needs thumbnails later, that is a separate decision with a cache.

No new endpoint, no new permission.

---

## 10. Business rules

1. **BR-RESOURCE-11** — A rendition is derived, regenerable and disposable. Nothing about a rendition is
   ever written into `fluentra-uploads`.
2. **BR-RESOURCE-12** — A rendition never upscales, and the `display` image never goes below a 2048 px long
   edge unless the source is smaller.
3. **BR-RESOURCE-13** — `skipped` is terminal and carries a reason. The renderer does not retry it.
4. **BR-RESOURCE-14** — After three failed attempts a rendition is `failed` and stays so until someone
   resets it. The resource itself stays `validated`: a missing thumbnail never makes a learner's upload
   unusable.
5. **BR-RESOURCE-15** — Erasing an account deletes that user's resources, originals and renditions.
6. **BR-RESOURCE-16** — External tools only ever read local files and run under a timeout.

---

## 11. Traps

1. **ffmpeg will fetch URLs if you let it.** A crafted HLS playlist or `concat` file uploaded as "video"
   can name `http://169.254.169.254/...` and ffmpeg will request it — the SSRF WO 17 closed in the
   fetcher, reopened in the renderer. `-protocol_whitelist file` on every invocation. The sniffing in P2
   narrows what arrives here; it does not make this unnecessary.
2. **Decompression bombs.** `DecodeConfig` before `Decode` for images; `-fs` and a timeout for ffmpeg; a
   timeout for pdftoppm, whose runtime on a hostile PDF is unbounded.
3. **The erasure purge must include renditions.** Step 1 lands before renditions exist, so it is easy to
   write it against `fluentra-uploads` alone and never revisit it.
4. **No GRANT.** §5.
5. **The TTS workflow's secrets are in an environment named `tts-production`.** A new workflow needs its
   own environment or an explicit decision to share that one; referencing it by copy-paste silently
   couples the two.
6. **LibreOffice profiles.** Two conversions sharing the default user profile lock each other. One
   profile directory per conversion.
7. **`FOR UPDATE SKIP LOCKED` needs a transaction around claim-and-increment**, or two runs claim the
   same row between the select and the update.
8. **R2 and `Content-Type`.** Set it explicitly on every upload to `fluentra-derived`; a presigned GET
   serves whatever was stored, and an MP4 served as `application/octet-stream` downloads instead of
   playing.
9. **`platform/media/AGENT.md` claims capabilities that do not exist.** Do not trust it for this work;
   correct it as part of step 9 of §12.

---

## 12. Steps

1. **Erasure purge** (§4), with its integration test. Commit it on its own.
2. **Migration** (§5), up and down twice through `cmd/migrate`, and the grants test extended.
3. **Repository and service** for renditions: plan, claim, record. Unit tests with a fake store.
4. **Images** in `cmd/media`, pure Go. This is the step with no external tool, so it is where the
   claim loop is proved.
5. **PDF**, then **Office**, then **audio**.
6. **Workflow** (§8) and the worker's dispatch. Run it once by hand with `-dry-run`.
7. **API** (§9): spec, `make gen`, handler, contract test.
8. **Video** (§7, within D18-6). If time or runner minutes run short, stop before this step; everything
   before it stands on its own.
9. **Docs**: the resource docgen entry, and `platform/media/AGENT.md` stripped of what it does not do.
10. **Verify**: `make check`, `make lint`, the integration suite, and the worker boots.

---

## 13. Testing

The gate:

```text
upload a 4000x3000 PNG screenshot -> validated
cmd/media -all
GET /me/resources/{id}
  -> renditions: thumbnail 320x240 PNG, display 2048x1536 PNG, both with URLs that download
  -> the original in fluentra-uploads is byte-identical to what was uploaded
```

Also required:

- **Unit.** The rendition plan per `detected_mime`, including every `skipped` case.
- **Unit.** A PNG whose header claims 60000×60000 is refused before it is decoded.
- **Unit.** Every ffmpeg argument list contains `-protocol_whitelist file` — assert it on the builder,
  so a later edit cannot drop it silently.
- **Integration.** Two concurrent claims over the same pending rows never return the same row.
- **Integration.** Erasure removes rows and objects in both buckets (§4).
- **Integration.** `resource.renditions` is usable by `fluentra_app`.
- **Manual, once.** The workflow run against staging, with the run's minutes recorded in this document.

---

## 14. Risks

| Risk | Mitigation |
|---|---|
| **GitHub Actions minutes** run out on a private repository | D18-6 bounds video, the hourly run exits early when nothing is pending, and `-limit` caps a run. Record real minute usage after the first week |
| **The first hourly run meets a backlog** of every resource validated before this shipped | `-limit` per run; the backlog drains over a few hours instead of one run timing out |
| **A hostile file crashes or hangs a tool** | D18-7 and Trap 2. A crash fails one rendition; the run moves on |
| **Office conversion is slow and heavy** | It is its own `-kinds` value and can be dropped from the workflow without touching anything else |
| **HLS is wanted later** | D18-2 records why not now; renditions are keyed by kind, so an `hls` kind is additive |

---

## 15. What this work order does not answer

- **Transcripts and text extraction** — P4, work order 19, the next work order.
- **Whether renditions count towards a learner's quota.** They do not, today: they are ours, regenerable,
  and bounded per resource. If storage cost says otherwise, count them.
- **A thumbnail in the resource list.** §9 explains why not yet.
