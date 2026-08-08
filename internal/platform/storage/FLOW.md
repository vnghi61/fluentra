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

# storage — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Presigned upload with verification

```mermaid
sequenceDiagram
    autonumber
    actor B as Browser
    participant A as API (calling module)
    participant ST as storage
    participant M as MinIO

    B->>A: POST …/upload-intent { content_type, size }
    A->>A: authorise + quota check
    A->>ST: PresignPut(bucket, key, 5m, max_bytes, content_type)
    ST->>M: sign
    ST-->>A: UploadIntent
    A-->>B: { upload_url, asset_id }
    B->>M: PUT bytes directly
    B->>A: POST …/confirm { asset_id }
    A->>ST: Stat(key)
    alt missing, wrong size, or wrong type
        A-->>B: 422 UPLOAD_VERIFICATION_FAILED
    else ok
        A->>A: mark asset uploaded; enqueue processing
        A-->>B: 202
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
