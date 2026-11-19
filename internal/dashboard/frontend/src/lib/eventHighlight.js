import { normalizeUnitName } from './gameAssets.js';

const TILE = 32;
const REACH = 4 * TILE;
const SINGLE_NEAR_POINT = new Set(['expansion', 'proxy_gate', 'manner_pylon']);
const NOT_HIGHLIGHTED = new Set(['player_start', 'leave_game', 'player_dropped', 'player_stopped_playing', 'long_supply_block', 'became_terran', 'nuke']);

// The engine and the replay name a Siege Tank three ways; the highlight only cares that it is one.
const unitKey = (name) => normalizeUnitName(name).replace(/^(terran|protoss|zerg)/, '').replace(/^siegetank.*/, 'siegetank');

const inside = (box, x, y) => x >= box.x - REACH && x <= box.x + box.w + REACH && y >= box.y - REACH && y <= box.y + box.h + REACH;

// highlightedItems picks what an event map points at: the event's actor's own units and buildings of the
// types the event names, in the patch of map the event is about. An expansion, a proxy or a manner pylon
// is one building, the one nearest where it happened.
export const highlightedItems = (event, snapshot) => {
  const none = { buildings: new Set(), units: new Set() };
  const types = new Set((event?.attack_unit_types || []).map(unitKey));
  const actor = event?.actor?.player_id;
  const area = event?.engine?.focus;
  if (!snapshot || types.size === 0 || actor == null || !area || NOT_HIGHLIGHTED.has(event?.type)) return none;
  const mine = (item) => item.player_id === actor && types.has(unitKey(item.unit_type)) && inside(area, item.x, item.y);
  const buildings = new Set();
  const units = new Set();
  (snapshot.buildings || []).forEach((b, i) => { if (mine(b)) buildings.add(i); });
  (snapshot.units || []).forEach((u, i) => { if (mine(u)) units.add(i); });
  const point = event?.engine?.point;
  if (SINGLE_NEAR_POINT.has(event?.type) && point && buildings.size > 1) {
    const dist = (i) => Math.hypot(snapshot.buildings[i].x - point.x, snapshot.buildings[i].y - point.y);
    const nearest = [...buildings].reduce((a, b) => (dist(b) < dist(a) ? b : a));
    return { buildings: new Set([nearest]), units: new Set() };
  }
  return { buildings, units };
};
