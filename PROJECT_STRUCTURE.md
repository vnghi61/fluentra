---
doc_type: structure
project: fluentra
last_verified: 2026-08-06
---

# PROJECT_STRUCTURE.md

Every folder in this repository, what it is for, and what an AI agent should do with it.

---

## 1. Top level

```
fluentra/
├── AGENT.md                  # AI entry point — read first
├── CLAUDE.md                 # → AGENT.md
├── AGENTS.md                 # → AGENT.md (Codex/OpenAI convention)
├── GEMINI.md                 # → AGENT.md
├── README.md                 # human entry point
├── ARCHITECTURE.md           # the SAD
├── PROJECT_STRUCTURE.md      # this file
├── MODULE_INDEX.md           # module map
├── *_GUIDELINE.md            # conventions (see §8)
├── Makefile                  # every command an agent may run
├── go.mod / go.sum
├── .go-arch-lint.yml         # module boundary enforcement
├── .golangci.yml             # Go linters
├── sqlc.yaml                 # query codegen config
├── .env.example              # every config key, documented
│
├── .github/                  # CI/CD, issue and PR templates
├── .claude/                  # Claude Code project config: commands, settings
├── api/                      # API + event contracts (source of truth)
├── cmd/                      # binaries
├── internal/                 # all Go code
├── db/                       # migrations, queries, seeds
├── web/                      # React SPA
├── deploy/                   # Docker Compose + infra configs
├── docs/                     # the knowledge base
├── scripts/                  # dev/ops shell scripts
├── test/                     # cross-module tests and fixtures
└── tools/                    # repo tooling (docgen, lint helpers)
```

---

## 2. `cmd/` — binaries

| Path | Binary | Purpose | Agent note |
|---|---|---|---|
| `cmd/api` | `api` | HTTP server. Composition root: config → telemetry → pools → modules → router. | The **only** place concrete implementations are wired. Adding a module means adding a line here. |
| `cmd/worker` | `worker` | River worker + cron scheduler. | Register new job kinds here. |
| `cmd/migrate` | `migrate` | Runs goose migrations, then exits. | Never called from `api`. |
| `cmd/seed` | `seed` | Loads dev/demo data. | Never run against production. |
| `cmd/docgen` | `docgen` | Generates module doc scaffolding from the manifest. | `make docs` calls this. |

---

## 3. `internal/` — application code

```
internal/
├── shared/          # cross-cutting primitives, no business meaning
│   ├── apperr/          typed errors + Problem Details rendering
│   ├── config/          koanf loading, validation, defaults
│   ├── flags/           feature flags
│   ├── id/              UUIDv7 / ULID generation
│   ├── clock/           injectable time
│   ├── money/           minor-unit money type
│   ├── pagination/      cursor encode/decode
│   ├── validation/      validator wiring, custom rules, i18n messages
│   ├── eventbus/        publish/subscribe interface + in-process impl
│   ├── outbox/          transactional outbox writer + publisher
│   ├── idempotency/     idempotency-key store and replay
│   ├── httpx/           middleware, response writers, decoders
│   ├── dbx/             pgx pool, tx helper, Querier interface
│   └── i18n/            message catalogue
│
├── platform/        # technical capabilities (see MODULE_INDEX §2.1)
│   ├── telemetry/  cache/  storage/  job/  mailer/  ai/  media/  search/
│
└── modules/         # business modules (see MODULE_INDEX §2.2–2.4)
    ├── auth/  user/  rbac/  audit/  admin/  notification/
    ├── content/  lesson/  learning/  srs/
    ├── vocabulary/  grammar/  reading/  listening/  speaking/  writing/
    ├── questionbank/  exam/  gamification/
    └── analytics/  subscription/  payment/
```

### 3.1 Inside a module

| Subfolder | Contains | May be imported by other modules? |
|---|---|---|
| `contract/` | Interfaces, DTOs, event types — the module's public API | ✅ **Only this one** |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go. | ❌ |
| `service/` | Use cases, orchestration, transactions, event publishing | ❌ |
| `repository/` | sqlc-generated code + mappers | ❌ |
| `transport/http/` | Handlers, request/response DTOs, route registration | ❌ |
| `job/` | Background job handlers owned by the module | ❌ |
| `module.go` | `New(deps) (*Module, error)` — wiring | ✅ (by `cmd/` only) |

---

## 4. `api/` — contracts

| Path | Contains | Rule |
|---|---|---|
| `api/openapi/openapi.yaml` | The full OpenAPI 3.1 spec | **Single source of truth for the HTTP API.** Edit before writing a handler. |
| `api/openapi/components/` | Reusable schemas, parameters, responses split by module | Keeps the root file readable |
| `api/events/*.json` | JSON Schema for every domain event | Versioned; breaking change = new file |
| `api/proto/` | Reserved for gRPC when a module is extracted | Empty in v1 — do not add files without an ADR |

Generated from these: Go server stubs + client (`oapi-codegen`), TypeScript types
(`openapi-typescript`), MSW handlers, and the docs site.

---

## 5. `db/`

| Path | Contains | Rule |
|---|---|---|
| `db/migrations/<module>/` | goose SQL migrations, `NNNN_description.sql` | One folder per module (rule L3). Always reversible. |
| `db/queries/<module>/` | `.sql` files annotated for sqlc | The only place SQL is written |
| `db/seeds/` | Reference data (roles, permissions, CEFR levels, taxonomies) | Idempotent, safe to re-run |
| `db/fixtures/` | Test data sets for integration tests | Never loaded in production |

---

## 6. `web/` — React SPA

```
web/
├── AGENT.md               # frontend AI entry point
├── src/                   # see ARCHITECTURE.md §7.1
├── e2e/                   # Playwright specs + page objects
├── public/
├── vite.config.ts
├── tsconfig.json
├── eslint.config.js       # includes boundary rules between feature slices
└── package.json
```

---

## 7. `deploy/`

| Path | Contains |
|---|---|
| `deploy/compose/` | `compose.yaml` + `compose.{dev,prod,observability}.yaml` overlays |
| `deploy/otel/` | Collector pipelines (receivers → processors → exporters) |
| `deploy/prometheus/` | scrape config, recording rules, alert rules |
| `deploy/grafana/` | provisioning, datasources, dashboards-as-JSON |
| `deploy/loki/`, `deploy/tempo/` | storage + retention config |
| `deploy/nginx/` | TLS, gzip/brotli, SPA fallback, security headers, rate limits |
| `deploy/minio/` | bucket creation, policies, lifecycle rules |

---

## 8. `docs/` — the AI knowledge base

| Folder | Purpose | Contents | How AI agents use it |
|---|---|---|---|
| `docs/architecture/` | System-level design | SAD sections, plan review, boundary rules, microservice migration, C4 sources | Read before proposing any structural change |
| `docs/backend/` | Go conventions | Layering, error handling, transactions, concurrency, pagination, background work | Read before writing Go in an unfamiliar area |
| `docs/frontend/` | React conventions | Structure, state, routing, forms, data fetching, a11y, performance budget | Read before writing React |
| `docs/database/` | Data conventions | Naming, types, indexing, migrations, partitioning, ER diagrams per schema | Read before writing a migration |
| `docs/api/` | HTTP standards | REST rules, versioning, pagination, errors, auth, idempotency, webhooks | Read before adding an endpoint |
| `docs/deployment/` | Running the system | Local setup, configuration reference, compose topology, production checklist, backup/restore | Read for env/config questions |
| `docs/security/` | Security | Threat model, authn/authz design, data protection, ASVS mapping, incident response | Read before touching auth, uploads, or PII |
| `docs/testing/` | Testing | Pyramid, testcontainers, fixtures, golden files, coverage policy, AI test generation | Read before writing tests |
| `docs/modules/` | Module registry | `manifest.yaml` (generator source), per-module deep dives, boundary matrix | The generator reads it; agents read the manifest to learn ownership |
| `docs/prompts/` | Prompt library | `dev/` (prompts that generate code) and `runtime/` (prompts the app sends to LLMs) | Copy a dev prompt to do a task; never inline a runtime prompt in code |
| `docs/adr/` | Decisions | Numbered ADRs, immutable once accepted | **Read before proposing an alternative approach** — it may already be rejected |
| `docs/decisions/` | Decision index | Index + template + superseded log | Entry point into `adr/` |
| `docs/development/` | How we work | Getting started, git workflow, code review, docs-as-code, definition of done | Read on day one |
| `docs/guides/` | Task recipes | "Add a module", "add an endpoint", "add a job", "add an AI feature", "debug CI" | **Start here for a concrete task** |
| `docs/knowledge/` | Domain knowledge | CEFR, FSRS, pronunciation scoring, IELTS/TOEIC formats, item response theory, English pedagogy | Read before designing learning logic — this is where domain correctness lives |
| `docs/ai/` | AI engineering | Context strategy, agent playbooks, eval harness, cost model, safety | Read before any LLM work |
| `docs/examples/` | Canonical code shapes | Reference module walkthrough, example handler/service/repo/test/component | **Copy these patterns instead of inventing new ones** |
| `docs/diagrams/` | Mermaid sources | C4, sequence, ER, state, flow | Reuse and extend; do not redraw |
| `docs/templates/` | Templates | Module docs, ADR, PR, RFC, runbook, test plan | Use verbatim, fill placeholders |
| `docs/operations/` | Run the thing | Runbooks, SLOs, on-call, incident postmortems, capacity planning | Read when something is on fire |
| `docs/product/` | Product context | Personas, journeys, feature specs, business glossary, metrics tree | Read before deciding what a feature should *do* |

---

## 9. `test/` and `tools/`

| Path | Purpose |
|---|---|
| `test/integration/` | Cross-module integration tests (a single module's live in the module) |
| `test/e2e/` | API-level end-to-end tests (browser E2E lives in `web/e2e/`) |
| `test/load/` | k6 scenarios and thresholds |
| `test/fixtures/` | Shared builders, golden files, mock AI fixtures |
| `tools/docgen/` | Templates and manifest schema for the doc generator |

---

## 10. Naming conventions (repo-wide)

| Thing | Convention | Example |
|---|---|---|
| Go package | short, lowercase, no underscores | `vocabulary`, `apperr` |
| Go file | `snake_case.go` | `add_word.go` |
| Go interface | noun or `-er` | `Grader`, `WordRepository` |
| Go test | `TestXxx_Scenario_Expected` | `TestAddWord_DuplicateLemma_ReturnsConflict` |
| SQL table | plural `snake_case`, in the owning schema | `skill.word_senses` |
| SQL column | `snake_case`; FKs `<singular>_id` | `deck_id` |
| Migration | `NNNN_verb_object.sql` | `0007_add_word_family_index.sql` |
| API path | kebab-case plural | `/api/v1/writing/submissions` |
| JSON field | `snake_case` | `next_due_at` |
| Event | `<module>.<aggregate>_<past_verb>` | `writing.submission_created` |
| Metric | `<domain>_<unit>_<suffix>` | `ai_request_duration_seconds` |
| Cache key | `fluentra:{env}:{module}:{entity}:{id}:v{n}` | see ARCHITECTURE §12.2 |
| React component | `PascalCase.tsx` | `ReviewCard.tsx` |
| React hook | `useCamelCase.ts` | `useDueReviews.ts` |
| Feature slice | lowercase noun | `web/src/features/vocabulary/` |
| Branch | `type/scope-short-desc` | `feat/srs-fsrs-scheduler` |
| Commit | Conventional Commits | `feat(srs): add FSRS scheduler` |
