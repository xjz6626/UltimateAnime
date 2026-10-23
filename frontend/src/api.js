// The same Vue views run inside Wails and in the browser served by the Go app.
export const isWebMode = () => window.__ULTIMATEANIME_WEB__ === true;

function invoke(method, ...args) {
  return invokeWithSignal(undefined, method, ...args);
}

async function invokeWithSignal(signal, method, ...args) {
  if (!isWebMode()) {
    return window.go.main.App[method](...args);
  }
  const response = await fetch('/api/rpc', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method, args }),
    signal,
  });
  if (!response.ok) {
    throw new Error((await response.text()).trim());
  }
  return (await response.json()).result;
}

export const GetPikPakStatus = () => invoke('GetPikPakStatus');
export const GetAutoSelectMagnet = () => invoke('GetAutoSelectMagnet');
export const GetBangumiCalendar = () => invoke('GetBangumiCalendar');
export const GetLocalFollows = () => invoke('GetLocalFollowsChecked');
export const GetFollowAirings = () => invoke('GetFollowAirings');
export const GetAnimeDetail = (id) => invoke('GetAnimeDetail', id);
export const FollowLocal = (item) => invoke('FollowLocal', item);
export const UnfollowLocal = (id) => invoke('UnfollowLocal', id);
export const ToggleEpisodeWatched = (id, episode) => invoke('ToggleEpisodeWatched', id, episode);
export const SearchEpisodeMagnet = (id, episode) => invoke('SearchEpisodeMagnet', id, episode);
export const SearchEpisodeMagnetList = (id, episode, keywords) => invoke('SearchEpisodeMagnetList', id, episode, keywords);
export const SaveEpisodeMagnet = (id, episode, magnet) => invoke('SaveEpisodeMagnet', id, episode, magnet);
export const DownloadEpisode = (id, episode, magnet) => invoke('DownloadEpisode', id, episode, magnet);
export const PlayMagnet = (magnet) => invoke('PlayMagnet', magnet);
export const PlayLocalEpisode = (id, episode) => invoke('PlayLocalEpisode', id, episode);
export const RestartLocalEpisode = (id, episode) => invoke('RestartLocalEpisode', id, episode);
export const GetContinueWatching = () => invoke('GetContinueWatching');
export const HideContinueWatching = (id, episode) => invoke('HideContinueWatching', id, episode);
export const GetNextPlayback = () => invoke('GetNextPlayback');
export const CancelNextPlayback = () => invoke('CancelNextPlayback');
export const DeleteEpisodeData = (id, episode) => invoke('DeleteEpisodeData', id, episode);
export const GetLogs = () => invoke('GetLogs');
export const GetAppConfig = () => invoke('GetAppConfig');
export const SaveAppConfig = (config) => invoke('SaveAppConfig', config);
export const GetBlockedAccounts = () => invoke('GetBlockedAccounts');
export const SetAccountBlockStatus = (username, blocked) => invoke('SetAccountBlockStatus', username, blocked);
export const ClearPikPakStorage = (username) => invoke('ClearPikPakStorage', username);
export const GetDanmakuPreferences = () => invoke('GetDanmakuPreferences');
export const GetEpisodeDanmakuSource = (id, episode) => invoke('GetEpisodeDanmakuSource', id, episode);
export const SearchBilibiliDanmaku = (keyword, kind, page) => invoke('SearchBilibiliDanmaku', keyword, kind, page);
export const GetBilibiliDanmakuParts = (kind, id) => invoke('GetBilibiliDanmakuParts', kind, id);
export const SaveEpisodeDanmakuSource = (id, episode, source) => invoke('SaveEpisodeDanmakuSource', id, episode, source);
export const RefreshEpisodeDanmaku = (id, episode) => invoke('RefreshEpisodeDanmaku', id, episode);
export const StartBilibiliLogin = () => invoke('StartBilibiliLogin');
export const PollBilibiliLogin = (key) => invoke('PollBilibiliLogin', key);
export const CancelBilibiliLogin = (key) => invoke('CancelBilibiliLogin', key);
export const GetBilibiliLoginStatus = () => invoke('GetBilibiliLoginStatus');
export const ClearBilibiliLogin = () => invoke('ClearBilibiliLogin');

const listeners = new Map();
let eventSource;

function waitForEventStream(signal) {
  if (!isWebMode() || eventSource?.readyState === EventSource.OPEN) return Promise.resolve();
  const source = eventSource;
  return new Promise((resolve, reject) => {
    const finish = (error) => {
      clearTimeout(timeout);
      source.removeEventListener('open', opened);
      signal?.removeEventListener('abort', aborted);
      error ? reject(error) : resolve();
    };
    const opened = () => finish();
    const aborted = () => finish(new DOMException('搜索已取消', 'AbortError'));
    // If SSE is temporarily unavailable, the RPC still returns a complete list.
    const timeout = setTimeout(opened, 3000);
    source.addEventListener('open', opened, { once: true });
    signal?.addEventListener('abort', aborted, { once: true });
    if (signal?.aborted) aborted();
  });
}

export async function StreamEpisodeMagnets(id, episode, keywords, onUpdate, signal) {
  const requestID = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const stop = EventsOn('magnet-search-results', (update) => {
    if (!signal?.aborted && update.request_id === requestID) onUpdate(update.items || []);
  });
  const cancel = () => { invoke('CancelEpisodeMagnetSearch', requestID).catch(() => {}); };
  let started = false;
  try {
    await waitForEventStream(signal);
    if (signal?.aborted) return [];
    signal?.addEventListener('abort', cancel, { once: true });
    started = true;
    const items = await invokeWithSignal(signal, 'SearchEpisodeMagnetListStream', id, episode, keywords, requestID);
    if (!signal?.aborted) onUpdate(items || []);
    return items || [];
  } finally {
    stop();
    signal?.removeEventListener('abort', cancel);
    if (started && signal?.aborted) cancel();
  }
}

export function EventsOn(name, callback) {
  if (!isWebMode()) {
    return window.runtime.EventsOn(name, callback);
  }
  if (!listeners.has(name)) listeners.set(name, new Set());
  listeners.get(name).add(callback);
  if (!eventSource) {
    eventSource = new EventSource('/api/events');
    eventSource.addEventListener('open', () => {
      for (const listener of listeners.get('connection-restored') || []) listener();
    });
    eventSource.onmessage = (event) => {
      const message = JSON.parse(event.data);
      for (const listener of listeners.get(message.name) || []) {
        listener(message.data);
      }
    };
  }
  return () => {
    listeners.get(name)?.delete(callback);
    if ([...listeners.values()].every((set) => set.size === 0)) {
      eventSource.close();
      eventSource = undefined;
    }
  };
}

export const GetDownloadTasks = () => invoke('GetDownloadTasks');
export const RetryDownload = (id) => invoke('RetryDownload', id);
export const PauseDownload = (id) => invoke('PauseDownload', id);
export const CancelDownload = (id) => invoke('CancelDownload', id);
