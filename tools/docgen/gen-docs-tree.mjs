#!/usr/bin/env node
/**
 * Generates the docs/ knowledge-base folder READMEs and the ADR set.
 * Run once to scaffold; thereafter edit the files directly (they are hand-maintained
 * from that point on — this script will not overwrite a file that already exists
 * unless --force is passed).
 */
import { writeFileSync, mkdirSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const FORCE = process.argv.includes("--force");

function write(rel, content) {
  const abs = join(ROOT, rel);
  if (existsSync(abs) && !FORCE) { console.log(`skip   ${rel}`); return; }
  mkdirSync(dirname(abs), { recursive: true });
  writeFileSync(abs, content, "utf8");
  console.log(`wrote  ${rel}`);
}

// ---------------------------------------------------------------- docs tree

const folders = [
  {
    dir: "architecture",
    title: "Architecture",
    purpose: "System-wide structure: how the parts fit together, where the boundaries are, and why.",
    contents: [
      "`00-plan-review.md` — the review of the original brief and the optimisations applied (Vietnamese)",
      "`boundaries.md` — the five module boundary rules and how CI enforces them",
      "`microservice-migration.md` — trigger conditions, extraction order and mechanics",
      "`c4/` — Mermaid sources for the context, container and component diagrams",
      "`quality-attributes.md` — the scenarios the architecture is designed to satisfy",
      "`capacity-model.md` — traffic assumptions and where they break",
    ],
    ai: "Read before proposing **any** structural change: a new module, a new dependency arrow, a new datastore, or a change to how modules communicate. If your change contradicts something here, you need an ADR, not a workaround.",
  },
  {
    dir: "backend",
    title: "Backend",
    purpose: "Go conventions beyond what the linters can express.",
    contents: [
      "`layering.md` — what belongs in transport, service, repository, domain and contract",
      "`transactions.md` — where transactions open, why they never span modules",
      "`concurrency.md` — errgroup patterns, goroutine ownership, shutdown",
      "`pagination.md` — the cursor implementation and when offset is allowed",
      "`background-work.md` — job design, idempotency, the outbox",
      "`composition-root.md` — how `cmd/api` wires 30 modules without a DI framework",
    ],
    ai: "Read the specific file for the concern you are touching. Do not read the whole folder — each file is self-contained by design.",
  },
  {
    dir: "frontend",
    title: "Frontend",
    purpose: "React and TypeScript conventions for the SPA.",
    contents: [
      "`structure.md` — feature slices and the import rules between them",
      "`state.md` — the state classification table and why server state never enters a store",
      "`data-fetching.md` — TanStack Query patterns, query keys, invalidation",
      "`forms.md` — React Hook Form + Zod, and mapping Problem Details onto fields",
      "`routing.md` — TanStack Router, typed search params, loaders",
      "`accessibility.md` — the WCAG 2.2 AA baseline and the exercise-specific requirements",
      "`performance.md` — the bundle budget and how it is enforced",
    ],
    ai: "Read `structure.md` and the file for your concern before writing components. Never hand-write an API type — regenerate from the OpenAPI spec.",
  },
  {
    dir: "database",
    title: "Database",
    purpose: "Schema conventions, indexing strategy and the ER diagrams.",
    contents: [
      "`conventions.md` — naming, types, constraints (mirrors `/DATABASE_GUIDELINE.md` with worked examples)",
      "`indexing.md` — how to choose an index, with `EXPLAIN` walkthroughs",
      "`migrations.md` — goose usage, expand→migrate→contract, concurrent index creation",
      "`partitioning.md` — which tables, why, and how partitions are managed",
      "`er/` — one ER diagram per schema",
      "`data-inventory.md` — every table that holds personal data, and its retention",
    ],
    ai: "Read `conventions.md` and `migrations.md` before writing a migration. Read `er/<schema>.md` to understand a schema without opening every migration file.",
  },
  {
    dir: "api",
    title: "API",
    purpose: "HTTP contract standards and the reasoning behind them.",
    contents: [
      "`rest-standards.md` — resources, methods, status codes (mirrors `/API_GUIDELINE.md`)",
      "`versioning.md` — the versioning and deprecation policy",
      "`pagination.md` — the cursor format and client guidance",
      "`errors.md` — the Problem Details contract and the full code catalogue",
      "`idempotency.md` — where keys are required and how replay works",
      "`streaming.md` — SSE conventions for long-running operations",
      "`webhooks.md` — inbound webhook verification and replay",
    ],
    ai: "Read before adding or changing an endpoint. The spec at `api/openapi/openapi.yaml` is the contract; these documents explain the rules that spec must obey.",
  },
  {
    dir: "deployment",
    title: "Deployment",
    purpose: "Running the system, locally and in production.",
    contents: [
      "`configuration.md` — **every** environment variable, its default, and whether it is required",
      "`compose-topology.md` — services, networks, volumes, startup order",
      "`production-checklist.md` — what must be true before a first production deploy",
      "`backup-restore.md` — procedures and drill results",
      "`scaling.md` — the staged scaling path and its triggers",
    ],
    ai: "`configuration.md` is authoritative. If a config key is not there, it does not exist — add it deliberately rather than inventing one.",
  },
  {
    dir: "security",
    title: "Security",
    purpose: "Threat model, controls, and privacy operations.",
    contents: [
      "`threat-model.md` — STRIDE analysis with per-threat controls",
      "`authentication.md` — token design, rotation, MFA, OAuth linking",
      "`authorization.md` — the three enforcement layers and why all three are needed",
      "`data-protection.md` — classification, encryption, retention, erasure",
      "`asvs-mapping.md` — OWASP ASVS L2 checklist and current status",
      "`secrets.md` — where secrets live and how each is rotated",
      "`ai-safety.md` — prompt injection, output validation, moderation, provider terms",
    ],
    ai: "Read before touching authentication, authorization, uploads, payments, or anything handling learner content. Security changes require a second reviewer.",
  },
  {
    dir: "testing",
    title: "Testing",
    purpose: "How we know the system works.",
    contents: [
      "`pyramid.md` — what to test at which level, and why",
      "`testcontainers.md` — container reuse, template databases, isolation",
      "`fixtures.md` — builders, seeds, golden files, factories",
      "`contract-tests.md` — how handlers are checked against the OpenAPI spec",
      "`e2e.md` — the Playwright journeys and the anti-flake policy",
      "`load.md` — k6 scenarios and thresholds",
      "`ai-test-generation.md` — generating tests from specs, and reviewing what comes back",
    ],
    ai: "Generate tests from the module's `AGENT.md` §9 business rules and the OpenAPI spec — **never** from the implementation, or the test inherits the implementation's bugs.",
  },
  {
    dir: "modules",
    title: "Modules",
    purpose: "The module registry and the generator's source of truth.",
    contents: [
      "`manifest.yaml` — the human-readable module manifest",
      "`GENERATED_INDEX.md` — regenerated by `make docs`; do not edit",
      "`boundary-matrix.md` — the full allowed-dependency matrix",
      "Per-module deep dives that do not fit in a module's own `AGENT.md`",
    ],
    ai: "Read the manifest to learn ownership and dependencies quickly. Per-module context lives in `internal/*/<module>/AGENT.md`, not here.",
  },
  {
    dir: "prompts",
    title: "Prompts",
    purpose: "Two libraries: prompts that generate code, and prompts the application sends to LLMs.",
    contents: [
      "`dev/` — development prompts, organised by area (backend, frontend, database, testing, devops, ai, docs)",
      "`runtime/` — versioned production prompt templates with input/output schemas and eval suites",
      "`README.md` — how to choose and use a prompt",
    ],
    ai: "Use a `dev/` prompt verbatim rather than paraphrasing it; the constraints in it are load-bearing. Never inline a `runtime/` prompt into Go code (rule L11).",
  },
  {
    dir: "adr",
    title: "Architecture Decision Records",
    purpose: "Why things are the way they are. Immutable once accepted.",
    contents: [
      "`ADR-NNNN-*.md` — one file per decision",
      "Each records context, decision, at least two rejected alternatives, and consequences including the bad ones",
    ],
    ai: "**Read the relevant ADR before proposing an alternative approach.** If you disagree with a decision, write a superseding ADR — never deviate silently.",
  },
  {
    dir: "decisions",
    title: "Decisions (index)",
    purpose: "Entry point into the ADR set, plus the decisions we have deliberately deferred.",
    contents: [
      "Index of all ADRs by status and topic",
      "The deferred-decision register — questions we have consciously not answered yet, and what triggers answering them",
      "Superseded-decision log",
    ],
    ai: "Check the deferred register before proposing something that sounds new — it may already be a known, dated, deliberate non-decision.",
  },
  {
    dir: "development",
    title: "Development",
    purpose: "How the team works.",
    contents: [
      "`getting-started.md` — from clone to a traced request in 15 minutes",
      "`workflow.md` — branches, commits, PRs, review",
      "`code-review.md` — what reviewers look for, in order",
      "`docs-as-code.md` — front-matter, generated regions, drift checks",
      "`definition-of-done.md` — the shared checklist",
      "`troubleshooting.md` — common local failures and their fixes",
    ],
    ai: "Read `docs-as-code.md` before editing any generated documentation region.",
  },
  {
    dir: "guides",
    title: "Guides",
    purpose: "Task recipes. The fastest path from a task description to correct code.",
    contents: [
      "`add-a-module.md`, `add-an-endpoint.md`, `add-a-table.md`, `add-a-job.md`",
      "`add-an-ai-feature.md`, `add-a-page.md`, `write-an-integration-test.md`",
      "`debug-a-failing-ci-job.md`, `investigate-a-production-issue.md`",
    ],
    ai: "**Start here for any concrete task.** A recipe exists precisely so you do not have to invent a sequence of steps and get one of them subtly wrong.",
  },
  {
    dir: "knowledge",
    title: "Domain knowledge",
    purpose: "The English-teaching knowledge the software encodes. This is where correctness of *learning* lives, as opposed to correctness of code.",
    contents: [
      "`cefr.md` — the CEFR framework, level descriptors, and our rubrics",
      "`fsrs.md` — the spaced repetition algorithm, its parameters and their meaning",
      "`pronunciation-scoring.md` — how phoneme-level assessment works and its limits",
      "`grammar-taxonomy.md` — the structure of our grammar point hierarchy",
      "`exam-formats.md` — IELTS and TOEIC structure, timing and band conversion",
      "`item-writing.md` — how to write a good test item, and the common failure modes",
      "`pedagogy.md` — the learning principles the product is built on",
    ],
    ai: "Read the relevant file **before** designing anything that makes a pedagogical judgement. Getting the code right and the pedagogy wrong produces software that works and does not teach.",
  },
  {
    dir: "ai",
    title: "AI engineering",
    purpose: "Everything about LLMs as a component of the product.",
    contents: [
      "`context/` — the context engineering strategy and its measurements",
      "`playbooks/` — agent playbooks for recurring multi-step work",
      "`evals/` — the evaluation harness, golden sets and thresholds",
      "`cost-model.md` — per-task cost, per-learner unit economics",
      "`provider-comparison.md` — measured quality and cost by provider and task",
      "`safety.md` — injection defence, moderation, output validation",
    ],
    ai: "Read before adding or changing anything that calls a model. `cost-model.md` and `evals/` are what make an AI feature shippable rather than merely demonstrable.",
  },
  {
    dir: "examples",
    title: "Examples",
    purpose: "Canonical reference implementations. The shape everything else should follow.",
    contents: [
      "`reference-module/` — a complete walkthrough of one module, layer by layer",
      "`handler.md`, `service.md`, `repository.md`, `domain.md` — annotated exemplars",
      "`integration-test.md`, `unit-test.md` — annotated test exemplars",
      "`react-feature.md` — a complete frontend feature slice",
    ],
    ai: "**Copy these patterns rather than inventing new ones.** Consistency is worth more than local cleverness in a codebase multiple agents work in.",
  },
  {
    dir: "diagrams",
    title: "Diagrams",
    purpose: "Mermaid sources for every diagram in the documentation.",
    contents: ["`c4/`, `sequence/`, `er/`, `state/`, `flow/`"],
    ai: "Reuse and extend an existing diagram rather than drawing a new one that says almost the same thing.",
  },
  {
    dir: "templates",
    title: "Templates",
    purpose: "Fill-in-the-blank starting points.",
    contents: [
      "`module/` — the nine module documentation templates",
      "`adr.md`, `rfc.md`, `runbook.md`, `test-plan.md`, `postmortem.md`",
      "`pull-request.md`, `issue.md`",
    ],
    ai: "Use verbatim and fill the placeholders. Do not restructure a template — the structure is what makes these documents comparable to each other.",
  },
  {
    dir: "operations",
    title: "Operations",
    purpose: "Running the system when it misbehaves.",
    contents: [
      "`runbooks/` — one per alert: symptom, impact, diagnosis, mitigation, escalation",
      "`slos.md` — the service level objectives and their error budgets",
      "`on-call.md` — rotation, escalation, expectations",
      "`postmortems/` — blameless incident write-ups",
      "`capacity-planning.md` — headroom and the triggers to add more",
    ],
    ai: "Every alert must have a runbook. If you add an alert without one, you have added a page in the middle of the night with no instructions attached.",
  },
  {
    dir: "product",
    title: "Product",
    purpose: "What we are building and for whom.",
    contents: [
      "`personas.md` — who the learners are",
      "`journeys.md` — the end-to-end experiences we are designing for",
      "`features/` — one specification per significant feature",
      "`metrics.md` — every metric's definition, including what it excludes",
      "`entitlements.md` — what each plan grants",
      "`content-strategy.md` — how learning material gets produced, the real bottleneck",
    ],
    ai: "Read before deciding what a feature should *do*. The architecture documents tell you how to build it; these tell you what is worth building.",
  },
];

for (const f of folders) {
  write(
    `docs/${f.dir}/README.md`,
    `---
doc_type: folder_index
folder: docs/${f.dir}
last_verified: 2026-08-06
---

# docs/${f.dir} — ${f.title}

## Purpose

${f.purpose}

## Contents

${f.contents.map((c) => `- ${c}`).join("\n")}

## How AI agents should use this folder

${f.ai}

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
`
  );
}

// ----------------------------------------------------------------- the ADRs

const adrs = [
  {
    n: 1, slug: "modular-monolith", title: "Modular monolith over microservices", tags: "architecture",
    context: "We are a team of 2–6 engineers building an English learning platform with an unstable domain, a single deployment target, and no organisational pressure to split ownership. The system must nevertheless be modifiable in bounded pieces, understandable by AI coding assistants without loading the whole repository, and capable of scaling out later.",
    decision: "Build a single Go application (plus a worker binary) internally partitioned into modules with enforced boundaries. Modules communicate only through explicit `contract` packages and an event bus. Boundaries are enforced by `go-arch-lint` in CI, not by convention.",
    alts: [
      { name: "Microservices from day one", pros: "Independent deploy and scale; hard boundaries; team autonomy", cons: "Distributed transactions, network failure modes, 10× operational surface, slower feature delivery, painful local development", why: "We have none of the problems microservices solve and all of the costs. At 2–6 engineers the coordination overhead alone would dominate." },
      { name: "Unstructured monolith", pros: "Fastest to start; no boundary ceremony", cons: "Boundaries erode; every change risks everything; AI agents cannot scope their reading; extraction later is a rewrite", why: "The erosion is not hypothetical — it is the default outcome, and it is what makes later change expensive." },
      { name: "Serverless functions", pros: "Scale to zero; no server management", cons: "Cold starts on a latency-sensitive path; awkward with long-running AI grading and ffmpeg; vendor lock-in", why: "Our workload has long-running CPU-heavy jobs that fit poorly, and the cost model is worse at steady traffic." },
    ],
    pos: ["One repository, one deploy, one debugger", "In-process calls: no network failure modes between modules", "Boundaries are compiler- and CI-enforced, so they hold under deadline pressure", "AI agents read one module's `AGENT.md` (~4k tokens) instead of scanning the repo (~200k)", "Refactoring across a boundary is a normal refactor, not a cross-team negotiation"],
    neg: ["One deployment unit: a change anywhere redeploys everything", "Scaling is all-or-nothing until a module is extracted", "A memory leak or panic in one module affects the whole process", "Boundary discipline requires active enforcement — without the linter it would decay"],
    compliance: "`.go-arch-lint.yml` declares the allowed dependency graph; CI fails on a violation. `MODULE_INDEX.md` §3 and the linter config must agree, checked by the `dep-drift` job.",
    revisit: "When any trigger in `ARCHITECTURE.md` §20.1 fires: a module needs independent scaling, a different runtime, an isolation boundary for compliance, or more than three teams contend on the same deploy.",
  },
  {
    n: 2, slug: "go-http-stack", title: "chi + stdlib net/http for the HTTP layer", tags: "backend",
    context: "We need routing, middleware, and route groups that map onto module boundaries, with instrumentation that works out of the box and no framework types leaking into handler signatures.",
    decision: "Use `go-chi/chi` v5 on top of the standard library's `net/http`. Handlers are `http.HandlerFunc`; each module exposes a `chi.Router` that `cmd/api` mounts under a prefix.",
    alts: [
      { name: "Gin or Echo", pros: "Batteries included: binding, validation, rendering", cons: "Custom `Context` type in every handler signature; framework lock-in; their binding hides validation we want explicit", why: "The framework `Context` propagates into every layer boundary we touch and makes handlers non-portable." },
      { name: "Fiber", pros: "Very fast benchmarks", cons: "Built on fasthttp, not `net/http`-compatible; loses otelhttp, httptest and the wider middleware ecosystem", why: "Incompatibility with the standard interface costs us more than the benchmark gains, which are irrelevant at our latency budget." },
      { name: "Bare `net/http.ServeMux` (Go 1.22+)", pros: "Zero dependencies; pattern matching is now decent", cons: "No route groups, no middleware chaining, no `Mount`", why: "We would end up writing chi, worse." },
    ],
    pos: ["Every `net/http` tool works unmodified, including `otelhttp` and `httptest`", "`Mount` maps cleanly onto module boundaries", "Route patterns give low-cardinality metric labels for free", "~1k lines of dependency, easily audited"],
    neg: ["We supply binding, validation and rendering ourselves", "Slightly more code per handler than a batteries-included framework"],
    compliance: "A handler whose signature is not `func(http.ResponseWriter, *http.Request)` fails review.",
    revisit: "If the boilerplate of manual binding becomes a measurable drag — at which point `huma` on top of chi is the likely answer, not a different router.",
  },
  {
    n: 3, slug: "sqlc-over-orm", title: "sqlc + pgx instead of an ORM", tags: "data",
    context: "We need type-safe database access that is fast, transparent about the queries it runs, and — importantly for this project — reliably writable by AI coding assistants.",
    decision: "Write SQL in `db/queries/<module>/*.sql`, generate typed Go with `sqlc`, and execute through `pgx` v5.",
    alts: [
      { name: "GORM", pros: "Fast for simple CRUD; large community", cons: "Hidden queries, N+1 by default, struct tags encode behaviour, performance is hard to reason about, migrations drift from the model", why: "Opacity is the problem: you cannot review a query you cannot see, and the failure mode is a production performance cliff." },
      { name: "ent", pros: "Excellent type safety; graph traversal API", cons: "Heavy code generation, its own schema language, steep learning curve, awkward for hand-tuned SQL", why: "Another DSL to learn and, for an AI agent, another language to hallucinate in." },
      { name: "sqlx", pros: "Thin, close to SQL", cons: "Type safety only at runtime; a renamed column fails in production, not at compile time", why: "sqlc gives the same transparency with compile-time checking, for the cost of a codegen step." },
    ],
    pos: ["A wrong column, type or arity is a compile error", "`EXPLAIN` works on exactly the SQL that runs", "Full access to Postgres features: CTEs, window functions, `jsonb`, partial indexes", "AI agents write SQL far more reliably than ORM DSL — this is the highest-leverage choice in the stack for AI-assisted work", "Reviewers can read the query in the diff"],
    neg: ["More boilerplate for trivial CRUD", "A codegen step in the build, which CI must verify is current", "Dynamic filtering needs explicit query variants rather than a builder"],
    compliance: "String-concatenated SQL fails `golangci-lint`. CI runs `make gen` and fails if the generated code differs.",
    revisit: "If a use case genuinely requires runtime-composed queries across many optional filters — at which point a narrowly scoped query builder for that one case, not an ORM everywhere.",
  },
  {
    n: 4, slug: "schema-per-module", title: "One PostgreSQL schema per module", tags: "data",
    context: "Modules must own their data so that boundaries mean something, but running a database per module in v1 would multiply operational cost for no benefit at our scale.",
    decision: "One PostgreSQL instance; one schema per module tier (`core`, `learn`, `skill`, `assess`, `content`, `comm`, `billing`, `ai`, `ops`, `audit`, `analytics`). A module reads and writes only its own tables. Cross-schema foreign keys are forbidden, with one documented exception: `→ core.users(id)`.",
    alts: [
      { name: "One shared schema", pros: "Simplest; joins anywhere", cons: "No ownership signal; any module can couple to any table; extraction later is archaeology", why: "It makes the boundary unenforceable in exactly the place it matters most." },
      { name: "A database per module", pros: "Hard isolation; extraction is trivial", cons: "N connection pools, N backup jobs, N migration pipelines, no cross-module transactions even where they would be legitimate", why: "Operational cost now for a benefit we do not need until extraction, which the schema boundary already prepares for." },
    ],
    pos: ["Ownership is visible in every query", "Extraction means moving a schema, not untangling tables", "Per-schema permissions are possible", "One instance to operate, back up and monitor"],
    neg: ["Discipline is required — Postgres will happily join across schemas", "The `core.users` exception is a real coupling that must be handled at extraction time"],
    compliance: "Migrations live in `db/migrations/<module>/`; a migration touching another module's schema fails review. Cross-schema joins are caught in review and by a query linter.",
    revisit: "At extraction: the extracted module's schema becomes its own database, and the `core.users` foreign key becomes a local `user_id` maintained by events.",
  },
  {
    n: 5, slug: "openapi-spec-first", title: "OpenAPI 3.1, spec-first", tags: "api",
    context: "The HTTP contract is consumed by the Go server, the TypeScript client, the MSW mocks, the contract tests and the documentation. Any of these drifting from the others produces bugs that are only found in integration.",
    decision: "`api/openapi/openapi.yaml` is the single source of truth. Server interfaces are generated with `oapi-codegen`, TypeScript types with `openapi-typescript`, and mocks from the spec's examples. Editing the spec is the first step of any API change (rule L10).",
    alts: [
      { name: "Code-first with huma", pros: "One place to change; spec always matches code", cons: "The spec cannot be reviewed or agreed before implementation; frontend work must wait for backend code", why: "We want the contract reviewable and parallelisable, and we want an AI agent's first edit to be the contract, not the handler." },
      { name: "Comment annotations (swaggo)", pros: "Low ceremony", cons: "OpenAPI 3.0 only, drifts silently, no compile-time link between comment and code", why: "Drift is the exact failure we are trying to eliminate." },
      { name: "No spec", pros: "Nothing to maintain", cons: "Types hand-written twice; mocks hand-written a third time; all three diverge", why: "Not viable with a separate frontend." },
    ],
    pos: ["Frontend and backend can proceed in parallel from an agreed contract", "Generated types make a breaking change a compile error on both sides", "MSW handlers generated from spec examples cannot drift from the API", "An AI agent cannot invent an endpoint — the generated interface will not compile"],
    neg: ["YAML editing is less pleasant than Go", "A codegen step CI must verify", "The spec file grows large and needs splitting into `components/`"],
    compliance: "CI runs `spectral` on the spec, regenerates code and fails on a diff, and runs contract tests asserting handlers match the spec.",
    revisit: "If spec maintenance visibly slows delivery without preventing proportionate bugs.",
  },
  {
    n: 6, slug: "dependency-injection", title: "Manual constructor injection, no DI framework", tags: "backend",
    context: "Thirty modules need wiring. Options range from a reflective container to compile-time code generation to writing it by hand.",
    decision: "Wire everything by hand in `cmd/api/main.go` and `cmd/worker/main.go` using plain constructors. No wire, no fx, no dig, no service locator, no global state.",
    alts: [
      { name: "google/wire", pros: "Compile-time, no reflection", cons: "Generated code is hard to read; a wiring error produces a confusing generator message; another tool in the build", why: "It solves a problem — wiring tedium — that is not actually painful at 30 constructors, and it makes the dependency graph less legible." },
      { name: "uber/fx or dig", pros: "Handles complex graphs and lifecycles", cons: "Runtime reflection; failures at startup rather than compile time; the graph is invisible in the source", why: "The explicit graph in `main.go` is documentation; hiding it behind reflection removes the clearest artefact we have." },
    ],
    pos: ["The entire dependency graph is readable in one file, in order", "A missing dependency is a compile error", "No magic for a newcomer — human or agent — to learn", "Test wiring uses the same constructors with fakes"],
    neg: ["`main.go` grows to several hundred lines", "Adding a module means editing the composition root (which is arguably a feature: it makes the addition visible)"],
    compliance: "A global variable holding a dependency, or an `init()` performing wiring, fails review.",
    revisit: "If `main.go` exceeds roughly 500 lines and becomes genuinely hard to follow — the first remedy is per-tier wiring functions, not a framework.",
  },
  {
    n: 7, slug: "auth-jwt-refresh-rotation", title: "JWT access tokens with rotating refresh tokens", tags: "security",
    context: "We need stateless request authentication that does not require a database round trip per request, combined with a revocation story that works when a token is stolen.",
    decision: "Short-lived JWT access tokens (15 minutes) carried in the `Authorization` header and held only in browser memory. Long-lived opaque refresh tokens (30 days) stored hashed, single-use and rotating, grouped by a `family_id`, delivered in an `HttpOnly; Secure; SameSite=Lax` cookie scoped to `/api/v1/auth`. Reuse of a spent refresh token revokes the entire family and raises a security event.",
    alts: [
      { name: "Server-side sessions only", pros: "Immediate revocation; simplest mental model", cons: "A datastore read on every request; session affinity considerations", why: "Rejected mainly on the per-request read; the 15-minute window plus a denylist gives adequate revocation." },
      { name: "Long-lived JWTs without refresh", pros: "Simplest client", cons: "No practical revocation; a stolen token is valid for its whole lifetime", why: "Unacceptable for an application holding personal learning data." },
      { name: "Non-rotating refresh tokens", pros: "Simpler", cons: "A stolen refresh token is usable indefinitely and undetectably", why: "Rotation with reuse detection is what turns theft from silent into visible." },
    ],
    pos: ["No database read on the authentication path", "Theft of a refresh token is *detectable*, because the legitimate client will eventually replay a spent token", "Revocation window bounded at 15 minutes; explicit logout is immediate via the denylist", "Access token never touches `localStorage`, so XSS cannot exfiltrate it"],
    neg: ["More moving parts than plain sessions", "A revoked user can act for up to 15 minutes unless the denylist is consulted", "Refresh races need single-flight handling in the client", "Key rotation requires supporting two active signing keys"],
    compliance: "Integration tests assert family revocation on reuse, timing equalisation on login, and that access tokens are never persisted client-side.",
    revisit: "When the system is split into services — at that point `HS256` becomes `EdDSA` with a JWKS endpoint.",
  },
  {
    n: 8, slug: "rbac-simple-policy", title: "Table-driven permissions, no Casbin or OPA", tags: "security",
    context: "The product has exactly two roles and roughly forty named permissions. Authorization must be deny-by-default and enforced at the service layer as well as the route.",
    decision: "Roles, permissions and their mapping live in four database tables. A `rbac.Require(ctx, permission)` guard is called in service methods. Ownership filtering happens in each module's queries. No policy engine, no policy language.",
    alts: [
      { name: "Casbin", pros: "Mature; supports RBAC, ABAC, resource scoping", cons: "A policy DSL to learn and debug; model file becomes a second source of truth; overkill for two roles", why: "The complexity is not repaid until we need resource-scoped or hierarchical policies." },
      { name: "OPA / Rego", pros: "Extremely expressive; policy as code", cons: "A separate language and evaluation model; latency or an embedded engine; a large conceptual jump for the team", why: "Same reasoning, more so." },
      { name: "Hard-coded role checks", pros: "Trivial", cons: "`if role == \"admin\"` scattered everywhere; adding a role means finding every check", why: "Named permissions cost almost nothing now and make a future role a data change." },
    ],
    pos: ["Readable and debuggable with a SQL query", "Named permissions mean a third role would be data, not code", "No new language for humans or agents", "Fast, cached, no external dependency"],
    neg: ["No resource-scoped permissions (\"can edit *these* lessons\")", "No time-bounded grants", "We own the evaluation logic, including its tests"],
    compliance: "CI checks that every non-public OpenAPI operation declares `x-permission` and that the handler enforces it. A permission written as a string literal at a call site fails review.",
    revisit: "If resource-scoped permissions become a requirement — content ownership per author is the likely trigger. That is when Casbin earns its keep.",
  },
  {
    n: 9, slug: "event-bus-in-process", title: "In-process event bus with a transactional outbox", tags: "architecture",
    context: "Modules must react to each other's events without calling each other synchronously, and an event must never be lost or emitted for a transaction that rolled back.",
    decision: "An in-process publish/subscribe bus, fed by a transactional outbox. Publishers write the event row inside the business transaction; a publisher loop polls unpublished rows with `FOR UPDATE SKIP LOCKED` and dispatches after commit. Consumers are idempotent because delivery is at-least-once.",
    alts: [
      { name: "Direct synchronous calls between modules", pros: "Simple; immediately consistent", cons: "Tight coupling; a slow or failing consumer breaks the publisher; fan-out becomes a distributed transaction problem", why: "Notification failing must not fail essay grading." },
      { name: "A message broker (NATS/Kafka) now", pros: "Real durability and decoupling; ready for extraction", cons: "New infrastructure; still needs an outbox to be transactional; operational burden today for a benefit at extraction time", why: "The outbox is the part that makes it correct, and we can have that without the broker." },
      { name: "Publishing after commit, without an outbox", pros: "No extra table", cons: "A crash between commit and publish loses the event silently", why: "Silent loss is the worst failure mode available." },
    ],
    pos: ["No event is emitted for rolled-back data, and no committed data lacks its event", "No new infrastructure", "The bus interface is broker-shaped, so swapping in NATS later is an adapter change", "Consumers are already idempotent, which is a prerequisite for any broker"],
    neg: ["Polling introduces a small publish latency (sub-second in practice)", "In-process delivery means a slow consumer occupies a worker", "Ordering is per-aggregate, not global", "Outbox rows add write volume to the primary database"],
    compliance: "Publishing outside a transaction fails review. Every consumer has a test asserting idempotency under duplicate delivery.",
    revisit: "When the first module is extracted, or when outbox lag becomes a real constraint.",
  },
  {
    n: 10, slug: "job-queue-river", title: "River (Postgres-backed) for background jobs", tags: "backend",
    context: "AI grading, media processing, emails and scheduled work must run outside the request path, reliably, with retries — and must never exist for data that was rolled back.",
    decision: "Use `riverqueue/river`, which stores jobs in PostgreSQL, so a job can be enqueued inside the business transaction. Five queues by workload shape: `ai`, `media`, `notify`, `batch`, `default`.",
    alts: [
      { name: "Asynq (Redis)", pros: "Mature, fast, good tooling", cons: "Enqueue cannot participate in the database transaction — you must choose between orphan jobs and lost jobs, or build an outbox for jobs too", why: "The transactional property is the whole point." },
      { name: "NATS JetStream", pros: "High throughput; ready for a distributed future", cons: "New infrastructure; same transactional gap", why: "Deferred until extraction." },
      { name: "Temporal", pros: "Excellent for long multi-step workflows", cons: "A cluster to operate; a large programming-model shift", why: "Our workflows are short and simple; the cost is not repaid." },
    ],
    pos: ["Transactional enqueue: no orphaned jobs, no lost work", "No additional infrastructure", "Unique jobs, periodic jobs, retries and a web UI included", "Job state is queryable with SQL, which makes operational tooling trivial"],
    neg: ["Throughput bounded by Postgres (thousands/second — far above our need)", "Adds write load to the primary database", "A younger project than the Redis alternatives"],
    compliance: "Enqueueing after commit fails review. Every job handler has a test that runs it twice and asserts one effect.",
    revisit: "If sustained job throughput approaches the low thousands per second, or when a module is extracted and needs cross-service work distribution.",
  },
  {
    n: 11, slug: "ai-provider-abstraction", title: "Task-based AI provider abstraction", tags: "ai",
    context: "LLM providers change pricing, deprecate models and suffer outages. Business code must not encode any of that, and cost and quality must be governable in one place.",
    decision: "Business code calls `ai.Client.Run(ctx, TaskRequest{Task: \"writing.grade_essay\", ...})`. The platform module resolves the task to a model tier and a concrete model from configuration, renders the pinned prompt version, enforces quota and budget, caches, retries, falls back across providers, validates the output schema and records usage. Provider SDKs may be imported only inside `provider/<name>/`.",
    alts: [
      { name: "Call provider SDKs directly", pros: "Least indirection", cons: "Vendor lock-in in every feature; no central cost control, caching, fallback or usage tracking; swapping a model is a code change everywhere", why: "It puts the most volatile external dependency in the most places." },
      { name: "A thin `Complete(prompt)` wrapper", pros: "Simple", cons: "Callers still choose the model and own the prompt; no routing policy, no per-task budgets or cache policies", why: "It abstracts the wrong thing — the transport rather than the decision." },
      { name: "LangChain-style framework", pros: "Many building blocks", cons: "Large surface, opinionated abstractions, Go support is weaker than Python", why: "We need six well-understood tasks, not a general orchestration framework." },
    ],
    pos: ["Changing a model is a configuration change plus an eval run", "Cost, quota, caching, fallback and usage tracking exist in exactly one place", "Tests use a `mock` provider — deterministic, offline, free", "A provider outage degrades rather than fails"],
    neg: ["Indirection between the caller and the model", "The routing configuration becomes an important artefact in its own right", "Provider-specific capabilities must be surfaced deliberately or they are unavailable"],
    compliance: "An import of a provider SDK outside `internal/platform/ai/provider/<name>/` fails `go-arch-lint`. A prompt string literal in Go fails review.",
    revisit: "If a provider-specific capability becomes essential and cannot be expressed through the task abstraction.",
  },
  {
    n: 12, slug: "prompt-versioning", title: "Prompts as versioned, evaluated artefacts", tags: "ai",
    context: "Prompts determine the quality of grading and feedback — the product's core value. A prompt change is as consequential as a code change and, unlike code, its effects are statistical rather than deterministic.",
    decision: "Runtime prompts live in `docs/prompts/runtime/<task>/v<N>.md` with YAML front-matter and JSON input/output schemas. A published version is immutable; a change creates `vN+1`. Configuration pins the active version per environment. A version cannot become active until its eval suite meets its thresholds and is no worse than the current active version. Rollout is shadow → 10 % → 100 %.",
    alts: [
      { name: "Prompts as Go string constants", pros: "Type-safe, co-located with the code", cons: "Not reviewable by non-engineers, not rollback-able without a deploy, not independently evaluable", why: "It treats a statistical artefact as if it were deterministic code." },
      { name: "Prompts in a database, editable in an admin UI", pros: "Change without a deploy", cons: "No review, no version control, no CI evaluation — the fastest available path to a silent quality regression", why: "Speed of change is not the constraint; confidence in change is." },
      { name: "A third-party prompt management service", pros: "Purpose-built tooling", cons: "Another vendor in a critical path; our prompts leave the repository", why: "The repository already gives us versioning and review; we need the evaluation, which we build ourselves." },
    ],
    pos: ["A past grade can always be explained by the exact prompt that produced it", "Rollback is a configuration change", "Quality regressions are caught in CI rather than by learners", "Prompts are reviewable by the learning team, not only by engineers"],
    neg: ["Building and maintaining golden sets is real ongoing work", "Version proliferation over time", "Evaluation runs cost money and time"],
    compliance: "`ai-eval.yml` runs the suite on every change under `docs/prompts/runtime/` and blocks a merge on regression. Promotion to `active` requires a human approval.",
    revisit: "If eval maintenance cost exceeds the regressions it prevents — measured, not assumed.",
  },
  {
    n: 13, slug: "observability-otel", title: "OpenTelemetry SDK with a Collector; Tempo over Jaeger", tags: "observability",
    context: "We need traces, metrics and logs, correlated, from Phase 0. The original brief listed both Jaeger and Tempo as trace backends.",
    decision: "The application emits only OTLP to an OpenTelemetry Collector. The Collector routes to Prometheus, Loki and Tempo, and performs tail sampling and PII redaction. Jaeger runs only in the local development profile; production uses Tempo.",
    alts: [
      { name: "Direct exporters from the application", pros: "One fewer component", cons: "Backend changes, sampling and redaction all become code changes and deploys", why: "The Collector converts those into configuration." },
      { name: "Run both Jaeger and Tempo in production", pros: "Two UIs", cons: "Two trace stores to operate and pay for, storing the same data; roughly 700 MB additional memory; no team benefit", why: "Redundant. Tempo shares MinIO for storage and integrates natively with Grafana." },
      { name: "A commercial APM", pros: "Least operational work; excellent UX", cons: "Per-host or per-GB pricing at a scale we cannot predict; vendor lock-in on instrumentation", why: "OTLP keeps this reversible — we can point the Collector at a vendor later without touching code." },
    ],
    pos: ["Backend changes are Collector configuration", "One ingest point for redaction and sampling policy", "Metrics, logs and traces correlate in one Grafana instance", "A commercial APM remains a configuration change away"],
    neg: ["The Collector is another component to run and monitor", "Tail sampling costs memory in the Collector", "Grafana's trace UI is less polished than Jaeger's for some workflows — hence Jaeger in dev"],
    compliance: "A direct exporter in application code fails review. Cardinality is checked in review against the 10,000-series budget.",
    revisit: "If self-hosting the Grafana stack costs more engineering time than a commercial APM would cost in licence fees.",
  },
  {
    n: 14, slug: "frontend-stack", title: "React + Vite + TanStack + shadcn/ui", tags: "frontend",
    context: "A single-page application behind a login, with rich interactive exercises, audio, a rich-text editor, and an admin back office. No SEO requirement.",
    decision: "React 19 with TypeScript on Vite; TanStack Router and TanStack Query; Zustand for the small amount of true global state; shadcn/ui on Radix and Tailwind; React Hook Form with Zod; Vitest, Testing Library, MSW and Playwright.",
    alts: [
      { name: "Next.js", pros: "SSR, routing, image optimisation, large ecosystem", cons: "SSR complexity we do not need behind a login; a Node server to operate alongside the Go API; framework opinions we would fight", why: "We are building an application, not a content site." },
      { name: "A component library (MUI, Mantine, Ant)", pros: "Comprehensive; fast initially", cons: "Theming fights; components are in `node_modules` where neither we nor an agent can read or adapt them; heavy bundles", why: "shadcn/ui puts the components in our repository, which matters enormously for AI-assisted work — an agent can read and modify the button." },
      { name: "Svelte or SolidJS", pros: "Smaller bundles, better raw performance", cons: "Smaller talent pool; less training data, so AI assistance is measurably weaker", why: "Model familiarity is a real factor in a project designed around AI-assisted development." },
    ],
    pos: ["Fast development loop; no server-side rendering complexity", "End-to-end type safety from the OpenAPI spec", "Design-system components live in the repository and are editable", "TanStack Query removes an entire category of client-state bugs", "Excellent AI assistance quality for React and TypeScript"],
    neg: ["No SEO (mitigated: marketing lives on a separate site)", "We maintain the design system components ourselves", "TanStack Router is newer and less widely known than React Router"],
    compliance: "`eslint-plugin-boundaries` enforces feature-slice isolation. Hand-written API types fail review. The bundle budget is enforced in CI.",
    revisit: "If a public, SEO-relevant surface becomes part of this application rather than the marketing site.",
  },
  {
    n: 15, slug: "content-exercise-core", title: "Shared content model and exercise engine for all skill modules", tags: "architecture",
    context: "The original plan had six skill modules — vocabulary, grammar, reading, listening, speaking, writing — each owning its own items, attempts and progress. Inspection showed roughly 70 % of that would be identical.",
    decision: "Extract two shared pieces. `content` owns items, immutable versions, media and taxonomy. `learning` owns the exercise engine: attempt lifecycle, idempotent submission, grader dispatch, scoring, progress rollup and event emission. Each skill module implements only `learning.ExerciseGrader` plus its genuinely skill-specific data.",
    alts: [
      { name: "Six independent skill modules", pros: "Complete independence; no shared abstraction to get wrong", cons: "~18 near-identical tables; six copies of the attempt lifecycle; six places to fix the same idempotency bug; mixed-skill lessons become very hard", why: "The duplication is not incidental — it is the majority of the code, and copies diverge." },
      { name: "One monolithic learning module", pros: "No cross-module calls at all", cons: "A single package containing every skill's logic; unclear ownership; a change to speaking risks reading", why: "It trades one kind of duplication for a loss of boundaries." },
    ],
    pos: ["Estimated 40 % less backend code and 55 % fewer tables across the six skills", "Attempt lifecycle correctness is implemented and tested once", "Mixed-skill lessons are natural", "A seventh skill is a grader, not a module rebuild"],
    neg: ["The shared abstractions must be right; getting `ExerciseGrader` wrong would be expensive to change later", "A skill with a genuinely unusual shape may strain the model", "`learning` becomes a large and important module"],
    compliance: "A skill module that defines its own attempt table fails review. The grader registry is validated at startup.",
    revisit: "If a skill genuinely cannot express itself through `ExerciseGrader` — the response would be to widen the interface deliberately, not to fork the engine.",
  },
  {
    n: 16, slug: "srs-fsrs", title: "FSRS instead of SM-2 for spaced repetition", tags: "learning",
    context: "Vocabulary and grammar retention is the core learning mechanism. The scheduling algorithm determines how much of a learner's time produces durable memory.",
    decision: "Implement FSRS (Free Spaced Repetition Scheduler) with four grades, in pure functions in `srs/domain/`, with globally tuned parameters initially and per-learner optimisation later.",
    alts: [
      { name: "SM-2 (the Anki classic)", pros: "Simple; well documented; easy to implement", cons: "Ease-factor model does not represent memory well; over-reviews easy material and under-reviews hard material", why: "Published comparisons consistently show FSRS reaching equivalent retention with materially fewer reviews. That difference is learner time." },
      { name: "A fixed Leitner box schedule", pros: "Trivial to implement and explain", cons: "Ignores individual item difficulty and learner history entirely", why: "Adequate for a hobby app, not for a product whose value proposition is efficient learning." },
      { name: "Build our own model", pros: "Tailored to our data", cons: "A research project requiring data we do not yet have", why: "FSRS is published, validated and free; our review logs will let us tune it later." },
    ],
    pos: ["Explicit stability and difficulty modelling per item and learner", "Roughly 20–30 % fewer reviews for equivalent retention", "Published parameters give a good starting point without our own data", "Pure functions make it provably correct with property-based tests"],
    neg: ["More complex than SM-2; the parameters are not independent knobs", "Per-learner optimisation needs several hundred reviews per learner", "Team must understand the model before changing it — hence `docs/knowledge/fsrs.md`"],
    compliance: "Scheduling logic performing I/O fails review. Property-based tests assert the algorithm's invariants. Parameter changes require simulation against historical logs.",
    revisit: "If a materially better published algorithm appears, or if our own review data supports a custom model.",
  },
  {
    n: 17, slug: "error-problem-details", title: "RFC 9457 Problem Details for all API errors", tags: "api",
    context: "Errors cross four boundaries: domain to service, service to transport, API to client, client to user. Without one shape, each boundary invents its own.",
    decision: "One `shared/apperr.Error` type with a kind, a stable machine `code`, a safe message, optional field errors and metadata, and non-exposed cause and internal detail. Rendered as `application/problem+json` per RFC 9457. Clients branch on `code`, never on message text.",
    alts: [
      { name: "A custom error envelope", pros: "Exactly our shape", cons: "No tooling understands it; every client integration re-learns it", why: "A standard costs nothing here and buys interoperability." },
      { name: "Plain HTTP status codes with a text body", pros: "Simplest", cons: "A client cannot distinguish two different 409s; localisation is impossible", why: "Insufficient for a UI that must respond differently to different conflicts." },
      { name: "GraphQL-style errors in a 200 response", pros: "Uniform transport", cons: "Breaks HTTP caching, monitoring and every intermediary's error handling", why: "Fights the protocol." },
    ],
    pos: ["Clients branch on stable codes, so messages can change and be localised freely", "Internal details cannot leak — the type separates exposed from internal fields", "One rendering path means one place to get security right", "Field-level validation errors map directly onto form fields"],
    neg: ["Every error needs a code, which is a small ongoing discipline", "Codes are public API and cannot be changed once released", "The catalogue must be maintained in `ERROR_HANDLING.md`"],
    compliance: "Returning an unclassified error to HTTP produces a generic 500 and an error log — visible in monitoring. Contract tests assert the error shape for each documented failure.",
    revisit: "Unlikely. This is a standard doing exactly what standards are for.",
  },
  {
    n: 18, slug: "media-presigned-upload", title: "Presigned direct-to-storage uploads", tags: "architecture",
    context: "Learners upload voice recordings and avatars. Recordings can be several megabytes and arrive in bursts during practice sessions.",
    decision: "The API never handles file bytes. It issues a presigned PUT URL with a pinned content type, maximum size and five-minute expiry; the browser uploads directly to MinIO; the API then verifies the object before accepting the reference.",
    alts: [
      { name: "Proxy uploads through the API", pros: "Full control; simpler client", cons: "API memory and bandwidth scale with file size and concurrency; a burst of uploads degrades every other request", why: "It couples the scalability of the whole API to the largest thing a user can send." },
      { name: "Base64 in a JSON body", pros: "One request", cons: "33 % size inflation; large request bodies; no resumability", why: "Strictly worse in every dimension." },
    ],
    pos: ["API memory stays flat regardless of upload volume", "Uploads scale with the object store, not the application", "A CDN can serve downloads directly", "Resumable and multipart uploads become possible without API changes"],
    neg: ["More client-side steps (intent, upload, confirm)", "Post-upload verification is mandatory — the client cannot be trusted about what it uploaded", "Presigned URLs are sensitive to clock skew"],
    compliance: "Streaming file bytes through a handler fails review. Every upload path has a test asserting post-upload verification rejects a mismatched object.",
    revisit: "Only if we move to a storage backend without presigning support, which would itself be a questionable choice.",
  },
  {
    n: 19, slug: "testing-strategy", title: "Testcontainers over mocked infrastructure", tags: "testing",
    context: "Repository code is mostly SQL. The bugs that reach production in data-access code are SQL bugs — wrong join, missing filter, wrong index assumption — none of which a mocked database can detect.",
    decision: "Unit-test domain and service layers with mocked *ports* (repository and contract interfaces). Integration-test repositories against real PostgreSQL, Redis and MinIO started by `testcontainers-go`, with a container per package and a template-database clone per test.",
    alts: [
      { name: "Mock the database", pros: "Fast; no Docker", cons: "Proves the mock works, not the SQL; every real SQL bug survives", why: "It tests the wrong thing precisely where the risk is." },
      { name: "A shared CI database", pros: "Fast startup", cons: "Test pollution; ordering dependencies; cannot run in parallel; different behaviour locally and in CI", why: "Flakiness and environment divergence." },
      { name: "SQLite in tests", pros: "Fast, embedded", cons: "Different SQL dialect, no `jsonb`, no partial indexes, no partitioning — we would test a database we do not run", why: "The differences are exactly where our queries live." },
    ],
    pos: ["SQL correctness is actually verified", "Identical behaviour locally and in CI", "Redis degradation and MinIO presigning are exercised for real", "Migrations are tested on every integration run"],
    neg: ["Slower than mocks (minutes, not seconds) — hence the build tag and a separate `make test-int`", "Docker required for the full suite", "Container startup adds CI time, mitigated by per-package reuse"],
    compliance: "Integration tests carry `//go:build integration`. A repository with no integration test fails review.",
    revisit: "If integration suite runtime exceeds roughly ten minutes and becomes a delivery drag — the remedy is better parallelism and template databases, not mocks.",
  },
  {
    n: 20, slug: "agent-md-convention", title: "AGENT.md per module as the unit of AI context", tags: "ai",
    context: "This project is built with AI coding assistants. Their most common failure modes are reading too much context, reading the wrong context, and inventing things that do not exist. All three are addressable by how the repository is organised.",
    decision: "Every module carries an `AGENT.md` with fourteen fixed sections in a fixed order, plus machine-readable YAML front-matter. A root `AGENT.md` is the single entry point; `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` and the Copilot instructions are thin pointers to it. Drift is caught by CI.",
    alts: [
      { name: "One large root instruction file", pros: "Single place to maintain", cons: "Either too long to load usefully or too shallow to answer module-specific questions", why: "Context budget is the constraint; a 40k-token file defeats the purpose." },
      { name: "Rely on code comments and good naming", pros: "No extra artefacts", cons: "Cannot express business rules, ownership, boundaries or 'do not do this'; an agent still has to read everything to find out", why: "Comments describe code; agents need to know what *not* to read." },
      { name: "A vector-indexed documentation search", pros: "Scales to any size", cons: "Retrieval is probabilistic; the agent cannot know what it missed; another system to run", why: "Deterministic, named files beat probabilistic retrieval when the file set is small enough to enumerate — and ours is." },
    ],
    pos: ["An agent reads roughly 10k tokens instead of 150k+ to start work correctly", "Fixed section order makes the documents skimmable and machine-checkable", "The §14 'Do NOT' section prevents the specific mistakes agents actually make", "Front-matter lets CI verify documentation against reality", "Human onboarding benefits identically"],
    neg: ["Thirty documents to keep current — mitigated by generation and drift checks", "Discipline required: an out-of-date `AGENT.md` is worse than none", "Some duplication between `AGENT.md` and code"],
    compliance: "`docs.yml` validates front-matter, required sections, table drift against migrations, dependency drift against `.go-arch-lint.yml`, and flags `last_verified` older than 90 days.",
    revisit: "Measured quarterly: files read before the first edit, rework rate on AI-authored PRs, median tokens per completed task. If the numbers do not improve, the format is wrong and should change.",
  },
];

for (const a of adrs) {
  const num = String(a.n).padStart(4, "0");
  write(
    `docs/adr/ADR-${num}-${a.slug}.md`,
    `---
adr: ${num}
title: "${a.title}"
status: Accepted
date: 2026-08-06
tags: [${a.tags}]
---

# ADR-${num}: ${a.title}

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | ${a.tags} |

## Context

${a.context}

## Decision

${a.decision}

## Alternatives considered

${a.alts
  .map(
    (alt, i) => `### ${String.fromCharCode(65 + i)}. ${alt.name}

| | |
|---|---|
| **Pros** | ${alt.pros} |
| **Cons** | ${alt.cons} |
| **Why rejected** | ${alt.why} |`
  )
  .join("\n\n")}

## Consequences

### Positive

${a.pos.map((p) => `- ${p}`).join("\n")}

### Negative — accepted knowingly

${a.neg.map((p) => `- ${p}`).join("\n")}

## Compliance

${a.compliance}

## Revisit when

${a.revisit}

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
`
  );
}

console.log("\ndone.");
