import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// Regression guard for #147: a JSX component (`<Histogram .../>`) was rendered
// after its import + definition had been deleted. Vite/esbuild does NOT flag
// references to undefined identifiers, so the build stayed green while the tab
// crashed at runtime with "Histogram is not defined". This test statically
// resolves every capitalised JSX tag in our source to an import or a local
// definition, catching that whole class of bug before it ships.

const SRC_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

// Tags that look capitalised but are not user components needing a binding.
const IGNORED_TAGS = new Set(['React', 'Fragment']);

function listJsxFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...listJsxFiles(full));
    } else if (entry.endsWith('.jsx')) {
      out.push(full);
    }
  }
  return out;
}

// Names brought into scope: imports (default / named / namespace) plus
// local declarations (const/let/var/function/class).
function collectBoundNames(src) {
  const names = new Set();

  for (const m of src.matchAll(/import\s+([\s\S]*?)\s+from\s+['"][^'"]+['"]/g)) {
    const clause = m[1];
    // default and namespace specifiers: `Foo`, `* as Foo`
    for (const d of clause.matchAll(/(?:^|,)\s*(?:\*\s+as\s+)?([A-Za-z_$][\w$]*)\s*(?=,|{|$)/g)) {
      names.add(d[1]);
    }
    // named specifiers inside { ... }, honouring `as` aliases
    const braced = clause.match(/\{([\s\S]*?)\}/);
    if (braced) {
      for (const part of braced[1].split(',')) {
        const t = part.trim();
        if (!t) continue;
        const alias = t.split(/\s+as\s+/).pop().trim();
        if (alias) names.add(alias);
      }
    }
  }

  for (const m of src.matchAll(/\b(?:const|let|var|function|class)\s+([A-Za-z_$][\w$]*)/g)) {
    names.add(m[1]);
  }

  return names;
}

// Capitalised JSX opening tags, e.g. `<Histogram` or `<Foo.Bar` (root = Foo).
function collectComponentRefs(src) {
  const refs = new Set();
  for (const m of src.matchAll(/<([A-Z][\w$]*)/g)) {
    refs.add(m[1].split('.')[0]);
  }
  return refs;
}

test('every JSX component reference resolves to an import or local definition', () => {
  const files = listJsxFiles(SRC_DIR);
  assert.ok(files.length > 0, 'expected to find .jsx source files');

  const problems = [];
  for (const file of files) {
    const src = readFileSync(file, 'utf8');
    const bound = collectBoundNames(src);
    for (const ref of collectComponentRefs(src)) {
      if (IGNORED_TAGS.has(ref)) continue;
      if (!bound.has(ref)) {
        problems.push(`${path.relative(SRC_DIR, file)}: <${ref}> is used but never imported or defined`);
      }
    }
  }

  assert.deepEqual(problems, [], `Unresolved JSX components:\n${problems.join('\n')}`);
});
