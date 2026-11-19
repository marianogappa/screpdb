import test from 'node:test';
import assert from 'node:assert/strict';
import { highlightedItems } from './eventHighlight.js';

const focus = { x: 0, y: 0, w: 640, h: 640 };

test('an attack highlights the attacker\'s units of the attacking types in its patch of map', () => {
  const event = { type: 'drop', actor: { player_id: 1 }, attack_unit_types: ['Dropship', 'Terran Siege Tank (Siege Mode)'], engine: { focus } };
  const snapshot = {
    units: [
      { unit_type: 'Dropship', player_id: 1, x: 100, y: 100 },
      { unit_type: 'Siege Tank (Tank Mode)', player_id: 1, x: 120, y: 100 },
      { unit_type: 'SCV', player_id: 1, x: 130, y: 100 },
      { unit_type: 'Dropship', player_id: 2, x: 140, y: 100 },
      { unit_type: 'Dropship', player_id: 1, x: 3000, y: 3000 },
    ],
  };
  assert.deepEqual([...highlightedItems(event, snapshot).units], [0, 1]);
});

test('an expansion highlights the one town hall nearest where it happened', () => {
  const event = { type: 'expansion', actor: { player_id: 1 }, attack_unit_types: ['Nexus'], engine: { focus, point: { x: 500, y: 500 } } };
  const snapshot = {
    buildings: [
      { unit_type: 'Nexus', player_id: 1, x: 100, y: 100 },
      { unit_type: 'Nexus', player_id: 1, x: 480, y: 520 },
    ],
  };
  assert.deepEqual([...highlightedItems(event, snapshot).buildings], [1]);
});

test('a player leaving highlights nothing', () => {
  const event = { type: 'leave_game', actor: { player_id: 1 }, attack_unit_types: ['Nexus'], engine: { focus } };
  const snapshot = { buildings: [{ unit_type: 'Nexus', player_id: 1, x: 100, y: 100 }] };
  assert.equal(highlightedItems(event, snapshot).buildings.size, 0);
});
