---
module: subscription
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [plans, entitlements, subscriptions, subscription_events]
depends_on: [payment, user, notification, cache, job, audit]
depended_on_by: [writing, speaking, vocabulary, exam, ai, admin, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# subscription — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `commerce` |
| Path | `internal/modules/subscription` |
| Schema | `billing` |
| Delivery phase | 4 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Plans, entitlements and the subscription lifecycle: trials, upgrades, downgrades, renewals, grace periods and cancellation. It decides what a learner is allowed to do; `payment` decides whether money moved.
<!-- END GENERATED: overview -->

**Context.** Entitlements are read on nearly every AI-consuming request, so they are cached aggressively and invalidated eagerly. The separation from `payment` matters: a gateway outage must not change what an existing subscriber can do.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Plan catalogue with prices and entitlements
- Entitlement resolution: what this learner may do right now
- Subscription lifecycle and state transitions
- Trials, upgrades, downgrades and proration rules
- Renewal scheduling and grace periods on failed payment
- Cancellation and reactivation
- Feature gating helpers used by other modules

**This module does NOT own:**

- Taking money — that is `payment`
- Enforcing a quota in the moment — the consuming module does that using the entitlement
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/subscription/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/subscription/contract/` | You are calling this module from another module |
| `internal/modules/subscription/service/` | You are changing behaviour |
| `db/migrations/subscription/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/subscription/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `subscription.EntitlementReader` | `Entitlement(ctx, userID, key) (value, bool)` — the single question every gated feature asks |
| interface | `subscription.Lifecycle` | `Activate`, `Expire`, `EnterGrace` — called by `payment` on webhook events |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `subscription.activated` | publishes | `{user_id, plan_code, period_end}` |
| `subscription.changed` | publishes | `{user_id, from_plan, to_plan}` |
| `subscription.expiring` | publishes | `{user_id, days_remaining}` |
| `subscription.expired` | publishes | `{user_id, previous_plan}` |
| `payment.succeeded` | consumes | Activate or renew |
| `payment.failed` | consumes | Enter the grace period |
| `payment.refunded` | consumes | Revoke or adjust |
| `user.deleted` | consumes | Cancel and anonymise |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `billing` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/subscription/` · Queries: `db/queries/subscription/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `billing.plans` | Purchasable tier | `code` UNIQUE, `name`, `price` numeric + `currency`, `interval`, `status` |
| `billing.entitlements` | What a plan grants | `plan_id`, `key` (e.g. `ai.writing.daily`), `value`. A missing key means not entitled |
| `billing.subscriptions` | A learner's subscription | `user_id`, `plan_id`, `status`, `current_period_start/end`, `cancel_at_period_end`, `trial_ends_at`, `grace_until` |
| `billing.subscription_events` | Lifecycle audit | `subscription_id`, `kind`, `from_status`, `to_status`, `reason`, `actor` |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `subscription`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/billing/plans` | `public` | Plan catalogue with prices |
| `GET` | `/api/v1/me/subscription` | `self` | Current subscription and entitlements |
| `POST` | `/api/v1/me/subscription` | `self` | Start a subscription or trial (delegates checkout to `payment`) |
| `POST` | `/api/v1/me/subscription/change` | `self` | Upgrade or downgrade |
| `POST` | `/api/v1/me/subscription/cancel` | `self` | Cancel at period end |
| `POST` | `/api/v1/me/subscription/reactivate` | `self` | Undo a pending cancellation |
| `GET` | `/api/v1/admin/subscriptions` | `billing.read` | Search subscriptions |
| `POST` | `/api/v1/admin/subscriptions/{id}/grant` | `billing.manage` | Grant a complimentary period |
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
| [`payment`](../../modules/payment/AGENT.md) | → depends on | see its contract |
| [`user`](../../modules/user/AGENT.md) | → depends on | see its contract |
| [`notification`](../../modules/notification/AGENT.md) | → depends on | see its contract |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | see its contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`ai`](../../platform/ai/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-SUBSCRIPTION-01** — Entitlements are resolved from the **subscription state**, never from payment status directly — a gateway outage must not de-entitle a paying learner.
2. **BR-SUBSCRIPTION-02** — A missing entitlement key means not entitled. Defaults are explicit in the free plan, never implicit in code.
3. **BR-SUBSCRIPTION-03** — Entitlement changes take effect within 60 seconds; the cache TTL and eager invalidation together guarantee it.
4. **BR-SUBSCRIPTION-04** — One active subscription per learner.
5. **BR-SUBSCRIPTION-05** — An upgrade is immediate with proration; a downgrade takes effect at the period end, so a learner never loses paid-for capability early.
6. **BR-SUBSCRIPTION-06** — A failed renewal enters a grace period (default 7 days) with full access, then downgrades to free — access is not cut off the moment a card fails.
7. **BR-SUBSCRIPTION-07** — Cancellation is always at period end; a mid-period refund is a `payment` decision, handled separately.
8. **BR-SUBSCRIPTION-08** — A trial requires no payment method and converts only with explicit consent — no silent charge at trial end.
9. **BR-SUBSCRIPTION-09** — Complimentary grants require a reason and are audited.
10. **BR-SUBSCRIPTION-10** — Downgrading never deletes learner data; it only restricts what can be created going forward.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add an entitlement

1. Choose a key following `<domain>.<feature>.<unit>` and document it in `docs/product/entitlements.md`.
2. Add values for every plan, including free — an absent key means not entitled, so being explicit avoids accidental gating.
3. Read it through `EntitlementReader` in the consuming module; never hard-code a plan name.
4. Add the upgrade prompt copy for the denied case.
5. Test the free, trial, paid and grace states.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- One subscription per learner; there are no add-ons or metered top-ups in Phase 4.
- Proration follows a simple day-count model rather than the gateway's own proration engine.
- Currency is single per plan; multi-currency pricing is not modelled.
- No family or group plans — that would edge towards the multi-tenancy the product deliberately avoids.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:subscription:entitlements:{user_id}:v1` | 60 s | Any subscription event |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `SUBSCRIPTION_ALREADY_ACTIVE` | 409 | Duplicate subscribe |
| `PLAN_NOT_AVAILABLE` | 409 | Plan withdrawn or region-restricted |
| `TRIAL_ALREADY_USED` | 409 | One trial per learner |
| `ENTITLEMENT_REQUIRED` | 403 | The feature requires a higher plan |
| `INVALID_STATE_TRANSITION` | 409 | e.g. reactivating an expired subscription |


## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/subscription/...                    # unit
go test -tags=integration ./internal/modules/subscription/...  # integration (testcontainers)
```

**Focus areas**

- Entitlement resolution across free, trial, active, grace and pending-cancel
- Cache invalidation within the 60-second promise
- Grace period entry and recovery
- Upgrade immediate, downgrade deferred
- Trial cannot be reused
- Downgrade restricts creation but destroys nothing
- Complimentary grants are audited with a reason
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not read payment status to decide access.
- Do not hard-code a plan name in a feature check — read an entitlement key.
- Do not delete learner data on downgrade.
- Do not convert a trial without explicit consent.
- Do not grant a complimentary period without a recorded reason.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
