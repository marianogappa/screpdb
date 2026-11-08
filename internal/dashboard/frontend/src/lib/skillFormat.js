export const PERCENT_METRICS = new Set(['army_cadence', 'production_idle_share', 'idle_army_share']);

export const skillStep = (model, key) => model?.metrics?.find((m) => m.key === key)?.step || 0;

// formatSkillValue prints a strip value at the step the server rounded it to before placing it, so two players
// printed alike always sit at the same spot.
export function formatSkillValue(key, value, step) {
  const percent = PERCENT_METRICS.has(key);
  const shown = percent ? value * 100 : value;
  const unit = (percent ? 100 : 1) * (step || (percent ? 0.01 : 1));
  const decimals = Math.max(0, Math.round(-Math.log10(unit)));
  return `${shown.toFixed(decimals)}${percent ? '%' : ''}`;
}
