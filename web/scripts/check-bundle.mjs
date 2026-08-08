import { gzipSync } from 'node:zlib';
import { readFileSync, existsSync } from 'node:fs';
import path from 'node:path';

/**
 * Bundle budget, enforced.
 *
 * `build.chunkSizeWarningLimit` only prints a warning, and a warning in a build
 * log is not a budget — it is a number that scrolls past. This walks the Vite
 * manifest from the entry chunk through its *static* imports, which is exactly
 * what a first visit downloads, and exits non-zero when the gzipped total goes
 * over. Lazily imported chunks (the OpenTelemetry SDK, for one) are excluded on
 * purpose: keeping them out of the initial load is the whole reason they are
 * dynamic.
 *
 * Raise the budget deliberately, in a commit that says why.
 */
const BUDGET_GZIP_KB = 200;

const dist = path.resolve(import.meta.dirname, '..', 'dist');
const manifestPath = path.join(dist, '.vite', 'manifest.json');

if (!existsSync(manifestPath)) {
  console.error(`no manifest at ${manifestPath}; run \`vite build\` with build.manifest enabled`);
  process.exit(1);
}

const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));

const entry = Object.values(manifest).find((chunk) => chunk.isEntry);
if (!entry) {
  console.error('manifest contains no entry chunk');
  process.exit(1);
}

/** Entry plus everything it statically imports, transitively. */
const initial = new Set();
const queue = [entry];
while (queue.length > 0) {
  const chunk = queue.pop();
  if (!chunk || initial.has(chunk.file)) continue;
  initial.add(chunk.file);
  for (const key of chunk.imports ?? []) {
    if (manifest[key]) queue.push(manifest[key]);
  }
  for (const css of chunk.css ?? []) initial.add(css);
}

let total = 0;
const rows = [];
for (const file of initial) {
  const bytes = gzipSync(readFileSync(path.join(dist, file))).length;
  total += bytes;
  rows.push({ file, kb: bytes / 1024 });
}

rows.sort((a, b) => b.kb - a.kb);
for (const row of rows) {
  console.log(`  ${row.kb.toFixed(1).padStart(7)} kB  ${row.file}`);
}

const totalKb = total / 1024;
console.log(`\ninitial download: ${totalKb.toFixed(1)} kB gzipped (budget ${BUDGET_GZIP_KB} kB)`);

if (totalKb > BUDGET_GZIP_KB) {
  console.error(
    `\nbundle budget exceeded by ${(totalKb - BUDGET_GZIP_KB).toFixed(1)} kB. ` +
      `Split something out with a dynamic import, or raise the budget on purpose.`,
  );
  process.exit(1);
}
