---
doc_type: guide
task: add_an_endpoint
last_verified: 2026-08-06
---

# Guide: Add an API endpoint

The order of these steps is the whole point. Writing the handler first means the spec, the
generated client, the mocks and the tests all get retrofitted to whatever the handler happened
to do — and they drift from that moment on.

Estimated time: 30–60 minutes including tests.

---

## Step 1 — Confirm ownership

Open [`/MODULE_INDEX.md`](../../MODULE_INDEX.md) §1 and find which module owns this resource.
If the endpoint needs data from two modules, it still belongs to **one** of them; the other's
data comes through its `contract`.

If no module clearly owns it, stop. That is a boundary question, not an endpoint question.

## Step 2 — Design the contract, in the spec

Edit `api/openapi/openapi.yaml` (schemas go in `api/openapi/components/<module>.yaml`).

| Element | Requirement |
|---|---|
| Path | Plural, kebab-case, ≤ 2 levels of nesting |
| Method | See [`/API_GUIDELINE.md`](../../API_GUIDELINE.md) §2 |
| `operationId` | `<module><Action><Resource>` — this becomes the generated function name |
| `tags` | The module name |
| `x-permission` | Required unless the operation is genuinely public |
| Request schema | `snake_case` fields; unknown fields rejected |
| Response schema | Include a realistic example — MSW handlers are generated from it |
| Errors | Document every failure with its stable `code` |
| Pagination | Cursor, unless this is a bounded admin list |
| Idempotency | Required if it creates an attempt, a submission, or a charge |

```bash
npx @stoplight/spectral-cli lint api/openapi/openapi.yaml
make gen-api
```

The generated server interface now has a method your handler must implement. If you invented a
path or a shape, this is where you find out.

## Step 3 — Schema, if needed

If the endpoint needs new data:

```bash
make migrate-new MODULE=<module> NAME=<verb>_<object>
```

Then the query in `db/queries/<module>/`, then `make gen-sql`. Rules in
[`/DATABASE_GUIDELINE.md`](../../DATABASE_GUIDELINE.md). Every FK gets an index; every `:many`
query has a `LIMIT`.

## Step 4 — Domain and service

Business rules live here, not in the handler.

- New invariants go in `domain/` as pure functions with their own tests.
- The service method orchestrates: authorise, load, apply rules, persist, publish events.
- Call `rbac.Require(ctx, perm)` — the route group is not sufficient on its own.
- Filter by `actor.UserID` in the query for anything user-owned. **This is the line that
  prevents IDOR**, and no amount of middleware substitutes for it.
- Open a transaction only if you write more than one row; never let it span modules.
- Map rule violations to `apperr` with the codes you documented in step 2.

## Step 5 — Handler

Implement the generated interface method. It should decode, validate shape, call the service,
and render. Nothing else — no SQL, no `if` on business conditions, no direct repository access.

Register it in the module's `transport/http/routes.go`. `cmd/api` already mounts the module.

## Step 6 — Observability

The HTTP middleware gives you a span and access logging for free. Add:

- a span on the service method if it is not already instrumented
- a business counter if the endpoint represents a meaningful outcome
- structured log fields on state changes — IDs, never content

## Step 7 — Tests

| Test | Asserts |
|---|---|
| Domain unit | Each new invariant, including its boundaries |
| Service unit | Happy path, each rule violation → the right `apperr` code, repository error handling, events published |
| Repository integration | The SQL is correct against real Postgres; query count is what you expect |
| Contract | Response matches the OpenAPI schema; golden JSON |
| Authorization | A user without the permission gets 403; another user's resource gets **404**, not 403 |

## Step 8 — Frontend

```bash
cd web && pnpm run gen:api
```

Then add the query or mutation hook in the feature slice's `api/` folder, add the query key,
add the MSW handler (generated from the spec example), and use it. Never hand-write the type.

## Step 9 — Documentation

- The module's `API.md` — add the row and the detail block
- The module's `AGENT.md` §6 — add the endpoint; bump `last_verified`
- `CHANGELOG.md` under `Unreleased` if it is user-visible

## Step 10 — Verify

```bash
make check
make docs-check
```

---

## Checklist

- [ ] Spec edited **before** the handler
- [ ] `x-permission` declared and enforced in the service
- [ ] Ownership filter in the query
- [ ] Errors use documented, stable codes
- [ ] Cursor pagination on any list
- [ ] `Idempotency-Key` required where it should be
- [ ] Integration test proves the SQL
- [ ] Contract test proves the shape
- [ ] Another user gets 404, not 403
- [ ] Frontend types regenerated, not hand-written
- [ ] Module docs updated

---

## Prompt for an AI assistant

```
Read /AGENT.md, then internal/<group>/<module>/AGENT.md, then
docs/guides/add-an-endpoint.md.

Add: <METHOD> <path> — <one-sentence purpose>
Permission: <permission>
Request:  <fields>
Response: <fields>
Rules:    <the business rules that apply>

Follow the steps in the guide **in order**. Edit api/openapi/openapi.yaml first
and show me that diff before writing any Go.

Do not touch any module other than <module>. If you need data from another
module, use its contract package and tell me which method you used.
```
