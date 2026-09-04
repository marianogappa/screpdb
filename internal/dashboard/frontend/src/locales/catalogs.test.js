import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const ROOT = path.dirname(fileURLToPath(import.meta.url));
const LOCALES = ['en', 'ko'];

const loadLocale = (locale) => {
  const dir = path.join(ROOT, locale);
  const merged = {};
  const owners = {};
  for (const file of readdirSync(dir).filter((f) => f.endsWith('.json')).sort()) {
    const entries = JSON.parse(readFileSync(path.join(dir, file), 'utf8'));
    for (const [key, value] of Object.entries(entries)) {
      assert.equal(owners[key], undefined, `${locale}/${file}: key "${key}" already defined in ${owners[key]}`);
      assert.equal(typeof value, 'string', `${locale}/${file}: "${key}" must be a string`);
      owners[key] = file;
      merged[key] = value;
    }
  }
  return { merged, owners };
};

const placeholders = (template) => [...template.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();

test('every locale defines the same keys in the same files', () => {
  const en = loadLocale('en');
  for (const locale of LOCALES.filter((l) => l !== 'en')) {
    const other = loadLocale(locale);
    const missing = Object.keys(en.merged).filter((k) => other.merged[k] === undefined);
    const extra = Object.keys(other.merged).filter((k) => en.merged[k] === undefined);
    assert.deepEqual(missing, [], `${locale} is missing keys present in en`);
    assert.deepEqual(extra, [], `${locale} has keys absent from en`);
    for (const key of Object.keys(en.merged)) {
      assert.equal(other.owners[key], en.owners[key], `"${key}" lives in ${en.owners[key]} for en but ${other.owners[key]} for ${locale}`);
    }
  }
});

test('translations keep the same placeholders as the English source', () => {
  const en = loadLocale('en').merged;
  for (const locale of LOCALES.filter((l) => l !== 'en')) {
    const other = loadLocale(locale).merged;
    for (const [key, template] of Object.entries(en)) {
      assert.deepEqual(placeholders(other[key]), placeholders(template), `"${key}" placeholders differ in ${locale}`);
    }
  }
});

test('no locale string is empty or contains an em dash', () => {
  for (const locale of LOCALES) {
    const { merged } = loadLocale(locale);
    for (const [key, value] of Object.entries(merged)) {
      assert.ok(value.length > 0, `${locale}: "${key}" is empty`);
      assert.ok(!value.includes('—'), `${locale}: "${key}" contains an em dash`);
    }
  }
});
