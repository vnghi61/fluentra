---
doc_type: folder_index
folder: docs/prompts
last_verified: 2026-08-06
---

# docs/prompts — The Prompt Library

Two libraries. They look similar and are governed completely differently.

| | `dev/` | `runtime/` |
|---|---|---|
| Who runs it | A person, in an AI coding assistant | The application, at request time |
| Purpose | Generate code, tests, docs | Grade essays, explain grammar, generate items |
| Failure mode | A bad pull request — caught in review | A wrong grade shown to a learner — often not caught at all |
| Versioning | Git history | Immutable `vN` files, pinned by config |
| Gate to change | Normal review | Eval suite thresholds + human approval |
| Rollback | Revert a commit | Change one config value |

Design rationale: [`/PROMPT_LIBRARY.md`](../../PROMPT_LIBRARY.md) ·
[ADR-0012](../adr/ADR-0012-prompt-versioning.md)

---

## Contents

- `dev/` — development prompts by area: `backend/`, `frontend/`, `database/`, `testing/`,
  `devops/`, `ai/`, `docs/`, plus `_shared/` (context header, definition of done, review checklist)
- `runtime/` — versioned production prompt templates with input/output schemas and eval suites

> **Where the runtime templates actually live: `internal/platform/ai/prompts/`.**
>
> Two mechanics moved them out of this folder. `//go:embed` cannot reach outside
> its own package directory, so the embed must sit beside the templates; and
> `.go-arch-lint.yml` sets `workdir: internal`, so a package under `docs/` is
> outside every component it can declare and importing one fails the boundary
> linter with no way to grant it.
>
> Everything else in this document still governs them: `<task>.v<N>.md`,
> immutable once live, a new version rather than an edit in place, pinned by
> configuration. Rule L11's intent — versioned template files, reviewable on
> their own, no prompt string in Go — is met in full.

---

## Using a development prompt

1. Find it in [`/PROMPTS.md`](../../PROMPTS.md) §2.
2. Fill in the `inputs` from its front-matter.
3. Give it to your assistant **verbatim**. Do not paraphrase — the constraints are load-bearing,
   and the ones that look like boilerplate are usually the ones preventing a specific past mistake.
4. Check the output against the prompt's own acceptance criteria.
5. Run `make check`.

If a prompt produces a poor result, **improve the prompt** rather than working around it once.
That fix is permanent and helps everyone; a workaround helps you once.

## Structure of a development prompt

```
---
id: backend/generate-service
title: Generate a service layer method
version: 1.0.0
owner: "@backend-team"
inputs: [module, use_case, contract_method]
reads: [AGENT.md, internal/modules/{{module}}/AGENT.md, CODING_STANDARD.md, ERROR_HANDLING.md]
produces: [service/{{use_case}}.go, service/{{use_case}}_test.go]
model_hint: mid-tier
---

## Context       ← always starts with _shared/context-header.md
## Task          ← imperative, unambiguous, single-purpose
## Constraints   ← numbered, checkable, referencing repo rules by ID (L1, L4, …)
## Steps         ← the order matters; spec before code
## Output        ← exactly which files, in which order
## Acceptance    ← a checklist the output must satisfy
## Anti-patterns ← the specific mistakes agents make on this task
```

## Working with a runtime prompt

**Never edit a published version.** Copy `vN` to `vN+1`, edit that, and let the eval suite decide
whether it is better. A learner's grade from last month must remain explainable by the prompt
that produced it.

```
runtime/<task>/
├── v1.md  v2.md  v3.md      ← immutable once published
├── input.schema.json
├── output.schema.json
└── evals/
    ├── golden/*.json        ← human-labelled examples, including adversarial ones
    └── thresholds.yaml      ← the bar a new version must clear
```

Promotion path: `draft` → eval passes → human approval → `shadow` → 10 % → `active`.

## The two rules that matter most

1. **No prompt string literal in Go code** (rule L11). If it is going to a model, it lives here.
2. **Learner text goes inside the untrusted-content wrapper**, never concatenated into the
   instruction block. `_shared/user-content-wrapper.md` is the only sanctioned way to include it.

## How AI agents should use this folder

- Prefer an existing `dev/` prompt over inventing a sequence of steps.
- When writing code that calls a model, reference a **task name**; the prompt is resolved by
  `platform/ai` from configuration, not by you.
- When asked to change grading behaviour, the change is almost always a new runtime prompt
  version plus an eval run — not Go code.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
