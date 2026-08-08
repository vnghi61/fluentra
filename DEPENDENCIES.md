---
doc_type: dependency_register
project: fluentra
last_verified: 2026-08-06
---

# DEPENDENCIES.md

> **Principle: never reinvent a solved problem.** Every dependency below was chosen against
> named alternatives. Adding a dependency requires a row here (rule L12) and, if it is
> architecturally significant, an ADR.

Scoring: **Maturity** (years + API stability) · **Community** (stars, contributors, ecosystem) ·
**Maintenance** (release cadence, open-issue hygiene) · **Prod-ready** (used at scale in
production by others). Scale 1–5.

---

## 1. Backend — Go

### 1.1 HTTP router

| Library | Maturity | Community | Maintenance | Prod-ready | Verdict |
|---|---|---|---|---|---|
| **`go-chi/chi` v5** ✅ | 5 | 5 | 5 | 5 | **Chosen** |
| stdlib `net/http.ServeMux` (1.22+) | 5 | 5 | 5 | 5 | Close second |
| `gin-gonic/gin` | 5 | 5 | 4 | 5 | Rejected |
| `labstack/echo` | 5 | 4 | 4 | 5 | Rejected |
| `gofiber/fiber` | 4 | 5 | 4 | 4 | Rejected |

**Why chi:** it *is* `net/http` — handlers are `http.Handler`, so every piece of the Go HTTP
ecosystem (otelhttp, httptest, middleware) works unmodified. Sub-routers map cleanly onto our
module boundaries (`r.Mount("/vocabulary", vocabModule.Routes())`).
**Advantages:** zero lock-in, ~1k LOC, excellent middleware model, route-pattern metric labels.
**Disadvantages:** no built-in binding/validation/rendering — we supply those (which we want,
since it keeps handlers explicit and readable by agents).
**Why not gin/echo/fiber:** they introduce their own `Context` type, which leaks a framework
type into every handler signature and makes extraction to another transport harder. Fiber is
built on fasthttp and is *not* `net/http`-compatible — that alone disqualifies it here.
**Why not bare ServeMux:** no route groups, no middleware chaining, no `Mount`; we would
rebuild chi badly.

### 1.2 PostgreSQL access

| Library | Maturity | Community | Maintenance | Prod-ready | Verdict |
|---|---|---|---|---|---|
| **`jackc/pgx` v5** ✅ | 5 | 5 | 5 | 5 | **Chosen (driver)** |
| **`sqlc`** ✅ | 4 | 5 | 5 | 5 | **Chosen (codegen)** |
| `database/sql` + `lib/pq` | 5 | 4 | 2 | 4 | Rejected (pq is in maintenance mode) |
| `jmoiron/sqlx` | 5 | 4 | 3 | 5 | Rejected |
| `go-gorm/gorm` | 5 | 5 | 4 | 4 | Rejected |
| `ent` (Facebook) | 4 | 4 | 4 | 4 | Rejected |
| `uptrace/bun` | 4 | 3 | 4 | 4 | Rejected |
| `sqlboiler` | 4 | 3 | 3 | 4 | Rejected |

**Why pgx + sqlc:** you write real SQL in `.sql` files; sqlc generates typed Go structs and
methods at build time. Wrong column name, wrong type, wrong arity ⇒ **compile error**.
**Advantages:** no N+1 surprises, no hidden queries, `EXPLAIN` works on the exact SQL that
runs, full access to Postgres features (CTEs, window functions, `jsonb`, arrays, partial
indexes), best-in-class performance.
**Disadvantages:** more boilerplate for simple CRUD; a codegen step in the build; dynamic
filtering needs care (we use explicit query variants, not a query builder).
**Why this matters for AI agents specifically:** an ORM DSL is a second language an agent
hallucinates in. SQL is a language every model knows well, and the generated Go is checked by
the compiler. This is the single highest-leverage choice in the stack for AI-assisted work.
**Why not GORM/ent:** hidden queries, magic tags, harder to reason about performance, and
migration/schema drift between the ORM's model and reality.

### 1.3 Migrations

| Library | Maturity | Community | Maintenance | Prod-ready | Verdict |
|---|---|---|---|---|---|
| **`pressly/goose` v3** ✅ | 5 | 4 | 5 | 5 | **Chosen** |
| `golang-migrate/migrate` | 5 | 5 | 3 | 5 | Close second |
| `ariga/atlas` | 4 | 4 | 5 | 4 | Reconsider at scale |
| `amacneil/dbmate` | 4 | 3 | 4 | 4 | Rejected |

**Why goose:** plain `.sql` with `-- +goose Up/Down`, embeddable via `embed.FS`, supports Go
migrations for data backfills, and — decisively — works cleanly with **multiple migration
directories**, which is how we give each module its own migrations (rule L3).
**Disadvantages:** no schema diffing (Atlas has it); ordering across module dirs needs a
convention (we use a global timestamp prefix).
**Why not golang-migrate:** maintenance has been uneven and multi-directory support is awkward.
**Atlas** is genuinely better at declarative schema management — revisit if the schema exceeds
~150 tables.

### 1.4 Background jobs

| Library | Backing store | Maturity | Community | Maintenance | Verdict |
|---|---|---|---|---|---|
| **`riverqueue/river`** ✅ | Postgres | 4 | 4 | 5 | **Chosen** |
| `hibiken/asynq` | Redis | 5 | 5 | 3 | Close second |
| `gocraft/work` | Redis | 4 | 3 | 1 | Rejected (unmaintained) |
| `machinery` | Redis/AMQP | 4 | 3 | 2 | Rejected |
| NATS JetStream | NATS | 5 | 5 | 5 | Deferred (post-extraction) |
| Temporal | own cluster | 5 | 5 | 5 | Overkill for v1 |

**Why River:** jobs are enqueued **inside the business transaction**. A `writing.grade` job can
never exist for a submission that was rolled back, and a submission can never exist without its
grading job. With a Redis queue you must choose between losing jobs and creating orphans, or
build an outbox for jobs too.
**Advantages:** no new infrastructure, transactional enqueue, unique jobs, periodic jobs,
built-in retries with backoff, a web UI, strong typing.
**Disadvantages:** throughput ceiling is Postgres-bound (thousands/s — far above our ~50/s);
adds load to the primary DB; younger project than asynq.
**Migration path:** the `job` platform module exposes an `Enqueuer` interface, so moving to
NATS later is an adapter swap.

### 1.5 Configuration

| Library | Verdict | Notes |
|---|---|---|
| **`knadh/koanf` v2** ✅ | **Chosen** | Layered (defaults → file → env), no global state, small, typed unmarshalling |
| `spf13/viper` | Rejected | Global singleton, heavy dependency tree, historically surprising precedence rules |
| `caarlos0/env` | Considered | Excellent and tiny, but no layering/file support if we later want a config file |
| stdlib `os.Getenv` | Rejected | No validation, no defaults, no documentation surface |

Config is validated at startup; a missing required key **fails fast** with a message naming
the key and the doc section.

### 1.6 Logging

| Library | Verdict | Notes |
|---|---|---|
| **stdlib `log/slog`** ✅ | **Chosen** | Structured, zero dependency, stable since Go 1.21 |
| `go.opentelemetry.io/contrib/bridges/otelslog` ✅ | **Chosen** | Bridges slog → OTLP; adds `trace_id`/`span_id` automatically |
| `uber-go/zap` | Rejected | Faster, but slog is fast enough and is *the* standard now |
| `rs/zerolog` | Rejected | Same reasoning |
| `sirupsen/logrus` | Rejected | Maintenance mode |

Benchmark reality check: at our volume the difference between slog and zap is < 1 % of request
time. Standard-library gravity wins.

### 1.7 Validation

| Library | Verdict | Notes |
|---|---|---|
| **`go-playground/validator` v10** ✅ | **Chosen** | De-facto standard, struct tags, custom rules, translations |
| `ozzo-validation` | Rejected | Programmatic API is nicer for complex rules but far less common; agents know validator |
| Hand-written | Used **in `domain/`** | Invariants belong in the domain, not in tags |

Two-layer approach: struct tags validate *shape* at the transport edge; the domain validates
*rules* (a word cannot enter a deck twice; an exam cannot be submitted after the deadline).

### 1.8 API code generation

| Library | Verdict | Notes |
|---|---|---|
| **`oapi-codegen` v2** ✅ | **Chosen** | Spec-first: generates chi server interfaces, typed clients, request validation middleware |
| **`getkin/kin-openapi`** ✅ | **Chosen** | OpenAPI 3.0/3.1 model and validator in Go, dependency of oapi-codegen generated server |
| `danielgtaylor/huma` v2 | Strong alternative | Code-first, generates the spec — rejected because we want the spec reviewable *before* implementation, and shared with the frontend team |
| `ogen` | Considered | Very fast generated code, stricter spec support; smaller community |
| `swaggo/swag` | Rejected | Comment-driven, OpenAPI 2/3.0 only, drifts easily |

**Consequence of spec-first:** an AI agent's first edit for any API change is
`api/openapi/openapi.yaml`. The generated interface then *forces* the handler to match.
This is the mechanism that stops agents inventing endpoints.

### 1.9 Auth & crypto

| Concern | Chosen | Alternatives | Why |
|---|---|---|---|
| JWT | `golang-jwt/jwt` v5 | `lestrrat-go/jwx`, `o1egl/paseto` | Maintained successor to dgrijalva; safe defaults; jwx is more capable but heavier. PASETO is arguably better-designed — rejected only for ecosystem/tooling familiarity |
| Password hash | `alexedwards/argon2id` | bcrypt, scrypt, raw `x/crypto/argon2` | Argon2id is OWASP's first recommendation; this wrapper encodes params in the hash so rehashing on upgrade is trivial |
| TOTP | `pquerna/otp` | `xlzd/gotp` | Standard, well-tested, QR helper included |
| OAuth client | `golang.org/x/oauth2` | `markbates/goth` | Official; goth adds providers we do not need |
| Secure random / IDs | `google/uuid` (v7) + `oklog/ulid` | `rs/xid`, `segmentio/ksuid` | UUIDv7 is time-ordered (good B-tree locality) and universally understood |

### 1.10 Resilience & rate limiting

| Concern | Chosen | Alternatives | Why |
|---|---|---|---|
| Retry/backoff | `cenkalti/backoff` v5 | `avast/retry-go`, hand-rolled | Context-aware, jitter, tiny |
| Circuit breaker | `sony/gobreaker` v2 | `afex/hystrix-go` (unmaintained), `failsafe-go` | ~300 LOC, obvious semantics, no goroutine surprises |
| Rate limit (distributed) | `go-redis/redis_rate` v10 | `ulule/limiter`, `throttled` | GCRA, matches the Redis we already run |
| Rate limit (in-process) | `golang.org/x/time/rate` | — | Official, for per-provider client-side throttling |
| Single-flight | `golang.org/x/sync/singleflight` | — | Official; cache stampede protection |

### 1.11 Infrastructure clients

| Concern | Chosen | Alternatives | Why |
|---|---|---|---|
| Redis | `redis/go-redis` v9 | `rueidis`, `radix` | Most mature, OTel instrumentation, cluster-ready. `rueidis` is measurably faster with client-side caching — revisit if Redis becomes hot |
| S3 / MinIO | `minio/minio-go` v7 | `aws-sdk-go-v2/s3` | Lighter, MinIO-native, presigned URLs are one call; the AWS SDK is the fallback if we move to real S3 |
| In-process cache | `dgraph-io/ristretto` v2 | `hashicorp/golang-lru`, `patrickmn/go-cache` | Admission policy (TinyLFU) gives a much better hit ratio under scan-heavy loads |

### 1.12 AI provider SDKs

| Provider | SDK | Notes |
|---|---|---|
| Anthropic | `anthropics/anthropic-sdk-go` | Official; streaming, tool use, prompt caching |
| OpenAI | `openai/openai-go` | Official; also used for embeddings, Whisper, TTS |
| Google | `google/generative-ai-go` (or the Vertex SDK) | Official |
| OpenRouter | plain HTTP (OpenAI-compatible) | No SDK needed |
| Local | Ollama HTTP API | Dev/CI only |

**Rule:** these SDK types **never** appear outside `internal/platform/ai/provider/<name>/`.
Business code sees only `ai.Client` and task names.

### 1.13 Observability

| Concern | Chosen |
|---|---|
| SDK | `go.opentelemetry.io/otel` (+ `sdk`, `otlptracegrpc`, `otlpmetricgrpc`, `otlploggrpc`) |
| HTTP instrumentation | `otelhttp` |
| pgx instrumentation | `exaring/otelpgx` |
| Redis instrumentation | `redisotel` (bundled with go-redis) |
| Runtime metrics | `otel/instrumentation/runtime` |
| Prometheus exposition | via Collector (app never exposes `/metrics` directly except for a health scrape) |

### 1.14 Testing

| Concern | Chosen | Alternatives | Why |
|---|---|---|---|
| Assertions | `stretchr/testify` | `is`, `gotest.tools`, stdlib | Ubiquitous; every model knows it |
| Containers | `testcontainers-go` | `ory/dockertest`, docker-compose in CI | Programmatic, waits for readiness, auto-cleanup, works locally and in CI identically |
| Mocks | `matryer/moq` | `mockery`, `gomock` | Generates plain readable Go; diffs are reviewable; no DSL for agents to get wrong |
| Golden files | `sebdah/goldie` v2 | hand-rolled | `-update` flag, good diffs |
| Property-based | `pgregory.net/rapid` | `gopter`, stdlib fuzz | Used for FSRS scheduling and scoring invariants |
| HTTP assertions | stdlib `httptest` | `gavv/httpexpect` | Keep it boring |
| Load | `grafana/k6` | Vegeta, Locust, JMeter | JS scenarios, great thresholds, Grafana-native output |
| Mutation | `go-mutesting` | `gremlins` | Quarterly audit that tests actually assert |

### 1.15 Tooling

| Tool | Purpose |
|---|---|
| `golangci-lint` | Meta-linter (errcheck, govet, staticcheck, revive, gosec, bodyclose, sqlclosecheck, …) |
| `fe3dback/go-arch-lint` | **Enforces module boundaries (rules L1/L2)** — the most important lint we run |
| `govulncheck` | Known vulnerabilities in dependencies and stdlib |
| `gitleaks` | Secret scanning |
| `air` | Hot reload in dev |
| `goimports-reviser` | Import ordering |
| `dbmate`/`pgtyped` | *not used* — sqlc covers it |

---

## 2. Frontend — TypeScript

| Concern | Chosen | Alternatives | Why chosen | Trade-off accepted |
|---|---|---|---|---|
| Framework | React 19 | Vue, Svelte, Solid | Largest talent pool + best AI model familiarity | Heavier than Solid/Svelte |
| Build | Vite 7 | Next.js, Remix, Rspack | Pure SPA; no SSR requirement; fastest HMR | No SSR/SEO (acceptable — app is behind login) |
| Language | TypeScript 5.9 strict | JS | Types from OpenAPI end-to-end | Build step |
| Routing | TanStack Router | React Router 7 | Type-safe params **and** search params; loader integration | Smaller community than React Router |
| Server state | TanStack Query v5 | SWR, RTK Query, Apollo | Best-in-class caching, invalidation, mutations, devtools | Learning curve |
| Client state | Zustand 5 | Redux Toolkit, Jotai, Valtio | Minimal API for the small amount of true global state | Less structure than RTK (fine at this size) |
| UI components | shadcn/ui + Radix + Tailwind 4 | MUI, Mantine, Chakra, Ant | **Components live in our repo** — agents can read and modify them; Radix gives real a11y | We own the maintenance |
| Forms | React Hook Form + Zod | Formik, TanStack Form | Uncontrolled = fast; Zod schemas shared with API types | — |
| Tables | TanStack Table v8 | AG Grid, MRT | Headless; styled by our own design system | More assembly |
| Charts | Recharts | Chart.js, visx, ECharts | Declarative React API; sufficient for progress/skill charts | Less powerful than ECharts |
| Dates | `date-fns` v4 + `@date-fns/tz` | Day.js, Luxon, Temporal polyfill | Tree-shakeable, immutable, good TZ story | — |
| API client | `openapi-typescript` + `openapi-fetch` | Orval, axios + hand types, tRPC | Types generated from the same spec the server implements; 6 KB runtime | Codegen step |
| Audio | `wavesurfer.js` v7 | howler.js, raw Web Audio | Waveform + region UI needed for speaking/listening | Bundle size — lazy-loaded |
| Rich text | `Tiptap` v3 | Slate, Lexical, Quill | Needed for the writing editor + admin authoring; ProseMirror-based, extensible | Bundle size — lazy-loaded |
| i18n | `i18next` + `react-i18next` | FormatJS, Lingui | Namespace lazy-loading; huge ecosystem | Config verbosity |
| Testing | Vitest + Testing Library + MSW v2 | Jest + nock | Shares Vite's transform; MSW handlers generated from OpenAPI | — |
| E2E | Playwright | Cypress, WebdriverIO | Multi-browser, trace viewer, sharding, best CI ergonomics | — |
| Component workshop | Storybook 9 | Ladle, Histoire | a11y + interaction + visual regression addons | Build time |
| Lint/format | ESLint 9 flat + `eslint-plugin-boundaries` + Prettier | Biome | Boundaries plugin enforces feature-slice isolation; Biome is faster but lacks the plugin | Slower than Biome |

---

## 3. Infrastructure

| Concern | Chosen | Version | Alternatives | Why |
|---|---|---|---|---|
| Database | PostgreSQL | 17 | MySQL, MongoDB, CockroachDB | `jsonb`, FTS, partitioning, `pgvector` path, LISTEN/NOTIFY, and River all depend on it |
| Cache | Redis | 7.4 | Valkey, KeyDB, Memcached | Sorted sets (leaderboards), Lua (rate limits), ubiquity. **Valkey** is the licence-safe fork — swap is a config change if licensing matters |
| Object storage | MinIO | latest | SeaweedFS, Garage, S3 | S3 API compatibility means production can move to real S3 with no code change |
| Reverse proxy | Nginx | 1.27 | Caddy, Traefik | Boring, documented, battle-tested. Caddy's automatic TLS is tempting — noted as an option |
| Container runtime | Docker + Compose v2 | — | Podman, K8s | Simplicity; K8s is a Phase-5 decision, not a v1 one |
| Metrics | Prometheus | 3.x | VictoriaMetrics, Mimir | Standard; VictoriaMetrics if cardinality/retention becomes painful |
| Logs | Loki | 3.x | Elasticsearch, OpenSearch | Index-free = cheap; joins to traces by `trace_id` |
| Traces | Tempo | 2.x | Jaeger, Zipkin | Object-storage backed (uses our MinIO); native Grafana integration |
| Dashboards | Grafana | 11.x | — | One pane for metrics + logs + traces |
| Collector | OpenTelemetry Collector (contrib) | — | direct exporters | One ingest point; changing a backend is a config change |
| Dev mail | Mailpit | — | MailHog | Maintained successor, better UI |
| Trace UI (dev only) | Jaeger all-in-one | — | — | Convenience for local debugging; **not run in production** (see plan review §6) |

---

## 4. CI/CD & supply chain

| Concern | Chosen | Notes |
|---|---|---|
| CI | GitHub Actions | Matrix builds, reusable workflows, OIDC to registries |
| Registry | GHCR | Same auth domain as the repo |
| Image build | Docker Buildx + cache mounts | Multi-stage, distroless base for the API |
| Image scan | Trivy | Also generates SBOM |
| SBOM | Syft → CycloneDX | Attached to every release |
| Signing | cosign (keyless, OIDC) | Verifiable provenance |
| Dependency updates | Renovate | Grouped, scheduled, auto-merge for patch-level dev deps only |
| SAST | CodeQL + gosec (via golangci-lint) | |
| Secret scan | gitleaks | Pre-commit hook **and** CI |
| Changelog | git-cliff (Conventional Commits) | Feeds the GitHub release |

---

## 5. Deliberately rejected across the board

| Thing | Why not |
|---|---|
| Kubernetes (v1) | Operational cost far exceeds benefit at 10k DAU on one host. Documented as a Phase-5 option. |
| Kafka | We have no stream-processing requirement. River + outbox covers our eventing. |
| GraphQL | One first-party client. REST + OpenAPI gives better codegen and caching for our shape of data. |
| gRPC (internal, v1) | In-process calls are function calls. gRPC arrives only when a module is extracted. |
| Elasticsearch | Postgres FTS handles our corpus. Revisit above ~1M searchable items or when we need real relevance tuning. |
| A dedicated vector DB | `pgvector` in the existing Postgres is enough for semantic caching and content similarity. |
| Microservices (v1) | See ADR-0001 and the decision matrix in the plan review. |
| A DI framework (wire/fx/dig) | 30 constructors wired explicitly in `cmd/api` is more readable — especially for an AI agent — than generated or reflective wiring. |
| An ORM | See §1.2. |
| Server-side rendering | The app is behind a login; SEO is served by a separate marketing site. |
| Multi-tenancy | Explicit product decision. Adding it later is a large but bounded change; adding it now taxes every query forever. |

---

## 6. Upgrade & review policy

| Class | Policy |
|---|---|
| Security patches | Merged within 48 h |
| Patch versions | Auto-merged weekly if CI is green |
| Minor versions | Reviewed weekly, batched by ecosystem |
| Major versions | Requires an issue, a compatibility note, and a dedicated PR |
| Go / Node | Track the latest stable; upgrade within one minor release |
| Unmaintained dependency (> 12 months, open CVEs) | Triggers a replacement ADR |

This document is reviewed every quarter. Last full review: **2026-08-06**.
