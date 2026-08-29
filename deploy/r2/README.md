---
doc_type: folder_index
folder: deploy/r2
last_verified: 2026-08-28
---

# deploy/r2 — Cloudflare R2 bucket configuration

## Purpose

Avatars are uploaded **from the browser straight to object storage**; the bytes
never pass through the API. That is why `POST /api/v1/me/avatar/upload-intent`
hands back a presigned URL instead of accepting a file.

A browser upload is a cross-origin `PUT`, and a `PUT` is never a CORS "simple
request" — so every one of them is preceded by an `OPTIONS` preflight. A bucket
with no CORS policy refuses that preflight, and the upload never leaves the
browser. R2 says so precisely:

```console
$ curl -i -X OPTIONS "https://<account>.r2.cloudflarestorage.com/fluentra-avatars/probe" \
    -H "Origin: https://fluentra.io.vn" \
    -H "Access-Control-Request-Method: PUT" \
    -H "Access-Control-Request-Headers: content-type"

HTTP/1.1 403 Forbidden
<Error><Code>Unauthorized</Code><Message>CORS not configured for this bucket</Message></Error>
```

That is a bucket setting, not application configuration. No environment
variable and no code change makes an upload work without it.

## The other R2 setting: the region

`S3_REGION` must be one of R2's own names — `auto`, or `wnam`, `enam`, `weur`,
`eeur`, `apac`, `oc`. An AWS region name is refused.

It is worth calling out because of *when* it fails. The region appears only in
the presigned URL's credential scope, so a deployment with `ap-southeast-1`
starts cleanly, serves every request, and issues URLs that look perfectly
well-formed. R2 checks the signature and the CORS policy first and the region
last, so this is the error that surfaces only after everything else has been
fixed — as a cross-origin `400 InvalidRegionName` in a response the page is not
allowed to read.

`storage.ValidateRegion` now refuses that configuration at boot, in both the API
and the worker, so it cannot reach a learner again.

## Contents

- `cors.json` — the policy the avatar bucket needs

## Applying it

Cloudflare dashboard: **R2 → the bucket → Settings → CORS policy**, and paste
`cors.json`.

Or with the S3 API, using an R2 API token that may write bucket settings:

```bash
aws s3api put-bucket-cors \
  --endpoint-url "https://<account-id>.r2.cloudflarestorage.com" \
  --bucket fluentra-avatars \
  --cors-configuration file://deploy/r2/cors.json
```

Verify with the `curl` above: a configured bucket answers `200` and echoes
`Access-Control-Allow-Origin`.

## Why the policy is shaped this way

`AllowedHeaders` lists `content-type` and nothing else because that is the only
header the client sends. The presigned URL carries its credentials in the query
string — `X-Amz-Signature` and friends — so there is no `Authorization` header
to allow, and widening this list grants more than the upload needs.

`AllowedOrigins` is the site, not `*`. A presigned URL is a bearer credential
for one object for five minutes; there is no reason for any other origin to
spend one.

`ExposeHeaders: ETag` lets the client read the stored object's ETag. Nothing
depends on it today; it is the one header a resumable or verified upload would
want, and it costs nothing.

## The other buckets

CORS is a rule about **`fetch` and `XHR`**, not about the browser touching a URL.
Navigating to a link, loading `<img src>`, playing `new Audio(url)` — none of
those are cross-origin requests in the CORS sense, and none need a policy. What
needs one is script reading a cross-origin response.

Measured against how the code actually uses each bucket today:

| Bucket | Needs CORS | Why |
|---|---|---|
| `fluentra-avatars` | **Yes** | The browser `PUT`s the file itself, through `fetch`. A `PUT` is never a simple request, so it is always preflighted. |
| `fluentra-media` | No | Nothing reads it yet. When it does, `FlashcardFront` plays pronunciation with `new Audio(url)` — a media element load, not a fetch. |
| `fluentra-exports` | No | `ExportWorker` presigns a `GET` and **emails** the link. The learner clicks it and the browser navigates; no script reads the response. |

Displaying an avatar needs no policy either — `<img src>` is a plain image load,
and the upload modal previews from `URL.createObjectURL(file)`, which never
leaves the machine. Only the upload itself is cross-origin script traffic.

Two things would change this, and both are on the roadmap rather than in the
tree:

- **Waveform rendering.** Drawing an audio waveform means *fetching* the bytes,
  not playing them. A wavesurfer-style reader over `fluentra-media` needs a
  policy allowing `GET` from the site.
- **In-app export download.** If the archive stops arriving by email and becomes
  a button that fetches into a blob, `fluentra-exports` needs one too.

Neither is speculative future-proofing worth doing now: an unused CORS policy is
a grant nobody is using, and the `curl` above tells you the moment one is
missing.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
