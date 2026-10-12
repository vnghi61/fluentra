# CLAUDE.md

**→ Read [AGENT.md](AGENT.md). It is the single source of truth for all AI assistants.**

This file exists only because Claude Code looks for it. Do not duplicate content here.

## Claude-specific notes

| Situation | Guidance |
|---|---|
| Design, boundary, or security question | Use extended thinking before answering |
| Broad read-only exploration | Subagents are fine for *finding* things; do the writing in the main thread so the boundary rules stay in context |
| Editing files | Prefer `Edit` over rewriting whole files — smaller diffs review better |
| Long tasks | Follow the four-step loop in `docs/AI.md#working-with-and-building-on-llms` (§A3): specify → generate → verify → record |
| Before finishing | Run `make check` and update the module's `AGENT.md` (§18 is its backlog) |

## The three rules most often broken

1. Do not read other modules' internals — only their `contract/` package.
2. Do not write a handler before updating `api/openapi/openapi.yaml`.
3. Do not invent an endpoint, table, config key, or metric. If it is not in the spec, a
   migration, or `.env.example`, it does not exist yet.
