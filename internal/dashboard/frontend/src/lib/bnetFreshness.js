const DAY_MS = 86400000;
// Battle.net's last played is only true as of the profile refresh: after a week it may well be wrong, after two
// it is more likely wrong than right.
const OUTDATED_DAYS = 7;
const EXPIRED_DAYS = 14;

// bnetFreshness says how far to trust a figure from a Battle.net profile refreshed at fetchedAt: fresh,
// outdated (show it dimmed) or expired (do not show it).
export function bnetFreshness(fetchedAt, now = Date.now()) {
  const at = Date.parse(fetchedAt || '');
  if (Number.isNaN(at)) return 'expired';
  const days = (now - at) / DAY_MS;
  if (days >= EXPIRED_DAYS) return 'expired';
  return days >= OUTDATED_DAYS ? 'outdated' : 'fresh';
}
