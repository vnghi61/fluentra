# Project Skills & Standards Policy

Every AI assistant working in this repository MUST strictly apply both the **Workspace Standards (Fluentra Guidelines)** and the **Global Configured Skills**.

---

## 1. Workspace Standards (Fluentra Native Guidelines)

Always follow the established project architecture and guidelines:

- **Architecture & Boundaries**: Read [AGENT.md](../../AGENT.md) and [MODULE_INDEX.md](../../MODULE_INDEX.md). Modules MUST only communicate via their `contract/` package. Never import another module's internal code or access its tables directly.
- **Single Source of Truth for API**: All API changes start in `api/openapi/openapi.yaml`.
- **Database & Migrations**: Follow [DATABASE_GUIDELINE.md](../../DATABASE_GUIDELINE.md). Migrations use goose in `db/migrations/<module>/`. Queries use `sqlc` in `db/queries/`.
- **Error Handling**: Use `shared/apperr` and follow [ERROR_HANDLING.md](../../ERROR_HANDLING.md).
- **Coding Standard**: Follow [CODING_STANDARD.md](../../CODING_STANDARD.md).
- **Verification**: Always run `make check` (linting, tests, architecture checks) before finishing any task.

---

## 2. Global Skills Integration

Apply the corresponding global skills automatically based on the domain of the task:

### A. Backend & Go Development

- **`golang-patterns`**: Apply idiomatic Go patterns, clean architecture, context propagation, robust concurrency handling, and standard package structure.
- **`api-database-postgresql`**: Apply production PostgreSQL patterns: parameterized queries, safe transaction handling (BEGIN/COMMIT/ROLLBACK), connection pool management, and query optimization.
- **`security-review`**: Enforce strict validation on user inputs, RBAC verification, secure token handling, avoidance of SQL/path injection, and data leakage prevention.
- **`testing-best-practices`**: Design comprehensive test suites (unit tests with table-driven tests, integration tests with testcontainers/database fixtures, mocking only at contract boundaries).

### B. Frontend & UI/UX Development

- **`typescript-best-practices`**: Enforce strong typing, strict null checks, modern ES features, proper type inference, and no `any`.
- **`vercel-react-best-practices`**: Follow modern React 19 + Next.js/Vite performance practices (memoization where necessary, optimal TanStack Query caching, bundle splitting, avoiding unnecessary re-renders).
- **`ui-ux-pro-max` & `frontend-design` & `design-taste-frontend`**:
  - Build rich, polished, responsive UI with modern typography, harmonic palettes, and fluid micro-interactions.
  - Avoid generic AI-generated templates ("AI slop") by adhering to bespoke design systems and UX standards.

### C. DevOps, Containerization & CI/CD

- **`docker-patterns`**: Maintain optimized multi-stage Docker builds, layer caching, non-root users, minimal image sizes, and reproducible Docker Compose configs.
- **`infra-containers-kubernetes`**: Ensure container orchestration, health checks (liveness/readiness probes), and environment variable configurations are production-ready.
- **`ci-cd`**: Maintain fast, secure GitHub Actions workflows with caching, linting, and automated checks.

### D. Senior Engineering Mindset & Communication

- **`ponytail`**: Apply YAGNI (You Aren't Gonna Need It), prefer standard library/native solutions before adding dependencies, and choose minimal, elegant architectures over over-engineering.
- **`caveman`**: When concise communication is requested, provide direct, high-density technical responses without fluff.
