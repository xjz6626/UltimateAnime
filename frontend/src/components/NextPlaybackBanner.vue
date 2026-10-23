<template>
  <aside v-if="pending || error" class="sticky top-0 z-40 flex flex-wrap items-center justify-between gap-3 border-b border-pink-800 bg-gray-800 p-4" aria-live="polite">
    <span v-if="pending">{{ remaining }} 秒后播放第 {{ pending.ep_sort }} 集</span>
    <span v-if="error" role="alert" class="text-red-300">{{ error }}</span>
    <button v-if="pending" @click="cancel" :disabled="busy" class="rounded bg-pink-700 px-4 py-2 disabled:opacity-50">取消自动播放</button>
    <button v-else @click="error = ''" class="rounded bg-gray-700 px-4 py-2">关闭</button>
  </aside>
</template>
<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { EventsOn, GetNextPlayback, CancelNextPlayback } from '../api.js';
const pending = ref(null), error = ref(''), busy = ref(false), now = ref(Date.now());
const remaining = computed(() => Math.max(0, Math.ceil((Date.parse(pending.value?.starts_at) - now.value) / 1000)));
let generation = 0, alive = true, timer, stops = [];
const cancel = async () => {
  const requested = pending.value;
  generation++; busy.value = true;
  try { await CancelNextPlayback(); if (alive) { if (pending.value === requested) pending.value = null; error.value = ''; } }
  catch (err) { if (alive) error.value = String(err); }
  finally { busy.value = false; }
};
onMounted(async () => {
  stops = [EventsOn('playback-next', value => { generation++; pending.value = value; now.value = Date.now(); }), EventsOn('playback-error', value => { error.value = value; })];
  timer = setInterval(() => { now.value = Date.now(); }, 250);
  const request = ++generation;
  try { const value = await GetNextPlayback(); if (alive && request === generation) pending.value = value; }
  catch (err) { if (alive) error.value = String(err); }
});
onUnmounted(() => { alive = false; generation++; clearInterval(timer); stops.forEach(stop => stop()); });
</script>
