---
doc_type: handoff
phase: 4
status: in_progress
last_verified: 2026-09-22
---

# Phase 3 — work order 20: documents and videos in a community course

> **Where this stands (2026-09-22).** Stages A–E are implemented: the `lesson_material`
> kind, Gate 1's `material_ready`, the publish-time copy into `fluentra-media`, read-time
> signed `sources`, the `lesson_material` grader, the Studio editor, the runner renderer,
> and the business rules/docs. Two things remain and are deliberate:
> the Stage D gate (watch a video on a phone over the LAN) needs a running stack, and
> Stage E's reviewer view is cut per §5 — reviewers open materials from the draft.
> The outline-icon item (§D.4) does not apply: the course outline lists lessons, not
> activities, so there is no exercise icon to replace.
>
> **Where the code differs from D20-2.** Copies land under
> `course-materials/{course_id}/{activity content slug}/`, not `{content_version_id}`:
> the version id only exists after `EnsurePublished`, which needs the copied keys first.
>
> **Review fixes (2026-09-22).** A video no longer gets a `document` source pointing at its
> raw upload; Gate 1 refuses a material that is audio or an image instead of reporting it
> as "still processing"; the runner plays one `src` so an expired URL actually triggers
> the refetch (errors on `<source>` children never reach `<video>`); a reopened draft
> resumes polling its material instead of staying at "processing" and blocking submit.

**Purpose.** Let a creator build a course where a chapter is something to *read* or *watch*, not only
exercises: "Chapter 1 — read this document, Chapter 2 — do these exercises, Chapter 3 — watch this
video". Today the Studio editor can only author graded exercises, and nothing in the learner's lesson
runner can show a document or play a video.

**Why this is its own work order.** Nothing planned so far builds it, on purpose:

- [Work order 17](phase-3-work-order-17.md) made an upload a **resource that is private to its owner**
  (BR-RESOURCE-01, BR-RESOURCE-12). Its §13 names this exact gap: *"Whether a resource can become a shared
  course … connecting the two is neither P2 nor P4."*
- [Work order 19](phase-3-work-order-19.md) §4 kept extracted text private too, and put video first on its
  list of things to cut.
- [Work order 15](phase-3-work-order-15.md) Studio accepts only the twelve graded kinds (BR-STUDIO-10),
  and Gate 1 rejects any activity body that holds a URL.

**Read first.** The `AGENT.md` of `studio`, `resource`, `lesson`, `learning` and `content`; WO 17 §§2, 13;
[WO 18](phase-3-work-order-18.md) §§2, 5 (renditions); `web/src/features/studio/model/activityKinds.ts`,
which is how the editor turns a form into the body Gate 1 checks.

---

## 1. What the code says today

Verified 2026-09-22 on `feat/phase-3-work-order-19`.

| Fact | Consequence | Where |
|---|---|---|
| An upload already becomes a validated resource, with **renditions**: `video_360p`, `video_720p`, `poster`, `audio_web` for video; `thumbnail`, `preview` for documents, rendered in GitHub Actions by `cmd/media` | No new transcoding. A course material **is a resource plus its renditions** | `resource/domain/rendition.go:5-33` |
| Resource MIME allow-list includes PDF, DOC/DOCX, PPT/PPTX, MP4, WebM; type is sniffed from the bytes | Upload validation is done; reuse it | `resource/domain/mime.go:30-51` |
| A resource is private to its owner, **deleted on account erasure** (`DeleteUserResources`), and counts toward a 250 MB per-user quota | A published course **must not point at the creator's resource**. Erasing the creator's account would delete a course learners paid for | `resource/service/service.go:455`, `resource/domain/resource.go:22-30` |
| Publishing turns each draft activity into `content.EnsurePublished` + `lesson.SyncActivities`, passing `kind`, `body`, `config` straight through | A material can ride the same path as one more activity kind | `studio/service/service.go:841-936` |
| `lesson` already asks `studio.AccessReader.MayOpen` before serving a lesson | The paywall for materials comes for free, if URLs are issued in `lesson` at read time | `lesson/module.go:40` |
| Gate 1 checks every activity with `learning.VerifyItem` and requires a non-empty body; `hasContactDetails` rejects any `https://` in a body | A material's body stores **object keys, never URLs** | `studio/domain/structure.go:100-269`, `learning/service/verifier.go:26` |
| Gate 1 requires 3–30 activities per lesson and 20 per course | A lesson that is "just a video" fails today. The rule must change | `studio/domain/structure.go:80-89` |
| `lesson` may depend on `content`, `studio` contracts and `cache` only | Presigning a URL in `lesson` is a **new architecture edge** to `platform/storage` | `.go-arch-lint.yml:466` |
| Android Chrome does not render a PDF inline; iOS Safari does | The runner cannot rely on `<iframe src=pdf>` | — |

---

## 2. Decisions

| Question | Default | Why | Owner may override |
|---|---|---|---|
| D20-1. What is a material? | A new activity kind **`lesson_material`**, with `material_kind` `document` or `video` | Ordering inside a lesson ("read, then answer") comes for free from `position`, and publishing, content versions (BR-STUDIO-08) and the runner already work per activity | — |
| D20-2. Who owns the published file? | **The course.** At publish, the resource's original and its renditions are **copied** to `fluentra-media` under `course-materials/{course_id}/{content_version_id}/`. The creator's resource stays private and can be deleted independently | Decouples a sold course from the creator's quota and account erasure (§1, row 3) | — |
| D20-3. Does a material count toward the 20-activity minimum? | **No.** The 20 counts graded exercises. A lesson is valid with **≥ 1 material, or ≥ 3 exercises** (or both) | "Chapter 3: watch a video" must be a valid lesson. Counting materials would let a course of 20 videos pass as a practice course | yes |
| D20-4. Can a learner who has not bought the course see materials? | **No — materials follow `MayOpen`, like exercises.** A creator may later mark one lesson as a free preview; not in this work order | One paywall function (BR-STUDIO-05); a preview flag is a pricing feature with its own review | yes |
| D20-5. File size | **50 MB**, the existing resource limit; video is served as `video_720p`/`video_360p` renditions, not the original | Keeps within storage budget on the free tier. 50 MB is about 10 minutes at 720p | yes, a per-role limit is one constant |
| D20-6. Rights | The creator ticks "I own or have the right to publish this material" at upload; Gate 2 (human review) sees every material | Materials become public; WO 19 §4.5's default only covers learner uploads | — |
| D20-7. How a document is shown | A PDF (or the PDF rendition of DOC/PPT) opens **in a new tab** from a card showing its `preview` rendition and page count. No in-page PDF viewer | Android cannot inline PDFs; a viewer library is a dependency for a problem a link solves | — |
| D20-8. What completes a material | Opening it. The learner taps "Mark as done"; a `lesson_material` grader awards full marks. Video progress is not tracked | Time-on-video is easy to fake and not worth a table in this run | — |

---

## 3. Order

```text
A  contract and schema first        spec, kind registration, resource and lesson contracts
B  authoring                        editor step, upload, draft shape, Gate 1 rules
C  publishing                       copy to course storage, content version, activity
D  delivery                         URLs at read time, runner renderer, completion
E  review and docs                  Gate 2 view, BRs, AGENT.md, final gate
```

Each stage ends at a gate; run it and stop on failure (the WO 19 §0 rules apply). One commit per stage,
subject `feat(<module>): wo20 stage X — …`.

## 4. Numbers reserved

| | Range |
|---|---|
| Migrations | `1700000900`–`1700000919` |
| Advisory lock IDs | `1_700_000_900`–`0919` |

---

## Stage A — contract and schema

**Spec first** (CLAUDE.md rule 2).

1. `api/openapi/components/studio.yaml`: document the draft activity shape the editor already sends,
   plus the material variant:
   `{kind: "lesson_material", material: {resource_id, material_kind, title, description?}}`.
2. `api/openapi/components/lesson.yaml` (lesson read): a `lesson_material` activity's `config` gains
   `sources` — `{poster_url?, video: [{url, height}], document: {url, preview_url?, page_count?}}` —
   issued at read time, expiring.
3. **`resource` contract** — new, narrow, read-and-copy only:
   - `MaterialForOwner(ctx, ownerID, resourceID) (Material, error)`: MIME, status, and rendition keys
     with their status. Refuses a resource the caller does not own (the same 404 as today).
   - `CopyForPublication(ctx, resourceID, destPrefix) (PublishedObjects, error)`: server-side copy of the
     original and `ready` renditions into `fluentra-media`. Idempotent on `destPrefix`.
4. `content`: register content kind `lesson_material`, body
   `{material_kind, title, description?, objects: {original, poster?, video_360p?, video_720p?, preview?,
   pdf?}, page_count?, duration_seconds?}` — **object keys only**.
5. `.go-arch-lint.yml`: `m_studio_service` may depend on `c_resource`; `m_lesson_service` may depend on
   `p_storage`. Update `MODULE_INDEX.md` §3 to match.
6. Migration `1700000900`: nothing new in `studio` (the draft is jsonb). If `content.content_kinds` or an
   equivalent registry table exists, add the kind there, with its `GRANT`s.

**Gate.** WO 19 §2 standing checks; Spectral; arch-lint run in the Linux container (memory: deepScan
differs on Windows).

---

## Stage B — authoring

1. **Editor** (`web/src/features/studio`):
   - New kind `lesson_material` in `activityKinds.ts`, with a form: material type (document / video),
     title, file picker, the rights checkbox (D20-6), and upload status.
   - Upload uses the existing `POST /me/resources/upload-intent` → PUT → `POST /me/resources` flow.
     Write the missing frontend helper once in `features/resource/api` rather than inside Studio.
   - Show rendition status ("Đang xử lý video…") by polling `GET /me/resources/{id}` until renditions are
     `ready` or `failed`. A draft may be **saved** while processing; it may not be **submitted**.
   - Label the kind in `vi.json`/`en.json` (`studio.activity.kind.lesson_material`: "Tài liệu / Video").
2. **Draft shape**: the editor saves `{kind: "lesson_material", weight: 0, material: {...}, body: {...}}`.
   `body` holds the resource id and title only, so `learning.VerifyItem`'s non-empty check passes and no
   URL is ever in it.
3. **Gate 1** (`studio/domain/structure.go`):
   - Add `lesson_material` to `AllowedActivityKinds`.
   - Per lesson: valid if **≥ 1 material, or 3–30 exercises**. Per course: **≥ 20 exercises**, materials
     excluded (D20-3). Report messages name which rule failed.
   - New check `material_ready` in the service: for each material, `MaterialForOwner` must return the
     draft owner's resource, status `validated`, and the renditions D20-7 needs (`video_360p` for video;
     `pdf` or original PDF plus `preview` for a document) in `ready`.
   - `learning.VerifyItem`: a `lesson_material` case that checks structure only — no answer key, no blind
     solve.

**Traps.**

1. **Someone else's resource id.** A creator pasting another user's resource id into the jsonb must fail
   Gate 1, not publish that user's private file. `MaterialForOwner` takes the *draft owner*, never the
   caller of the review endpoint.
2. The URL safety check must still run on `title` and `description` of a material.
3. Draft jsonb is creator-controlled; never trust `material_kind` — derive it from the resource's MIME.

**Gate.** A draft with a video lesson, a document lesson and 20 exercises passes Gate 1; the same draft
with the video still processing fails `material_ready`; a draft pointing at another user's resource fails.

---

## Stage C — publishing

1. In `publishDraft`, for a `lesson_material` activity: `CopyForPublication(resourceID,
   "course-materials/{course_id}/{slug}-u{n}-l{n}-a{n}/")`, then `EnsurePublished` with the body of §A.4,
   then the activity with `Weight: 0` and `Config` = the body.
2. The copy happens **before** the listing is made public, inside the existing "built unlisted, made
   public at the end" sequence. A failed copy fails the publish; nothing half-published is reachable.
3. Re-publishing an approved revision copies again under the new content version's prefix; old prefixes
   stay until no published version references them (a sweeper is out of scope — record it in `TODO.md`).

**Traps.**

1. `fluentra-media` must be in `DefaultBuckets()` in production, or the copy succeeds in dev only
   (WO 17 §12 made this mistake once).
2. Copy with the storage client's server-side copy, not download-and-upload through the free-tier worker.

**Gate.** Approving the Stage B draft creates objects under `course-materials/`; deleting the creator's
resource afterwards leaves the course intact; `user.deleted` for the creator leaves the course intact.

---

## Stage D — delivery

1. **`lesson` read path**: after `MayOpen`, for each `lesson_material` activity, presign GET URLs (short
   TTL, e.g. 1 hour) for the keys in its config and put them in `config.sources`. Redaction runs before
   this and must not strip `sources`.
2. **`learning`**: register a `lesson_material` grader: any submission `{"done": true}` scores full
   marks (D20-8). Weight 0 keeps it out of the lesson score.
3. **Runner** (`web/src/features/learning/components/Runner/ExerciseMaterial.tsx`, wired in
   `routes/LessonPage.tsx`):
   - Video: `<video controls playsInline preload="metadata" poster>` with 720p and 360p `<source>`s, and
     URLs passed through `reachableStorageUrl` so a phone on the LAN can play them in dev.
   - Document: preview image, title, page count, an "Mở tài liệu" button (new tab), then "Đã xem xong".
   - 44 px touch targets; the fixed bottom nav must not cover the button (the runner already hides it).
4. A material shows in the course outline with a document or video icon instead of the exercise icon.

**Traps.**

1. **Expired URLs.** A learner who leaves the tab open past the TTL gets a dead video; on a `<video>`
   `error`, refetch the lesson once.
2. `playsInline` is required on iOS or the video jumps to full screen.
3. A learner without access must get the lesson's existing 403/paywall, never a URL.

**Gate.** On a phone over the LAN: open the published course, watch the video, open the document, mark
both done, finish the exercises; lesson completes. A signed-in non-buyer of a paid course sees the paywall
and no URL appears in any response.

---

## Stage E — review and docs

1. Gate 2 review screen (admin/moderator) renders materials with the same component, so a reviewer
   watches what learners will watch (D20-6).
2. Business rules, through `tools/docgen/data/` then `make docs`:
   - **BR-STUDIO-** (next) — A lesson holds at least one material or three exercises; a course holds at
     least twenty exercises, materials not counted.
   - **BR-STUDIO-** (next) — A published material is a copy owned by the course; the creator's resource
     and account can be deleted without affecting it.
   - **BR-RESOURCE-** (next) — A resource leaves its owner's private space only by being copied into a
     published course, and only the owner can do that.
   - **BR-LESSON-** (next) — Material URLs are issued at read time, after the paywall, and expire.
3. Update BR-STUDIO-10's wording ("twelve runner kinds plus `lesson_material`") and each touched module's
   `AGENT.md` and `TODO.md`.

**Final gate.** WO 19 §2 in full on the final commit, including arch-lint uninterrupted, `make
gen-check && make gen-check-web` after committing, and the worker booting for 30 seconds.

---

## 5. What to cut, in order, if the run must end early

1. Stage E's reviewer view — reviewers open materials from the draft instead.
2. Documents — ship video only (it is the case the user asked about last).
3. The outline icons.

**Never cut:** the ownership check in Stage B trap 1, the copy in Stage C (pointing a course at a private,
erasable resource), and issuing URLs only after `MayOpen`.

## 6. Out of scope

- Free-preview lessons for non-buyers (D20-4).
- Video progress tracking, resume position, subtitles from the Stage B transcript.
- Materials larger than 50 MB, streaming (HLS), DRM.
- A sweeper for copies no published version references.
