import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createTranslator } from './i18n.js';
import { turningPointSentence } from './turningPoint.js';

const catalog = (locale) => Object.assign({}, ...['app', 'server'].map((f) => JSON.parse(fs.readFileSync(new URL(`../locales/${locale}/${f}.json`, import.meta.url)))));
const t = createTranslator({ en: catalog('en'), ko: catalog('ko') }, 'en');

const you = { player_id: 1, name: 'chobo86' };
const ally = { player_id: 2, name: 'FallenAngel' };
const enemy = { player_id: 3, name: 'fdfdfdfd' };

const told = (event) => {
  const s = turningPointSentence(t, event);
  const params = Object.fromEntries(Object.entries(s.params).map(([k, v]) => [k, v?.player ? v.player.name : v?.possessive ? `${v.possessive.name}'s` : v]));
  return (s.lead ? `${t(s.lead)} ` : '') + t(s.key, params);
};

const lings = { value: 600, defence: 950, count: 24, units: [{ name: 'Zergling', count: 24 }], mostly: 'Zergling' };

test('you leave because of what happened to your only ally, told without names for either of you', () => {
  const event = { type: 'attack', actor: enemy, target: ally, outcome: { town_hall: true }, why_you: you, why_after_ally: ally, why_allies: 1, why_force: lings };
  assert.equal(told(event), "You leave because: fdfdfdfd destroys one of your ally's bases with Zerglings");
});

test('with several allies the ally keeps their name', () => {
  const event = { type: 'attack', actor: enemy, target: ally, outcome: { town_hall: true }, why_you: you, why_after_ally: ally, why_allies: 2, why_force: lings };
  assert.equal(told(event), "You leave because: fdfdfdfd destroys one of FallenAngel's bases with Zerglings");
});

test('a force bigger than the whole answer is told as larger, and its main unit named', () => {
  const force = { value: 9350, defence: 4000, count: 20, units: [{ name: 'Carrier', count: 12 }, { name: 'Arbiter', count: 3 }], mostly: 'Carrier' };
  const event = { type: 'overrun', actor: enemy, target: you, why_you: you, why_force: force };
  assert.equal(told(event), 'fdfdfdfd overruns you with a much larger army of mostly Carriers');
});

test('your own thrown away attack is told in the second person', () => {
  const force = { value: 500, defence: 2000, count: 2, units: [{ name: 'Siege Tank (Tank Mode)', count: 2 }], mostly: 'Siege Tank (Tank Mode)' };
  const event = { type: 'drop', actor: you, target: enemy, outcome: { killed: 75, lost: 1850 }, why_you: you, why_force: force };
  assert.equal(told(event), 'You lose a drop of Siege Tanks attacking fdfdfdfd');
});

test('several Recalls are counted and a tiny force is told exactly', () => {
  const recalls = { value: 1500, defence: 10000, count: 7, units: [{ name: 'Dragoon', count: 6 }, { name: 'Arbiter', count: 1 }], mostly: 'Dragoon', recalls: 2 };
  assert.equal(told({ type: 'recall', actor: enemy, target: ally, outcome: { town_hall: true }, why_force: recalls }), "fdfdfdfd destroys one of FallenAngel's bases with 2 Recalls of mostly Dragoons");
  const two = { value: 200, defence: 0, count: 2, units: [{ name: 'Zealot', count: 2 }], mostly: 'Zealot' };
  assert.equal(told({ type: 'overrun', actor: enemy, target: you, why_you: you, why_force: two }), 'fdfdfdfd overruns you with only 2 Zealots');
});

test('a player with one base loses their base, not one of their bases', () => {
  const event = { type: 'attack', actor: enemy, target: you, outcome: { town_hall: true }, why_you: you, why_target_bases: 1, why_force: lings };
  assert.equal(told(event), 'fdfdfdfd destroys your base with Zerglings');
});
