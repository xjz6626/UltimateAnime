import test from 'node:test';
import assert from 'node:assert/strict';
import { danmakuPicker, downloadWithDanmaku, finishDanmakuPicker } from './composables/useDanmakuPicker.js';

function mockApp(preferences) {
  const downloads = [];
  globalThis.window = { go: { main: { App: {
    GetDanmakuPreferences: async () => preferences,
    DownloadEpisode: async (...args) => { downloads.push(args); return 'Started'; },
  } } } };
  return downloads;
}

test('manual mode waits for the source picker before starting the video download', async () => {
  const downloads = mockApp({ enabled: true, auto_select: false });
  const pending = downloadWithDanmaku(42, 2, 'magnet:test');
  await new Promise(setImmediate);
  assert.equal(danmakuPicker.value.episode, 2);
  assert.deepEqual(downloads, []);
  finishDanmakuPicker({ kind: 'video', id: 'BV13x41117TL', cid: 123 });
  assert.equal(await pending, 'Started');
  assert.deepEqual(downloads, [[42, 2, 'magnet:test']]);
});

test('closing the source picker cancels downloading, while skipping comments downloads the video', async () => {
  const downloads = mockApp({ enabled: true, auto_select: false });
  const cancelled = downloadWithDanmaku(42, 1, 'magnet:test');
  await new Promise(setImmediate);
  finishDanmakuPicker(null);
  assert.equal(await cancelled, 'Cancelled');
  assert.deepEqual(downloads, []);
  const skip = downloadWithDanmaku(42, 1, 'magnet:test');
  await new Promise(setImmediate);
  finishDanmakuPicker({ kind: 'skip' });
  assert.equal(await skip, 'Started');
  assert.equal(downloads.length, 1);
});

test('automatic mode and disabled comments do not open the picker', async () => {
  for (const preferences of [{ enabled: false }, { enabled: true, auto_select: true }]) {
    const downloads = mockApp(preferences);
    assert.equal(await downloadWithDanmaku(42, 1, 'magnet:test'), 'Started');
    assert.equal(danmakuPicker.value, null);
    assert.equal(downloads.length, 1);
  }
});
