import test from 'node:test';
import assert from 'node:assert/strict';
import { youTemplate } from './youTemplate.js';

const words = { subject: 'you', object: 'you', possessive: 'your' };
const en = (template, ...roles) => youTemplate(template, roles, words, true);

test('the viewer as the subject agrees with the verb', () => {
  assert.equal(en('{actor} attacks {target} {locationClause}', 'actor'), 'You attack {target} {locationClause}');
  assert.equal(en('{actor} zergling rushes {target}', 'actor'), 'You zergling rush {target}');
  assert.equal(en('{actor} proxies barracks at {location}', 'actor'), 'You proxy barracks at {location}');
  assert.equal(en('{actor} is supply blocked for {duration}', 'actor'), 'You are supply blocked for {duration}');
  assert.equal(en('{actor} expands to their natural', 'actor'), 'You expand to your natural');
  assert.equal(en('{actor} drops{count}{icon}{from} on {target} at {location}', 'actor'), 'You drop{count}{icon}{from} on {target} at {location}');
});

test('the viewer as the object or owner is you and your', () => {
  assert.equal(en("{actor} destroys {target}'s base {locationClause}", 'target'), '{actor} destroys your base {locationClause}');
  assert.equal(en('{actor} overruns {target} {locationClause} with {count}{icon}, more than they have left to answer it', 'target'),
    '{actor} overruns you {locationClause} with {count}{icon}, more than you have left to answer it');
  assert.equal(en("{target} holds off {actor}'s attack {locationClause}", 'actor'), '{target} holds off your attack {locationClause}');
  assert.equal(en("{actor}'s Ghost dies before the nuke lands on {target} at {location}", 'actor'), 'Your Ghost dies before the nuke lands on {target} at {location}');
  assert.equal(en("{actor} kills {count} of {target}'s Overlords; {target} is supply blocked for {duration}", 'target'),
    "{actor} kills {count} of your Overlords; you are supply blocked for {duration}");
});

test('other languages only swap the name', () => {
  assert.equal(youTemplate('{actor}이(가) {target} 공격', ['target'], { subject: '나', object: '나', possessive: '나의' }, false), '{actor}이(가) 나 공격');
});
