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

## What this does not cover

`fluentra-media` and `fluentra-exports` are read through presigned `GET` URLs,
which *are* simple requests when the browser merely navigates to them. If either
ever gains a browser-side upload or an `XHR` read, it needs a policy too.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
