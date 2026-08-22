# GEMINI.md

**→ Read [AGENT.md](AGENT.md). It is the single source of truth for all AI assistants.**

This file exists only because the Gemini CLI looks for it. Do not duplicate content here.

## Gemini-specific note

A large context window makes it tempting to load the whole repository. Don't. The context
pyramid in [AI_CONTEXT.md](AI_CONTEXT.md) exists because *relevant* context produces better
work than *complete* context — the same reason a human engineer reads one module rather than
the whole codebase before making a change.

Read: `AGENT.md` → `MODULE_INDEX.md` → the one module's `AGENT.md` → the recipe in
`docs/guides/` → the 2–4 source files the module doc points at. That is the whole procedure.
