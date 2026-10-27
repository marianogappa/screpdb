import test from 'node:test';
import assert from 'node:assert/strict';

import {
  parseMainRouteSearch,
  buildMainRouteSearch,
} from './mainRoute.js';

test('old Skill proxies links open Skill', () => {
  assert.equal(parseMainRouteSearch('?view=player&player=chobo86&playerTab=skill-proxies').playerTab, 'skill');
  assert.equal(parseMainRouteSearch('?view=players&playersTab=apm-histogram').playersTab, 'skill');
  assert.equal(parseMainRouteSearch('?view=players&playersTab=skill').playersTab, 'skill');
  assert.equal(buildMainRouteSearch({ activeView: 'players', mainPlayersTab: 'skill' }), 'view=players&playersTab=skill');
});

test('build order permalink round-trips on the Build Orders tab only', () => {
  const state = { activeView: 'game', selectedReplayId: 42, mainGameTab: 'build-orders', mainGameBuildOrder: '7:bo_terran_2_fact' };
  const search = buildMainRouteSearch(state);
  assert.equal(search, 'view=game&replay=42&gameTab=build-orders&bo=7%3Abo_terran_2_fact');
  assert.equal(parseMainRouteSearch(search).buildOrder, '7:bo_terran_2_fact');
  assert.equal(buildMainRouteSearch({ ...state, mainGameTab: 'summary' }), 'view=game&replay=42');
  assert.equal(parseMainRouteSearch('view=game&replay=42&gameTab=timings&bo=7:x').buildOrder, '');
});
