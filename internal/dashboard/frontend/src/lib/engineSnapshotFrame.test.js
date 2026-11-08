import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// The engine event map sits in a centring flex stage that hides overflow. Its wrapper is a flex item, so
// unless the wrapper caps its own width the map frame keeps its height-driven width and the stage clips
// both edges of the map: buildings at a map edge vanished from every event shown there.
const CSS = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'styles.css'), 'utf8');

const rule = (selector) => {
  const m = new RegExp(`(?:^|\\n)${selector.replace(/[.]/g, '\\.')}\\s*\\{([^}]*)\\}`).exec(CSS);
  return m ? m[1] : '';
};

test('the engine snapshot wrapper never grows past its stage', () => {
  const body = rule('.engine-snapshot');
  assert.match(body, /max-width:\s*100%/);
  assert.match(body, /min-width:\s*0/);
});

test('the map frame caps itself at its parent width', () => {
  const src = readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'components', 'GameMap.jsx'), 'utf8');
  assert.match(src, /maxWidth:\s*'100%'/);
});
