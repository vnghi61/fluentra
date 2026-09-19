---
module: studio
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@commerce-team"
schema: studio
tables: [creator_profiles, payout_accounts, course_drafts, submissions, listings, purchases, creator_ledger]
depends_on: [content, lesson, learning, payment, job]
depended_on_by: [admin, lesson, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
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
| Status | **ACTIVE** |
| Owner | @commerce-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Creator Studio: authoring community courses, draft editing, submissions, automated Gate 1 checks, Gate 2 human moderation, and course publishing.
<!-- END GENERATED: overview -->

**Context.** Community creators build courses, preview them, and submit them through a two-gate verification pipeline before publication.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Creator profile and payout account registration
- Course draft CRUD and structure validation
- Gate 1 automated verification (structure, CEFR, safety, runner kinds)
- Gate 2 human moderation queue and approval/rejection decisions
- Course publishing to catalogue with content versioning

**This module does NOT own:**

- Lesson execution and grading — that is `lesson` and `learning`
- Bank transfer matching — that is `payment`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/studio/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/studio/contract/` | You are calling this module from another module |
| `internal/modules/studio/service/` | You are changing behaviour |
| `db/migrations/studio/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/studio/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `studio.AccessReader` | MayOpen(ctx, userID, courseID) evaluates paywall access across 3 call sites |
| interface | `studio.ListingReader` | Course pricing and listing lookup for catalogue |
| interface | `studio.Publisher` | Course publication and gating |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `studio.course_published` | publishes | `{course_id, creator_id, title}` |
| `payment.succeeded` | consumes | Fulfill paid course purchase and credit creator ledger with 70/30 split |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `studio` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/studio/` · Queries: `db/queries/studio/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `studio.creator_profiles` | Creator profiles | `user_id` PK, `display_name`, `bio`, `headline`, `trusted_at`, `upheld_report_count` |
| `studio.payout_accounts` | Creator payout bank accounts | `creator_id` PK, `bank_name`, `account_number_hash`, `encrypted_account_number`, `account_holder_name` |
| `studio.course_drafts` | In-flight course authoring drafts | `id` PK, `creator_id`, `title`, `description`, `units` jsonb, `status` |
| `studio.submissions` | Course submissions undergoing review | `id` PK, `draft_id`, `status`, `gate1_report` jsonb, `gate2_required`, `submitted_by` |
| `studio.listings` | Course pricing and catalogue listings | `course_id` PK, `creator_id`, `pricing_model`, `price_vnd`, `revenue_share_bps`, `status` |
| `studio.purchases` | User course purchases and free claims | `id` PK, `user_id`, `course_id`, `price_paid_vnd`, `status`, `refund_reason` |
| `studio.creator_ledger` | Double-entry accounting ledger for creator earnings | `id` PK, `creator_id`, `entry_type`, `amount_vnd`, `balance_after_vnd`, `reference_id` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `studio`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/studio/creator/profile` | `self` | Get current creator profile |
| `POST` | `/api/v1/studio/creator/profile` | `self` | Create or update creator profile |
| `GET` | `/api/v1/studio/creator/payout-account` | `self` | Get payout account info |
| `POST` | `/api/v1/studio/creator/payout-account` | `self` | Set payout account |
| `GET` | `/api/v1/studio/courses` | `self` | List creator course drafts |
| `POST` | `/api/v1/studio/courses` | `self` | Create course draft |
| `GET` | `/api/v1/studio/courses/{id}` | `self` | Get course draft |
| `PUT` | `/api/v1/studio/courses/{id}` | `self` | Update course draft |
| `POST` | `/api/v1/studio/courses/{id}/submit` | `self` | Submit draft for verification |
| `POST` | `/api/v1/courses/{id}/claim` | `self` | Claim access to a free community course |
| `POST` | `/api/v1/courses/{id}/purchase` | `self` | Initiate purchase of a paid course via VietQR |
| `GET` | `/api/v1/me/purchases` | `self` | List courses purchased or claimed by learner |
| `POST` | `/api/v1/me/purchases/{id}/refund` | `self` | Self-service refund for course purchase |
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
| [`content`](../../modules/content/AGENT.md) | → depends on | Publishes activity content versions |
| [`lesson`](../../modules/lesson/AGENT.md) | → depends on | Creates courses, units, and lessons in the core catalogue |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | ItemVerifier candidate checks and learner progress lookup for refund eligibility |
| [`payment`](../../modules/payment/AGENT.md) | → depends on | Order creation and payment matching for paid courses |
| [`job`](../../platform/job/AGENT.md) | → depends on | Gate 1 verification worker |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`lesson`](../../modules/lesson/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-STUDIO-01** — BR-STUDIO-01: A price is a whole number of VND inside configured bounds (49,000 to 5,000,000).
2. **BR-STUDIO-02** — BR-STUDIO-02: Free -> paid requires a new review. Paid -> free does not.
3. **BR-STUDIO-03** — BR-STUDIO-03: A purchase records the price and the 70/30 creator/platform split in creator_ledger at purchase time.
4. **BR-STUDIO-04** — BR-STUDIO-04: Taking a course down never revokes a purchase.
5. **BR-STUDIO-05** — BR-STUDIO-05: The paywall is one function (AccessReader.MayOpen) called by enrollment, course detail, and lesson reads.
6. **BR-STUDIO-06** — BR-STUDIO-06: A reviewer may not decide their own submission (submitted_by != reviewer_id).
7. **BR-STUDIO-07** — BR-STUDIO-07: A submission that fails Gate 1 never reaches a human.
8. **BR-STUDIO-08** — BR-STUDIO-08: Every activity in a published community course is a real content_version.
9. **BR-STUDIO-09** — BR-STUDIO-09: A creator's payout account is never returned in a list response and never logged.
10. **BR-STUDIO-10** — BR-STUDIO-10: A course may not contain a kind the lesson runner cannot render (11 runner kinds only).
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

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `DRAFT_NOT_FOUND` | 404 | Course draft not found |
| `SUBMISSION_NOT_FOUND` | 404 | Submission not found |
| `CANNOT_REVIEW_OWN_SUBMISSION` | 403 | Reviewer submitted this course |

### Security considerations

- Gate 2 moderation requires content.review / content.publish permissions.
- Reviewer cannot review their own course.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80%**

```bash
go test ./internal/modules/studio/...                    # unit
go test -tags=integration ./internal/modules/studio/...  # integration (testcontainers)
```

**Focus areas**

- Gate 1 automated checks
- Gate 2 moderation decisions
- BR-STUDIO-06 self-review prevention
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not allow a creator to review their own submission.
- Do not log or expose raw payout bank account numbers.
- Do not allow activity kinds outside the 11 runner kinds.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
