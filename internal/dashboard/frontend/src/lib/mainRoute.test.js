import test from 'node:test';
import assert from 'node:assert/strict';

import {
  parseMainRouteSearch,
  buildMainRouteSearch,
  shouldLoadPlayerSkillProxyInsights,
  MAIN_PLAYER_SKILL_PROXY_SUBTABS,
} from './mainRoute.js';

// Regression coverage for #147: the player Skill-proxies "Population
// comparison" panel went permanently empty because the in-page tab button set
// the subtab to '' while the insight loader required subtab === 'summary'.

test('shouldLoadPlayerSkillProxyInsights: fires for any subtab while on skill-proxies', () => {
  const base = { activeView: 'player', selectedPlayerKey: 'chobo86', mainPlayerTab: 'skill-proxies' };
  // The exact bug: button set subtab to '' — must still load.
  assert.equal(shouldLoadPlayerSkillProxyInsights(base), true);
  // And it must not regress for the canonical / alternate subtab values either.
  for (const sub of ['', 'summary', 'usage-signals', undefined]) {
    assert.equal(
      shouldLoadPlayerSkillProxyInsights({ ...base, mainPlayerSubtab: sub }),
      true,
      `expected load for subtab=${JSON.stringify(sub)}`,
    );
  }
});

test('shouldLoadPlayerSkillProxyInsights: does not fire outside the skill-proxies tab', () => {
  assert.equal(
    shouldLoadPlayerSkillProxyInsights({ activeView: 'player', selectedPlayerKey: 'x', mainPlayerTab: 'summary' }),
    false,
  );
  assert.equal(
    shouldLoadPlayerSkillProxyInsights({ activeView: 'player', selectedPlayerKey: 'x', mainPlayerTab: 'recent-games' }),
    false,
  );
});

test('shouldLoadPlayerSkillProxyInsights: requires player view and a selected player', () => {
  assert.equal(
    shouldLoadPlayerSkillProxyInsights({ activeView: 'games', selectedPlayerKey: 'x', mainPlayerTab: 'skill-proxies' }),
    false,
  );
  assert.equal(
    shouldLoadPlayerSkillProxyInsights({ activeView: 'player', selectedPlayerKey: '', mainPlayerTab: 'skill-proxies' }),
    false,
  );
  assert.equal(shouldLoadPlayerSkillProxyInsights(null), false);
});

test('skill-proxies subtab normalizes to summary and round-trips', () => {
  // Bare skill-proxies link → canonical summary subtab.
  const parsed = parseMainRouteSearch('?view=player&player=chobo86&playerTab=skill-proxies');
  assert.equal(parsed.playerTab, 'skill-proxies');
  assert.equal(parsed.playerSubtab, 'summary');
  assert.ok(MAIN_PLAYER_SKILL_PROXY_SUBTABS.includes(parsed.playerSubtab));

  // An empty subtab (what the tab button used to set) still resolves to summary.
  const built = buildMainRouteSearch({
    activeView: 'player',
    selectedPlayerKey: 'chobo86',
    mainPlayerTab: 'skill-proxies',
    mainPlayerSubtab: '',
  });
  assert.equal(parseMainRouteSearch(`?${built}`).playerSubtab, 'summary');
});
