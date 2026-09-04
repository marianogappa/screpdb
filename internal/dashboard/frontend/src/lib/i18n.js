export const SUPPORTED_LOCALES = Object.freeze(['en', 'ko']);
export const DEFAULT_LOCALE = 'en';
export const LOCALE_STORAGE_KEY = 'dashboard_locale';

export const normalizeLocale = (value) => {
  const tag = String(value || '').trim().toLowerCase();
  if (!tag) return null;
  const base = tag.split(/[-_]/)[0];
  return SUPPORTED_LOCALES.includes(base) ? base : null;
};

export const detectLocale = ({ stored, languages } = {}) => {
  const fromStorage = normalizeLocale(stored);
  if (fromStorage) return fromStorage;
  for (const lang of languages || []) {
    const match = normalizeLocale(lang);
    if (match) return match;
  }
  return DEFAULT_LOCALE;
};

export const slugKey = (text) => String(text || '')
  .toLowerCase()
  .replace(/[^a-z0-9]+/g, '_')
  .replace(/^_+|_+$/g, '');

const FUZZY_OPENER_LABEL = /^~(\d+) (Overpool|Pool|Hatch)$/;
const HATCH_TECH_LABEL = /^(\d+) Hatch (Muta|Lurker|Hydra)$/;

export const buildLabelKey = (label) => {
  const text = String(label || '').trim();
  let match = text.match(FUZZY_OPENER_LABEL);
  if (match) return { key: `server.payload.fuzzy.${match[2].toLowerCase()}`, params: { n: match[1] } };
  match = text.match(HATCH_TECH_LABEL);
  if (match) return { key: `server.payload.nhatch.${match[2].toLowerCase()}`, params: { n: match[1] } };
  return null;
};

export const interpolate = (template, params) => {
  if (!params || typeof template !== 'string') return template;
  return template.replace(/\{(\w+)\}/g, (whole, name) => (
    params[name] === undefined || params[name] === null ? whole : String(params[name])
  ));
};

export const mergeCatalogModules = (modules) => {
  const merged = {};
  Object.keys(modules).sort().forEach((path) => {
    const entries = modules[path];
    Object.entries(entries || {}).forEach(([key, value]) => {
      merged[key] = value;
    });
  });
  return merged;
};

export const createTranslator = (catalogs, locale) => {
  const primary = catalogs[locale] || {};
  const fallback = catalogs[DEFAULT_LOCALE] || {};
  const translate = (key, params) => {
    const template = primary[key] ?? fallback[key];
    if (template === undefined) return key;
    return interpolate(template, params);
  };
  translate.locale = locale;
  translate.has = (key) => primary[key] !== undefined || fallback[key] !== undefined;
  translate.server = (key, serverText, params) => {
    const template = primary[key];
    if (template === undefined) return interpolate(serverText, params);
    return interpolate(template, params);
  };
  translate.serverExact = (key, serverText, params) => {
    if (fallback[key] !== undefined && fallback[key] !== serverText) return interpolate(serverText, params);
    return translate.server(key, serverText, params);
  };
  translate.buildLabel = (label) => {
    const resolved = buildLabelKey(label);
    return resolved ? translate.server(resolved.key, label, resolved.params) : label;
  };
  translate.plural = (key, count, params) => {
    const merged = { count, ...params };
    const exact = `${key}.${Number(count) === 1 ? 'one' : 'other'}`;
    for (const catalog of [primary, fallback]) {
      if (catalog[exact] !== undefined) return interpolate(catalog[exact], merged);
      if (catalog[key] !== undefined) return interpolate(catalog[key], merged);
    }
    return key;
  };
  return translate;
};
