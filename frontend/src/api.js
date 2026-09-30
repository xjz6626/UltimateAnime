// The same Vue views run inside Wails and in the browser served by the Go app.
export const isWebMode = () => window.__ULTIMATEANIME_WEB__ === true;

async function invoke(method, ...args) {
  if (!isWebMode()) {
    return window.go.main.App[method](...args);
  }
  const response = await fetch('/api/rpc', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ method, args }),
  });
  if (!response.ok) {
    throw new Error((await response.text()).trim());
  }
  return (await response.json()).result;
}

export const GetPikPakStatus = () => invoke('GetPikPakStatus');
export const GetBangumiCalendar = () => invoke('GetBangumiCalendar');
export const GetLocalFollows = () => invoke('GetLocalFollows');
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
export const DeleteEpisodeData = (id, episode) => invoke('DeleteEpisodeData', id, episode);
export const GetLogs = () => invoke('GetLogs');
export const GetAppConfig = () => invoke('GetAppConfig');
export const SaveAppConfig = (config) => invoke('SaveAppConfig', config);
export const GetBlockedAccounts = () => invoke('GetBlockedAccounts');
export const SetAccountBlockStatus = (username, blocked) => invoke('SetAccountBlockStatus', username, blocked);
export const ClearPikPakStorage = (username) => invoke('ClearPikPakStorage', username);

const listeners = new Map();
let eventSource;

export function EventsOn(name, callback) {
  if (!isWebMode()) {
    return window.runtime.EventsOn(name, callback);
  }
  if (!listeners.has(name)) listeners.set(name, new Set());
  listeners.get(name).add(callback);
  if (!eventSource) {
    eventSource = new EventSource('/api/events');
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
