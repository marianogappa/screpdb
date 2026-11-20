// The backend names bases in English ("7's natural", "Skins_'s main base", "expa 5"), so localizeBaseName
// reads that grammar back and says it in the reader's language. Anything it does not recognise is kept.

const CLOCK = /^\d{1,2}$/;

const PLACES = [
  [/^main base$/, (t) => t('events.place.main')],
  [/^natural$/, (t) => t('events.place.natural')],
  [/^natural expansion$/, (t) => t('events.place.naturalExpansion')],
  [/^expa at (.+)$/, (t, m) => t('events.place.expaAt', { base: localizeBaseName(m[1], t) })],
];

const owned = (t, owner, place) => (CLOCK.test(owner)
  ? t('events.place.clockOwned', { clock: owner, place })
  : t('events.place.owned', { owner, place }));

const RULES = [
  [/^(.+) \((\d+)\)$/, (t, m) => `${localizeBaseName(m[1], t)} (${m[2]})`],
  [/^(\d+)'s natural \(center base\)$/, (t, m) => t('events.place.naturalCenter', { clock: m[1] })],
  [/^(\d+)'s natural near (\d+)$/, (t, m) => t('events.place.naturalNear', { clock: m[1], near: m[2] })],
  [/^(.+?)'s (expa at .+)$/, (t, m) => owned(t, m[1], localizePlace(m[2], t))],
  [/^(.+?)'s (main base|natural)$/, (t, m) => owned(t, m[1], localizePlace(m[2], t))],
  [/^(\d+)$/, (t, m) => t('events.place.clock', { clock: m[1] })],
  [/^start (\d+)$/, (t, m) => t('events.place.start', { clock: m[1] })],
  [/^natural of (.+)$/, (t, m) => t('events.place.naturalOf', { clocks: m[1] })],
  [/^inner expa (\d+)$/, (t, m) => t('events.place.innerExpa', { clock: m[1] })],
  [/^expa (\d+)$/, (t, m) => t('events.place.expa', { clock: m[1] })],
  [/^center expa$/, (t) => t('events.place.centerExpa')],
  [/^center base$/, (t) => t('events.place.center')],
  [/^starting base$/, (t) => t('events.place.starting')],
  [/^an expansion near (\d+)$/, (t, m) => t('events.place.expansionNear', { clock: m[1] })],
  [/^expansion$/, (t) => t('events.place.expansion')],
];

const apply = (rules, label, t) => {
  for (const [re, render] of rules) {
    const m = label.match(re);
    if (m) return render(t, m);
  }
  return null;
};

// localizePlace says the part of a base name left once its owner is taken off ("main base", "expa at 3's natural").
export const localizePlace = (label, t) => {
  const text = String(label || '').trim();
  return apply(PLACES, text, t) ?? localizeBaseName(text, t);
};

export const localizeBaseName = (label, t) => {
  const text = String(label || '').trim();
  if (!text) return '';
  return apply(RULES, text, t) ?? apply(PLACES, text, t) ?? text;
};

const OPEN_FIELD = [
  [/^in the middle$/, (t) => t('events.location.inTheMiddle')],
  [/^near (.+?)'s base$/, (t, m) => t('events.location.nearPlayerBase', { player: m[1] })],
  [/^on (.+?)'s side of the map$/, (t, m) => t('events.location.playerSide', { player: m[1] })],
  [/^near (.+)$/, (t, m) => t('events.location.nearBase', { location: localizeBaseName(m[1], t) })],
];

// localizeOpenField says a place away from bases that came as an English phrase ("near White's base").
export const localizeOpenField = (label, t) => {
  const text = String(label || '').trim();
  return apply(OPEN_FIELD, text, t) ?? text;
};
