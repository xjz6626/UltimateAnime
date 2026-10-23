import { ref, onMounted, onUnmounted } from 'vue';
import { GetContinueWatching, HideContinueWatching, PlayLocalEpisode, RestartLocalEpisode, EventsOn } from '../api.js';

export function useContinueWatching() {
  const items = ref([]), error = ref(''), loadError = ref(''), busy = ref({});
  let generation = 0, alive = true, timer, stops = [];
  const refresh = async () => {
    const request = ++generation;
    try {
      const result = await GetContinueWatching();
      if (alive && request === generation) { items.value = result || []; loadError.value = ''; }
    } catch (err) { if (alive && request === generation) loadError.value = String(err); }
  };
  const perform = async (item, operation) => {
    const key = `${item.subject_id}:${item.ep_sort}`;
    if (busy.value[key]) return;
    busy.value[key] = true; error.value = '';
    try {
      const result = await operation(item.subject_id, item.ep_sort);
      if (!alive) return;
      if (result !== 'Success') error.value = result === 'Error: FileMissing' ? '文件已移动或删除，请在番剧详情中重新下载。' : result;
      await refresh();
    } catch (err) { if (alive) error.value = String(err); }
    finally { delete busy.value[key]; }
  };
  onMounted(() => {
    stops = ['playback-progress', 'download-complete', 'connection-restored'].map(event => EventsOn(event, refresh));
    refresh(); timer = setInterval(refresh, 15000);
  });
  onUnmounted(() => { alive = false; generation++; clearInterval(timer); stops.forEach(stop => stop()); });
  return { items, error, loadError, busy, refresh,
    play: (item) => perform(item, PlayLocalEpisode),
    restart: (item) => perform(item, RestartLocalEpisode),
    hide: (item) => perform(item, HideContinueWatching) };
}
