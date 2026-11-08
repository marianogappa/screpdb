import test from 'node:test';
import assert from 'node:assert/strict';
import { formatSkillValue, skillStep } from './skillFormat.js';

test('prints each metric at the step the server placed it at', () => {
  assert.equal(formatSkillValue('eapm', 117, 1), '117');
  assert.equal(formatSkillValue('used_recalls_per_min', 0, 1), '0');
  assert.equal(formatSkillValue('production_idle_share', 0.37, 0.01), '37%');
  assert.equal(formatSkillValue('idle_army_share', 0.219, 0.001), '21.9%');
  assert.equal(formatSkillValue('micro_actions_per_min', 0.5, 0.1), '0.5');
});

test('distinct server values never print alike', () => {
  for (const [key, step] of [['idle_army_share', 0.001], ['army_cadence', 0.01], ['micro_actions_per_min', 0.1], ['eapm', 1]]) {
    const printed = new Set();
    for (let i = 0; i < 1000; i += 1) printed.add(formatSkillValue(key, Math.round(i) * step, step));
    assert.equal(printed.size, 1000, key);
  }
});

test('reads the step from the model summary', () => {
  const model = { metrics: [{ key: 'idle_army_share', step: 0.001 }] };
  assert.equal(skillStep(model, 'idle_army_share'), 0.001);
  assert.equal(skillStep(model, 'eapm'), 0);
  assert.equal(skillStep(undefined, 'eapm'), 0);
});
