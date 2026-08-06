#!/usr/bin/env node
/**
 * Fluentra module documentation generator.
 *
 * Reads the module manifest (tools/docgen/data/*.json) and writes the nine-file
 * documentation set for every module under internal/modules/<name>/ or
 * internal/platform/<name>/.
 *
 * Hand-written prose is preserved: anything outside the
 *   <!-- BEGIN GENERATED: <id> --> … <!-- END GENERATED: <id> -->
 * markers in an existing file is kept when regenerating.
 *
 * Usage:  node tools/docgen/generate.mjs [--check]
 *         --check exits non-zero if regenerating would change any file (used in CI).
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync, readdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..", "..");
const DATA_DIR = join(__dirname, "data");
const CHECK = process.argv.includes("--check");

// ---------------------------------------------------------------- load data

const modules = readdirSync(DATA_DIR)
  .filter((f) => f.endsWith(".json"))
  .flatMap((f) => JSON.parse(readFileSync(join(DATA_DIR, f), "utf8")));

modules.sort((a, b) => a.name.localeCompare(b.name));
const byName = new Map(modules.map((m) => [m.name, m]));

// ------------------------------------------------------------------ helpers

const S = (v, fallback = "_Not yet defined._") =>
  v === undefined || v === null || (Array.isArray(v) && v.length === 0) ? fallback : v;

const bullets = (items, fallback = "_None yet._") =>
  items && items.length ? items.map((i) => `- ${i}`).join("\n") : fallback;

const numbered = (items, prefix = "", fallback = "_None yet._") =>
  items && items.length
    ? items.map((i, n) => `${prefix}${n + 1}. ${i}`).join("\n")
    : fallback;

const checkboxes = (items, fallback = "_None yet._") =>
  items && items.length ? items.map((i) => `- [ ] ${i}`).join("\n") : fallback;

const table = (headers, rows, fallback = "_None yet._") => {
  if (!rows || rows.length === 0) return fallback;
  const head = `| ${headers.join(" | ")} |`;
  const sep = `|${headers.map(() => "---").join("|")}|`;
  const body = rows.map((r) => `| ${r.join(" | ")} |`).join("\n");
  return [head, sep, body].join("\n");
};

const modPath = (m) => `internal/${m.group}/${m.name}`;
const link = (name) => {
  const m = byName.get(name);
  return m ? `[\`${name}\`](../../${m.group === "platform" ? "platform" : "modules"}/${name}/AGENT.md)` : `\`${name}\``;
};

const generated = (id, body) =>
  `<!-- BEGIN GENERATED: ${id} -->\n${body}\n<!-- END GENERATED: ${id} -->`;

/** Merge newly generated blocks into an existing file, preserving hand-written prose. */
function merge(existingPath, rendered) {
  if (!existsSync(existingPath)) return rendered;
  let current = readFileSync(existingPath, "utf8");
  const re = /<!-- BEGIN GENERATED: ([\w.-]+) -->[\s\S]*?<!-- END GENERATED: \1 -->/g;
  const fresh = new Map();
  let m;
  while ((m = re.exec(rendered)) !== null) fresh.set(m[1], m[0]);
  let touched = false;
  const out = current.replace(re, (whole, id) => {
    if (fresh.has(id)) { touched = true; return fresh.get(id); }
    return whole;
  });
  return touched ? out : rendered;
}

let changed = 0;
function emit(relPath, content) {
  const abs = join(ROOT, relPath);
  mkdirSync(dirname(abs), { recursive: true });
  const final = merge(abs, content);
  const prev = existsSync(abs) ? readFileSync(abs, "utf8") : null;
  if (prev === final) return;
  changed++;
  if (CHECK) { console.error(`would change: ${relPath}`); return; }
  writeFileSync(abs, final, "utf8");
  console.log(`wrote ${relPath}`);
}

// ---------------------------------------------------------------- templates

function frontMatter(m) {
  return [
    "---",
    `module: ${m.name}`,
    `tier: ${m.tier}`,
    `group: ${m.group}`,
    `status: ${m.status}`,
    `phase: ${m.phase}`,
    `owner: "${m.owner}"`,
    `schema: ${m.schema ?? "none"}`,
    `tables: [${(m.tables ?? []).map((t) => t.name).join(", ")}]`,
    `depends_on: [${(m.dependsOn ?? []).join(", ")}]`,
    `depended_on_by: [${(m.dependedOnBy ?? []).join(", ")}]`,
    `spec_version: 1.0.0`,
    `last_verified: 2026-08-06`,
    "---",
  ].join("\n");
}

function agentMd(m) {
  const tablesTable = table(
    ["Table", "Purpose", "Key columns / notes"],
    (m.tables ?? []).map((t) => [`\`${m.schema}.${t.name}\``, t.purpose, t.notes ?? "—"])
  );
  const endpointsTable = table(
    ["Method", "Path", "Permission", "Purpose"],
    (m.endpoints ?? []).map((e) => [`\`${e.method}\``, `\`${e.path}\``, `\`${e.permission}\``, e.purpose])
  );
  const foldersTable = table(
    ["Path", "Contains"],
    (m.folders ?? defaultFolders(m)).map((f) => [`\`${f.path}\``, f.contains])
  );
  const relatedTable = table(
    ["Module", "Direction", "Why"],
    [
      ...(m.dependsOn ?? []).map((d) => [link(d), "→ depends on", (m.dependsWhy ?? {})[d] ?? "see its contract"]),
      ...(m.dependedOnBy ?? []).map((d) => [link(d), "← used by", (m.usedByWhy ?? {})[d] ?? "consumes this module's contract"]),
    ]
  );
  const eventsTable = table(
    ["Event", "Direction", "Payload summary"],
    [
      ...((m.events?.publishes) ?? []).map((e) => [`\`${e.name}\``, "publishes", e.payload]),
      ...((m.events?.consumes) ?? []).map((e) => [`\`${e.name}\``, "consumes", e.reason]),
    ]
  );
  const tasksBody = (m.tasks ?? []).length
    ? m.tasks.map((t) => `### ${t.title}\n\n${numbered(t.steps)}`).join("\n\n")
    : "_No recipes recorded yet. Add one the first time you do something twice._";

  return `${frontMatter(m)}

# ${m.name} — AGENT.md

> AI entry point for this module. Read [\`/AGENT.md\`](../../../AGENT.md) and
> [\`/MODULE_INDEX.md\`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | \`${m.tier}\` |
| Path | \`${modPath(m)}\` |
| Schema | \`${m.schema ?? "none"}\` |
| Delivery phase | ${m.phase} |
| Status | **${m.status}** |
| Owner | ${m.owner} |

---

## 1. Overview

${generated("overview", m.purpose)}

${m.context ? `**Context.** ${m.context}\n` : ""}
## 2. Responsibilities

${generated("responsibilities", `**This module owns:**\n\n${bullets(m.responsibilities)}\n\n**This module does NOT own:**\n\n${bullets(m.notResponsibilities)}`)}

## 3. Entry points

${generated("entrypoints", table(["File", "Read it when"], (m.entryPoints ?? defaultEntryPoints(m)).map((e) => [`\`${e.file}\``, e.when])))}

## 4. Public API (contract)

Other modules may import **only** \`${modPath(m)}/contract\`.

${generated("contract", `${table(["Kind", "Name", "Purpose"], (m.contract ?? []).map((c) => [c.kind, `\`${c.name}\``, c.purpose]))}

### Events

${eventsTable}`)}

## 5. Database schema

${generated("schema", m.schema ? `All tables live in the \`${m.schema}\` schema and are owned exclusively by this module (rule DB1).\nMigrations: \`db/migrations/${m.name}/\` · Queries: \`db/queries/${m.name}/\`\n\n${tablesTable}\n\n${m.indexes ? `**Indexes of note**\n\n${bullets(m.indexes)}` : ""}` : "This module owns no tables.")}

## 6. HTTP endpoints

Full definitions are in [\`api/openapi/openapi.yaml\`](../../../api/openapi/openapi.yaml)
(tag: \`${m.name}\`). See also [\`API.md\`](API.md).

${generated("endpoints", endpointsTable)}

## 7. Folder map

${generated("folders", foldersTable)}

## 8. Related modules

${generated("related", relatedTable)}

**Boundary reminder:** you may call these through their \`contract\` package only.
Reaching into \`service/\`, \`repository/\`, \`domain/\` or their tables violates rules L1/L2
and fails \`go-arch-lint\` in CI.

## 9. Business rules

${generated("rules", numbered((m.rules ?? []).map((r) => `**BR-${m.name.toUpperCase()}-${String((m.rules ?? []).indexOf(r) + 1).padStart(2, "0")}** — ${r}`)))}

${m.validation ? `### Validation rules\n\n${table(["Field / input", "Rule", "Error code"], m.validation.map((v) => [`\`${v.field}\``, v.rule, `\`${v.code}\``]))}\n` : ""}
## 10. Common tasks

${generated("tasks", tasksBody)}

## 11. Known limitations

${generated("limitations", bullets(m.limitations, "_None recorded. Add one the moment you take a shortcut._"))}

## 12. Coding conventions (module-specific)

Global rules: [\`/CODING_STANDARD.md\`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

${generated("conventions", bullets(m.conventions, "_No deviations from the global standard._"))}

${m.cache ? `### Cache strategy\n\n${table(["Key", "TTL", "Invalidated by"], m.cache.map((c) => [`\`${c.key}\``, c.ttl, c.invalidation]))}\n` : ""}
${m.errors ? `### Error codes owned by this module\n\n${table(["Code", "Status", "Meaning"], m.errors.map((e) => [`\`${e.code}\``, e.status, e.meaning]))}\n` : ""}
${m.security ? `### Security considerations\n\n${bullets(m.security)}\n` : ""}
## 13. Testing

See [\`TESTING.md\`](TESTING.md) for the full plan.

${generated("testing", `Coverage target: **${m.coverage ?? "80% service, 90% domain"}**

\`\`\`bash
go test ./${modPath(m)}/...                    # unit
go test -tags=integration ./${modPath(m)}/...  # integration (testcontainers)
\`\`\`

**Focus areas**

${bullets(m.testFocus)}`)}

## 14. Do NOT

${generated("donot", bullets(m.doNot, "_No module-specific prohibitions beyond the global rules._"))}

---

*Generated by \`tools/docgen\` from \`tools/docgen/data/\`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run \`make docs\`.*
`;
}

function defaultFolders(m) {
  const base = [
    { path: "contract/", contains: "Interfaces, DTOs and event types other modules may import — the only public package" },
    { path: "domain/", contains: "Entities, value objects, invariants, domain errors. Pure Go, no I/O" },
    { path: "service/", contains: "Use cases, orchestration, transactions, event publishing" },
    { path: "repository/", contains: "sqlc-generated queries and row↔domain mappers" },
    { path: "transport/http/", contains: "Handlers, request/response DTOs, route registration" },
    { path: "module.go", contains: "`New(deps)` — wiring; the only symbol `cmd/` imports" },
  ];
  if (m.hasJobs) base.splice(5, 0, { path: "job/", contains: "Background job handlers owned by this module" });
  return base;
}

function defaultEntryPoints(m) {
  return [
    { file: `${modPath(m)}/module.go`, when: "You need to see what this module depends on and what it exposes" },
    { file: `${modPath(m)}/contract/`, when: "You are calling this module from another module" },
    { file: `${modPath(m)}/service/`, when: "You are changing behaviour" },
    { file: `db/migrations/${m.name}/`, when: "You need the real schema" },
  ];
}

function readmeMd(m) {
  return `${frontMatter(m)}

# ${m.name}

${m.purpose}

> **AI assistants: read [\`AGENT.md\`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

${generated("purpose", m.businessPurpose ?? m.purpose)}

## Responsibilities

${generated("readme-resp", bullets(m.responsibilities))}

## Where things are

${generated("readme-folders", table(["Path", "Contains"], (m.folders ?? defaultFolders(m)).map((f) => [`\`${f.path}\``, f.contains])))}

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**${m.status}** — planned for delivery phase ${m.phase}. See [/ROADMAP.md](../../../ROADMAP.md).
`;
}

function apiMd(m) {
  const eps = m.endpoints ?? [];
  const detail = eps.length
    ? eps
        .map(
          (e) => `### \`${e.method} ${e.path}\`

${e.purpose}

| | |
|---|---|
| Permission | \`${e.permission}\` |
| Success | ${e.success ?? "200"} |
| Errors | ${(e.errors ?? []).map((c) => `\`${c}\``).join(", ") || "standard set"} |
${e.notes ? `| Notes | ${e.notes} |` : ""}
`
        )
        .join("\n")
    : "_This module exposes no HTTP endpoints. It is consumed through its `contract` package._";

  return `${frontMatter(m)}

# ${m.name} — API Reference

> The **contract** is [\`api/openapi/openapi.yaml\`](../../../api/openapi/openapi.yaml), tag \`${m.name}\`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's \`api-drift\` check will fail on the discrepancy.

Conventions: [\`/API_GUIDELINE.md\`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [\`/ERROR_HANDLING.md\`](../../../ERROR_HANDLING.md).

## Endpoint summary

${generated("api-summary", table(["Method", "Path", "Permission", "Purpose"], eps.map((e) => [`\`${e.method}\``, `\`${e.path}\``, `\`${e.permission}\``, e.purpose])))}

## Endpoint detail

${generated("api-detail", detail)}

## Error codes

${generated("api-errors", table(["Code", "Status", "Meaning"], (m.errors ?? []).map((e) => [`\`${e.code}\``, e.status, e.meaning])))}

## Rate limits

${generated("api-rate", bullets(m.rateLimits, "Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11."))}
`;
}

function flowMd(m) {
  const flows = (m.flows ?? [])
    .map((f) => `## ${f.title}\n\n${f.description ?? ""}\n\n\`\`\`mermaid\n${f.mermaid}\n\`\`\`\n`)
    .join("\n");
  const states = m.stateMachine
    ? `## State machine\n\n${m.stateMachine.description ?? ""}\n\n\`\`\`mermaid\n${m.stateMachine.mermaid}\n\`\`\`\n`
    : "";
  return `${frontMatter(m)}

# ${m.name} — Flows

Sequence diagrams, state machines and business processes owned by this module.

${generated("flows", flows || "_No flows documented yet. Add one before implementing a non-trivial use case — the diagram is cheaper to review than the code._")}

${generated("states", states || "_This module has no explicit state machine._")}

## Failure paths

${generated("failures", table(["Failure", "Detected by", "Behaviour"], (m.failures ?? []).map((f) => [f.failure, f.detection, f.behaviour])))}
`;
}

function testingMd(m) {
  return `${frontMatter(m)}

# ${m.name} — Testing

Global policy: [\`/TESTING_GUIDELINE.md\`](../../../TESTING_GUIDELINE.md).

## Targets

| Layer | Target |
|---|---|
| \`domain/\` | 90 % |
| \`service/\` | 80 % |
| \`repository/\` | every query exercised by an integration test |
| \`transport/http/\` | every endpoint has a contract test |

## What to test

${generated("test-focus", bullets(m.testFocus))}

## Edge cases that have bitten similar modules

${generated("test-edges", bullets(m.testEdges, "_Add them here as you find them. This list is the module's institutional memory._"))}

## Fixtures and test data

${generated("test-fixtures", bullets(m.fixtures, `- Builders in \`test/fixtures/builders/${m.name}.go\`\n- Golden files in \`${modPath(m)}/testdata/\``))}

## Mocks

${generated("test-mocks", (m.dependsOn ?? []).length
    ? `Generated by \`moq\` into \`${modPath(m)}/mocks/\`:\n\n${bullets((m.dependsOn ?? []).map((d) => `\`${d}.Service\` — the contract of ${link(d)}`))}\n\nInfrastructure (Postgres, Redis, MinIO) is **not** mocked; integration tests use testcontainers.`
    : "This module has no outbound dependencies to mock.")}

## Running

\`\`\`bash
go test ./${modPath(m)}/...
go test -tags=integration ./${modPath(m)}/...
go test -run TestXxx -race -v ./${modPath(m)}/...
\`\`\`

## AI-assisted test generation

Use \`docs/prompts/dev/testing/generate-unit-test.md\`. Give the agent this module's
\`AGENT.md\` §9 (business rules) as the oracle — **never the implementation**, or the test will
inherit the implementation's bugs.
`;
}

function decisionsMd(m) {
  const rows = (m.decisions ?? []).map((d) => [d.question, d.choice, d.why]);
  return `${frontMatter(m)}

# ${m.name} — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [\`/DECISIONS.md\`](../../../DECISIONS.md).

## Decisions taken

${generated("decisions", table(["Question", "Decision", "Rationale"], rows))}

## Related repository ADRs

${generated("decisions-adr", bullets((m.adrs ?? []).map((a) => `[${a.id}](../../../docs/adr/${a.file}) — ${a.title}`), "_None specific to this module._"))}

## Open questions

${generated("decisions-open", bullets(m.openQuestions, "_None._"))}
`;
}

function promptsMd(m) {
  return `${frontMatter(m)}

# ${m.name} — Prompts

Two kinds. Do not confuse them — see [\`/PROMPT_LIBRARY.md\`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

${generated("prompts-dev", table(["Task", "Prompt"], (m.devPrompts ?? []).map((p) => [p.task, `\`docs/prompts/dev/${p.file}\``]),
  "Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet."))}

### Context to give the agent

\`\`\`
Read /AGENT.md, then internal/${m.group}/${m.name}/AGENT.md.
Work only inside internal/${m.group}/${m.name}/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
\`\`\`

## 2. Runtime prompts — LLM calls this module makes

${generated("prompts-runtime", (m.runtimePrompts ?? []).length
    ? table(["AI task", "Prompt directory", "Purpose"], m.runtimePrompts.map((p) => [`\`${p.task}\``, `\`docs/prompts/runtime/${p.task}/\``, p.purpose]))
    : "**This module makes no LLM calls.** If you think it should, check whether an algorithm solves the problem better — see [/AI_GUIDE.md](../../../AI_GUIDE.md) §B2.")}

${(m.runtimePrompts ?? []).length ? `### Rules

- Never inline a prompt string in Go code (rule L11).
- Call \`platform/ai\` by **task name**; never name a model.
- A prompt change is a new \`vN+1\` file, gated by its eval suite.
- Learner text goes inside the untrusted-content wrapper, always.
` : ""}`;
}

function todoMd(m) {
  const groups = m.todo ?? [];
  const body = groups.length
    ? groups.map((g) => `## ${g.title}\n\n${checkboxes(g.items)}`).join("\n\n")
    : "## Backlog\n\n_Empty. Add items with acceptance criteria as they are identified._";
  return `${frontMatter(m)}

# ${m.name} — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

${generated("todo", body)}

## Deferred (deliberately not doing yet)

${generated("todo-deferred", bullets(m.deferred, "_Nothing deferred._"))}

## Future improvements

${generated("todo-future", bullets(m.future, "_None recorded._"))}
`;
}

function readmeAiMd(m) {
  return `# README_AI.md — ${m.name}

**→ [AGENT.md](AGENT.md)**

This file exists only for tools that look for \`README_AI.md\`. All AI context for the
\`${m.name}\` module lives in [\`AGENT.md\`](AGENT.md); duplicating it here would guarantee the
two drift apart.
`;
}

// -------------------------------------------------------------------- write

for (const m of modules) {
  const dir = modPath(m);
  emit(`${dir}/AGENT.md`, agentMd(m));
  emit(`${dir}/README.md`, readmeMd(m));
  emit(`${dir}/API.md`, apiMd(m));
  emit(`${dir}/FLOW.md`, flowMd(m));
  emit(`${dir}/TESTING.md`, testingMd(m));
  emit(`${dir}/DECISIONS.md`, decisionsMd(m));
  emit(`${dir}/PROMPTS.md`, promptsMd(m));
  emit(`${dir}/TODO.md`, todoMd(m));
  emit(`${dir}/README_AI.md`, readmeAiMd(m));
}

// generated index of modules, for docs/modules/
const indexRows = modules.map((m) => [
  `[\`${m.name}\`](../../${modPath(m)}/AGENT.md)`,
  m.tier,
  String(m.phase),
  m.status,
  m.schema ?? "—",
  (m.dependsOn ?? []).join(", ") || "—",
]);
emit(
  "docs/modules/GENERATED_INDEX.md",
  `# Generated module index\n\n_Do not edit — regenerated by \`make docs\`. Curated index: [/MODULE_INDEX.md](../../MODULE_INDEX.md)._\n\n${table(
    ["Module", "Tier", "Phase", "Status", "Schema", "Depends on"],
    indexRows
  )}\n\nTotal: **${modules.length}** modules.\n`
);

if (CHECK && changed > 0) {
  console.error(`\ndocgen: ${changed} file(s) out of date. Run \`make docs\`.`);
  process.exit(1);
}
console.log(`\ndocgen: ${modules.length} modules, ${changed} file(s) written.`);
