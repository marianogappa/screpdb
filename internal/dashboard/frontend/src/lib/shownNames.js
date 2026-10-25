// A game payload names each player in many places (events, build orders, timings, hotkeys). When the
// game title shows a player under another name, such as a progamer's for a barcode account, every tab
// should too, so the payload is rewritten once: each object that has a player_id gets that player's
// shown name in its name and player_name fields, and place names such as "near X's base" name the
// player the same way. The players keep their real name in raw_name for the Summary row, which shows both.

const NAME_KEYS = ['name', 'player_name'];

export const shownNamesByPlayerID = (players, shownName) => {
  const byID = new Map();
  for (const player of Array.isArray(players) ? players : []) {
    const shown = shownName(player);
    if (player?.player_id != null && shown && shown !== player.name) byID.set(player.player_id, shown);
  }
  return byID;
};

const rewrite = (value, byID, possessives) => {
  if (Array.isArray(value)) return value.map((item) => rewrite(item, byID, possessives));
  if (!value || typeof value !== 'object') return value;
  const out = {};
  for (const [key, item] of Object.entries(value)) out[key] = rewrite(item, byID, possessives);
  const shown = value.player_id != null ? byID.get(value.player_id) : undefined;
  if (shown) {
    for (const key of NAME_KEYS) {
      if (typeof value[key] === 'string') out[key] = shown;
    }
  } else if (typeof value.name === 'string') {
    out.name = possessives.reduce((name, [from, to]) => name.split(from).join(to), value.name);
  }
  return out;
};

export const applyShownNames = (payload, byID) => {
  if (!payload || !byID || byID.size === 0) return payload;
  const possessives = (Array.isArray(payload.players) ? payload.players : [])
    .filter((player) => byID.has(player?.player_id) && player.name)
    .map((player) => [`${player.name}'s`, `${byID.get(player.player_id)}'s`]);
  const out = rewrite(payload, byID, possessives);
  if (Array.isArray(payload.players) && Array.isArray(out.players)) {
    out.players = out.players.map((player, i) => ({ ...player, raw_name: payload.players[i]?.name ?? player.name }));
  }
  return out;
};

export const rawName = (player) => player?.raw_name ?? player?.name ?? '';
