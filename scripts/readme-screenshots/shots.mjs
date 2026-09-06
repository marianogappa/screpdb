// shots.mjs — the README screenshot list.
//
// Each shot names a dashboard route, the thing on screen it is meant to show,
// and how to frame it. Replays and players are resolved against the running
// dashboard's own API by player name, never by id: ids are derived from file
// checksums, so they are stable for a given corpus but meaningless to a reader
// and wrong the moment the corpus is re-staged.

const has = (g, key) => g.players.some((p) => p.player_key === key);
const longestFirst = (a, b) => b.duration_seconds - a.duration_seconds;

/** The longest game between two named handles. */
const gameBetween = (a, b) => (games) =>
  games.filter((g) => has(g, a) && has(g, b)).sort(longestFirst)[0];

/** A handle's longest game, optionally bounded by matchup and length. */
const gameOf = (key, { matchup, maxSeconds } = {}) => (games) =>
  games
    .filter((g) => has(g, key))
    .filter((g) => !matchup || g.matchup === matchup)
    .filter((g) => !maxSeconds || g.duration_seconds <= maxSeconds)
    .sort(longestFirst)[0];

const byFileName = (name) => (games) => games.find((g) => g.file_name === name);

/** The in-replay name of a handle, for framing a shot on that player's panel. */
const nameOf = (g, key) => g.players.find((p) => p.player_key === key)?.name ?? '';

export const SHOTS = [
  {
    name: 'game-list',
    caption: 'Filtering and finding replays by high-level semantic features',
    route: () => '/?view=games',
    clip: { height: 900 },
  },
  {
    name: 'game-summary',
    caption:
      'Game summary, with progamer identification from play style and one-click staging for watching on the game client',
    // Larva (Zerg) vs BishOp (Terran): two handles the fingerprint matcher
    // names, so the "Possibly …" line under each name has something to say.
    pick: gameBetween('jsa_larva', 'k_bishop'),
    route: (g) => `/?view=game&replay=${g.replay_id}`,
    clip: { throughSelector: '.workflow-panel' },
  },
  {
    name: 'game-events',
    caption: 'Rich game events browser with map overlays',
    pick: gameBetween('jsa_larva', 'k_bishop'),
    route: (g) => `/?view=game&replay=${g.replay_id}&gameTab=events`,
    // Select an attack: that is the event kind that draws movement on the map.
    click: { selector: '.workflow-event-row', hasText: { en: 'attacks', ko: '공격' } },
    clip: { throughSelector: '.workflow-events-layout' },
  },
  {
    name: 'game-hotkeys',
    caption: 'Hotkey intel: what each player keeps on which key, all game long',
    // A mid-length TvT. Terrans hotkey production buildings heavily, so both
    // timelines are dense, and a shorter game fits the timeline on screen.
    pick: gameOf('hm_ssak', { matchup: 'TvT', maxSeconds: 960 }),
    route: (g) => `/?view=game&replay=${g.replay_id}&gameTab=hotkeys`,
    clip: { selector: '.hk-timeline' },
  },
  {
    name: 'game-hotkeys-map',
    caption: 'The hotkeyed buildings, drawn on the map where they were built',
    pick: gameOf('hm_ssak', { matchup: 'TvT', maxSeconds: 960 }),
    route: (g) => `/?view=game&replay=${g.replay_id}&gameTab=hotkeys`,
    // The rendered map alone, for one player. Each panel crops to that
    // player's own base, so the pair rarely shares a shape, and the panel box
    // is a grid cell — much larger than the image sitting in it.
    clip: (g) => ({
      selector: `.hk-map-panel:has-text("${nameOf(g, 'hm_ssak')}") .hk-map-img`,
    }),
  },
  {
    name: 'build-orders',
    caption: 'Build order detection, charted against usual progamer timings',
    pick: gameBetween('jsa_larva', 'k_bishop'),
    route: (g) => `/?view=game&replay=${g.replay_id}&gameTab=build-orders`,
    clip: { height: 1000 },
  },
  {
    name: 'players-list',
    caption: 'Players list, with built-in progamer profiles alongside your own',
    route: () => '/?view=players',
    clip: { height: 760 },
  },
  {
    name: 'skill-proxies',
    caption:
      'Skill proxies: where every player in your library sits on the distribution, with progamers for reference',
    route: () => '/?view=players&playersTab=viewport-multitasking',
    clip: { throughSelector: '.workflow-panel' },
  },
  {
    name: 'player-hotkey-signature',
    caption: 'Per-player hotkey signature, including the built-in progamers',
    route: () => '/?view=player&player=pro%3Abisu&playerTab=hotkeys',
    // One signature card: the page stacks one per matchup, and the first is
    // enough to show what the surface is.
    clip: { throughSelector: '.hk-sig-card', nth: 0 },
  },
  {
    name: 'alliances',
    caption: 'Alliance timeline and team stacking detection on multiplayer melee games',
    pick: byFileName('bgh_team_stacking.rep'),
    route: (g) => `/?view=game&replay=${g.replay_id}&gameTab=alliances`,
    clip: { height: 1000 },
  },
];
