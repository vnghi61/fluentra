---
module: content
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: content
tables: [content_items, content_versions, media_assets, taxonomies, content_tags, content_reviews]
depends_on: [storage, search, audit, ai, media]
depended_on_by: [lesson, learning, vocabulary, grammar, reading, listening, speaking, writing, questionbank]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# content — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Publish



```mermaid
sequenceDiagram
    autonumber
    actor AD as Admin
    participant C as content
    participant DB as PostgreSQL
    participant OB as outbox
    participant ME as media
    participant SE as search
    participant CA as cache

    AD->>C: POST /admin/content/{id}/publish
    C->>C: require content.publish; state must be Approved
    C->>ME: are all referenced assets processed?
    alt not ready
        C-->>AD: 409 MEDIA_NOT_READY
    else ready
        C->>DB: BEGIN
        C->>DB: version.status = published; item.current_version_id = version
        C->>DB: INSERT outbox(content.published)
        C->>DB: COMMIT
        C-->>AD: 200
        OB->>CA: invalidate content keys
        OB->>ME: enqueue TTS for new text
        OB->>SE: reindex
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
## State machine

The authoring workflow. The transition that matters most is `Published → Draft`: creating a new version does **not** take the live version down.

```mermaid
stateDiagram-v2
    [*] --> Draft: create
    Draft --> InReview: submit
    InReview --> Draft: changes requested
    InReview --> Approved: approved by a different admin
    Approved --> Draft: withdrawn
    Approved --> Published: publish
    Published --> Draft: new version started (live version stays up)
    Published --> Archived: archive (blocked if in use)
    Archived --> Draft: clone into a new version

    note right of Published
        On publish, via the outbox:
        · invalidate content cache
        · enqueue TTS generation
        · reindex for search
        · emit content.published
    end note
```

<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
