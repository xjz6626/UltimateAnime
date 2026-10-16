<template>
  <div v-if="picker" role="dialog" aria-modal="true" aria-label="选择 B 站弹幕来源" class="fixed inset-0 z-[90] flex items-center justify-center bg-black/80 p-0 backdrop-blur-sm sm:p-4" @click.self="close">
    <div class="flex h-[100dvh] w-full flex-col overflow-hidden bg-gray-900 text-white shadow-2xl sm:h-auto sm:max-h-[90vh] sm:max-w-4xl sm:rounded-xl sm:border sm:border-gray-700">
      <div class="shrink-0 space-y-3 border-b border-gray-700 p-4 sm:p-6">
        <div class="flex items-center justify-between gap-3">
          <h2 class="text-lg font-bold">选择 B 站弹幕 · 第 {{ episode }} 集</h2>
          <button :disabled="saving" @click="close" aria-label="关闭弹幕选择" class="h-11 w-11 text-2xl text-gray-400 disabled:opacity-40">×</button>
        </div>
        <label v-if="picker.configure" class="flex items-center gap-3 text-sm text-gray-300">要设置的集数
          <select v-model.number="episode" @change="initialize" :disabled="saving" class="rounded border border-gray-600 bg-gray-800 p-2">
            <option v-for="ep in picker.episodes" :key="ep.sort" :value="ep.sort">第 {{ ep.sort }} 集 {{ ep.name_cn || ep.name }}</option>
          </select>
        </label>
        <p class="text-sm text-gray-400">选择提供弹幕的 B 站视频，再确认对应集数或分 P。这里只获取弹幕。</p>
        <div class="flex flex-wrap gap-2">
          <button v-for="tab in tabs" :key="tab.kind" @click="kind = tab.kind; search(1)" :disabled="saving" class="rounded px-4 py-2 text-sm" :class="kind === tab.kind ? 'bg-pink-600' : 'bg-gray-800'">{{ tab.label }}</button>
        </div>
        <div class="flex gap-2">
          <input v-model="keyword" @keyup.enter="search(1)" :disabled="saving" placeholder="番剧名、视频关键词、BV 号或番剧链接" class="min-w-0 flex-1 rounded border border-gray-700 bg-gray-800 px-3 py-2 text-base focus:border-pink-500 focus:outline-none">
          <button @click="search(1)" :disabled="saving" class="min-h-[44px] rounded bg-pink-600 px-4">搜索</button>
        </div>
        <div v-if="names.length" class="flex flex-wrap gap-2 text-xs text-gray-400">
          <button v-for="name in names" :key="name" @click="keyword = name; search(1)" :disabled="saving" class="rounded bg-gray-800 px-2 py-1">{{ name }}</button>
        </div>
      </div>
      <div class="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain p-4 sm:p-6">
        <p v-if="error" role="alert" class="text-sm text-red-400">{{ error }}</p>
        <p v-if="loading" role="status" class="text-sm text-gray-400">正在加载 B 站信息…</p>
        <div v-if="details" class="rounded-lg border border-pink-500/50 bg-gray-800 p-4">
          <button @click="details = null" :disabled="saving || loading" class="mb-3 text-sm text-pink-300">← 返回搜索结果</button>
          <h3 class="mb-2 font-bold">{{ details.title }}</h3>
          <label class="block text-sm text-gray-300">确认 B 站集数／分 P
            <select v-model.number="cid" :disabled="saving || loading" class="mt-2 w-full rounded border border-gray-600 bg-gray-900 p-3">
              <option :value="0" disabled>请选择对应集数或分 P</option>
              <option v-for="part in details.parts" :key="part.cid" :value="part.cid">{{ part.title }}</option>
            </select>
          </label>
          <p class="mt-2 text-xs text-gray-400">请核对标题和集数；不同剪辑版本的弹幕时间可能有偏移。</p>
        </div>
        <template v-else>
          <p v-if="!loading && !candidates.length && !error" class="py-8 text-center text-gray-400">暂无结果，可换关键词或直接粘贴 B 站链接。</p>
          <button v-for="item in candidates" :key="item.kind + item.id" @click="loadParts(item)" :disabled="saving" class="block w-full rounded-lg border border-gray-700 bg-gray-800 p-4 text-left hover:border-pink-500">
            <h3 class="font-medium">{{ item.title }}</h3>
            <p class="mt-1 text-xs text-gray-400">{{ item.kind === 'season' ? '番剧' : '普通视频' }} · {{ item.id }} <span v-if="item.author">· {{ item.author }}</span><span v-if="item.episode_count"> · {{ item.episode_count }} 集／P</span></p>
            <p v-if="item.description" class="mt-2 max-h-12 overflow-hidden text-xs text-gray-400">{{ item.description }}</p>
          </button>
          <div class="flex items-center justify-center gap-4 pt-2 text-sm">
            <button @click="search(page - 1)" :disabled="page <= 1 || loading || saving" class="rounded bg-gray-800 px-3 py-2 disabled:opacity-30">上一页</button>
            <span>第 {{ page }} 页</span>
            <button @click="search(page + 1)" :disabled="page >= 50 || !candidates.length || loading || saving" class="rounded bg-gray-800 px-3 py-2 disabled:opacity-30">下一页</button>
          </div>
        </template>
      </div>
      <div class="flex shrink-0 flex-wrap justify-end gap-2 border-t border-gray-700 p-4">
        <button @click="save({ kind: 'skip' })" :disabled="saving" class="min-h-[44px] rounded bg-gray-800 px-4 text-sm">{{ picker.configure ? '该集不使用弹幕' : '仅下载视频' }}</button>
        <button @click="save({ kind: 'auto' })" :disabled="saving" class="min-h-[44px] rounded bg-gray-700 px-4 text-sm">自动匹配</button>
        <button @click="saveSelected" :disabled="!details || !cid || loading || saving" class="min-h-[44px] rounded bg-pink-600 px-4 font-bold disabled:bg-gray-700 disabled:text-gray-500">{{ saving ? '保存中…' : picker.configure ? '保存并更新弹幕' : '确认弹幕并下载' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue';
import { danmakuPicker as picker, finishDanmakuPicker } from '../composables/useDanmakuPicker';
import { GetEpisodeDanmakuSource, SearchBilibiliDanmaku, GetBilibiliDanmakuParts, SaveEpisodeDanmakuSource, RefreshEpisodeDanmaku } from '../api';

const tabs = [{ kind: 'season', label: '番剧' }, { kind: 'video', label: '普通视频' }];
const keyword = ref(''), kind = ref('season'), names = ref([]), episode = ref(1);
const candidates = ref([]), details = ref(null), cid = ref(0), page = ref(1);
const loading = ref(false), saving = ref(false), error = ref('');
let generation = 0;
const close = () => { if (!saving.value) { generation++; finishDanmakuPicker(null); } };
const keydown = (event) => { if (picker.value && event.key === 'Escape') { event.stopImmediatePropagation(); close(); } };
onMounted(() => window.addEventListener('keydown', keydown, true));
onUnmounted(() => { window.removeEventListener('keydown', keydown, true); finishDanmakuPicker(null); });

async function search(nextPage = 1) {
  if (!picker.value || saving.value) return;
  const current = ++generation;
  loading.value = true; error.value = ''; details.value = null; candidates.value = []; page.value = nextPage;
  try {
    const results = await SearchBilibiliDanmaku(keyword.value, kind.value, nextPage);
    if (current === generation) candidates.value = results || [];
  } catch (err) { if (current === generation) error.value = String(err); }
  finally { if (current === generation) loading.value = false; }
}
async function loadParts(item, selectedCID = 0) {
  const current = ++generation;
  loading.value = true; error.value = ''; cid.value = 0;
  try {
    const result = await GetBilibiliDanmakuParts(item.kind, item.id);
    if (current !== generation) return;
    details.value = result;
    const matched = result.parts.find(part => part.cid === selectedCID) || result.parts.find(part => part.number === episode.value) || (result.parts.length === 1 ? result.parts[0] : undefined);
    cid.value = matched?.cid || 0;
  } catch (err) { if (current === generation) error.value = String(err); }
  finally { if (current === generation) loading.value = false; }
}
async function initialize() {
  const current = ++generation;
  loading.value = true; error.value = ''; details.value = null; candidates.value = [];
  const request = picker.value;
  try {
    const context = await GetEpisodeDanmakuSource(request.subjectID, episode.value);
    if (current !== generation) return;
    names.value = [...new Set(context.names.filter(Boolean))];
    keyword.value = names.value[0] || '';
    const choice = context.choice;
    if (choice.kind === 'season' || choice.kind === 'video') {
      kind.value = choice.kind;
      await loadParts(choice, choice.cid);
    } else { kind.value = 'season'; await search(1); }
  } catch (err) { if (current === generation) error.value = String(err); }
  finally { if (current === generation) loading.value = false; }
}
async function save(choice) {
  if (saving.value || !picker.value) return;
  saving.value = true; error.value = '';
  const request = picker.value;
  try {
    const result = await SaveEpisodeDanmakuSource(request.subjectID, episode.value, choice);
    if (result !== 'Success') throw new Error(result);
    if (request.configure) {
      const refreshed = await RefreshEpisodeDanmaku(request.subjectID, episode.value);
      if (refreshed !== 'Started' && refreshed !== 'Success') throw new Error(refreshed);
    }
    generation++; finishDanmakuPicker(choice);
  } catch (err) { error.value = String(err); }
  finally { saving.value = false; }
}
function saveSelected() { if (details.value && cid.value) save({ kind: details.value.kind, id: details.value.id, cid: cid.value }); }
watch(() => picker.value?.token, (token) => { generation++; if (token) { episode.value = picker.value.episode; initialize(); } });
</script>
