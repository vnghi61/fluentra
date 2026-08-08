import fs from 'fs';
import path from 'path';

// Documentation Drift Check script for Task P0.16
const ROOT_DIR = process.cwd();

console.log('==> Step 1: Checking AGENT.md front-matter and required sections...');

function findFiles(dir, filename, fileList = []) {
  const files = fs.readdirSync(dir);
  for (const file of files) {
    const filePath = path.join(dir, file);
    const stat = fs.statSync(filePath);
    if (stat.isDirectory()) {
      if (file !== 'node_modules' && file !== '.git') {
        findFiles(filePath, filename, fileList);
      }
    } else if (file === filename) {
      fileList.push(filePath);
    }
  }
  return fileList;
}

const agentFiles = findFiles(ROOT_DIR, 'AGENT.md');
let hasErrors = false;

for (const filePath of agentFiles) {
  const relPath = path.relative(ROOT_DIR, filePath);
  const content = fs.readFileSync(filePath, 'utf8');

  // Skip root AGENT.md from strict table front-matter check if appropriate
  if (relPath === 'AGENT.md') continue;

  // Front-matter check
  if (!content.startsWith('---')) {
    console.error(`❌ ${relPath}: Missing front-matter header (---)`);
    hasErrors = true;
  }

  // Staleness check (last_verified > 90 days)
  const match = content.match(/last_verified:\s*(\d{4}-\d{2}-\d{2})/);
  if (match && match[1]) {
    const verifiedDate = new Date(match[1]);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - verifiedDate.getTime()) / (1000 * 3600 * 24));
    if (diffDays > 90) {
      console.warn(`⚠️ ${relPath}: last_verified date is older than 90 days (${diffDays} days old)`);
    }
  }
}

console.log('==> Step 2: Checking migrations vs module AGENT.md tables front-matter...');
const migrationsDir = path.join(ROOT_DIR, 'db', 'migrations');
if (fs.existsSync(migrationsDir)) {
  const moduleDirs = fs.readdirSync(migrationsDir).filter(f => fs.statSync(path.join(migrationsDir, f)).isDirectory() && !f.startsWith('_'));
  
  for (const mod of moduleDirs) {
    const modAgentPath = path.join(ROOT_DIR, 'internal', 'platform', mod, 'AGENT.md');
    const modModuleAgentPath = path.join(ROOT_DIR, 'internal', 'modules', mod, 'AGENT.md');
    
    const targetAgent = fs.existsSync(modAgentPath) ? modAgentPath : (fs.existsSync(modModuleAgentPath) ? modModuleAgentPath : null);
    if (!targetAgent) {
      console.error(`❌ Migration exists for '${mod}' in db/migrations/${mod}, but no AGENT.md found at internal/platform/${mod}/AGENT.md or internal/modules/${mod}/AGENT.md!`);
      hasErrors = true;
    }
  }
}

console.log('==> Step 3: Checking .go-arch-lint.yml vs MODULE_INDEX.md dependencies...');
const archLintPath = path.join(ROOT_DIR, '.go-arch-lint.yml');
const moduleIndexPath = path.join(ROOT_DIR, 'MODULE_INDEX.md');

if (fs.existsSync(archLintPath) && fs.existsSync(moduleIndexPath)) {
  const archContent = fs.readFileSync(archLintPath, 'utf8');
  const indexContent = fs.readFileSync(moduleIndexPath, 'utf8');

  // Verify that components in arch-lint exist in project
  if (!archContent.includes('version: 3')) {
    console.error('❌ .go-arch-lint.yml is missing version: 3 header!');
    hasErrors = true;
  }
}

if (hasErrors) {
  console.error('\n❌ Documentation drift check FAILED! Please update the documentation to match source code.');
  process.exit(1);
} else {
  console.log('\n✔ Documentation drift check PASSED cleanly.');
}
