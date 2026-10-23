<template>
  <section v-if="items.length || error || loadError" class="mb-6" aria-label="继续观看">
    <h2 class="mb-3 text-lg font-bold">继续观看 <span v-if="isWebMode()" class="text-sm font-normal text-gray-400">家中电脑的观看进度</span></h2>
    <p v-if="error || loadError" role="alert" class="mb-3 text-red-300">{{ error || loadError }} <button @click="refresh" class="underline">刷新</button></p>
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <article v-for="item in items" :key="item.subject_id" class="rounded-lg border border-gray-700 bg-gray-800 p-4">
        <h3 class="truncate font-bold" :title="item.name">{{ item.name }} · 第 {{ item.ep_sort }} 集</h3>
        <p class="my-2 text-sm text-gray-400">{{ formatTime(item.position) }} / {{ item.duration > 0 ? formatTime(item.duration) : '时长未知' }}</p>
        <progress v-if="item.duration > 0" :value="item.position" :max="item.duration" class="w-full accent-pink-500" aria-label="观看进度"></progress>
        <p v-if="!item.available" class="my-2 text-sm text-amber-300">原视频已移动、删除或更换，请在番剧详情中处理。</p>
        <div class="mt-3 flex flex-wrap gap-3 text-sm">
          <template v-if="!isWebMode()">
            <button @click="play(item)" :disabled="!item.available || busy[key(item)]" class="rounded bg-pink-700 px-3 py-2 disabled:opacity-50">继续播放</button>
            <button @click="restart(item)" :disabled="!item.available || busy[key(item)]" class="rounded bg-gray-700 px-3 py-2 disabled:opacity-50">从头播放</button>
          </template>
          <button @click="hide(item)" :disabled="busy[key(item)]" class="px-2 py-2 text-gray-300 disabled:opacity-50" title="只从继续观看中移除，保留进度和观看记录">移除此卡片</button>
        </div>
      </article>
    </div>
  </section>
</template>
<script setup>
import { isWebMode } from '../api.js';
import { useContinueWatching } from '../composables/useContinueWatching.js';
const { items, error, loadError, busy, refresh, play, restart, hide } = useContinueWatching();
const key = item => `${item.subject_id}:${item.ep_sort}`;
const formatTime = seconds => {
  const total = Math.max(0, Math.floor(seconds || 0));
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
};
</script>
