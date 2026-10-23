import test from 'node:test';
import assert from 'node:assert/strict';
import { mountComponent } from './testUtils/renderComponent.js';

const deferred = () => { let resolve; const promise = new Promise(yes => { resolve = yes; }); return { promise, resolve }; };
const task = (id, state = 'downloading') => ({ id: `${id}:1`, subject_id: id, ep_sort: 1, name: `作品${id}`, state, progress: 50, updated_at: '2026-10-22T10:00:00Z' });

test('download events during initial snapshot preserve both history and the freshest progress', async () => {
  const snapshot = deferred();
  const view = await mountComponent('../views/Downloads.vue', { GetDownloadTasks: () => snapshot.promise });
  try {
    view.events.get('download-task')({ ...task(1), progress: 75 });
    snapshot.resolve([task(1), task(2, 'completed')]); await view.flush();
    assert.match(view.text(), /作品2/, 'initial task history disappeared behind a progress event');
    assert.match(view.text(), /75.0%/, 'snapshot overwrote newer progress');
    assert.deepEqual(view.errors, []); assert.deepEqual(view.warnings, []);
  } finally { view.dispose(); }
});

test('download buttons await stopping state and keep action errors through refresh', async () => {
  let current = task(1); const pause = deferred(); const calls = [];
  const view = await mountComponent('../views/Downloads.vue', {
    GetDownloadTasks: async () => [current],
    PauseDownload: id => { calls.push(id); return pause.promise; },
    CancelDownload: async () => 'Error: 缓存无法删除',
  });
  try {
    const pausing = view.button('暂停').props.onClick(); await view.flush();
    assert.equal(view.button('暂停').props.disabled, true);
    current = task(1, 'pausing'); view.events.get('download-task')(current);
    pause.resolve('Success'); await pausing; await view.flush();
    assert.equal(view.button('重试 / 继续'), undefined);
    current = task(1, 'paused'); view.events.get('download-task')(current); await view.flush();
    assert.ok(view.button('重试 / 继续'));
    await view.button('取消并清理缓存').props.onClick(); await view.flush();
    await view.button('刷新').props.onClick(); await view.flush();
    assert.match(view.text(), /缓存无法删除/); assert.deepEqual(calls, ['1:1']);
    assert.deepEqual(view.errors, []); assert.deepEqual(view.warnings, []);
  } finally { view.dispose(); }
  assert.equal(view.events.size, 0);
});

test('snapshot failure is not presented as an empty download history', async () => {
  const view = await mountComponent('../views/Downloads.vue', { GetDownloadTasks: async () => { throw new Error('记录损坏'); } });
  try {
    assert.match(view.text(), /记录损坏/);
    assert.doesNotMatch(view.text(), /暂无下载任务/);
  } finally { view.dispose(); }
});

test('next episode banner ignores old snapshot and late cancellation of a different countdown', async () => {
  const snapshot = deferred(), cancel = deferred();
  const pending = ep => ({ subject_id: 42, ep_sort: ep, starts_at: new Date(Date.now() + 8000).toISOString() });
  const view = await mountComponent('../components/NextPlaybackBanner.vue', { GetNextPlayback: () => snapshot.promise, CancelNextPlayback: () => cancel.promise });
  try {
    view.events.get('playback-next')(pending(2)); snapshot.resolve(null); await view.flush();
    assert.match(view.text(), /第 2 集/);
    const cancelling = view.button('取消自动播放').props.onClick();
    view.events.get('playback-next')(pending(3)); cancel.resolve(); await cancelling; await view.flush();
    assert.match(view.text(), /第 3 集/);
    view.events.get('playback-next')(null); await view.flush();
    assert.equal(view.button('取消自动播放'), undefined);
    assert.deepEqual(view.errors, []); assert.deepEqual(view.warnings, []);
  } finally { view.dispose(); }
  assert.equal(view.events.size, 0);
});
