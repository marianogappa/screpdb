import test from 'node:test';
import assert from 'node:assert/strict';
import { cardRaces, skillCardFileName } from './skillCard.js';

test('the card shows the races a player really plays, most played first', () => {
  const breakdown = [
    { race: 'Zerg', game_count: 3 },
    { race: 'Terran', game_count: 60 },
    { race: 'Protoss', game_count: 37 },
  ];
  assert.deepEqual(cardRaces(breakdown), ['Terran', 'Protoss']);
});

test('a player with no race breakdown shows no races', () => {
  assert.deepEqual(cardRaces(undefined), []);
  assert.deepEqual(cardRaces([{ race: 'Zerg', game_count: 0 }]), []);
});

test('a saved card is named after the player and its month', () => {
  assert.equal(skillCardFileName('chobo86', new Date(2026, 10, 3)), 'screpdb_skill_chobo86_2026-11.png');
  assert.equal(skillCardFileName('-=FallenAngel=-', new Date(2026, 0, 1)), 'screpdb_skill_FallenAngel_2026-01.png');
  assert.equal(skillCardFileName('☆☆', new Date(2026, 0, 1)), 'screpdb_skill_player_2026-01.png');
});
