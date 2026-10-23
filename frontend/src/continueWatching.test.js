import test from 'node:test';
import assert from 'node:assert/strict';
import { createRenderer } from 'vue';
import { useContinueWatching } from './composables/useContinueWatching.js';

const deferred = () => { let resolve; const promise = new Promise(yes => { resolve = yes; }); return { promise, resolve }; };
const tick = () => new Promise(setImmediate);
const item = (id, position = 30) => ({ subject_id: id, ep_sort: 1, position, duration: 100, available: true });
function setup(api) {
  const events = new Map();
  globalThis.window = { go: { main: { App: api } }, runtime: { EventsOn: (name, callback) => { events.set(name, callback); return () => events.delete(name); } } };
  const renderer = createRenderer({ createComment: () => ({}), insert() {}, remove() {}, parentNode() {}, nextSibling() {} });
  let state;
  const app = renderer.createApp({ setup() { state = useContinueWatching(); return () => null; } });
  app.mount({});
  return { state, events, dispose: () => app.unmount() };
}

test('playback event refresh wins over a stale initial snapshot and cleans subscriptions', async () => {
  const first = deferred(), latest = deferred(); let calls = 0;
  const { state, events, dispose } = setup({ GetContinueWatching: () => calls++ ? latest.promise : first.promise });
  try {
    events.get('playback-progress')();
    latest.resolve([item(2, 40)]); await tick();
    first.resolve([item(1)]); await tick();
    assert.deepEqual(state.items.value, [item(2, 40)]);
  } finally { dispose(); }
  assert.equal(events.size, 0);
});

test('resume captures the original episode and missing files remain visible as an actionable error', async () => {
  const request = deferred(), calls = [];
  const { state, events, dispose } = setup({
    GetContinueWatching: async () => [item(2)],
    PlayLocalEpisode: (...args) => { calls.push(args); return request.promise; },
  });
  try {
    const playing = state.play(item(1));
    events.get('playback-progress')(); await tick();
    request.resolve('Error: FileMissing'); await playing;
    assert.deepEqual(calls, [[1, 1]]);
    assert.match(state.error.value, /文件已移动或删除/);
    await state.refresh();
    assert.match(state.error.value, /重新下载/);
  } finally { dispose(); }
});

test('hide failure retains the card; successful hide refreshes without deleting history locally', async () => {
  let hidden = false, fail = true;
  const { state, dispose } = setup({
    GetContinueWatching: async () => hidden ? [] : [item(1)],
    HideContinueWatching: async () => { if (fail) return 'Error: 保存失败'; hidden = true; return 'Success'; },
  });
  try {
    await tick(); await state.hide(item(1));
    assert.equal(state.items.value.length, 1); assert.match(state.error.value, /保存失败/);
    fail = false; await state.hide(item(1));
    assert.equal(state.items.value.length, 0); assert.equal(state.error.value, '');
  } finally { dispose(); }
});
