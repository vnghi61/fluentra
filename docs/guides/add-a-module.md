---
doc_type: guide
task: add_a_module
last_verified: 2026-08-06
---

# Guide: Add a new module

> Before starting, be sure you actually need a module. A new module is justified when the
> concept has its **own tables, its own business rules, and its own reason to change**.
> If it is a new capability of an existing bounded context, add it there instead.
> If you are unsure, ask — a wrongly placed module is expensive to move later.

Estimated time: 60–90 minutes for the scaffolding, before any feature work.

---

## Step 0 — Decide and record

| Question | Where the answer goes |
|---|---|
| What does it own that nothing else owns? | Its `AGENT.md` §2 |
| What does it explicitly *not* own? | Its `AGENT.md` §2 |
| Which tier: `platform` or a business tier? | The manifest |
| Which schema will its tables live in? | The manifest and `ARCHITECTURE.md` §8.2 |
| Which modules will it depend on, and why each? | The manifest and `.go-arch-lint.yml` |
| Which modules will depend on it? | Its `contract` design |
| Does it introduce an external dependency? | If yes, **write an ADR first** |

If it crosses an existing boundary or adds a vendor, stop and write the ADR. Everything below
assumes the decision is made.

---

## Step 1 — Register it (four files, one commit)

1. `docs/modules/manifest.yaml` — add the entry.
2. `tools/docgen/data/<tier>.json` — add the full module definition (purpose, responsibilities,
   tables, endpoints, rules, tasks, limitations, testing, decisions, TODO).
3. `MODULE_INDEX.md` — add it to §1 quick lookup, §2 register, and the §3 dependency graph.
4. `.go-arch-lint.yml` — add the `c_<name>` and `m_<name>` components and the `deps` entry.

A module that is not in all four is not real. CI's `dep-drift` job checks that the manifest and
the arch-lint config agree.

## Step 2 — Generate the documentation

```bash
make docs
```

This writes the nine files under `internal/<group>/<name>/`. Read the generated `AGENT.md` and
fill in anything the manifest could not express. **The documentation exists before the code —
that is deliberate.** It is the specification you will implement against, and the context the
next agent will read.

## Step 3 — Create the code skeleton

```
internal/modules/<name>/
├── contract/          service.go · dto.go · events.go
├── domain/            entities, value objects, invariants, errors
├── service/           use cases
├── repository/        sqlc output + mappers
├── transport/http/    handlers + routes
├── job/               (only if it owns background work)
└── module.go          New(deps) (*Module, error)
```

`module.go` is the module's only export to `cmd/`:

- it accepts a dependency struct containing **contract interfaces**, never concrete types
- it returns the module, exposing `Routes() chi.Router`, `Contract() contract.Service`, and
  `JobWorkers()` if it has any
- it performs no I/O at construction beyond validating that its dependencies are non-nil

## Step 4 — Schema

```bash
make migrate-new MODULE=<name> NAME=create_<table>
```

Rules: tables go in the module's own schema; no foreign key crosses a schema boundary except to
`core.users(id)`; every migration is reversible; every FK gets an index. See
[`/DATABASE_GUIDELINE.md`](../../DATABASE_GUIDELINE.md).

Then write queries in `db/queries/<name>/*.sql` and run `make gen-sql`.

## Step 5 — API surface

Edit `api/openapi/openapi.yaml` **before** writing a handler:

- add a tag matching the module name
- put schemas in `api/openapi/components/<name>.yaml`
- give every operation an `operationId`, a description, at least one example, and — unless it is
  public — an `x-permission`

Then `make gen-api`. The generated server interface will not compile until your handlers match
the spec. That is the mechanism that stops invented endpoints.

## Step 6 — Permissions

Add the module's permissions to `rbac`: constants in `contract/permissions.go`, rows in a
migration, and mapping to the roles that should hold them. Call `rbac.Require` in the service
methods, not only in middleware.

## Step 7 — Wire it up

In `cmd/api/main.go`, construct the module after its dependencies and mount its router.
In `cmd/worker/main.go`, register its job workers. Both are plain constructor calls in
dependency order — no framework, no magic (ADR-0006).

## Step 8 — Observability

- Spans on every service method: `<module>.<Operation>`, with `module` and `operation` attributes
- Use the shared instruments from `platform/telemetry`; add a new one only if none fits
- Business counters for outcomes that matter
- A dashboard panel if this is a new user-facing capability
- An alert **and a runbook** if it can fail in a way learners notice

## Step 9 — Tests

| Layer | What |
|---|---|
| `domain` | Table-driven tests for every invariant. Target 90 % |
| `service` | Use cases with mocked ports; every business-rule violation maps to the right `apperr` code. Target 80 % |
| `repository` | Integration tests with testcontainers; assert query counts to catch N+1 |
| `transport` | A contract test per endpoint against the OpenAPI spec |

## Step 10 — Close the loop

- [ ] `make check` green
- [ ] `make docs-check` green (front-matter, drift, links)
- [ ] The module's `AGENT.md` reflects what you actually built, with `last_verified` bumped
- [ ] `TODO.md` populated with the remaining work and acceptance criteria
- [ ] `CHANGELOG.md` entry under `Unreleased` if anything is user-visible
- [ ] `MODULE_INDEX.md` status changed from `PLANNED` to `IN_PROGRESS`

---

## Prompt for an AI assistant

```
Read /AGENT.md, then docs/guides/add-a-module.md.

Create the module <name>:
  tier:        <platform|core|learning|commerce>
  schema:      <schema>
  owns:        <one sentence>
  depends on:  <modules, and why each>

Do steps 1–3 only (registration, docs generation, code skeleton).
Do not implement business logic yet.
Do not add a dependency that is not listed above — if you think one is needed, stop and say so.

When done, show me:
  · the diff to the four registration files
  · the generated AGENT.md
  · the module.go skeleton
```

Keeping the first pass to scaffolding is deliberate: it is the part where a mistake is cheap to
fix and expensive to discover later.
