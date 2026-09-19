---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-19
---

# Phase 3 — work order 15

**Purpose.** Let a learner publish their own course, and let them charge for it.

Today every course in the catalogue was written by us and seeded from `cmd/seed`, or generated
into a pool by the worker. A learner can upload a word list and nothing else. This work order adds
a **creator studio**: a learner drafts a course with units, lessons and exercises across any topic,
submits it, it is checked and reviewed, and it appears in the catalogue as **free** or **paid**.
Money arrives through **SePay** — a Vietnamese bank-transfer gateway, not a card processor — and
that distinction shapes most of the payment design below.

**Read first.** [ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0018](../adr/ADR-0018-media-presigned-upload.md),
[ADR-0025](../adr/ADR-0025-anonymous-curriculum-access.md), and the `AGENT.md`, `API.md` and
`DECISIONS.md` of `content`, `lesson`, `learning`, `payment`, `subscription`, `rbac` and `admin`.

**Do not start by writing a module.** Section 1 lists what already exists. Roughly half of this
work order is wiring machinery that is already built and unreachable.

---

## 1. What exists today

Checked on 2026-09-19 against `feat/phase-3-work-order-13` at `0ee3dcb`.

| Piece | State | Where |
|---|---|---|
| Authoring state machine | **Built.** `content.authoring_status` is an enum: `draft`, `in_review`, `approved`, `published`, `archived`. A trigger makes a published version immutable | `db/migrations/content/1700000190_create_content_tables.sql` |
| Review records | **Built.** `content.content_reviews` holds `version_id`, `reviewer_id`, `decision` (`approved` / `changes_requested`), `comments` | same migration |
| Submit / review / publish / archive | **Built and mounted**, behind `content.edit`, `content.review`, `content.publish` | `internal/modules/content/transport/http/handler.go:79` |
| BR-CONTENT-03 | **Built.** An author cannot approve their own version | `content/AGENT.md` §9 |
| Topic taxonomy | **Built and unused.** `content.taxonomies` (`namespace`, `code`, `label`, `parent_id`) and `content.content_tags`. No rows, no writer, no reader | `1700000190`, lines 133–165 |
| Reports from learners | **Built.** `POST /api/v1/content/versions/{id}/reports`, `content.item_reports`, `GET /api/v1/admin/content/reports` | `handler.go:75`, `:81` |
| Permissions | **Built.** `content.create`, `content.edit`, `content.review`, `content.publish`, `moderation.read`, `moderation.act` exist in `core.permissions` | `internal/modules/rbac/contract/permissions.go` |
| Roles | **Only two.** `admin` and `user`. `moderation.*` is defined and **granted to nobody**, used by nothing | `core.roles` |
| Course authoring contract | **Built.** `lesson`'s author contract has `EnsureCourse`, `EnsureUnit`, `EnsureLesson`, `AppendActivity` — the pools and `cmd/seed` are its only callers | `internal/modules/lesson/contract` |
| `learn.courses` | `slug`, `title`, `description`, `cefr_from`, `cefr_to`, `status` (`draft`/`published`/`archived`), `estimated_hours`. **No owner, no price, no topic, no visibility** | `db/migrations/lesson/1700000200` |
| Admin course creation | `POST /api/v1/admin/courses` behind `content.create` | `lesson/AGENT.md` §6 |
| Learner upload precedent | **Built.** `POST /me/vocabulary/uploads` returns 202, an hourly job checks each word, the learner watches counts move | `openapi.yaml:5417`, `vocabulary/module.go:201` |
| Presigned upload | **Built.** `/me/avatar/upload-intent`, `/speaking/upload-intent`, ADR-0018 | `openapi.yaml:1352`, `:5779` |
| Six-check item verification | **Built, and the best thing in the codebase for this work order.** Parse → own answer scores full marks → structure → blind solve → deduplicate → redaction, over `reading_comprehension`, `grammar_tense_choice`, `grammar_sentence_transform`, `listening_comprehension`, `writing_prompt`, `speaking_task`. **Unexported**, callable only from inside `learning/service` | `internal/modules/learning/service/practice_pool.go:316`, `exam_pool.go:352` |
| `payment` module | **Specification only.** `AGENT.md` describes 8 endpoints, 5 tables and 11 business rules. No `service/`, no `repository/`, no `transport/`, no migrations, no schema. `billing` exists as an empty Postgres schema | `internal/modules/payment/` |
| `subscription` module | Specification only, same state | `internal/modules/subscription/` |
| Enrolment | `POST /api/v1/courses/{id}/enroll` — creates a row, checks nothing about payment | `openapi.yaml:3408` |
| Highest migration number | `1700000740` | `db/migrations/speaking/` |
| Advisory lock IDs in use | `1_700_000_210`–`216`, `231`, `271`–`272`, `701` and others per module | `grep -rn "LockID.*=" internal/` |

**Two things follow from this table.**

The review workflow you need is already built; it is behind admin permissions and has no learner-facing
door. Most of section 8 is opening that door safely, not building a new one.

The `payment` module's specification was written for a **hosted card checkout** — `POST /billing/checkout`
returns a redirect URL, the learner comes back, a webhook confirms. SePay does not work that way, and
section 10 replaces those endpoints. Do not implement `payment/AGENT.md` as written.

---

## 2. How other platforms handle this

The owner asked for this comparison. It is here because the review model is the decision that
determines whether this feature costs one moderator or ten.

| Platform | Who may publish | Who approves | What that costs them |
|---|---|---|---|
| **Udemy** | Anyone, after making an instructor account | Automated quality checks (audio, length ≥ 30 min and ≥ 5 lectures, no promotional content) then human review, ~2 business days. Trust & Safety handles IP and plagiarism separately | A large permanent review team. The bar is *production quality*, not *pedagogical correctness* — nobody checks the answers |
| **Coursera / edX** | Only vetted partner institutions | Editorial staff, embedded in course production | Very high per course; the catalogue grows slowly and is uniformly good |
| **Teachable / Thinkific / Podia** | Anyone, instantly | **Nobody.** The creator owns their school; the platform enforces terms after the fact, driven by reports | Almost nothing up front, and the platform carries no quality signal at all — discovery is the creator's problem |
| **Skillshare** | Anyone, after a teacher application | Light automated review plus spot checks; quality is governed by the royalty pool, which pays by minutes watched | Moderate. The incentive does most of the work |
| **Duolingo Incubator** | Volunteers, by application | Staff moderation plus a three-phase beta (hatching → beta → graduated) | They **shut it down in 2021**. Moderation cost more than the courses were worth and quality stayed uneven |

The Duolingo row is the one that matters most here, because it is the closest analogue: a language
course made of *graded exercises*, where a wrong answer key is invisible to a reviewer skimming a
submission but poisons every learner who meets it.

**The conclusion this work order takes from that table:** the expensive part of reviewing a language
course is not judging taste, it is checking that the answers are right — and this codebase can already
do that automatically, because it does it to its own AI-generated items every hour. Spend the machine
on correctness, and spend the human only where the machine cannot help: topic suitability, plagiarism,
and whether money should change hands.

---

## 3. Decisions taken

### Answered here; change them only with a reason

| Question | Decision | Why |
|---|---|---|
| A new module, or extend `content`/`lesson`? | **New module `studio`**, schema `studio`, tier `learning` | The creator workflow is its own lifecycle with its own tables. `content` and `lesson` stay what they are, and `studio` drives them through their existing contracts. Widening `content`'s admin handlers to serve learners would put a learner-facing authorisation decision inside a module whose every route today assumes staff |
| Who approves? | **Two gates.** Gate 1 is automated and always runs. Gate 2 is a human holding `content.review`, and is required for every **paid** course and for a creator's **first** course. After a creator has 3 approved courses and no upheld report, their **free** submissions publish on Gate 1 alone | Section 2. The human is spent where the machine cannot help |
| What does Gate 1 check? | The six checks `learning` already runs on generated items, plus structural course rules, a profanity and PII scan, and a near-duplicate check against published content | It is built, it is tested, and it is the only cheap way to catch a wrong answer key |
| Who are the humans? | A new **`moderator`** role, granted `content.review`, `content.publish`, `moderation.read`, `moderation.act`. Admins keep everything | The permissions exist and are granted to nobody. A moderator must not be an admin |
| Pricing model | **Free** or **one-time purchase in VND.** No subscriptions, no instalments, no bundles | A per-course subscription is a different product. `subscription` stays unbuilt |
| Currency | **VND only**, stored as `bigint` minor-unit-free (VND has no subunit). Never `float` | BR-PAYMENT-10. A `numeric` with implied decimals invites a ₫100 course to be charged ₫1 |
| Revenue share | **70% creator / 30% platform**, recorded per order at purchase time | Udemy's instructor-referred rate is higher and its platform-referred rate lower; 70/30 is the common default and the number is the owner's to change. Recording it per order means changing it later cannot rewrite history |
| Payment provider | **SePay**, bank transfer with VietQR | The owner's choice. Section 10 |
| Payouts to creators | **Manual bank transfer by an admin**, recorded in `billing.payouts`, monthly, minimum ₫500,000 | SePay receives money; it does not send it. Pretending otherwise would be inventing an API |
| Refunds | 7 days from purchase **and** under 20% of the course completed, self-service; anything else is a moderator decision | `learn.progress` already measures completion exactly, which most platforms cannot do, so the rule can be precise rather than a flat window |
| Does a paid course gate `POST /courses/{id}/enroll`? | **Yes**, and it is the only gate. Every learner-facing read of a paid course's lessons goes through the same check | One gate, tested once |

### Open — the owner decides before step 6

1. **Revenue share and minimum price.** 70/30 and ₫49,000 are placeholders.
2. **Tax.** Vietnam withholds personal income tax on payments to individuals above a threshold, and
   digital services carry VAT. This work order records gross, share and net per order so the numbers
   exist, and **calculates no tax**. Confirm the obligation with an accountant before the first payout;
   it may add a withholding column and a creator tax code, both of which are cheap now and expensive
   after money has moved.
3. **Identity verification for paid creators.** This plan requires a verified email, a bank account
   number and an account holder name. It does **not** require an ID document. If Vietnamese law or the
   bank requires more, it belongs in step 6.
4. **Who is the first moderator?** The role is useless until somebody holds it.

---

## 4. Scope

**In.**

1. `studio` module: creator profiles, course drafts, submissions, the review queue, listings.
2. `learn.courses` gains an owner, a visibility, an origin and a topic.
3. Gate 1: automated verification, reusing `learning`'s six checks through a new contract.
4. Gate 2: a human review queue, the `moderator` role, and the decision trail.
5. `payment` module, built for SePay: orders, webhook, reconciliation, refunds, payouts.
6. Purchase and access: buying a course, and the gate on enrolment.
7. Creator earnings screen; moderator queue screen; learner purchase flow.
8. Topic taxonomy seeded with a starting set, and catalogue filters that use it.

**Out, deliberately.**

- **Subscriptions.** `subscription` stays a specification. A learner buys courses, not a plan.
- **Video lessons.** A course is made of the exercise kinds the runner already renders. Video is a
  media pipeline, a storage bill and a moderation problem of its own.
- **Creator-to-learner messaging, Q&A, reviews and ratings.** Each is a module.
- **Coupons, sales, affiliate links, bundles.**
- **Multi-currency and international cards.**
- **Automatic payouts.** Manual, recorded, reconciled.
- **Course versioning after publication.** v1: a published course is edited by submitting a new
  version that replaces it on approval. Learners mid-course keep the version they started.

---

## 5. Architecture

```
                        ┌──────────────────────────────────────┐
   creator ─── POST ───▶│  studio                              │
                        │  drafts, submissions, reviews,       │
                        │  listings, purchases, earnings       │
                        └───┬───────────┬───────────┬──────────┘
                            │ contract  │ contract  │ contract
                   ┌────────▼──┐  ┌─────▼─────┐  ┌──▼──────────┐
                   │  content  │  │  lesson   │  │  learning   │
                   │  items,   │  │  courses, │  │  ItemVerifier│
                   │  versions,│  │  units,   │  │  (new)      │
                   │  reviews  │  │  lessons  │  │             │
                   └───────────┘  └───────────┘  └─────────────┘

                        ┌──────────────────────────────────────┐
   SePay ─── webhook ──▶│  payment                             │
                        │  orders, sepay transactions,         │
                        │  webhooks, refunds, payouts          │
                        └──────────────┬───────────────────────┘
                                       │  payment.succeeded (event)
                                       ▼
                                    studio grants access
```

`payment` knows nothing about courses. It knows an order has a reference, an amount and a payer, and
it publishes when money arrives. `studio` decides what that buys. This is the split `payment/AGENT.md`
already states between `payment` and `subscription`, pointed at `studio` instead.

### New schema: `studio`

Migrations `db/migrations/studio/`, starting at **`1700000750`**.

| Table | Purpose | Key columns |
|---|---|---|
| `studio.creator_profiles` | One row per learner who has opened the studio | `user_id` PK FK `core.users` ON DELETE CASCADE, `display_name`, `bio`, `status` (`active`/`suspended`), `trusted_at` (null until 3 approved courses and no upheld report), `approved_course_count`, `upheld_report_count` |
| `studio.payout_accounts` | Bank details for a paid creator | `user_id` PK, `bank_code`, `account_number`, `account_holder`, `verified_at`. **Restricted**: never in a list response, never logged |
| `studio.course_drafts` | A course being written, before it is a `learn.course` | `id`, `owner_id`, `title`, `description`, `cefr_from`, `cefr_to`, `topic_taxonomy_id`, `body` jsonb (the whole unit/lesson/activity tree), `status` (`draft`/`submitted`/`changes_requested`/`approved`/`published`/`rejected`), `course_id` (null until first publish), `created_at`, `updated_at` |
| `studio.submissions` | One attempt at getting a draft published | `id`, `draft_id`, `submitted_by`, `submitted_at`, `gate1_status` (`pending`/`passed`/`failed`), `gate1_report` jsonb, `gate2_required` bool, `decision` (`approved`/`changes_requested`/`rejected`, null while open), `decided_by`, `decided_at`, `reviewer_notes` |
| `studio.listings` | What a published course costs | `course_id` PK, `creator_id`, `pricing_model` (`free`/`one_time`), `price_vnd` bigint null, `revenue_share_bps` int, `status` (`active`/`unlisted`/`taken_down`), `published_at` |
| `studio.purchases` | Who owns what | `id`, `user_id`, `course_id`, `order_id` (FK `billing.orders`, null for a free claim), `price_paid_vnd`, `granted_at`, `revoked_at`, `revoke_reason`. UNIQUE `(user_id, course_id) WHERE revoked_at IS NULL` |
| `studio.creator_ledger` | What the platform owes a creator | `id`, `creator_id`, `kind` (`sale`/`refund`/`payout`/`adjustment`), `amount_vnd` bigint signed, `purchase_id`, `payout_id`, `note`, `created_at`. Append-only; balance is a sum |

### `billing` schema, built by `payment`

Migrations `db/migrations/payment/`, starting at **`1700000760`**.

| Table | Purpose | Key columns |
|---|---|---|
| `billing.orders` | One intent to pay | `id`, `user_id`, `reference` UNIQUE (the code the payer types), `amount_vnd` bigint, `status` (`pending`/`paid`/`expired`/`cancelled`/`refunded`), `subject_kind` (`course`), `subject_id`, `expires_at`, `paid_at` |
| `billing.sepay_transactions` | A bank transaction SePay told us about | `id`, `sepay_id` bigint **UNIQUE**, `gateway`, `transaction_date`, `account_number`, `sub_account`, `code`, `content`, `transfer_type`, `transfer_amount` bigint, `reference_code`, `accumulated`, `order_id` null, `matched_at`, `unmatched_reason` |
| `billing.payment_webhooks` | Every webhook body, raw | `id`, `provider`, `provider_event_id` UNIQUE, `payload` jsonb, `signature_valid` bool, `received_at`, `processed_at`, `error` |
| `billing.refunds` | Money returned to a learner | `order_id`, `amount_vnd`, `reason`, `actor_id`, `status` (`requested`/`sent`/`failed`), `sent_at` |
| `billing.payouts` | Money sent to a creator, by hand | `id`, `creator_id`, `amount_vnd`, `status` (`pending`/`sent`/`failed`), `bank_reference`, `actor_id`, `sent_at` |

### `learn.courses` gains four columns

Migration `db/migrations/lesson/1700000745_courses_ownership.sql`. Owned by `lesson` (rule DB1) —
`studio` writes them through `lesson`'s contract, never with its own SQL.

```sql
ALTER TABLE learn.courses
    ADD COLUMN IF NOT EXISTS owner_id           uuid REFERENCES core.users (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS origin             text NOT NULL DEFAULT 'official',
    ADD COLUMN IF NOT EXISTS visibility         text NOT NULL DEFAULT 'public',
    ADD COLUMN IF NOT EXISTS topic_taxonomy_id  uuid REFERENCES content.taxonomies (id) ON DELETE SET NULL;
```

`origin` is `official` or `community`; `visibility` is `public` or `unlisted`. Both default to what
every existing row already is, so the migration needs no backfill and the catalogue does not change
shape on deploy. `owner_id` stays null for seeded and pool courses — they have no creator, and
inventing one would put a person's name on a machine's work.

> **A cross-schema foreign key.** `learn.courses.topic_taxonomy_id` references `content.taxonomies`.
> Rule DB1 forbids a module writing another's tables, not a foreign key between schemas, and the
> existing tree already does this (`skill.speaking_consents` → `core.users`). If the reviewer disagrees,
> drop the constraint and keep the column — but then a deleted taxonomy leaves dangling ids.

---

## 6. The creator's path

Each step is a screen and one or two endpoints.

1. **Open the studio.** `POST /api/v1/me/studio/profile` creates `creator_profiles`. Requires a
   verified email — the existing `auth` verification, not a new one.
2. **Start a draft.** `POST /api/v1/me/studio/drafts` with title, description, CEFR range and topic.
3. **Write it.** `PUT /api/v1/me/studio/drafts/{id}` saves the whole tree in `body`. The editor is a
   client-side outline: units → lessons → activities, where an activity is one of the kinds the runner
   renders. Saving is a full replace of `body`, not a patch — a partial update of a nested tree is a
   merge problem nobody needs.
4. **Preview.** `GET /api/v1/me/studio/drafts/{id}/preview` returns the draft in the exact shape
   `GET /lessons/{id}` returns, so the existing runner can render it without a second code path. The
   answers are redacted the same way, so the creator sees what a learner will see.
5. **Price it.** `PUT /api/v1/me/studio/drafts/{id}/listing` — `free`, or `one_time` with a price.
   Choosing `one_time` requires `payout_accounts` to be filled in; the endpoint answers
   `PAYOUT_ACCOUNT_REQUIRED` otherwise, rather than letting somebody sell a course they cannot be paid for.
6. **Submit.** `POST /api/v1/me/studio/drafts/{id}/submit` → **202**, a `submissions` row, Gate 1
   enqueued. Following the vocabulary upload's precedent exactly: the checks are slow and worth
   retrying, and a creator submitting forty exercises should not watch a request time out.
7. **Watch.** `GET /api/v1/me/studio/submissions` and `/{id}` show the gate report, per item, with the
   reason each failure failed. A `changes_requested` returns the draft to `draft` with the notes attached.
8. **Published.** The course appears in the catalogue. `GET /api/v1/me/studio/earnings` shows the ledger.

### The activity kinds a creator may author

The eleven the lesson runner renders, and no others — a course containing a kind the runner cannot
render is a course a learner opens and cannot do:

`vocab_multiple_choice`, `vocab_gap_fill`, `vocab_flashcard`, `vocab_listen_type`, `vocab_match`,
`vocab_reorder`, `vocab_context_choice`, `grammar_tense_choice`, `grammar_sentence_transform`,
`reading_comprehension`, `writing_prompt`, `speaking_task`.

**`listening_comprehension` is excluded in v1.** Its audio is rendered offline by `cmd/tts` against a
Piper voice on an operator's machine. A creator cannot trigger that, and a listening item whose clip
never renders is undrawable — the exact failure work order 14 spent a day diagnosing. Allowing it needs
a rendering queue the creator can watch, which is its own step.

---

## 7. Gate 1 — the automated gate

Runs as a job (`studio.verify_submission`), lock ID **`1_700_000_750`**, enqueued on submit.

### 7.1 Expose `learning`'s checks

`learning` verifies every AI-generated item with six checks before publishing it into a pool. They are
unexported. Add to `internal/modules/learning/contract`:

```go
// ItemVerifier runs the six checks that stand between a generated item and a
// learner. A community submission is checked by exactly the same machine, for
// exactly the same reason: a wrong answer key is invisible to a reader and
// wrong for everyone who meets it.
type ItemVerifier interface {
    // VerifyItem returns nil when the item passes, or an error naming the
    // check that failed. `blindSolve` is optional because check 4 costs an AI
    // call per item.
    VerifyItem(ctx context.Context, req VerifyItemRequest) error
}

type VerifyItemRequest struct {
    Kind       string
    TaskType   string          // read_aloud / respond, for speaking_task
    CEFRLevel  string
    Body       json.RawMessage
    Existing   []json.RawMessage // for the duplicate check
    BlindSolve bool
}
```

Implemented by `learning`'s service over the existing `checkAndPreparePoolCandidate` and
`checkCandidate`. This is a refactor with no behaviour change, and the pool tests already cover it.

### 7.2 The checks, per submission

| # | Check | Cost | Applies to |
|---|---|---|---|
| 1 | **Structure.** 1–12 units, 1–20 lessons per unit, 3–30 activities per lesson, every title non-empty, CEFR within the declared range | free | all |
| 2 | **Minimum size.** At least 3 lessons and at least 20 activities in total | free | all |
| 3 | **Kind allowed.** Every activity is one of the eleven | free | all |
| 4 | **`ItemVerifier` without blind solve** — checks 1, 2, 3, 5, 6: it parses, its own answer scores full marks against the real grader, its options are well formed and distinct, it is not a duplicate, and nothing answer-bearing survives redaction | free | every activity |
| 5 | **`ItemVerifier` with blind solve** — check 4: a model answers the redacted item and must agree with the key | **one AI call per item** | every activity of a **paid** course; a **sample of 20%, minimum 5** for a free course |
| 6 | **Language and safety.** Profanity, contact details, promotional URLs, and an email/phone scan (a course body is not a place for a creator's phone number) | free | all text |
| 7 | **Near-duplicate.** Normalised passage and prompt text against published `content_versions` | free | all |

Failure is per item, and the report names the item and the check. A submission fails Gate 1 if **any**
check 1–4, 6 or 7 fails, or if **more than 10%** of the blind-solved sample disagrees — a single
disagreement is often the model, not the item, which is why the pool retries three times rather than
rejecting on one.

> **Cost control.** A 40-activity paid course is ~40 AI calls at submit. The AI budget table seeded in
> `1700000720_budget_every_provider_and_task.sql` allows 500 requests per provider per task; a burst of
> submissions can exhaust it and stall the practice pools. Gate 1 must run under its own task budget,
> and a submission that cannot get budget waits rather than failing — `ai.ErrProvidersUnavailable`
> already carries that signal and `topUpExamSlot` already shows the pattern.

### 7.3 What Gate 1 does not do

It cannot tell whether a course is worth taking, whether the topic is appropriate, or whether the
material is somebody else's. That is Gate 2's whole job, and the review screen should say so.

---

## 8. Gate 2 — the human gate

### When it is required

```
gate2_required =
      listing.pricing_model == "one_time"          // money is involved
   || creator.trusted_at IS NULL                    // their first courses
   || creator.upheld_report_count > 0               // they have been wrong before
   || topic is in the sensitive set                 // see below
```

A creator becomes trusted at **3 approved courses with no upheld report**; an upheld report clears
`trusted_at` and they return to full review. This is a column, not a computation, so a moderator can
grant or revoke trust by hand.

The **sensitive set** is a flag on `content.taxonomies` — topics where a wrong or malicious course
does more harm than a bad grammar drill (medical, legal, financial, anything aimed at children).
Start it empty and let moderators mark topics as they appear.

### The queue

| Method | Path | Permission |
|---|---|---|
| `GET` | `/api/v1/admin/studio/submissions` | `content.review` |
| `GET` | `/api/v1/admin/studio/submissions/{id}` | `content.review` |
| `POST` | `/api/v1/admin/studio/submissions/{id}/decision` | `content.review` |
| `POST` | `/api/v1/admin/studio/courses/{id}/takedown` | `moderation.act` |
| `POST` | `/api/v1/admin/studio/creators/{id}/suspend` | `moderation.act` |

The detail screen shows the Gate 1 report first — a reviewer who does not have to re-check forty answer
keys can actually read the course. Then the course, rendered by the same preview the creator saw.

`POST .../decision` takes `approved`, `changes_requested` or `rejected`, plus notes. **Notes are
required for anything but `approved`**: "changes requested" with no changes named is the single most
common way a review queue becomes useless.

**BR-CONTENT-03 extends:** a reviewer may not decide their own submission. The check is
`submission.submitted_by != reviewer_id`, enforced in the service, and it is the one line in this work
order most likely to be forgotten.

### Publishing

On `approved`, in one transaction:

1. `lesson.EnsureCourse/EnsureUnit/EnsureLesson/AppendActivity` build the real course, with
   `origin = 'community'`, `owner_id = creator`, `status = 'published'`.
2. Each activity's body is published through `content`'s author contract, so it becomes a real
   `content_version` with the immutability trigger over it.
3. `studio.listings` is written.
4. The draft moves to `published` and keeps `course_id`.
5. Outbox: `studio.course_published`.

If any of that fails, none of it happens and the submission stays open.

### After publication

The existing `POST /content/versions/{id}/reports` already lets a learner report an item.
`GET /admin/content/reports` already lists them. A report whose subject belongs to a community course
also counts against the creator: three upheld reports on one course take it down automatically and
return the creator to full review.

---

## 9. Free and paid

### The listing

```
pricing_model: free | one_time
price_vnd:     null | 49_000 .. 5_000_000      (bigint, VND, no decimals)
revenue_share_bps: 7000                         (creator's share, recorded per listing)
```

Rules:

1. **BR-STUDIO-01** — A price is a whole number of VND between the configured minimum and maximum.
   VND has no subunit; a decimal price is rejected at the schema, not rounded.
2. **BR-STUDIO-02** — A free course may be made paid only by a new submission through Gate 2. A paid
   course may be made free at any time, immediately, with no review: the direction that costs a learner
   money is the direction that gets checked.
3. **BR-STUDIO-03** — Changing a price never changes what a past purchase was worth. `purchases`
   records `price_paid_vnd` and `creator_ledger` records the split, at purchase time.
4. **BR-STUDIO-04** — A course with no active listing is not purchasable and not in the catalogue, but
   every learner who already bought it keeps access. Taking a course down is not a refund.

### The learner's side

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/courses` | The catalogue, now with `origin`, `topic`, `price` and `owned` |
| `GET` | `/api/v1/courses/{slug}` | Course detail. For an unowned paid course, **lesson titles only** — no activities |
| `POST` | `/api/v1/courses/{id}/claim` | Free community course: creates the `purchases` row with no order |
| `POST` | `/api/v1/courses/{id}/purchase` | Paid: creates an order, returns the QR and the code |
| `GET` | `/api/v1/me/purchases` | What the learner owns |
| `POST` | `/api/v1/me/purchases/{id}/refund` | Within the window, self-service |

**BR-STUDIO-05** — The paywall is one function, `studio.AccessReader.MayOpen(ctx, userID, courseID)`,
and it is called by `POST /courses/{id}/enroll`, `GET /courses/{slug}` and `GET /lessons/{id}`. An
official course and a free community course always answer yes. Three call sites, one rule, one test
that walks all three.

**The trap worth naming.** `GET /lessons/{id}` is currently anonymous-readable by ADR-0025. A paid
community lesson must not be. The gate belongs in `lesson`'s handler via a `studio` contract, and the
ADR needs an amendment recording the exception — not a silent divergence from a written decision.

---

## 10. SePay

Verified against `docs.sepay.vn` on 2026-09-19. Do not paraphrase this section from memory.

### 10.1 What SePay is

SePay watches one or more of our bank accounts and calls our webhook when a transaction posts. It is
**not** a card gateway and it **never redirects the payer**. There is no hosted checkout, no session,
no return URL, and no card. The payer opens their own banking app and transfers.

That means the endpoints in `payment/AGENT.md` §6 — `POST /billing/checkout` returning a redirect URL,
`GET /billing/checkout/{id}` — describe a product we are not using. **Replace them**, and update that
`AGENT.md` through `tools/docgen/data/` so `make docs` does not put the old ones back.

### 10.2 The flow

```
learner presses Buy
   │
   ├─▶ POST /api/v1/courses/{id}/purchase
   │      creates billing.orders: reference = "FLU" + base32(order id)[:10]
   │      returns { reference, amount_vnd, qr_url, bank, account, expires_at }
   │
   ├─▶ the page shows the VietQR image and the transfer details
   │      https://vietqr.app/img?acc=<acc>&bank=<bank>&amount=<amount>
   │          &des=<reference>&template=compact
   │
   ├─▶ learner transfers from their banking app; the reference is the content
   │
   ├─▶ SePay POSTs our webhook, within seconds
   │      POST /api/v1/webhooks/payment/sepay
   │
   └─▶ we store it raw, answer {"success": true}, and match in a job
```

The page polls `GET /api/v1/me/orders/{id}` while it waits. Do not hold the request open.

### 10.3 The QR

```
https://vietqr.app/img?acc=<account>&bank=<bank>&amount=<vnd>&des=<reference>&template=compact
```

`acc` and `bank` are required; `bank` accepts a code, a BIN or a short name (`VCB`, `970436`,
`Vietcombank`). `des` must be **unaccented letters and digits only** — which is why the reference is
`FLU` plus base32, and never the course title. `template=compact` is the layout SePay documents for a
checkout page.

The image is served by SePay, so the page embeds the URL. Do not proxy it — that adds a failure mode
between the learner and their money for no gain.

### 10.4 The webhook

`POST /api/v1/webhooks/payment/sepay`, public by necessity. Body:

```json
{
  "id": 92704,
  "gateway": "Vietcombank",
  "transactionDate": "2024-07-02 11:08:33",
  "accountNumber": "1017588888",
  "subAccount": "",
  "code": "SEVN63DC8E5C",
  "content": "SEVN63DC8E5C chuyen tien",
  "transferType": "in",
  "description": "NGUYEN VAN A chuyen tien",
  "transferAmount": 5000000,
  "accumulated": 105000000,
  "referenceCode": "FT24012345678"
}
```

**Authentication.** SePay offers HMAC-SHA256, API Key, OAuth2 or none. Use **API Key**: the header is
`Authorization: Apikey <our key>`. Compare with `subtle.ConstantTimeCompare` — a naive `==` on a secret
is a timing oracle, and this endpoint is public. Add SePay's published sender IPs as a second factor;
they are listed in their docs and belong in configuration, not in code, because they change.

**Answering.** SePay counts the delivery as successful only on HTTP 200 or 201 **with a JSON body of
`{"success": true}`**, within 30 seconds. A 204, or a 200 with an empty body, is a failure and SePay
will retry — up to 7 times over 5 hours, at Fibonacci intervals. So:

1. Verify the key.
2. Insert into `billing.payment_webhooks` and `billing.sepay_transactions` (`sepay_id` UNIQUE).
3. Enqueue `payment.match_transaction`.
4. Answer `{"success": true}`.

Nothing slow happens inside the request. This is BR-PAYMENT-04 with SePay's real numbers.

**Idempotency.** SePay retries, a moderator can resend by hand, and several webhooks can point at one
endpoint. `sepay_id` is UNIQUE and the insert is `ON CONFLICT DO NOTHING`; a duplicate is acknowledged
and dropped. This is the one rule whose absence produces a creator paid twice.

### 10.5 Matching

In the job, not the request:

1. `transferType` must be `in`. An outgoing transaction is recorded and ignored.
2. Find the reference in `content`, case-insensitively, after stripping non-alphanumerics — banks
   mangle transfer content, and `des` arrives inside a longer string.
3. The order must be `pending` and unexpired.
4. **`transferAmount` must equal `amount_vnd` exactly.**
5. Mark the order `paid`, write `studio.purchases` and two `creator_ledger` rows (the creator's share
   and the platform's), publish `payment.succeeded`.

A transaction that matches nothing, or matches with the wrong amount, is left with an
`unmatched_reason` and appears in `GET /api/v1/admin/payments/unmatched` for a human. **Never
auto-refund and never partially credit**: someone transferring ₫49,000 for a ₫490,000 course is a
support conversation, not a state transition.

**Expiry.** An order lives 24 hours. A payment arriving against an expired order still matches — the
money is real — and the job reopens the order rather than leaving the learner paid and unenrolled.
Sweep expired unpaid orders hourly, lock ID **`1_700_000_751`**.

### 10.6 Reconciliation

Daily, lock ID **`1_700_000_752`**. BR-PAYMENT-07: the bank is the source of truth.

```
GET https://my.sepay.vn/userapi/transactions/list?since_id=<last seen>&limit=5000
Authorization: Bearer <SEPAY_API_TOKEN>
```

Returns `{status, error, messages, transactions[]}` where each row carries `id`, `amount_in`,
`amount_out`, `transaction_content`, `reference_number`, `account_number`, `sub_account`,
`transaction_date`, `bank_brand_name`. Rate limit is **3 requests per second**; over it, HTTP 429 with
`x-sepay-userapi-retry-after` in seconds. Honour that header.

Anything present at SePay and absent here is a **missed webhook** — insert and match it, which is the
whole reason this job exists. Anything paid here and absent there raises an alert and is **not**
auto-corrected.

### 10.7 Testing without money

SePay has a "Gửi thử" (send test) button that posts a sample payload to our URL, and a transaction
simulator. Use both. Beyond that, the webhook handler is an HTTP handler over a JSON body: the
fixtures that matter are success, wrong key, replayed `id`, wrong amount, unknown reference, expired
order, and `transferType: "out"`. None need a bank.

### 10.8 Configuration

Add to `.env.example` — and nowhere else, per `AGENT.md`'s rule about inventing config keys:

```
# ---------------------------------------------------------------- SePay
# Bank transfer payments (docs.sepay.vn). Empty SEPAY_WEBHOOK_API_KEY disables
# the webhook route entirely rather than leaving it open.
SEPAY_WEBHOOK_API_KEY=
SEPAY_API_TOKEN=
SEPAY_ACCOUNT_NUMBER=
SEPAY_BANK_CODE=
SEPAY_ACCOUNT_HOLDER=
# Comma-separated sender IPs; empty means the API key alone authenticates.
SEPAY_ALLOWED_IPS=
# Order lifetime and price bounds.
SEPAY_ORDER_TTL=24h
STUDIO_MIN_PRICE_VND=49000
STUDIO_MAX_PRICE_VND=5000000
STUDIO_REVENUE_SHARE_BPS=7000
```

---

## 11. Steps

Each ships on its own and leaves the tree green. Do not start a step before the one before it is merged.

| # | Step | Ships |
|---|---|---|
| 1 | **`learningcontract.ItemVerifier`** — expose the six checks, no behaviour change | A contract and a refactor, covered by the existing pool tests |
| 2 | **`learn.courses` ownership columns** + the topic taxonomy seeded with ~20 topics, and `GET /courses` filtering by them | The catalogue gains topics. Nothing else changes |
| 3 | **`moderator` role** granted the four permissions, and `cmd/seed` creating one moderator account | A role somebody can hold |
| 4 | **`studio` module, free courses only** — profiles, drafts, preview, submit, Gate 1, Gate 2, publish. No price anywhere | A learner can publish a free course. This is the whole feature minus money, and it is the honest place to stop if money slips |
| 5 | **`payment` module** — orders, SePay webhook, matching, expiry sweep, reconciliation, the unmatched queue. Tested against the simulator | Money can arrive and be matched. Nothing sells yet |
| 6 | **Listings and purchases** — price a course, buy it, `MayOpen` on three call sites, refunds, the ledger | A learner can buy a course |
| 7 | **Creator earnings and payouts** — the earnings screen, the admin payout screen, `billing.payouts`, the manual transfer trail | A creator can be paid |
| 8 | **Frontend polish** — the course editor, the moderator queue, the purchase page, i18n in both locales | |

Steps 1–4 are worth doing even if 5–8 never happen. Step 5 is worth nothing without 6.

---

## 12. Business rules

Add to `tools/docgen/data/`, not to the `AGENT.md` files directly — the generated blocks are rewritten
by `make docs` and a hand edit is silently reverted.

**`studio`**

1. **BR-STUDIO-01** — A price is a whole number of VND inside the configured bounds.
2. **BR-STUDIO-02** — Free → paid requires a new review. Paid → free does not.
3. **BR-STUDIO-03** — A purchase records the price and the split at the moment it happened.
4. **BR-STUDIO-04** — Taking a course down never revokes a purchase.
5. **BR-STUDIO-05** — Access is one function, called by enrolment, course detail and lesson reads.
6. **BR-STUDIO-06** — A reviewer may not decide their own submission, extending BR-CONTENT-03.
7. **BR-STUDIO-07** — A submission that fails Gate 1 never reaches a human.
8. **BR-STUDIO-08** — Every activity in a published community course is a real `content_version`,
   subject to the same immutability trigger and the same redaction as ours.
9. **BR-STUDIO-09** — A creator's payout account is never returned in a list response and never logged.
10. **BR-STUDIO-10** — A course may not contain a kind the lesson runner cannot render.

**`payment`** — keep BR-PAYMENT-02 through 11 from the existing specification, and revise:

- **BR-PAYMENT-01** — replace "card data never reaches our servers" with: **no card exists.** Payment
  is a bank transfer the payer initiates; we hold a reference, an amount and what the bank told us.
- **BR-PAYMENT-04** — the acknowledgement budget is SePay's **30 seconds**, and the body must be
  `{"success": true}`; we still answer in under two.
- **BR-PAYMENT-12** (new) — An amount that does not match exactly is never partially credited.
- **BR-PAYMENT-13** (new) — A payment against an expired order is honoured; the money is real.

---

## 13. Testing

| Layer | What |
|---|---|
| Unit | Reference generation and parsing, including bank-mangled content; price validation at the bounds; the trust transition; `gate2_required` over every combination |
| Unit | `MayOpen` for: official, free community, paid unowned, paid owned, paid refunded, paid taken down |
| Integration | Draft → submit → Gate 1 fail → fix → resubmit → Gate 2 approve → published course readable by the runner. One test, the whole path |
| Integration | The webhook: success, wrong key, replayed `sepay_id`, wrong amount, unknown reference, expired order, `transferType: "out"` |
| Integration | Reconciliation inserts a transaction the webhook never delivered, and matches it |
| Integration | A reviewer cannot approve their own submission |
| E2E | A creator publishes a free course and a second account takes it |
| E2E | The purchase page shows a QR and the reference, and the order moves to paid when the webhook fires |

**Gate 1 needs a fixture set of deliberately bad courses** — a wrong answer key, a duplicate passage,
an answer leaking through redaction, a phone number in a prompt, an unrenderable kind. Without those,
Gate 1's tests prove only that good courses pass, which is the half that does not matter.

---

## 14. Traps

1. **`make gen-check` uses `git status --porcelain`.** It is dirty on any uncommitted tree. Commit
   before trusting it.
2. **Adding an OpenAPI path writes generated code on both sides.** Run `make gen` and commit the
   generated Go and TypeScript in the same commit.
3. **The generated `AGENT.md` blocks come from `tools/docgen/data/`.** Editing `payment/AGENT.md` by
   hand to fix the checkout endpoints will be reverted by the next `make docs`.
4. **`check-drift.mjs` step 3** compares `.go-arch-lint.yml` against `MODULE_INDEX.md` §3. A new module
   must be added to both or the docs gate fails.
5. **`go-arch-lint` deepScan diverges on Windows.** Green locally is not green in CI; run it in the
   Linux container.
6. **The `billing` schema already exists** and is empty. Do not create it again.
7. **Migration numbers are flat across module folders.** `1700000740` is taken; this plan allocates
   `1700000745` (lesson), `1700000750+` (studio), `1700000760+` (payment).
8. **Advisory lock IDs must be unique across the process.** `1_700_000_750`, `751` and `752` are free
   as of this writing; check `grep -rn "LockID.*=" internal/` before taking one.
9. **`des` in the QR URL must be unaccented.** A Vietnamese course title in the transfer content
   produces a QR the bank app rejects.
10. **VND is not cents.** `transferAmount` from SePay is whole VND. Storing it in a "minor units"
    column that something later divides by 100 turns ₫490,000 into ₫4,900.
11. **`GET /lessons/{id}` is anonymous by ADR-0025.** Paid lessons are the exception, and the ADR needs
    an amendment rather than a quiet divergence.
12. **Prettier reformats two web test files** on `make check`. Revert those before committing; it is
    not part of this change.

---

## 15. Risks, and what to cut

| Risk | Mitigation |
|---|---|
| **Moderation becomes the bottleneck** — the Duolingo Incubator failure | Gate 1 removes the expensive half. Trusted creators remove most of the rest. If the queue still grows, raise the trust bar for *review* and lower it for *free* courses, not the other way round |
| **Gate 1's AI cost spikes on a submission burst** | Its own budget task; blind solve sampled for free courses; a submission that cannot get budget waits |
| **A creator sells a course that is somebody else's** | Gate 2 for every paid course. The near-duplicate check catches copies of *our* content, not the internet's — say so on the review screen rather than implying it is covered |
| **Money arrives and nothing grants access** | Reconciliation, the unmatched queue, and the rule that an expired order still honours a real payment |
| **Payouts are manual and someone forgets** | An append-only ledger, a balance that is a sum, and a payout screen that lists who is owed. Manual is fine; untracked is not |
| **Tax and invoicing are unresolved** | Section 3, open question 2. Record gross, share and net from day one so the numbers exist when the answer arrives |

**If this must be smaller:** ship steps 1–4 and stop. A studio that publishes free community courses
is a complete, useful product, and it is the part that makes the catalogue diverse — which was the
first thing the owner asked for. Money is the second half and it can wait for its own work order
without leaving anything half-built.

---

## 16. What this work order does not answer

- Discovery. A catalogue with 200 community courses needs search, ranking and a home page that is not
  a flat list. There is a `search` platform package; nothing uses it.
- Quality signal. No ratings, no completion rates surfaced, no "this course was reviewed on".
- The creator's second version. v1 replaces a course on approval; learners mid-course keep what they
  started. That is a decision, but it is not a versioning system.
- Anything about `subscription`. It remains a specification with no code, and this work order does not
  change that.
