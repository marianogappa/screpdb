import test from 'node:test';
import assert from 'node:assert/strict';
import { createTranslator, detectLocale, interpolate, mergeCatalogModules, normalizeLocale, slugKey } from './i18n.js';

test('slugKey turns backend display text into a stable catalog id', () => {
  assert.equal(slugKey('Psionic Storm'), 'psionic_storm');
  assert.equal(slugKey('  Forge Cannon (no expa) '), 'forge_cannon_no_expa');
  assert.equal(slugKey('Population stddev'), 'population_stddev');
  assert.equal(slugKey(''), '');
});

test('normalizeLocale maps region tags onto supported bases', () => {
  assert.equal(normalizeLocale('ko-KR'), 'ko');
  assert.equal(normalizeLocale('en_US'), 'en');
  assert.equal(normalizeLocale('EN'), 'en');
  assert.equal(normalizeLocale('ja-JP'), null);
  assert.equal(normalizeLocale(''), null);
});

test('detectLocale prefers the stored choice, then browser languages, then English', () => {
  assert.equal(detectLocale({ stored: 'en', languages: ['ko-KR'] }), 'en');
  assert.equal(detectLocale({ stored: null, languages: ['ko-KR', 'en-US'] }), 'ko');
  assert.equal(detectLocale({ stored: 'garbage', languages: ['fr-FR', 'ko'] }), 'ko');
  assert.equal(detectLocale({ stored: null, languages: ['fr-FR'] }), 'en');
  assert.equal(detectLocale({}), 'en');
});

test('interpolate fills named placeholders and leaves unknown ones intact', () => {
  assert.equal(interpolate('{count} games on {map}', { count: 3, map: 'Fighting Spirit' }), '3 games on Fighting Spirit');
  assert.equal(interpolate('{minute}분에 {subject}', { minute: 4 }), '4분에 {subject}');
  assert.equal(interpolate('plain', { x: 1 }), 'plain');
});

test('mergeCatalogModules flattens every module in a stable order', () => {
  const merged = mergeCatalogModules({
    './b.json': { 'b.key': 'B' },
    './a.json': { 'a.key': 'A' },
  });
  assert.deepEqual(merged, { 'a.key': 'A', 'b.key': 'B' });
});

test('createTranslator falls back to English, then to the key', () => {
  const catalogs = {
    en: { 'x.hello': 'Hello {name}', 'x.only_en': 'English only', 'n.games': '{count} games', 'n.games.one': '{count} game' },
    ko: { 'x.hello': '{name} 안녕', 'n.games': '{count} 게임' },
  };
  const ko = createTranslator(catalogs, 'ko');
  assert.equal(ko('x.hello', { name: 'Flash' }), 'Flash 안녕');
  assert.equal(ko('x.only_en'), 'English only');
  assert.equal(ko('x.missing'), 'x.missing');
  assert.equal(ko.plural('n.games', 1), '1 게임');
  const en = createTranslator(catalogs, 'en');
  assert.equal(en.plural('n.games', 1), '1 game');
  assert.equal(en.plural('n.games', 2), '2 games');
});

test('buildLabel translates the two resolved Zerg label formats and leaves everything else alone', () => {
  const catalogs = {
    en: { 'server.payload.fuzzy.overpool': '~{n} Overpool', 'server.payload.nhatch.muta': '{n} Hatch Muta' },
    ko: { 'server.payload.fuzzy.overpool': '~{n} 오버풀', 'server.payload.nhatch.muta': '{n}해처리 뮤탈' },
  };
  const ko = createTranslator(catalogs, 'ko');
  assert.equal(ko.buildLabel('~9 Overpool'), '~9 오버풀');
  assert.equal(ko.buildLabel('3 Hatch Muta'), '3해처리 뮤탈');
  assert.equal(ko.buildLabel('~9 Hatch'), '~9 Hatch');
  assert.equal(ko.buildLabel('12 Hatch'), '12 Hatch');
  assert.equal(ko.buildLabel(''), '');
  assert.equal(createTranslator(catalogs, 'en').buildLabel('~9 Overpool'), '~9 Overpool');
});

test('translator.serverExact only overrides text that matches the English catalog', () => {
  const catalogs = {
    en: { 'server.marker.nhatch_hydra.name': 'N Hatch Hydra', 'server.marker.bo_9_pool.name': '9 Pool' },
    ko: { 'server.marker.nhatch_hydra.name': 'N해처리 히드라', 'server.marker.bo_9_pool.name': '9풀' },
  };
  const ko = createTranslator(catalogs, 'ko');
  assert.equal(ko.serverExact('server.marker.bo_9_pool.name', '9 Pool'), '9풀');
  assert.equal(ko.serverExact('server.marker.nhatch_hydra.name', '3 Hatch Hydralisk'), '3 Hatch Hydralisk');
  assert.equal(ko.serverExact('server.marker.unknown.name', 'Whatever'), 'Whatever');
});

test('translator.server returns the server text unless the locale overrides it', () => {
  const catalogs = { en: {}, ko: { 'marker.bo_9_pool.name': '9풀' } };
  const ko = createTranslator(catalogs, 'ko');
  assert.equal(ko.server('marker.bo_9_pool.name', '9 Pool'), '9풀');
  assert.equal(ko.server('marker.bo_12_hatch.name', '12 Hatch'), '12 Hatch');
  const en = createTranslator(catalogs, 'en');
  assert.equal(en.server('marker.bo_9_pool.name', '9 Pool'), '9 Pool');
});
