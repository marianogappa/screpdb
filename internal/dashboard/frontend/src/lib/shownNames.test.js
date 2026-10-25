import test from 'node:test';
import assert from 'node:assert/strict';
import { applyShownNames, rawName, shownNamesByPlayerID } from './shownNames.js';

const game = {
  players: [
    { player_id: 1, name: 'lIlIIIIIIllllll', player_key: 'lIlIIIIIIllllll' },
    { player_id: 2, name: 'xdnjcj', player_key: 'xdnjcj' },
  ],
  game_events: [{
    actor: { player_id: 1, name: 'lIlIIIIIIllllll' },
    units: [{ name: 'Reaver', count: 2 }],
    base: { name: "near lIlIIIIIIllllll's base", kind: 'main' },
  }],
  unit_production_cadence: [{ player_id: 1, player_name: 'lIlIIIIIIllllll', player_key: 'lIlIIIIIIllllll' }],
};

const byID = shownNamesByPlayerID(game.players, (p) => (p.player_id === 1 ? 'Bisu' : p.name));

test('only players with another shown name are mapped', () => {
  assert.deepEqual([...byID.entries()], [[1, 'Bisu']]);
});

test('every name of a mapped player is rewritten, keys and unit names are not', () => {
  const out = applyShownNames(game, byID);
  assert.equal(out.players[0].name, 'Bisu');
  assert.equal(out.players[0].player_key, 'lIlIIIIIIllllll');
  assert.equal(rawName(out.players[0]), 'lIlIIIIIIllllll');
  assert.equal(out.players[1].name, 'xdnjcj');
  assert.equal(out.game_events[0].actor.name, 'Bisu');
  assert.equal(out.game_events[0].units[0].name, 'Reaver');
  assert.equal(out.game_events[0].base.name, "near Bisu's base");
  assert.equal(out.unit_production_cadence[0].player_name, 'Bisu');
  assert.equal(game.players[0].name, 'lIlIIIIIIllllll');
});

test('nothing to rewrite returns the payload as is', () => {
  assert.equal(applyShownNames(game, new Map()), game);
});
