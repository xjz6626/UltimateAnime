<template>
  <div class="p-4 sm:p-6">
    <div class="mb-5 flex items-center justify-between"><h1 class="text-2xl font-bold">下载任务</h1><button @click="refresh" class="rounded bg-gray-700 px-4 py-2">刷新</button></div>
    <p v-if="error" role="alert" class="mb-4 text-red-300">{{ error }}</p>
    <p v-if="loadError" role="alert" class="mb-4 text-red-300">{{ loadError }}</p>
    <p v-if="loading && !tasks.length" class="text-gray-400">正在读取下载任务…</p>
    <p v-if="!loading && !tasks.length && !error && !loadError" class="text-gray-400">暂无下载任务</p>
    <p v-if="tasks.length" class="mb-4 text-sm text-gray-400">暂停保留已完成的分块；取消清理未完成的本地缓存。PikPak 云端离线任务可能继续运行。</p>
    <div class="space-y-3">
      <article v-for="task in tasks" :key="task.id" class="rounded-lg border border-gray-700 bg-gray-800 p-4">
        <div class="flex flex-wrap items-center justify-between gap-2"><h2 class="font-bold">{{ task.name }} · 第 {{ task.ep_sort }} 集</h2><span :class="task.state === 'failed' ? 'text-red-300' : 'text-pink-300'">{{ states[task.state] || task.state }}</span></div>
        <progress v-if="task.state === 'downloading'" :value="task.progress" max="100" class="my-3 w-full accent-pink-500" :aria-label="`第 ${task.ep_sort} 集下载进度`"></progress>
        <p v-if="task.state === 'downloading'" class="text-sm text-gray-300">{{ task.progress.toFixed(1) }}%</p>
        <p v-if="['paused', 'interrupted'].includes(task.state)" class="mt-2 text-sm text-gray-300">上次进度 {{ task.progress.toFixed(1) }}%，继续时将校验已完成的分块。</p>
        <p v-if="task.state === 'cloud_pending'" class="mt-2 text-sm text-gray-400">正在跟踪已有云端任务，无需重复提交。</p>
        <p class="mt-2 text-sm text-gray-400">弹幕：{{ danmakuStates[task.danmaku_state] || '等待视频任务' }}</p>
        <p v-if="task.error" class="mt-2 break-words text-sm text-red-300">{{ task.error }}</p>
        <div class="mt-3 flex flex-wrap gap-3">
          <button v-if="['failed', 'interrupted', 'paused', 'cancelled'].includes(task.state)" @click="control(task, RetryDownload)" :disabled="busy[task.id]" class="rounded bg-pink-700 px-4 py-2 disabled:opacity-50">{{ task.state === 'cancelled' ? '重新下载' : '重试 / 继续' }}</button>
          <button v-if="['queued', 'cloud_pending', 'downloading'].includes(task.state)" @click="control(task, PauseDownload)" :disabled="busy[task.id]" class="rounded bg-gray-700 px-4 py-2 disabled:opacity-50">暂停</button>
          <button v-if="!['completed', 'cancelled', 'cancelling'].includes(task.state)" @click="control(task, CancelDownload)" :disabled="busy[task.id]" class="rounded border border-gray-600 px-4 py-2 disabled:opacity-50">取消并清理缓存</button>
          <button v-if="task.state === 'completed'" @click="refreshDanmaku(task)" :disabled="busy[task.id]" class="rounded bg-gray-700 px-4 py-2 disabled:opacity-50">重新获取弹幕</button>
        </div>
      </article>
    </div>
  </div>
</template>
<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { GetDownloadTasks, RetryDownload, PauseDownload, CancelDownload, RefreshEpisodeDanmaku, EventsOn } from '../api.js';
const tasks = ref([]), error = ref(''), busy = ref({}), loadError = ref(''), loading = ref(true);
const states = { queued: '排队中', cloud_pending: '云端准备中', downloading: '下载到本机', completed: '视频已就绪', failed: '失败，可重试', interrupted: '已中断，可继续', pausing: '正在暂停', paused: '已暂停，缓存已保留', cancelling: '正在取消', cancelled: '已取消' };
const danmakuStates = { preparing: '准备中', ready: '已就绪', unavailable: '未获取或已跳过，可重新选择来源', interrupted: '已中断，可随下载继续或在视频就绪后重新获取' };
let generation = 0, eventVersion = 0, alive = true, timer, stops = [];
const recentEvents = new Map();
const refresh = async () => {
  if (!alive) return;
  const request = ++generation;
  const since = eventVersion;
  loading.value = true;
  try {
    const result = await GetDownloadTasks();
    if (alive && request === generation) {
      const merged = new Map((result || []).map(task => [task.id, task]));
      // Keep the full snapshot, overlaying only events received during this read.
      for (const [id, update] of recentEvents) {
        if (update.version > since) merged.set(id, update.task);
        else recentEvents.delete(id);
      }
      tasks.value = [...merged.values()].sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at));
      loadError.value = '';
    }
  }
  catch (err) { if (alive && request === generation) loadError.value = String(err); }
  finally { if (alive && request === generation) loading.value = false; }
};
const control = async (task, action) => {
  if (busy.value[task.id]) return;
  busy.value[task.id] = true; error.value = '';
  try { const result = await action(task.id); if (!alive) return; if (!['Success', 'Started', 'Existing', 'Completed'].includes(result)) error.value = result; else await refresh(); }
  catch (err) { if (alive) error.value = String(err); }
  finally { delete busy.value[task.id]; }
};
const refreshDanmaku = (task) => control(task, () => RefreshEpisodeDanmaku(task.subject_id, task.ep_sort));
onMounted(() => {
  stops = [EventsOn('download-task', (task) => {
    recentEvents.set(task.id, { task, version: ++eventVersion });
    tasks.value = [task, ...tasks.value.filter(t => t.id !== task.id)];
  }), EventsOn('connection-restored', refresh)];
  refresh(); timer = setInterval(refresh, 10000);
});
onUnmounted(() => { alive = false; generation++; clearInterval(timer); stops.forEach(stop => stop()); });
</script>
