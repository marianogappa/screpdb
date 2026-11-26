const SITE_URL = 'https://marianogappa.github.io/screpdb/';

export function releaseNotesUrl(version) {
  const bare = String(version || '').trim().replace(/^v/i, '');
  return bare ? `${SITE_URL}#v${bare}` : `${SITE_URL}#news`;
}
