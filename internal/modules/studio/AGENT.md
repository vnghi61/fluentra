---
module: studio
tier: commerce
group: modules
status: IN_PROGRESS
phase: 3
owner: "@commerce-team"
schema: studio
tables: [creator_profiles, payout_accounts, course_drafts, submissions]
depends_on: [content, lesson, learning, job]
depended_on_by: [admin]
spec_version: 1.0.0
last_verified: 2026-09-20
---

# studio — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `commerce` |
| Path | `internal/modules/studio` |
| Schema | `studio` |
| Delivery phase | 3 |
| Status | **IN_PROGRESS** |
| Owner | @commerce-team |

---

## 1. Overview

Creator Studio, course drafting, two review gates (Gate 1 automated AI/rules verification via `ItemVerifier`, Gate 2 moderator review queue), and publishing.

## 2. Responsibilities

**This module owns:**

- Creator profiles and payout account management
- Course drafts authoring and editing (units -> lessons -> activities outline)
- Automated Gate 1 verification execution (`studio.verify_submission` job)
- Moderator queue and Gate 2 decision workflows (approve, reject, request changes)
- Publishing approved course drafts into `lesson` and `content`
- Creator listings, purchases, and access gating (`MayOpen`)

**This module does NOT own:**

- Payment gateway transactions or SePay reconciliation — that is `payment`
- Course playback and learner progression — that is `lesson` and `learning`
- Grader algorithms or AI provider routing — that is `learning` and `platform/ai`

## 3. Business Rules

- **BR-STUDIO-06**: A reviewer may not review or decide their own submission (`reviewer_id != submission.submitted_by`).
- **BR-STUDIO-07**: A submission that fails Gate 1 never reaches human moderators.
- **BR-STUDIO-08**: Every activity in a published community course is a real `content_version`.
- **BR-STUDIO-09**: A creator's payout account is never returned in a list response and never logged.
- **BR-STUDIO-10**: A course draft may not contain an activity kind the lesson runner cannot render.
