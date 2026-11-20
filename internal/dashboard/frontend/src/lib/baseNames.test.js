import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createTranslator } from './i18n.js';
import { localizeBaseName, localizeOpenField, localizePlace } from './baseNames.js';

const catalog = (locale) => JSON.parse(fs.readFileSync(new URL(`../locales/${locale}/app.json`, import.meta.url)));
const catalogs = { en: catalog('en'), ko: catalog('ko') };
const en = createTranslator(catalogs, 'en');
const ko = createTranslator(catalogs, 'ko');

const BACKEND_NAMES = [
  'start 3', '3', 'natural of 11', 'natural of 3 & 9', 'expa 5', 'inner expa 7', 'center expa', 'expa 5 (2)',
  "11's main base", "7's natural", "Skins_'s main base", "Skins_'s natural", "TZPNEWB's expa at 3's main base",
  "JSA_Larva's expa at 11's natural", "1's natural (center base)", "1's natural near 2", 'center base',
  'starting base', 'natural expansion', 'an expansion near 4', 'expansion', 'main base', 'natural', 'Some Odd Name',
];

test('English base names come back exactly as the backend sent them', () => {
  for (const name of BACKEND_NAMES) assert.equal(localizeBaseName(name, en), name);
  for (const phrase of ['in the middle', "near White's base", "on White's side of the map", "near 7's natural"]) {
    assert.equal(localizeOpenField(phrase, en), phrase);
  }
});

test('Korean base names say clocks, mains, naturals and expansions in Korean', () => {
  assert.equal(localizeBaseName("11's main base", ko), '11시 본진');
  assert.equal(localizeBaseName("7's natural", ko), '7시 앞마당');
  assert.equal(localizeBaseName("Skins_'s main base", ko), 'Skins_의 본진');
  assert.equal(localizeBaseName("TZPNEWB's expa at 3's main base", ko), 'TZPNEWB의 3시 본진 멀티');
  assert.equal(localizeBaseName('expa 5', ko), '5시 멀티');
  assert.equal(localizeBaseName('expa 5 (2)', ko), '5시 멀티 (2)');
  assert.equal(localizeBaseName('inner expa 7', ko), '7시 안쪽 멀티');
  assert.equal(localizeBaseName('start 3', ko), '3시 본진');
  assert.equal(localizeBaseName('3', ko), '3시');
  assert.equal(localizePlace('expa at 11\'s natural', ko), '11시 앞마당 멀티');
  assert.equal(localizeOpenField("near White's base", ko), 'White의 기지 근처에서');
  assert.equal(localizeOpenField('in the middle', ko), '맵 중앙에서');
  assert.equal(localizeBaseName('Some Odd Name', ko), 'Some Odd Name');
  for (const name of BACKEND_NAMES.slice(0, -1)) {
    assert.doesNotMatch(localizeBaseName(name, ko), /main base|natural|expa|center|starting|expansion|'s/, name);
  }
});

const drop = (t, place) => t('events.sentence.drops', {
  actor: 'K_BishOp', target: 'JSA_Larva', icon: '', from: '',
  count: t('events.fragment.transports', { count: 4 }),
  location: t('events.place.their', { place: localizePlace(place, t) }),
});
const expands = (t, name) => t('events.sentence.expandsTo', { actor: 'JSA_Larva', location: localizeBaseName(name, t) });
const attack = (t, place) => t('events.sentence.attacks', {
  actor: 'K_BishOp', target: 'JSA_Larva',
  locationClause: t('events.location.at', { location: t('events.place.their', { place: localizePlace(place, t) }) }),
});
const mineralOnly = (t, name) => t('events.sentence.expandsTo', {
  actor: 'K_BishOp', location: t('events.location.mineralOnly', { name: localizeBaseName(name, t) }),
});

test('Game Events sentences read in natural Korean, with the drop landing on the target', () => {
  assert.equal(drop(ko, 'main base'), 'K_BishOp, JSA_Larva 본진에 드랍 4기');
  assert.equal(expands(ko, "11's main base"), 'JSA_Larva, 11시 본진에 확장');
  assert.equal(attack(ko, "expa at 11's natural"), 'K_BishOp, 11시 앞마당 멀티에서 JSA_Larva 공격');
  assert.equal(mineralOnly(ko, 'expa 5'), 'K_BishOp, 5시 멀티 (미네랄 전용)에 확장');
});

test('the same Game Events sentences read naturally in English', () => {
  assert.equal(drop(en, 'main base'), 'K_BishOp drops with 4 on JSA_Larva at their main base');
  assert.equal(expands(en, "11's main base"), "JSA_Larva expands to 11's main base");
  assert.equal(attack(en, "expa at 11's natural"), "K_BishOp attacks JSA_Larva at their expa at 11's natural");
  assert.equal(mineralOnly(en, 'expa 5'), 'K_BishOp expands to expa 5 (mineral only)');
});
