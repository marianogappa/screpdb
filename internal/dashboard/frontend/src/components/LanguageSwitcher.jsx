import React from 'react';
import { useLocale, useT } from '../lib/i18nContext';

const LOCALE_NAMES = { en: 'English', ko: '한국어' };

export default function LanguageSwitcher() {
  const t = useT();
  const { locale, setLocale, locales } = useLocale();
  return (
    <div className="language-switcher" role="group" aria-label={t('footer.language')}>
      {locales.map((code) => (
        <button
          key={code}
          type="button"
          lang={code}
          className={`language-switcher-option${code === locale ? ' is-active' : ''}`}
          aria-pressed={code === locale}
          onClick={() => setLocale(code)}
        >
          {LOCALE_NAMES[code] || code}
        </button>
      ))}
    </div>
  );
}
