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

## Next Steps (Steps 5 - 8)
- [ ] Step 5: `payment` module integration with SePay bank transfer webhook & reconciliation
- [ ] Step 6: `studio.listings`, `studio.purchases`, `studio.creator_ledger`, and `MayOpen` paywall enforcement
- [ ] Step 7: Creator earnings dashboard and payout records
- [ ] Step 8: Web frontend for Creator Studio and Moderation Queue
