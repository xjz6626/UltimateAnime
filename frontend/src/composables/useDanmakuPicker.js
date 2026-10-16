import { shallowRef } from 'vue';
import { DownloadEpisode, GetDanmakuPreferences } from '../api.js';

export const danmakuPicker = shallowRef(null);
let complete;
let sequence = 0;

export function requestDanmakuSource(subjectID, episode, configure = false, episodes = []) {
  if (danmakuPicker.value) return Promise.resolve(null);
  return new Promise((resolve) => {
    complete = resolve;
    danmakuPicker.value = { subjectID, episode, configure, episodes, token: ++sequence };
  });
}

export function finishDanmakuPicker(choice) {
  const resolve = complete;
  complete = undefined;
  danmakuPicker.value = null;
  resolve?.(choice);
}

export async function downloadWithDanmaku(subjectID, episode, magnet) {
  const preferences = await GetDanmakuPreferences();
  if (preferences.enabled && !preferences.auto_select) {
    const choice = await requestDanmakuSource(subjectID, episode);
    if (!choice) return 'Cancelled';
  }
  return DownloadEpisode(subjectID, episode, magnet);
}
