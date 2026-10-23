import test from 'node:test';
import assert from 'node:assert/strict';
import { createRenderer } from 'vue';
import { useAnimeDetail } from './composables/useAnimeDetail.js';

const deferred = () => { let resolve, reject; const promise = new Promise((yes,no) => { resolve=yes; reject=no; }); return { promise,resolve,reject }; };
const detail = (id, extra = {}) => ({ subject: { id }, current_episode: 2, is_followed: true, watched_eps: [], downloaded_eps: [], ...extra });
function setup(api) {
 globalThis.alert = () => {};
 globalThis.confirm = () => true;
 globalThis.window = { go: { main: { App: api } }, runtime: { EventsOn: () => () => {} }, matchMedia: () => ({ matches: false }) };
 const renderer = createRenderer({ createComment: () => ({}), insert(){}, remove(){}, parentNode(){}, nextSibling(){} });
 let state;
 const app = renderer.createApp({ setup() { state = useAnimeDetail(); return () => null; } });
 app.mount({});
 return { state, dispose: () => app.unmount() };
}

test('out-of-order responses and closing/reopening cannot replace the selected detail', async () => {
 const a = deferred(), b = deferred(), reopened = deferred();
 let calls = 0;
 const { state, dispose } = setup({ GetAnimeDetail: () => [a,b,reopened][calls++].promise });
 try {
  const first = state.showDetail({ id:1 }); const second = state.showDetail({ id:2 });
  b.resolve(detail(2)); await second;
  a.resolve(detail(1)); await first;
  assert.equal(state.detailData.value.subject.id,2);
  assert.equal(state.detailLoading.value,false);
  const pending = state.showDetail({ id:2 }); state.closeDetail();
  reopened.reject(new Error('late failure')); await pending;
  assert.equal(state.detailData.value,null); assert.equal(state.detailError.value,'');
 } finally { dispose(); }
});

test('stale watching mutation stays on its captured subject', async () => {
 const mutation = deferred(); const calls = [];
 const { state, dispose } = setup({ GetAnimeDetail: async id => detail(id), ToggleEpisodeWatched: (...args) => { calls.push(args); return mutation.promise; } });
 try {
  await state.showDetail({ id:1 }); const pending = state.markSelectedEpisode;
  state.episodeAction.value = { sort:1 }; const marking = pending();
  await state.showDetail({ id:2 }); mutation.resolve('Success'); await marking;
  assert.deepEqual(calls,[[1,1]]); assert.deepEqual(state.detailData.value.watched_eps,[]);
 } finally { dispose(); }
});

test('switching shows during automatic preference lookup cannot download the new show', async () => {
 const preference = deferred(); const downloads = [];
 const { state, dispose } = setup({ GetAnimeDetail: async id => detail(id), GetAutoSelectMagnet: () => preference.promise, DownloadEpisode: (...args) => downloads.push(args) });
 try {
  await state.showDetail({ id:1 }); const downloading = state.handleEpisodeTap({ sort:1 });
  await state.showDetail({ id:2 }); preference.resolve(true); await downloading;
  assert.deepEqual(downloads,[]);
 } finally { dispose(); }
});

test('missing local video clears downloaded state and opens manual recovery', async () => {
 const { state, dispose } = setup({ GetAnimeDetail: async id => detail(id,{ downloaded_eps:[1] }), PlayLocalEpisode: async () => 'Error: FileMissing', SearchEpisodeMagnetListStream: async () => [] });
 try {
  await state.showDetail({ id:1 }); await state.handleEpisodeTap({ sort:1 });
  assert.deepEqual(state.detailData.value.downloaded_eps,[]);
  assert.equal(state.showMagnetPicker.value,true);
 } finally { dispose(); }
});

test('downloaded and watched states coexist even with unknown airing metadata', async () => {
 let plays = 0;
 const { state, dispose } = setup({ GetAnimeDetail: async id => detail(id,{ current_episode:0, downloaded_eps:[1],watched_eps:[1] }),PlayLocalEpisode: async () => { plays++;return 'Success'; } });
 try {
  await state.showDetail({ id:1 });const ep={sort:1};
  assert.equal(state.downloaded(ep),true);assert.equal(state.watched(ep),true);assert.equal(state.available(ep),true);
  assert.match(state.getEpisodeClass(ep),/green/);await state.handleEpisodeTap(ep);assert.equal(plays,1);
  await state.handleEpisodeTap({sort:2});assert.equal(state.episodeAction.value.sort,2);
 } finally { dispose(); }
});
