<template>
  <div class="p-4 sm:p-6">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl sm:text-2xl font-bold text-white">📺 我的追番 {{ isWebMode() ? '(家中电脑)' : '(本地)' }}</h1>
      <button v-if="collection.length" type="button" @click="toggleFinishedFilter" :aria-pressed="hideFinished"
              class="rounded-lg border border-gray-600 bg-gray-800 px-3 py-1.5 text-sm text-gray-200 hover:border-pink-500 hover:text-white">
        {{ hideFinished ? '显示完结' : '隐藏完结' }}<span v-if="finishedCount"> ({{ finishedCount }})</span>
      </button>
    </div>
    
    <ContinueWatching />
    <div v-if="loading" class="text-gray-400">加载中...</div>
    <div v-else-if="loadError" role="alert" class="text-red-300">{{ loadError }} <button @click="fetchCollection" class="underline">重试</button></div>
    <div v-else-if="collection.length === 0" class="text-gray-500 text-center mt-10">
      <p>你还没有正在追的番剧哦~</p>
      <p class="text-sm mt-2">去 <router-link to="/discovery" class="text-pink-500 hover:underline">当季新番</router-link> 看看吧！</p>
    </div>

    <div v-else-if="visibleCollection.length === 0" class="text-gray-500 text-center mt-10">
      完结的番剧已隐藏。点击“显示完结”可以查看。
    </div>

    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 sm:gap-4 md:grid-cols-4 lg:grid-cols-5">
      <div v-for="item in visibleCollection" :key="item.subject_id" @click="showDetail(item)" class="bg-gray-800 rounded-lg overflow-hidden hover:scale-105 transition cursor-pointer shadow-lg group relative">
        <div class="relative aspect-[2/3]">
          <img :src="proxyImg(item.image)" @error="onImgError($event, item.image)" referrerpolicy="no-referrer" class="w-full h-full object-cover"/>
          <span v-if="hasNewUpdate(item.subject_id)" aria-label="未读的番剧更新提醒" title="有未读更新，点击番剧后清除提醒"
                class="absolute right-2 top-2 z-10 rounded-full bg-pink-500 px-2 py-0.5 text-[10px] font-bold tracking-wide text-white shadow-md ring-1 ring-white/20">NEW</span>
          <div class="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-40 transition flex items-center justify-center">
            <span class="text-white opacity-0 group-hover:opacity-100 text-4xl">▶</span>
          </div>
        </div>
        <div class="p-3">
          <div class="font-bold text-gray-200 truncate text-sm" :title="item.name_cn || item.name">
            {{ item.name_cn || item.name }}
          </div>
          <div class="text-xs text-gray-500 mt-1 truncate">{{ item.name }}</div>
          <div v-if="airings[item.subject_id]?.release_status === 'FINISHED'" class="text-xs text-gray-400 mt-1">已完结</div>
          <div v-if="airings[item.subject_id]?.today_airing_at" class="text-xs text-pink-300 mt-1 font-semibold">
            今日预计播出 · 第 {{ airings[item.subject_id].today_episode }} 集 · {{ formatAirTime(airings[item.subject_id].today_airing_at) }}
          </div>
          <div v-else-if="airings[item.subject_id]?.next_airing_at * 1000 > Date.now()" class="text-xs text-blue-300 mt-1">
            下集预计 {{ formatAirTime(airings[item.subject_id].next_airing_at) }}
          </div>
          <div class="text-xs text-gray-300 mt-1">已看 {{ item.watched_eps?.length || 0 }} 集 · 已下载 {{ item.downloaded_eps?.length || 0 }} 集</div>
          <div class="text-xs text-gray-600 mt-1">添加于: {{ item.added_at.split(' ')[0] }}</div>
        </div>
      </div>
    </div>

    <!-- 详情弹窗 (复用 Discovery 的逻辑) -->
    <div v-if="selectedItem" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-0 backdrop-blur-sm sm:p-4" @click="closeDetail()">
      <div class="h-[100dvh] w-full overflow-y-auto overscroll-contain bg-gray-900 shadow-2xl sm:h-auto sm:max-h-[90vh] sm:max-w-4xl sm:rounded-xl sm:border sm:border-gray-700" @click.stop>
        <div v-if="detailLoading" class="p-10 text-center text-gray-400">
          <div class="animate-spin text-4xl mb-4">⏳</div>
          <p>正在获取详细信息...</p>
        </div>
        <div v-else-if="detailError" class="p-10 text-center text-red-400">
          <p>获取失败: {{ detailError }}</p>
          <button @click="closeDetail()" class="mt-4 px-4 py-2 bg-gray-800 rounded">关闭</button>
        </div>
        <div v-else-if="detailData" class="relative">
          <!-- 顶部大图背景 -->
          <div class="h-32 overflow-hidden relative sm:h-48">
             <img :src="proxyImg(detailData.subject.images.large)" @error="onImgError($event, detailData.subject.images.large)" referrerpolicy="no-referrer" class="w-full object-cover opacity-30 blur-sm transform scale-110">
             <div class="absolute inset-0 bg-gradient-to-b from-transparent to-gray-900"></div>
             <button @click="closeDetail()" aria-label="关闭详情" class="absolute top-3 right-3 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80 sm:top-4 sm:right-4">✕</button>
          </div>
          
          <div class="relative -mt-10 flex flex-col items-start gap-4 px-4 pb-6 sm:-mt-20 sm:gap-6 sm:px-8 sm:pb-8 md:flex-row">
            <!-- 封面图 -->
            <div class="w-28 flex-shrink-0 rounded-lg overflow-hidden border-4 border-gray-800 bg-gray-800 shadow-2xl sm:w-40 md:w-48">
              <img :src="proxyImg(detailData.subject.images.large || detailData.subject.images.common)" @error="onImgError($event, detailData.subject.images.large || detailData.subject.images.common)" referrerpolicy="no-referrer" class="w-full h-auto block">
            </div>
            
            <!-- 信息区域 -->
            <div class="min-w-0 flex-1 pt-1 text-left md:pt-20">
              <h2 class="mb-1 break-words text-2xl font-bold text-white sm:text-3xl">{{ detailData.subject.name_cn || detailData.subject.name }}</h2>
              <p class="text-gray-400 text-sm mb-4">{{ detailData.subject.name }}</p>
              
              <div class="mb-6 flex flex-wrap gap-2 text-sm sm:gap-4">
                <div class="bg-gray-800 px-3 py-1 rounded text-pink-400 font-bold">
                  评分: {{ detailData.subject.rating.score }}
                </div>
                <div class="bg-gray-800 px-3 py-1 rounded text-blue-400">
                  {{ detailData.subject.date }} 开播
                </div>
                <div class="bg-gray-800 px-3 py-1 rounded text-green-400">
                  总集数: {{ detailData.subject.total_episodes || detailData.subject.eps || '?' }}
                </div>
                <div :title="`进度来源：${detailData.progress_source || 'Bangumi'}`" class="bg-pink-900/50 px-3 py-1 rounded text-pink-300 border border-pink-500/30">
                  更新至: 第 {{ detailData.current_episode }} 话
                </div>
              </div>

              <div class="mb-6">
                <h3 class="text-lg font-bold text-white mb-2">简介</h3>
                <p class="text-gray-400 text-sm leading-relaxed max-h-32 overflow-y-auto pr-2">{{ detailData.subject.summary || '暂无简介' }}</p>
              </div>

              <button @click="toggleFollow" class="mb-4 mr-3 min-h-[44px] rounded border border-pink-500/40 bg-gray-800 px-4 text-sm text-pink-300">{{ detailData.is_followed ? '取消追番（保留记录）' : '追番' }}</button>
              <button @click="requestDanmakuSource(detailData.subject.id, detailData.episodes[0]?.sort || 1, true, detailData.episodes)" class="mb-4 min-h-[44px] rounded border border-pink-500/40 bg-gray-800 px-4 text-sm text-pink-300">💬 选择／更换 B 站弹幕</button>

              <!-- 剧集列表预览 -->
              <div>
                <h3 class="text-lg font-bold text-white mb-3">剧集列表 <span class="text-xs font-normal text-gray-500 ml-2" v-if="isWebMode()"><span class="lg:hidden">点按集数选择下载或标记观看</span><span class="hidden lg:inline">左键下载，右键标记观看</span></span><span v-else class="text-xs font-normal text-gray-500 ml-2">左键播放/下载，右键标记观看，中键删除</span></h3>
                <div class="grid grid-cols-4 gap-2 overflow-y-auto pr-2 sm:grid-cols-4 md:max-h-40 md:grid-cols-6">
                  <div v-for="ep in detailData.episodes" :key="ep.id || ep.sort" class="flex min-w-0 gap-0.5">
                  <button type="button"
                       @click="handleEpisodeTap(ep, $event)"
                       @contextmenu.prevent="handleEpisodeContext(ep)"
                       @mousedown.middle.prevent="handleEpisodeDelete(ep)"
                       @keydown.enter.prevent="episodeAction = ep"
                       class="relative min-w-0 flex-1 flex min-h-[44px] cursor-pointer items-center justify-center truncate rounded border border-transparent px-2 py-1.5 text-center text-sm transition-colors md:min-h-0 md:text-xs"
                       :class="getEpisodeClass(ep)"
                       :title="ep.name_cn || ep.name"
                  >
                    {{ ep.sort }}
                    <!-- 磁力链接指示器 -->
                    <div v-if="detailData.episode_magnets && detailData.episode_magnets[ep.sort]" 
                         class="absolute top-0.5 right-0.5 w-1.5 h-1.5 bg-blue-400 rounded-full shadow-sm"></div>
                    <span v-if="watched(ep)" aria-label="已观看" class="absolute left-1 top-0 text-xs">✓</span>
                    <span v-if="downloaded(ep)" aria-label="已下载" class="absolute bottom-0 right-1 text-xs">↓</span>
                  </button>
                  <button @click="episodeAction = ep" :aria-label="`第 ${ep.sort} 集操作`" class="rounded bg-gray-800 px-1 text-gray-300">⋯</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 磁力选择弹窗 -->
    <div v-if="showMagnetPicker" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/80 p-0 backdrop-blur-sm sm:p-4" @click.self="showMagnetPicker = false">
      <div class="flex h-[100dvh] w-full flex-col overflow-hidden bg-gray-900 shadow-2xl sm:h-auto sm:max-h-[80vh] sm:max-w-4xl sm:rounded-xl sm:border sm:border-gray-700">
        <div class="shrink-0 border-b border-gray-700 p-4 sm:p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-bold text-white sm:text-xl">选择磁力链接 - 第 {{ magnetPickerEp?.sort }} 集</h2>
            <button @click="showMagnetPicker = false" aria-label="关闭磁力选择" class="h-11 w-11 shrink-0 text-2xl text-gray-400 hover:text-white">&times;</button>
          </div>
          
          <!-- 搜索关键词输入 -->
          <div class="flex flex-col gap-2 sm:flex-row">
            <input 
              v-model="magnetSearchKeyword" 
              @keyup.enter="searchMagnetList"
              class="min-w-0 flex-1 rounded border border-gray-700 bg-gray-800 px-4 py-2 text-base text-white focus:border-pink-500 focus:outline-none"
              placeholder="输入关键词搜索（留空使用默认）"
            />
            <button 
              @click="searchMagnetList" 
              class="min-h-[44px] rounded bg-pink-600 px-6 py-2 font-bold text-white transition hover:bg-pink-700 disabled:bg-gray-600"
            >
              {{ magnetSearching ? '重新搜索' : '🔍 搜索' }}
            </button>
          </div>
        </div>

        <!-- 候选列表 -->
        <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6">
          <div v-if="magnetSearching && magnetCandidates.length === 0" class="text-center py-8 text-gray-400">
            <div class="animate-spin inline-block w-8 h-8 border-4 border-pink-500 border-t-transparent rounded-full mb-2"></div>
            <p>正在搜索...</p>
          </div>

          <div v-else-if="magnetCandidates.length === 0 && !magnetSearchError" class="text-center py-8 text-gray-400">
            暂无结果，请尝试修改关键词
          </div>

          <p v-if="magnetSearching && magnetCandidates.length" role="status" class="mb-3 text-sm text-gray-400">已找到 {{ magnetCandidates.length }} 项，仍在搜索其他资源…</p>
          <p v-if="operationMessage" role="status" class="mb-3 text-sm text-amber-300">{{ operationMessage }}</p>
          <p v-if="magnetSearchError" role="alert" class="mb-3 text-sm text-red-400">{{ magnetSearchError }}</p>
          <div v-if="magnetCandidates.length" class="space-y-2">
            <div 
              v-for="item in magnetCandidates"
              :key="item.magnet"
              @click="selectMagnet(item)"
              class="bg-gray-800 hover:bg-gray-750 border border-gray-700 hover:border-pink-500 rounded-lg p-4 cursor-pointer transition-all"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex-1 min-w-0">
                  <h3 class="text-white font-medium mb-2 break-words">{{ item.title }}</h3>
                  <p v-if="item.match_warning" class="mb-2 text-sm text-amber-300">{{ item.match_warning }}</p>
                  <div class="flex flex-wrap gap-3 text-xs text-gray-400">
                    <span>📦 {{ item.size }}</span>
                    <span>👤 {{ item.source }}</span>
                    <span>📅 {{ item.publish_date }}</span>
                  </div>
                </div>
                <button class="min-h-[44px] shrink-0 self-end rounded bg-pink-600 px-4 py-2 text-sm font-bold text-white hover:bg-pink-700 sm:ml-4 sm:self-auto">
                  选择
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="selectedItem && episodeAction" role="dialog" aria-modal="true" aria-label="剧集操作" class="fixed inset-0 z-[70] flex items-end bg-black/70" @click.self="episodeAction = null">
      <div class="mobile-action-sheet w-full rounded-t-2xl bg-gray-800 p-4 text-left shadow-2xl">
        <div class="mb-4 flex items-center justify-between gap-3">
          <h3 class="min-w-0 truncate text-lg font-bold">第 {{ episodeAction.sort }} 集 {{ episodeAction.name_cn || episodeAction.name }}</h3>
          <button @click="episodeAction = null" aria-label="关闭剧集操作" class="h-11 w-11 shrink-0 rounded-full bg-gray-700 text-xl">✕</button>
        </div>
        <p v-if="!available(episodeAction)" class="rounded-lg bg-gray-700 p-3 text-center text-gray-300">暂无已播信息，可手动搜索资源</p>
        <div class="space-y-3">
          <button v-if="available(episodeAction)" @click="downloadSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-pink-600 px-4 font-bold text-white">{{ detailData?.downloaded_eps?.includes(episodeAction.sort) ? (isWebMode() ? '已下载到家中电脑' : '播放本地视频') : '下载／选择资源' }}</button>
          <button v-if="!isWebMode() && downloaded(episodeAction)" @click="restartSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-gray-700 px-4 text-white">从头播放</button>
          <button v-if="!isWebMode()" @click="handleEpisodeDelete(episodeAction); episodeAction = null" class="min-h-[48px] w-full rounded-lg bg-gray-700 px-4 text-red-300">删除本地视频与磁力</button>
          <button @click="searchSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-gray-700 px-4 text-white">手动搜索资源</button>
          <button @click="markSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-gray-700 px-4 font-bold text-white">{{ detailData?.watched_eps?.includes(episodeAction.sort) ? '取消已观看' : '标记已观看' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { GetLocalFollows, GetFollowAirings, isWebMode, EventsOn } from '../api';
import { proxyImg } from '../utils/image';
import { useAnimeDetail } from '../composables/useAnimeDetail';
import ContinueWatching from '../components/ContinueWatching.vue';
import { requestDanmakuSource } from '../composables/useDanmakuPicker';
import { rememberUpdates, hasUnreadUpdate, readUpdate } from '../utils/updateReminders';

const collection = ref([]);
const airings = ref({});
const remindersKey = 'ultimateanime-update-reminders';
const loadStoredMap = (key) => {
  try {
    const saved = JSON.parse(localStorage.getItem(key) || '{}');
    return saved && typeof saved === 'object' && !Array.isArray(saved) ? saved : {};
  } catch {
    return {};
  }
};
const savedReminders = loadStoredMap(remindersKey);
for (const [id, seenKey] of Object.entries(loadStoredMap('ultimateanime-seen-updates'))) {
  if (!savedReminders[id]) savedReminders[id] = { seenKey };
}
const updateReminders = ref(savedReminders);
const saveReminders = () => {
  try {
    localStorage.setItem(remindersKey, JSON.stringify(updateReminders.value));
  } catch {
    // Keep dismissal working for this session when storage is unavailable.
  }
};
const refreshReminders = () => {
  updateReminders.value = rememberUpdates(updateReminders.value, airings.value);
  saveReminders();
};
const hasNewUpdate = (id) => hasUnreadUpdate(updateReminders.value[id]);
const markUpdateRead = (id) => {
  refreshReminders();
  updateReminders.value = {
    ...updateReminders.value,
    [id]: readUpdate(updateReminders.value[id], Boolean(airings.value[id])),
  };
  saveReminders();
};
const loading = ref(true);
const loadError = ref('');
const finishedFilterKey = 'ultimateanime-hide-finished';
const hideFinished = ref(localStorage.getItem(finishedFilterKey) === 'true');
const finishedCount = computed(() => collection.value.filter(item => airings.value[item.subject_id]?.release_status === 'FINISHED').length);
const visibleCollection = computed(() => {
  const items = hideFinished.value
    ? collection.value.filter(item => airings.value[item.subject_id]?.release_status !== 'FINISHED')
    : collection.value;
  return sortCollection(items);
});
const toggleFinishedFilter = () => {
  hideFinished.value = !hideFinished.value;
  localStorage.setItem(finishedFilterKey, String(hideFinished.value));
};

const formatAirTime = (timestamp) => new Date(timestamp * 1000).toLocaleString('zh-CN', {
  month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'
});

const sortCollection = (items) => [...items].reverse().sort((a, b) => {
  const aUnread = hasNewUpdate(a.subject_id);
  const bUnread = hasNewUpdate(b.subject_id);
  if (aUnread !== bUnread) return aUnread ? -1 : 1;
  if (aUnread && bUnread) {
    return updateReminders.value[b.subject_id].latest.airingAt - updateReminders.value[a.subject_id].latest.airingAt;
  }
  const aToday = airings.value[a.subject_id]?.today_airing_at || 0;
  const bToday = airings.value[b.subject_id]?.today_airing_at || 0;
  if (aToday && !bToday) return -1;
  if (!aToday && bToday) return 1;
  return bToday - aToday;
});

const {
 operationMessage, toggleFollow, selectedItem, detailData, detailLoading, detailError, showDetail, closeDetail,
 showMagnetPicker, episodeAction, magnetPickerEp, magnetSearchKeyword, magnetSearching, magnetCandidates, magnetSearchError,
 getEpisodeClass, available, downloaded, watched, searchMagnetList, selectMagnet, handleEpisodeTap, handleEpisodeContext,
 handleEpisodeDelete, downloadSelectedEpisode, restartSelectedEpisode, markSelectedEpisode, searchSelectedEpisode,
} = useAnimeDetail(markUpdateRead, () => fetchCollection());
const onImgError = (event, originalUrl) => {
 const img = event.target;
 if (!img || !originalUrl || img.dataset.fallback === '1') return;
 img.dataset.fallback = '1'; img.src = originalUrl.replace(/^http:\/\//i, 'https://');
};

const fetchCollection = async () => {
  try {
    if (collection.value.length === 0) loading.value = true;
    loadError.value = '';
    const res = await GetLocalFollows();
    if (res) {
      collection.value = res;
      refreshReminders();
      loading.value = false;
      try {
        airings.value = await GetFollowAirings();
        refreshReminders();
      } catch (err) {
        console.error('读取追番排期失败', err);
      }
    }
  } catch (err) {
    loadError.value = String(err);
    console.error(err);
  } finally {
    loading.value = false;
  }
};

let stopDownloadEvents, stopPlaybackEvents;
let refreshTimer;
let reminderTimer;
onMounted(() => {
  fetchCollection();
  stopPlaybackEvents = EventsOn('playback-progress', data => {
    if (!data.marked_watched) return;
    const item = collection.value.find(item => item.subject_id === data.subject_id);
    if (item && !item.watched_eps?.includes(data.ep_sort)) item.watched_eps = [...(item.watched_eps || []), data.ep_sort];
  });
  refreshTimer = setInterval(fetchCollection, 60 * 60 * 1000);
  reminderTimer = setInterval(refreshReminders, 30 * 1000);
  stopDownloadEvents = EventsOn('download-complete', (data) => {
    if (detailData.value?.subject.id === data.subject_id) {
      if (!detailData.value.downloaded_eps) detailData.value.downloaded_eps = [];
      if (!detailData.value.downloaded_eps.includes(data.ep_sort)) detailData.value.downloaded_eps.push(data.ep_sort);
    }
  });

});
onUnmounted(() => {
  clearInterval(refreshTimer);
  clearInterval(reminderTimer);
  stopDownloadEvents?.();
  stopPlaybackEvents?.();
});
</script>
