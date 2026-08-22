# AGENTS.md

**→ Read [AGENT.md](AGENT.md). It is the single source of truth for all AI assistants.**

This file exists for tools that look for `AGENTS.md` (OpenAI Codex, Cursor, and others).
Do not duplicate content here.

## Minimum you must know before editing anything

1. This is a Go modular monolith + React SPA. Two roles: `admin`, `user`. Not multi-tenant.
2. Start at `AGENT.md`, then `MODULE_INDEX.md`, then the one module's `AGENT.md`. Do not scan the repo.
3. A module may import only another module's `contract/` package. Never its internals, never its tables.
4. API changes start in `api/openapi/openapi.yaml`. Schema changes start in `db/migrations/<module>/`.
5. Errors use `shared/apperr`; SQL lives in `db/queries/`; prompts live in `docs/prompts/`.
6. Run `make check` before you finish, and update the module's `AGENT.md` and `TODO.md`.
7. Enforce all global and workspace skills defined in `.agents/rules/skills.md` (`golang-patterns`, `typescript-best-practices`, `docker-patterns`, `security-review`, `ui-ux-pro-max`, etc.).
