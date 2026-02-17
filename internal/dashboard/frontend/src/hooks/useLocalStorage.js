import { useState, useCallback } from 'react';

export function useLocalStorageJSON(keyPrefix) {
  const getStored = useCallback((suffix) => {
    try {
      const raw = localStorage.getItem(`${keyPrefix}_${suffix}`);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  }, [keyPrefix]);

  const setStored = useCallback((suffix, value) => {
    try {
      localStorage.setItem(`${keyPrefix}_${suffix}`, JSON.stringify(value));
    } catch {
      // ignore quota errors
    }
  }, [keyPrefix]);

  return { getStored, setStored };
}

export function usePreference(key, defaultValue) {
  const [value, setValueState] = useState(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw !== null ? JSON.parse(raw) : defaultValue;
    } catch {
      return defaultValue;
    }
  });

  const setValue = useCallback((next) => {
    setValueState(next);
    try {
      localStorage.setItem(key, JSON.stringify(next));
    } catch {
      // ignore
    }
  }, [key]);

  return [value, setValue];
}
