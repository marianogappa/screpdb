// A race the player played in less than this share of their games is practice, not one of their races.
const MIN_RACE_SHARE = 0.1;

// cardRaces is the races a player's Skill card shows, most played first.
export function cardRaces(breakdown) {
  const rows = (breakdown || []).filter((r) => r.game_count > 0);
  const total = rows.reduce((sum, r) => sum + r.game_count, 0);
  return rows
    .filter((r) => r.game_count >= MIN_RACE_SHARE * total)
    .sort((a, b) => b.game_count - a.game_count)
    .map((r) => r.race);
}

// skillCardFileName names a saved Skill card after the player and the month it shows.
export function skillCardFileName(name, date) {
  const slug = String(name || '').normalize('NFKD').replace(/[^\p{L}\p{N}]+/gu, '_').replace(/^_+|_+$/g, '') || 'player';
  const month = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
  return `screpdb_skill_${slug}_${month}.png`;
}
