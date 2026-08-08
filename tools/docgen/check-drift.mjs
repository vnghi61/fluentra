// Documentation drift checks (P0.16 / P0.R12).
//
// The rule this file enforces: a fact that exists in two places must agree in
// both. Every check below compares documentation against something executable —
// a migration, an OpenAPI path, an arch-lint rule — never against another
// document's prose. A check with no executable side is a spell-checker.
//
// Run: node tools/docgen/check-drift.mjs [--staleness]
//   --staleness  also report AGENT.md files whose last_verified is over 90 days
//                old. Off by default so the gate does not fail with the passage
//                of time on a branch nobody touched.

import fs from 'fs';
import path from 'path';

const ROOT = process.cwd();
const REPORT_STALENESS = process.argv.includes('--staleness');

const problems = [];
const notes = [];

function fail(check, message) {
  problems.push(`${check}: ${message}`);
}

// ---------------------------------------------------------------- front matter

/**
 * Minimal front-matter reader. Deliberately not a YAML parser: the front-matter
 * in this repository is flat scalars and inline `[a, b]` lists, and adding a
 * dependency to a checker that must run before `npm install` would be a
 * bootstrap problem.
 */
function frontMatter(content) {
  if (!content.startsWith('---')) return null;
  const end = content.indexOf('\n---', 3);
  if (end === -1) return null;

  const fields = {};
  for (const line of content.slice(3, end).split('\n')) {
    const match = line.match(/^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$/);
    if (!match) continue;
    const [, key] = match;
    let value = match[2].trim();
    if (value.startsWith('[') && value.endsWith(']')) {
      value = value
        .slice(1, -1)
        .split(',')
        .map((item) => item.trim().replace(/^["']|["']$/g, ''))
        .filter(Boolean);
    } else {
      value = value.replace(/^["']|["']$/g, '');
    }
    fields[key] = value;
  }
  return fields;
}

function walk(dir, filename, found = []) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return found;
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === '.git') continue;
      walk(full, filename, found);
    } else if (entry.name === filename) {
      found.push(full);
    }
  }
  return found;
}

const rel = (p) => path.relative(ROOT, p).split(path.sep).join('/');

// ------------------------------------------------------- module AGENT.md files

// Only per-module documents carry the module schema. The root AGENT.md, web/ and
// docs/ have their own shapes.
const moduleAgentFiles = walk(path.join(ROOT, 'internal'), 'AGENT.md');

const REQUIRED_FIELDS = {
  module: 'string',
  tier: 'string',
  status: 'string',
  phase: 'string',
  owner: 'string',
  tables: 'array',
  depends_on: 'array',
  spec_version: 'string',
  last_verified: 'date',
};

const REQUIRED_SECTIONS = [
  '1. Overview',
  '2. Responsibilities',
  '3. Entry points',
  '4. Public API (contract)',
  '5. Database schema',
  '6. HTTP endpoints',
  '7. Folder map',
  '8. Related modules',
  '9. Business rules',
  '10. Common tasks',
  '11. Known limitations',
  '12. Coding conventions (module-specific)',
  '13. Testing',
  '14. Do NOT',
];

/** module name -> { fields, tier } for every module document that parsed. */
const modules = new Map();

console.log('==> 1. AGENT.md front-matter schema and required sections');
for (const file of moduleAgentFiles) {
  const where = rel(file);
  const content = fs.readFileSync(file, 'utf8');
  const fields = frontMatter(content);

  if (!fields) {
    fail('front-matter', `${where} has no front-matter block`);
    continue;
  }

  for (const [key, kind] of Object.entries(REQUIRED_FIELDS)) {
    const value = fields[key];
    if (value === undefined) {
      fail('front-matter', `${where} is missing \`${key}\``);
      continue;
    }
    if (kind === 'array' && !Array.isArray(value)) {
      fail('front-matter', `${where}: \`${key}\` must be a list, got "${value}"`);
    }
    if (kind === 'date' && !/^\d{4}-\d{2}-\d{2}$/.test(value)) {
      fail('front-matter', `${where}: \`${key}\` must be YYYY-MM-DD, got "${value}"`);
    }
  }

  for (const heading of REQUIRED_SECTIONS) {
    if (!content.includes(`## ${heading}`)) {
      fail('sections', `${where} is missing the section "## ${heading}"`);
    }
  }

  if (REPORT_STALENESS && typeof fields.last_verified === 'string') {
    const age = Math.floor((Date.now() - Date.parse(fields.last_verified)) / 86400000);
    if (age > 90) {
      notes.push(`${where}: last_verified is ${age} days old`);
    }
  }

  if (typeof fields.module === 'string') {
    modules.set(fields.module, { fields, file: where, tier: fields.tier });
  }
}

// ------------------------------------------------- migrations vs `tables:`

console.log('==> 2. Migrations vs `tables:` front-matter');

// Only the forward direction is checked: a table created by a migration must be
// declared. The reverse would be wrong — `ops.river_job` is declared and real,
// but River owns and applies that schema itself, so no goose file creates it.
const migrationsRoot = path.join(ROOT, 'db', 'migrations');
if (fs.existsSync(migrationsRoot)) {
  const moduleDirs = fs
    .readdirSync(migrationsRoot, { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && !entry.name.startsWith('_'))
    .map((entry) => entry.name);

  for (const moduleName of moduleDirs) {
    const documented = modules.get(moduleName);
    if (!documented) {
      fail(
        'tables',
        `db/migrations/${moduleName}/ exists but no module AGENT.md declares \`module: ${moduleName}\``,
      );
      continue;
    }

    const declared = new Set(documented.fields.tables || []);
    const dir = path.join(migrationsRoot, moduleName);
    for (const sqlFile of fs.readdirSync(dir).filter((f) => f.endsWith('.sql'))) {
      const sql = fs.readFileSync(path.join(dir, sqlFile), 'utf8');
      // Only the Up half creates tables; the Down half drops them.
      const up = sql.split(/--\s*\+goose Down/i)[0];
      const created = up.matchAll(
        /CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:([A-Za-z_][\w]*)\.)?([A-Za-z_][\w]*)/gi,
      );
      for (const [, , table] of created) {
        if (!declared.has(table)) {
          fail(
            'tables',
            `db/migrations/${moduleName}/${sqlFile} creates "${table}" but ` +
              `${documented.file} does not list it in \`tables:\``,
          );
        }
      }
    }
  }
}

// ------------------------------------ arch-lint deps vs MODULE_INDEX.md arrows

console.log('==> 3. .go-arch-lint.yml contract edges vs MODULE_INDEX.md §3');

const archPath = path.join(ROOT, '.go-arch-lint.yml');
const indexPath = path.join(ROOT, 'MODULE_INDEX.md');

if (fs.existsSync(archPath) && fs.existsSync(indexPath)) {
  const arch = fs.readFileSync(archPath, 'utf8');
  const index = fs.readFileSync(indexPath, 'utf8');

  // Mermaid node aliases: `AUTH[auth]` -> AUTH = auth
  const alias = new Map();
  for (const [, id, name] of index.matchAll(/\b([A-Z][A-Z0-9]*)\[([a-z][a-z0-9_]*)\]/g)) {
    alias.set(id, name);
  }

  // Subgraph membership, so a tier-level arrow (`ADM --> core`) is read as the
  // set of edges it stands for. Without this the check would demand an explicit
  // arrow for every edge the graph already expresses in one line.
  const tiers = new Map();
  let openTier = null;
  for (const line of index.split('\n')) {
    const open = line.match(/^\s*subgraph\s+([A-Za-z0-9_]+)/);
    if (open) {
      openTier = open[1];
      tiers.set(openTier, []);
      continue;
    }
    if (/^\s*end\s*$/.test(line)) {
      openTier = null;
      continue;
    }
    if (openTier) {
      for (const [, id] of line.matchAll(/\b([A-Z][A-Z0-9]*)\[[a-z]/g)) {
        tiers.get(openTier).push(alias.get(id) || id);
      }
    }
  }

  const expand = (token) => {
    const name = alias.get(token) || token;
    return tiers.has(name) ? tiers.get(name) : [name];
  };

  // Arrows, including the multi-node form `VOC & GRM --> CNT & SRS`.
  const arrows = new Set();
  for (const [, left, right] of index.matchAll(/^\s*([A-Za-z0-9_ &]+?)\s*-->\s*([A-Za-z0-9_ &]+?)\s*$/gm)) {
    const sources = left.split('&').flatMap((s) => expand(s.trim()));
    const targets = right.split('&').flatMap((s) => expand(s.trim()));
    for (const source of sources) {
      for (const target of targets) {
        arrows.add(`${source}->${target}`);
      }
    }
  }

  // Only cross-module contract edges (`c_*`) are compared. Platform edges (`p_*`)
  // are covered by the tier-level arrows (`core --> platform`), which is how
  // MODULE_INDEX deliberately draws them.
  for (const [, component, deps] of arch.matchAll(/^\s*m_([a-z_]+):\s*\{\s*mayDependOn:\s*\[([^\]]*)\]/gm)) {
    for (const dep of deps.split(',').map((d) => d.trim())) {
      if (!dep.startsWith('c_')) continue;
      const target = dep.slice(2);
      if (!arrows.has(`${component}->${target}`)) {
        fail(
          'dep-drift',
          `.go-arch-lint.yml lets m_${component} depend on ${dep}, but MODULE_INDEX.md §3 ` +
            `draws no "${component} --> ${target}" arrow`,
        );
      }
    }
  }
} else {
  fail('dep-drift', 'missing .go-arch-lint.yml or MODULE_INDEX.md');
}

// --------------------------------------------- openapi.yaml vs module API.md

console.log('==> 4. openapi.yaml paths vs the owning module API.md');

// Direction matters. Every path that is *in the spec* must be documented by its
// owning module; a path documented as planned but not yet in the spec is fine,
// because every module is still status: PLANNED. This becomes a live check the
// moment a module ships its first endpoint.
const specPath = path.join(ROOT, 'api', 'openapi', 'openapi.yaml');
if (fs.existsSync(specPath)) {
  const spec = fs.readFileSync(specPath, 'utf8');
  const pathsBlock = spec.split(/^paths:/m)[1] || '';

  let currentPath = null;
  let currentTag = null;
  const owned = [];
  for (const line of pathsBlock.split('\n')) {
    const pathMatch = line.match(/^ {2}(\/[^\s:]*):\s*$/);
    if (pathMatch) {
      currentPath = pathMatch[1];
      currentTag = null;
      continue;
    }
    const tagMatch = line.match(/^\s+tags:\s*\[([a-z0-9_]+)\]/);
    if (tagMatch && currentPath) {
      currentTag = tagMatch[1];
      owned.push({ path: currentPath, tag: currentTag });
    }
  }

  let checked = 0;
  for (const { path: apiPath, tag } of owned) {
    const documented = modules.get(tag);
    if (!documented) continue; // No module owns this tag; `system` has none.
    const apiDoc = path.join(path.dirname(path.join(ROOT, documented.file)), 'API.md');
    if (!fs.existsSync(apiDoc)) {
      fail('api-drift', `${apiPath} is tagged "${tag}" but ${rel(apiDoc)} does not exist`);
      continue;
    }
    checked += 1;
    if (!fs.readFileSync(apiDoc, 'utf8').includes(apiPath)) {
      fail('api-drift', `${apiPath} is in openapi.yaml but ${rel(apiDoc)} does not mention it`);
    }
  }
  if (checked === 0) {
    notes.push(
      `api-drift compared 0 paths: every operation in openapi.yaml is tagged for a ` +
        `tag no module owns yet. It goes live with the first module endpoint.`,
    );
  }
} else {
  fail('api-drift', 'api/openapi/openapi.yaml not found');
}

// ------------------------------------------------------------------- report

if (notes.length > 0) {
  console.log('\nNotes:');
  for (const note of notes) console.log(`  - ${note}`);
}

if (problems.length > 0) {
  console.error(`\n${problems.length} drift problem(s):`);
  for (const problem of problems) console.error(`  x ${problem}`);
  console.error('\nDocumentation drift check FAILED.');
  process.exit(1);
}

console.log('\nDocumentation drift check passed.');
