import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// Em dashes must never reach the UI. They read as machine-written prose, and a
// dash is almost never the clearest punctuation available: a colon, a semicolon,
// a full stop or a pair of parentheses all say the same thing more plainly. For
// a "no value" placeholder the codebase already renders a plain hyphen.
//
// Comments are exempt, since nobody reads those in the product. The stripper
// below removes block comments (which is also how JSX comments are written,
// `{/* ... */}`) and whole-line `//` comments. A trailing `//` comment on a line
// of code is deliberately NOT stripped — guessing where a comment starts means
// mistaking `https://` for one, and a false positive that asks you to move a
// comment beats a false negative that ships an em dash.
const SRC = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const EXTENSIONS = new Set(['.js', '.jsx']);
const EM_DASH = '—';

function listSourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === 'build') continue;
    const full = path.join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...listSourceFiles(full));
      continue;
    }
    if (!EXTENSIONS.has(path.extname(entry))) continue;
    if (entry.endsWith('.test.js')) continue;
    out.push(full);
  }
  return out;
}

function stripComments(src) {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .split('\n')
    .map((line) => (line.trimStart().startsWith('//') ? '' : line))
    .join('\n');
}

test('no em dashes in user-visible strings', () => {
  const offenders = [];
  for (const file of listSourceFiles(SRC)) {
    const lines = stripComments(readFileSync(file, 'utf8')).split('\n');
    lines.forEach((line, idx) => {
      if (!line.includes(EM_DASH)) return;
      offenders.push(`${path.relative(SRC, file)}:${idx + 1}: ${line.trim()}`);
    });
  }
  assert.deepEqual(
    offenders,
    [],
    `Em dashes found outside comments. Use a colon, semicolon, full stop or `
      + `parentheses instead; for an empty-value placeholder use a plain hyphen.\n`
      + offenders.join('\n'),
  );
});
