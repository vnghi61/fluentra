#!/usr/bin/env node
/**
 * Fluentra module documentation generator.
 *
 * Reads the module manifest (tools/docgen/data/*.json) and writes one AGENT.md
 * for every module under internal/modules/<name>/ or internal/platform/<name>/.
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
function merge(existingPath, rendered, m) {
  if (!existsSync(existingPath)) return rendered;
  let current = readFileSync(existingPath, "utf8");

  const curFmMatch = /^---\n([\s\S]*?)\n---\n/.exec(current);
  const renFmMatch = /^---\n([\s\S]*?)\n---\n/.exec(rendered);
  if (curFmMatch && renFmMatch) {
    let renFm = renFmMatch[1];
    const curLastVerified = curFmMatch[1].match(/last_verified:\s*(.+)/);
    if (curLastVerified) {
      renFm = renFm.replace(/last_verified:\s*.+/, `last_verified: ${curLastVerified[1].trim()}`);
    }
    current = current.replace(/^---\n[\s\S]*?\n---\n/, `---\n${renFm}\n---\n`);
  }

  if (m && m.status) {
    current = current.replace(/\|\s*Status\s*\|\s*\*\*[^*]+\*\*\s*\|/, `| Status | **${m.status}** |`);
  }

  const re = /<!-- BEGIN GENERATED: ([\w.-]+) -->[\s\S]*?<!-- END GENERATED: \1 -->/g;
  const fresh = new Map();
  let match;
  while ((match = re.exec(rendered)) !== null) fresh.set(match[1], match[0]);
  let touched = false;
  const out = current.replace(re, (whole, id) => {
    if (fresh.has(id)) { touched = true; return fresh.get(id); }
    return whole;
  });
  return touched ? out : current;
}

/**
 * Templates that interpolate optional blocks leave runs of blank lines behind.
 * Markdown renders those identically, but markdownlint reports every one, and a
 * generator whose output fails the repository's own lint means `make docs` and
 * `make docs-check` cannot both pass. Normalising here keeps the two honest.
 */
function tidy(markdown) {
  return markdown.replace(/\n{3,}/g, "\n\n").replace(/\s+$/, "\n");
}

let changed = 0;
function emit(relPath, content, m) {
  const abs = join(ROOT, relPath);
  mkdirSync(dirname(abs), { recursive: true });
  const final = tidy(merge(abs, content, m));
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
(tag: \`${m.name}\`). Per-endpoint detail is in §15.

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

Global rules: [\`docs/CONVENTIONS.md\`](../../../docs/CONVENTIONS.md#coding-standard). Deviations and additions
for this module:

${generated("conventions", bullets(m.conventions, "_No deviations from the global standard._"))}

${m.cache ? `### Cache strategy\n\n${table(["Key", "TTL", "Invalidated by"], m.cache.map((c) => [`\`${c.key}\``, c.ttl, c.invalidation]))}\n` : ""}
${m.errors ? `### Error codes owned by this module\n\n${table(["Code", "Status", "Meaning"], m.errors.map((e) => [`\`${e.code}\``, e.status, e.meaning]))}\n` : ""}
${m.security ? `### Security considerations\n\n${bullets(m.security)}\n` : ""}
## 13. Testing

${generated("testing", `Coverage target: **${m.coverage ?? "80% service, 90% domain"}**

\`\`\`bash
go test ./${modPath(m)}/...                    # unit
go test -tags=integration ./${modPath(m)}/...  # integration (testcontainers)
\`\`\`

**Focus areas**

${bullets(m.testFocus)}`)}

## 14. Do NOT

${generated("donot", bullets(m.doNot, "_No module-specific prohibitions beyond the global rules._"))}

## 15. API detail

### Endpoint detail

${generated("api-detail", apiDetail(m))}

### Error codes

${generated("api-errors", table(["Code", "Status", "Meaning"], (m.errors ?? []).map((e) => [`\`${e.code}\``, e.status, e.meaning])))}

### Rate limits

${generated("api-rate", bullets(m.rateLimits, "Standard limits apply — see [/docs/CONVENTIONS.md](../../../docs/CONVENTIONS.md#rate-limiting)."))}

## 16. Flows

${generated("flows", flows(m) || "_No flows documented yet. Add one before implementing a non-trivial use case — the diagram is cheaper to review than the code._")}

${generated("states", states(m) || "_This module has no explicit state machine._")}

### Failure paths

${generated("failures", table(["Failure", "Detected by", "Behaviour"], (m.failures ?? []).map((f) => [f.failure, f.detection, f.behaviour])))}

## 17. Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [\`/DECISIONS.md\`](../../../DECISIONS.md).

### Decisions taken

${generated("decisions", table(["Question", "Decision", "Rationale"], (m.decisions ?? []).map((d) => [d.question, d.choice, d.why])))}

### Related repository ADRs

${generated("decisions-adr", bullets((m.adrs ?? []).map((a) => `[${a.id}](../../../docs/adr/${a.file}) — ${a.title}`), "_None specific to this module._"))}

### Open questions

${generated("decisions-open", bullets(m.openQuestions, "_None._"))}

## 18. Backlog

${generated("todo", todoBody(m))}

### Deferred (deliberately not doing yet)

${generated("todo-deferred", bullets(m.deferred, "_Nothing deferred._"))}

### Future improvements

${generated("todo-future", bullets(m.future, "_None recorded._"))}

---

_Generated by \`tools/docgen\` from \`tools/docgen/data/\`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run \`make docs\`._
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

function apiDetail(m) {
  const eps = m.endpoints ?? [];
  if (!eps.length) return "_This module exposes no HTTP endpoints. It is consumed through its `contract` package._";
  return eps
    .map(
      (e) => `#### \`${e.method} ${e.path}\`

${e.purpose}

| | |
|---|---|
| Permission | \`${e.permission}\` |
| Success | ${e.success ?? "200"} |
| Errors | ${(e.errors ?? []).map((c) => `\`${c}\``).join(", ") || "standard set"} |
${e.notes ? `| Notes | ${e.notes} |` : ""}
`
    )
    .join("\n");
}

const flows = (m) =>
  (m.flows ?? [])
    .map((f) => `### ${f.title}\n\n${f.description ?? ""}\n\n\`\`\`mermaid\n${f.mermaid}\n\`\`\`\n`)
    .join("\n");

const states = (m) =>
  m.stateMachine
    ? `### State machine\n\n${m.stateMachine.description ?? ""}\n\n\`\`\`mermaid\n${m.stateMachine.mermaid}\n\`\`\`\n`
    : "";

function todoBody(m) {
  const groups = m.todo ?? [];
  return groups.length
    ? groups.map((g) => `### ${g.title}\n\n${checkboxes(g.items)}`).join("\n\n")
    : "### Backlog\n\n_Empty. Add items with acceptance criteria as they are identified._";
}

// -------------------------------------------------------------------- write

for (const m of modules) {
  const dir = modPath(m);
  emit(`${dir}/AGENT.md`, agentMd(m), m);
}

if (CHECK && changed > 0) {
  console.error(`\ndocgen: ${changed} file(s) out of date. Run \`make docs\`.`);
  process.exit(1);
}
console.log(`\ndocgen: ${modules.length} modules, ${changed} file(s) written.`);
