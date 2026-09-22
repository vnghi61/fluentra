---
module: studio
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@commerce-team"
schema: studio
tables: [creator_profiles, payout_accounts, course_drafts, submissions, listings, purchases, creator_ledger, takedowns]
depends_on: [content, lesson, learning, payment, job, resource]
depended_on_by: [admin, lesson, learning]
spec_version: 1.0.0
last_verified: 2026-09-20
---

# studio — TODO

Ordered backlog for the Creator Studio module.

## Step 4 Landed (Free Community Courses)

- [x] Schema `studio` with `creator_profiles`, `payout_accounts`, `course_drafts`, `submissions`
- [x] Creator profile upsert and retrieval
- [x] Payout account creation (restricted, never listed or logged per BR-STUDIO-09)
- [x] Course draft authoring outline (units -> lessons -> activities)
- [x] Gate 1 automated verification (`learning.ItemVerifier`, structural size checks, safety scans, excluded activity kind checks)
- [x] Gate 2 moderation queue (reviews, approve, reject with feedback, BR-STUDIO-06 self-review prevention)
- [x] Course publishing into `lesson` and `content` upon moderator approval

## Step 5 Landed (Payment Module)

- [x] Schema `billing` with `orders`, `sepay_transactions`, `payment_webhooks`, `refunds`, `payouts`
- [x] SePay VietQR webhook ingestion with constant-time API key auth
- [x] Background order matching on alphanumeric transfer reference
- [x] Hourly expiry sweep and daily reconciliation job
- [x] Unmatched transaction queue for admin resolution

## Step 6 Landed (Listings, Purchases, and Paywall Gating)

- [x] Tables `studio.listings`, `studio.purchases`, `studio.creator_ledger`
- [x] Pricing bounds validation (₫49,000 to ₫5,000,000) per BR-STUDIO-01
- [x] Free course claiming (`POST /courses/{id}/claim`) and paid purchase order creation (`POST /courses/{id}/purchase`)
- [x] Self-service refunds (`POST /me/purchases/{id}/refund`) within 7 days and < 20% course completion
- [x] 70/30 creator/platform split recording in `creator_ledger` per BR-STUDIO-03
- [x] BR-STUDIO-05 paywall enforcement via single-sourced `studio.AccessReader.MayOpen`:
  - `POST /courses/{id}/enroll`
  - `GET /courses/{slug}`
  - `GET /lessons/{id}` (with ADR-0025 amendment)

## Step 7 Landed (Creator Earnings and Admin Payouts)

- [x] Creator earnings dashboard (`GET /me/studio/earnings`) with available balance, lifetime earnings, pending payouts, masked account
- [x] Payout request flow (`POST /me/studio/payouts`) with balance and threshold checks (₫500,000 minimum)
- [x] Admin payout listing and detail endpoints (`GET /admin/billing/payouts`, `GET /admin/billing/payouts/{id}`)
- [x] Admin payout fulfillment endpoint (`POST /admin/billing/payouts/{id}/fulfill`) emitting `payment.payout_sent`
- [x] Creator ledger debiting on payout sent event (`kind='payout'`, negative amount)

## Next Steps (Step 8)

- [ ] Step 8: Web frontend for Creator Studio, Moderation Queue, and Checkout UI

## Work Order 20 Landed (Documents and videos in a community course)

- [x] Activity kind `lesson_material` (document / video) accepted by Gate 1 (BR-STUDIO-10, twelve kinds)
- [x] Gate 1 `material_ready`: `MaterialForOwner` resolves the draft owner's validated resource with the renditions the runner needs
- [x] A lesson is valid with one material, or 3–30 exercises; a course needs twenty exercises, materials not counted (BR-STUDIO-11)
- [x] Publishing copies the resource's original and ready renditions into `fluentra-media` under `course-materials/{course_id}/{slug}-u{n}-l{n}-a{n}/` (BR-STUDIO-12)
- [ ] A sweeper for `course-materials/` prefixes no published version references (out of scope; recorded in `resource/TODO.md`)
- [ ] The Gate 2 review screen does not yet render a material with the runner component; reviewers open it from the draft (WO 20 §5, first cut)
- [ ] Free-preview lessons for non-buyers (D20-4)
- [ ] Video progress tracking, resume position, subtitles
