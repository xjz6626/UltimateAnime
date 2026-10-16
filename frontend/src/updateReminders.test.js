import test from 'node:test';
import assert from 'node:assert/strict';
import { rememberUpdates, hasUnreadUpdate, readUpdate } from './utils/updateReminders.js';

const time = (day, hour = 12) => new Date(2026, 9, day, hour).getTime();
const schedule = (day, episode) => ({ today_airing_at: time(day) / 1000, today_episode: episode });
const reload = (records) => JSON.parse(JSON.stringify(records));

test('yesterday and today both stay unread after reloading and later days', () => {
  let records = rememberUpdates({}, { 1: schedule(15, 2) }, time(15));
  records = rememberUpdates(reload(records), { 2: schedule(16, 5) }, time(16));
  assert.equal(hasUnreadUpdate(records[1]), true);
  assert.equal(hasUnreadUpdate(records[2]), true);
  records = rememberUpdates(reload(records), {}, time(20));
  assert.equal(hasUnreadUpdate(records[1]), true);
  assert.equal(hasUnreadUpdate(records[2]), true);
});

test('opening a show clears only its reminder and a later episode appears again', () => {
  let records = rememberUpdates({}, { 1: schedule(15, 2), 2: schedule(15, 4) }, time(15));
  records[1] = readUpdate(records[1], true, time(15));
  records = rememberUpdates(reload(records), {}, time(16));
  assert.equal(hasUnreadUpdate(records[1]), false);
  assert.equal(hasUnreadUpdate(records[2]), true);
  records = rememberUpdates(records, { 1: schedule(22, 3) }, time(22));
  assert.equal(hasUnreadUpdate(records[1]), true);
});

test('saved next schedule preserves an update missed while the app was closed', () => {
  let records = rememberUpdates({}, { 1: { next_episode: 2, next_airing_at: time(15) / 1000 } }, time(14));
  assert.equal(hasUnreadUpdate(records[1]), false);
  records[1] = readUpdate(records[1], true, time(14));
  records = rememberUpdates(reload(records), { 1: { next_episode: 3, next_airing_at: time(22) / 1000 } }, time(16));
  assert.equal(hasUnreadUpdate(records[1]), true);
  assert.equal(records[1].latest.episode, 2);
});

test('late metadata does not resurrect reminders already acknowledged by a click', () => {
  let records = { 1: readUpdate({}, false, time(16)) };
  records = rememberUpdates(records, { 1: schedule(15, 2) }, time(16));
  assert.equal(hasUnreadUpdate(records[1]), false);
  records = rememberUpdates(records, { 1: schedule(22, 3) }, time(22));
  assert.equal(hasUnreadUpdate(records[1]), true);
});

test('a second episode on the same day creates a new reminder after reading the first', () => {
  let records = rememberUpdates({}, { 1: schedule(15, 1) }, time(15));
  records[1] = readUpdate(records[1], true, time(15));
  records = rememberUpdates(records, { 1: schedule(15, 2) }, time(15));
  assert.equal(hasUnreadUpdate(records[1]), true);
});
