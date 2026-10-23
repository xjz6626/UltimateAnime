import { ref, watch, onMounted, onUnmounted } from 'vue';
import * as api from '../api.js';
import { useMagnetSearch } from './useMagnetSearch.js';
import { downloadWithDanmaku } from './useDanmakuPicker.js';

// Shared by both pages: each async operation owns a particular detail object.
// Closing/reopening invalidates requests even when the same subject is selected.
export function useAnimeDetail(onOpen = () => {}, onCollectionChange = () => {}) {
  const operationMessage = ref('');
  const selectedItem = ref(null), detailData = ref(null), detailLoading = ref(false), detailError = ref('');
  const showMagnetPicker = ref(false), episodeAction = ref(null), magnetPickerEp = ref(null), magnetSearchKeyword = ref('');
  const { searching: magnetSearching, candidates: magnetCandidates, error: magnetSearchError, search, cancel } = useMagnetSearch();
  let generation = 0;
  const pending = new Set();
  const current = (detail) => detail && detailData.value === detail && !!selectedItem.value;
  const closeDetail = () => { generation++; selectedItem.value = null; detailData.value = null; detailLoading.value = false; showMagnetPicker.value = false; episodeAction.value = null; cancel(); };
  const showDetail = async (item) => {
    closeDetail();
    const request = ++generation;
    const id = item.subject_id || item.id;
    selectedItem.value = item; detailLoading.value = true; detailError.value = ''; operationMessage.value = '';
    onOpen(id);
    try { const detail = await api.GetAnimeDetail(id); if (request === generation && selectedItem.value) detailData.value = detail; }
    catch (error) { if (request === generation) detailError.value = String(error); }
    finally { if (request === generation) detailLoading.value = false; }
  };
  watch(showMagnetPicker, (open) => { if (!open) cancel(); });
  let stopPlayback;
  onMounted(() => { stopPlayback = api.EventsOn('playback-progress', data => {
    const detail = detailData.value;
    if (data.marked_watched && current(detail) && detail.subject.id === data.subject_id && !detail.watched_eps?.includes(data.ep_sort)) {
      detail.watched_eps = [...(detail.watched_eps || []), data.ep_sort];
    }
  }); });
  onUnmounted(() => { closeDetail(); stopPlayback?.(); });

  const downloaded = (ep, detail = detailData.value) => !!detail?.downloaded_eps?.includes(ep.sort);
  const watched = (ep, detail = detailData.value) => !!detail?.watched_eps?.includes(ep.sort);
  const available = (ep, detail = detailData.value) => downloaded(ep, detail) || ep.sort <= (detail?.current_episode || 0);
  const getEpisodeClass = (ep) => {
    if (watched(ep)) return 'bg-green-700 text-white hover:bg-green-600';
    if (downloaded(ep)) return 'bg-blue-700 text-white hover:bg-blue-600';
    if (!available(ep)) return 'bg-gray-800 text-gray-400';
    return 'bg-pink-700 text-white hover:bg-pink-600';
  };
  const toggleWatched = async (ep) => {
    const detail = detailData.value;
    if (!current(detail)) return;
    try {
      const result = await api.ToggleEpisodeWatched(detail.subject.id, ep.sort);
      if (result !== 'Success') { alert(result); return; }
      if (current(detail)) detail.watched_eps = watched(ep, detail) ? detail.watched_eps.filter(v => v !== ep.sort) : [...(detail.watched_eps || []), ep.sort];
    } catch (error) { alert(String(error)); }
  };
  const searchMagnetList = async () => {
    const detail = detailData.value, ep = magnetPickerEp.value;
    if (current(detail) && ep) await search(detail.subject.id, ep.sort, magnetSearchKeyword.value);
  };
  const openMagnetPicker = async (ep) => {
    if (!detailData.value?.is_followed) { alert('请先追番，再选择资源'); return; }
    magnetPickerEp.value = ep; magnetSearchKeyword.value = ''; showMagnetPicker.value = true;
    await searchMagnetList();
  };
  const startEpisodeDownload = async (ep) => {
    const detail = detailData.value;
    if (!current(detail)) return;
    if (!detail.is_followed) { alert('请先追番，再下载这一集'); return; }
    const key = `${detail.subject.id}:${ep.sort}`;
    if (pending.has(key)) return;
    pending.add(key);
    try {
      const autoSelect = await api.GetAutoSelectMagnet();
      if (!current(detail)) return;
      if (!autoSelect) { await openMagnetPicker(ep); return; }
      const magnet = detail.episode_magnets?.[ep.sort] || await api.SearchEpisodeMagnet(detail.subject.id, ep.sort);
      if (!current(detail)) return;
      if (typeof magnet !== 'string' || !magnet.startsWith('magnet:')) { await openMagnetPicker(ep); return; }
      detail.episode_magnets = { ...detail.episode_magnets, [ep.sort]: magnet };
      const result = await downloadWithDanmaku(detail.subject.id, ep.sort, magnet);
      if (!['Started', 'Existing', 'Completed', 'Cancelled'].includes(result)) alert(result);
    } catch (error) { alert('准备下载失败: ' + error); }
    finally { pending.delete(key); }
  };
  const handleEpisodeClick = async (ep, restart = false) => {
    const detail = detailData.value;
    if (!current(detail)) return;
    if (downloaded(ep, detail)) {
      if (api.isWebMode()) { alert('这一集已下载到家中电脑'); return; }
      try {
        const result = await (restart ? api.RestartLocalEpisode : api.PlayLocalEpisode)(detail.subject.id, ep.sort);
        if (!current(detail)) return;
        if (result === 'Error: FileMissing') {
          detail.downloaded_eps = detail.downloaded_eps.filter(v => v !== ep.sort);
          operationMessage.value = '文件已移动或删除，请选择资源重新下载。';
          // Manual recovery remains available even when broadcast metadata is stale.
          await openMagnetPicker(ep);
        } else if (result !== 'Success') alert(result);
      } catch (error) { alert('播放请求失败: ' + error); }
      return;
    }
    if (!available(ep, detail)) { episodeAction.value = ep; return; }
    await startEpisodeDownload(ep);
  };
  const handleEpisodeTap = (ep) => {
    if (api.isWebMode() && window.matchMedia('(max-width: 1023px)').matches) episodeAction.value = ep;
    else return handleEpisodeClick(ep);
  };
  const handleEpisodeContext = (ep) => toggleWatched(ep);
  const downloadSelectedEpisode = () => { const ep = episodeAction.value; episodeAction.value = null; if (ep) return handleEpisodeClick(ep); };
  const restartSelectedEpisode = () => { const ep = episodeAction.value; episodeAction.value = null; if (ep) return handleEpisodeClick(ep, true); };
  const markSelectedEpisode = () => { const ep = episodeAction.value; episodeAction.value = null; if (ep) return toggleWatched(ep); };
  const searchSelectedEpisode = () => { const ep = episodeAction.value; episodeAction.value = null; if (ep) return openMagnetPicker(ep); };
  const selectMagnet = async (item) => {
    const detail = detailData.value, ep = magnetPickerEp.value;
    if (!current(detail) || !ep) return;
    const key = `${detail.subject.id}:${ep.sort}`;
    if (pending.has(key)) return;
    pending.add(key); cancel();
    try {
      const saved = await api.SaveEpisodeMagnet(detail.subject.id, ep.sort, item.magnet);
      if (saved !== 'Success') { alert(saved); return; }
      if (!current(detail)) return;
      detail.episode_magnets = { ...detail.episode_magnets, [ep.sort]: item.magnet };
      const result = await downloadWithDanmaku(detail.subject.id, ep.sort, item.magnet);
      if (['Started', 'Existing', 'Completed'].includes(result)) { if (current(detail)) showMagnetPicker.value = false; }
      else if (result !== 'Cancelled') alert(result);
    } catch (error) { alert(String(error)); }
    finally { pending.delete(key); }
  };
  const handleEpisodeDelete = async (ep) => {
    const detail = detailData.value;
    if (api.isWebMode() || !current(detail)) return;
    if (!confirm(`删除第 ${ep.sort} 集的本地视频和磁力记录？观看历史会保留。`)) return;
    try {
      const result = await api.DeleteEpisodeData(detail.subject.id, ep.sort);
      if (result !== 'Success') { alert(result); return; }
      if (current(detail)) {
        detail.downloaded_eps = (detail.downloaded_eps || []).filter(v => v !== ep.sort);
        if (detail.episode_magnets) delete detail.episode_magnets[ep.sort];
      }
    } catch (error) { alert(String(error)); }
  };
  const toggleFollow = async () => {
    const detail = detailData.value;
    if (!current(detail)) return;
    try {
      const result = detail.is_followed ? await api.UnfollowLocal(detail.subject.id) : await api.FollowLocal({ ...detail.subject, air_date: detail.subject.date });
      if (result !== 'Success') { alert(result); return; }
      if (current(detail)) detail.is_followed = !detail.is_followed;
      await onCollectionChange();
    } catch (error) { alert(String(error)); }
  };
  return { operationMessage, toggleFollow, selectedItem, detailData, detailLoading, detailError, showDetail, closeDetail,
    showMagnetPicker, episodeAction, magnetPickerEp, magnetSearchKeyword, magnetSearching, magnetCandidates, magnetSearchError,
    getEpisodeClass, available, downloaded, watched, searchMagnetList, selectMagnet, handleEpisodeTap, handleEpisodeContext,
    handleEpisodeDelete, downloadSelectedEpisode, restartSelectedEpisode, markSelectedEpisode, searchSelectedEpisode };
}
