import test from 'node:test';
import assert from 'node:assert/strict';
import { countryCodeToAlpha2, countryCodeToFlag, countryCodeToName } from './countries.js';

test('countryCodeToAlpha2 normalizes alpha-2 and alpha-3 codes', () => {
  assert.equal(countryCodeToAlpha2('AR'), 'AR');
  assert.equal(countryCodeToAlpha2('ar'), 'AR');
  assert.equal(countryCodeToAlpha2(' kr '), 'KR');
  assert.equal(countryCodeToAlpha2('ARG'), 'AR');
  assert.equal(countryCodeToAlpha2('KOR'), 'KR');
  assert.equal(countryCodeToAlpha2('XKX'), 'XK');
});

test('countryCodeToAlpha2 rejects junk codes', () => {
  assert.equal(countryCodeToAlpha2(''), null);
  assert.equal(countryCodeToAlpha2(null), null);
  assert.equal(countryCodeToAlpha2(undefined), null);
  assert.equal(countryCodeToAlpha2('A'), null);
  assert.equal(countryCodeToAlpha2('ZZZ'), null);
  assert.equal(countryCodeToAlpha2('12'), null);
});

test('countryCodeToFlag builds the regional-indicator pair', () => {
  assert.equal(countryCodeToFlag('AR'), '\u{1F1E6}\u{1F1F7}');
  assert.equal(countryCodeToFlag('ARG'), '\u{1F1E6}\u{1F1F7}');
  assert.equal(countryCodeToFlag('KR'), '\u{1F1F0}\u{1F1F7}');
  assert.equal(countryCodeToFlag('US'), '\u{1F1FA}\u{1F1F8}');
  assert.equal(countryCodeToFlag(''), null);
  assert.equal(countryCodeToFlag('ZZZ'), null);
});

test('countryCodeToName resolves English country names', () => {
  assert.equal(countryCodeToName('AR'), 'Argentina');
  assert.equal(countryCodeToName('ARG'), 'Argentina');
  assert.equal(countryCodeToName('KR'), 'South Korea');
  assert.equal(countryCodeToName('KP'), 'North Korea');
  assert.equal(countryCodeToName('US'), 'United States');
});

test('countryCodeToName names regions in the requested locale', () => {
  assert.equal(countryCodeToName('KR', 'en'), 'South Korea');
  assert.equal(countryCodeToName('KR', 'ko'), '대한민국');
  assert.equal(countryCodeToName('US', 'ko'), '미국');
  assert.equal(countryCodeToName('ARG', 'ko-KR'), '아르헨티나');
  assert.equal(countryCodeToName('AR', 'xx'), 'Argentina');
  assert.equal(countryCodeToName('ZZ', 'ko'), 'ZZ');
});

test('countryCodeToName falls back to the code for unnamed regions', () => {
  assert.equal(countryCodeToName('ZZ'), 'ZZ');
  assert.equal(countryCodeToName('QQ'), 'QQ');
  assert.equal(countryCodeToName(''), null);
  assert.equal(countryCodeToName('ZZZ'), null);
});
