import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  SUPPORTED_LOCALES,
  createTranslator,
  detectLocale,
  mergeCatalogModules,
  normalizeLocale,
} from './i18n';

const catalogs = {
  en: mergeCatalogModules(import.meta.glob('../locales/en/*.json', { eager: true, import: 'default' })),
  ko: mergeCatalogModules(import.meta.glob('../locales/ko/*.json', { eager: true, import: 'default' })),
};

const readStoredLocale = () => {
  try {
    return localStorage.getItem(LOCALE_STORAGE_KEY);
  } catch {
    return null;
  }
};

const writeStoredLocale = (locale) => {
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // Private mode or blocked storage: the choice still applies for this session.
  }
};

const initialLocale = () => detectLocale({
  stored: readStoredLocale(),
  languages: typeof navigator !== 'undefined' ? (navigator.languages || [navigator.language]) : [],
});

let currentLocale = initialLocale();
let currentTranslator = createTranslator(catalogs, currentLocale);
const listeners = new Set();

export const getLocale = () => currentLocale;

export const allTranslations = (key) => SUPPORTED_LOCALES
  .map((locale) => catalogs[locale]?.[key])
  .filter((value) => value !== undefined);

export const setLocale = (value) => {
  const next = normalizeLocale(value) || DEFAULT_LOCALE;
  if (next === currentLocale) return;
  currentLocale = next;
  currentTranslator = createTranslator(catalogs, next);
  writeStoredLocale(next);
  listeners.forEach((listener) => listener(next));
};

export const t = (key, params) => currentTranslator(key, params);
t.server = (key, serverText, params) => currentTranslator.server(key, serverText, params);
t.serverExact = (key, serverText, params) => currentTranslator.serverExact(key, serverText, params);
t.buildLabel = (label) => currentTranslator.buildLabel(label);
t.plural = (key, count, params) => currentTranslator.plural(key, count, params);
t.has = (key) => currentTranslator.has(key);

const LocaleContext = createContext({ locale: currentLocale, setLocale, t: currentTranslator });

export function LocaleProvider({ children }) {
  const [locale, setLocaleState] = useState(currentLocale);

  useEffect(() => {
    const listener = (next) => setLocaleState(next);
    listeners.add(listener);
    return () => listeners.delete(listener);
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
    document.title = currentTranslator('app.title');
  }, [locale]);

  const value = useMemo(() => ({ locale, setLocale, t: currentTranslator }), [locale]);
  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export const useLocale = () => {
  const { locale } = useContext(LocaleContext);
  const change = useCallback((next) => setLocale(next), []);
  return { locale, setLocale: change, locales: SUPPORTED_LOCALES };
};

export const useT = () => useContext(LocaleContext).t;
