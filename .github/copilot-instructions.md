# Copilot instructions — Fluentra

Go modular monolith + React SPA. English learning platform. Two roles only: `admin`, `user`.
Not multi-tenant — never add tenant/organization concepts.

Rules:

1. Layering: handler → service → repository → domain. No business logic in handlers, no SQL outside `db/queries/`.
2. A module may import only `internal/modules/<other>/contract`. Never another module's internals or tables.
3. Errors: `shared/apperr`, rendered as RFC 9457 Problem Details with a stable `code`.
4. SQL: sqlc-generated only. No string-built SQL. Every table is owned by one module.
5. Every I/O function takes `ctx context.Context` first.
6. API changes require editing `api/openapi/openapi.yaml` first.
7. LLM calls go through `internal/platform/ai` by task name — never a provider SDK directly.
8. Naming: Go `snake_case.go` files; JSON `snake_case`; SQL plural `snake_case`; React `PascalCase.tsx`.

Full instructions: `AGENT.md` at the repository root.
