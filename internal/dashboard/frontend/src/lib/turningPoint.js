import { slugKey } from './i18n.js';

// A turning point reads as a short story told to the viewer: they are "you", a lone ally is "your ally", and
// the force that did it is described loosely (bigger than the answer, mostly Carriers), never in numbers that
// a snapshot could get wrong.

const DROP_TYPES = new Set(['drop', 'reaver_drop', 'dt_drop', 'ht_drop', 'lurker_drop', 'cliff_drop', 'elevator']);
const SAME_UNIT = { 'Terran Siege Tank (Siege Mode)': 'Siege Tank', 'Siege Tank (Tank Mode)': 'Siege Tank' };

const samePlayer = (a, b) => a?.player_id != null && a.player_id === b?.player_id;

export const unitName = (t, raw) => {
  const name = SAME_UNIT[raw] || raw;
  return t.server(`server.name.${slugKey(name)}`, name);
};

export const unitsName = (t, raw) => {
  const name = SAME_UNIT[raw] || raw;
  const own = `turningPoint.units.${slugKey(name)}`;
  return t.has(own) ? t(own) : t('turningPoint.units', { unit: unitName(t, raw) });
};

// mostlyPhrase is what a force is made of, or empty when it is too mixed to say.
const mostlyPhrase = (t, force) => {
  const only = (force.units || []).length === 1;
  if (force.mostly) return t(only ? 'turningPoint.of.only' : 'turningPoint.of.mostly', { units: unitsName(t, force.mostly) });
  if (force.class) return t('turningPoint.of.mostly', { units: t(`turningPoint.class.${force.class}`) });
  if (force.pair?.length === 2) {
    const units = t('turningPoint.pair', { a: unitsName(t, force.pair[0]), b: unitsName(t, force.pair[1]) });
    return t(only ? 'turningPoint.of.only' : 'turningPoint.of.mostly', { units });
  }
  return '';
};

const armySize = (force) => {
  const value = Number(force.value || 0);
  const defence = Number(force.defence || 0);
  if (value >= 1000 && value >= 2 * defence) return 'muchLarger';
  if (value >= 1000 && value >= 1.25 * defence) return 'larger';
  if (value >= 6000) return 'huge';
  if (value >= 3000) return 'large';
  return '';
};

const dropSize = (force) => {
  const value = Number(force.value || 0);
  if (value >= 1500) return 'huge';
  if (value >= 800) return 'large';
  return 'plain';
};

// forcePhrase names the force as a noun: "a larger army of mostly Carriers", "a huge drop of Siege Tanks",
// "2 Recalls of mostly Dragoons". A small army with nothing to set it apart is just its units.
export const forcePhrase = (t, event) => {
  const force = event?.why_force;
  if (!force) return '';
  const of = mostlyPhrase(t, force);
  const type = String(event?.type || '');
  if (type === 'recall') {
    const recalls = Math.max(1, Number(force.recalls || 0));
    return t.plural('turningPoint.recalls', recalls, { of });
  }
  if (DROP_TYPES.has(type)) return t(`turningPoint.drop.${dropSize(force)}`, { of });
  const size = armySize(force);
  if (size) return t(`turningPoint.army.${size}`, { of });
  const count = Number(force.count || 0);
  if (count > 0 && count <= 3 && force.units?.length === 1) {
    return t.plural('turningPoint.few', count, { units: count === 1 ? unitName(t, force.units[0].name) : unitsName(t, force.units[0].name) });
  }
  if (force.mostly) return (force.units || []).length === 1 ? unitsName(t, force.mostly) : t('turningPoint.mostly', { units: unitsName(t, force.mostly) });
  if (force.class) return t('turningPoint.mostly', { units: t(`turningPoint.class.${force.class}`) });
  if (force.pair?.length === 2) return t('turningPoint.pair', { a: unitsName(t, force.pair[0]), b: unitsName(t, force.pair[1]) });
  return '';
};

const verbKey = (event) => {
  const type = String(event?.type || '');
  const o = event?.outcome || {};
  if (type === 'overrun') return 'overrun';
  if (type === 'undefended_air_harass') return 'airHarass';
  if (o.town_hall) return 'destroysBase';
  if (Number(o.lost || 0) >= 2 * Number(o.killed || 0) && Number(o.lost || 0) >= 1000) return 'loses';
  if (Number(o.workers || 0) >= 6) return 'killsWorkers';
  if (Number(o.buildings || 0) >= 3) return 'destroysBuildings';
  return 'winsFight';
};

// turningPointSentence is the sentence for a turning point card: { key, params, lead } with the viewer and a
// lone ally already in words and other players left as { player } for the caller to draw. Null when the event
// is not one it knows how to tell.
export const turningPointSentence = (t, event) => {
  if (!event?.actor || !event?.target) return null;
  const you = event.why_you;
  const ally = event.why_after_ally;
  const loneAlly = ally && Number(event.why_allies || 0) <= 1;
  const role = (player) => {
    if (samePlayer(player, you)) return 'you';
    if (loneAlly && samePlayer(player, ally)) return 'ally';
    return 'other';
  };
  const object = (player) => {
    const r = role(player);
    return r === 'other' ? { player } : t(`turningPoint.object.${r}`);
  };
  const possessive = (player) => {
    const r = role(player);
    return r === 'other' ? { possessive: player } : t(`turningPoint.possessive.${r}`);
  };
  const verb = verbKey(event);
  const actorIsYou = role(event.actor) === 'you';
  const force = forcePhrase(t, event);
  const o = event.outcome || {};
  const params = {
    actor: actorIsYou ? '' : { player: event.actor },
    target: object(event.target),
    targets: possessive(event.target),
    count: verb === 'destroysBuildings' ? Number(o.buildings || 0) : Number(o.workers || 0),
    force: force || t('turningPoint.anArmy'),
    with: force ? t('turningPoint.with', { force }) : '',
  };
  const only = verb === 'destroysBase' && Number(event.why_target_bases || 0) === 1 ? 'Only' : '';
  const key = `turningPoint.${verb}${only}${actorIsYou ? '.you' : ''}`;
  return { key, params, lead: ally ? 'turningPoint.leaveBecause' : '' };
};
