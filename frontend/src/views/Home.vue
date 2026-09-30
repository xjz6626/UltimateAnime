<template>
  <div class="p-4 sm:p-6">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl sm:text-2xl font-bold text-white">📺 我的追番 {{ isWebMode() ? '(家中电脑)' : '(本地)' }}</h1>
      <button v-if="collection.length" type="button" @click="toggleFinishedFilter" :aria-pressed="hideFinished"
              class="rounded-lg border border-gray-600 bg-gray-800 px-3 py-1.5 text-sm text-gray-200 hover:border-pink-500 hover:text-white">
        {{ hideFinished ? '显示完结' : '隐藏完结' }}<span v-if="finishedCount"> ({{ finishedCount }})</span>
      </button>
    </div>
    
    <div v-if="loading" class="text-gray-400">加载中...</div>
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
          <div class="text-xs text-gray-600 mt-1">添加于: {{ item.added_at.split(' ')[0] }}</div>
        </div>
      </div>
    </div>

    <!-- 详情弹窗 (复用 Discovery 的逻辑) -->
    <div v-if="selectedItem" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-0 backdrop-blur-sm sm:p-4" @click="selectedItem = null">
      <div class="h-[100dvh] w-full overflow-y-auto overscroll-contain bg-gray-900 shadow-2xl sm:h-auto sm:max-h-[90vh] sm:max-w-4xl sm:rounded-xl sm:border sm:border-gray-700" @click.stop>
        <div v-if="detailLoading" class="p-10 text-center text-gray-400">
          <div class="animate-spin text-4xl mb-4">⏳</div>
          <p>正在获取详细信息...</p>
        </div>
        <div v-else-if="detailError" class="p-10 text-center text-red-400">
          <p>获取失败: {{ detailError }}</p>
          <button @click="selectedItem = null" class="mt-4 px-4 py-2 bg-gray-800 rounded">关闭</button>
        </div>
        <div v-else-if="detailData" class="relative">
          <!-- 顶部大图背景 -->
          <div class="h-32 overflow-hidden relative sm:h-48">
             <img :src="proxyImg(detailData.subject.images.large)" @error="onImgError($event, detailData.subject.images.large)" referrerpolicy="no-referrer" class="w-full object-cover opacity-30 blur-sm transform scale-110">
             <div class="absolute inset-0 bg-gradient-to-b from-transparent to-gray-900"></div>
             <button @click="selectedItem = null" aria-label="关闭详情" class="absolute top-3 right-3 flex h-11 w-11 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80 sm:top-4 sm:right-4">✕</button>
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
                <div class="bg-pink-900/50 px-3 py-1 rounded text-pink-300 border border-pink-500/30">
                  更新至: 第 {{ detailData.current_episode }} 话
                </div>
              </div>

              <div class="mb-6">
                <h3 class="text-lg font-bold text-white mb-2">简介</h3>
                <p class="text-gray-400 text-sm leading-relaxed max-h-32 overflow-y-auto pr-2">{{ detailData.subject.summary || '暂无简介' }}</p>
              </div>

              <!-- 剧集列表预览 -->
              <div>
                <h3 class="text-lg font-bold text-white mb-3">剧集列表 <span class="text-xs font-normal text-gray-500 ml-2" v-if="isWebMode()"><span class="lg:hidden">点按集数选择下载或标记观看</span><span class="hidden lg:inline">左键下载，右键标记观看</span></span><span v-else class="text-xs font-normal text-gray-500 ml-2">左键播放/下载，右键标记观看，中键删除</span></h3>
                <div class="grid grid-cols-4 gap-2 overflow-y-auto pr-2 sm:grid-cols-4 md:max-h-40 md:grid-cols-6">
                  <div v-for="ep in detailData.episodes" :key="ep.id" 
                       @click="handleEpisodeTap(ep, $event)"
                       @contextmenu.prevent="handleEpisodeContext(ep)"
                       @mousedown.middle.prevent="handleEpisodeDelete(ep)"
                       class="relative flex min-h-[44px] cursor-pointer items-center justify-center truncate rounded border border-transparent px-2 py-1.5 text-center text-sm transition-colors md:min-h-0 md:text-xs"
                       :class="getEpisodeClass(ep)"
                       :title="ep.name_cn || ep.name"
                  >
                    {{ ep.sort }}
                    <!-- 磁力链接指示器 -->
                    <div v-if="detailData.episode_magnets && detailData.episode_magnets[ep.sort]" 
                         class="absolute top-0.5 right-0.5 w-1.5 h-1.5 bg-blue-400 rounded-full shadow-sm"></div>
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
              :disabled="magnetSearching"
              class="min-h-[44px] rounded bg-pink-600 px-6 py-2 font-bold text-white transition hover:bg-pink-700 disabled:bg-gray-600"
            >
              {{ magnetSearching ? '搜索中...' : '🔍 搜索' }}
            </button>
          </div>
        </div>

        <!-- 候选列表 -->
        <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6">
          <div v-if="magnetSearching" class="text-center py-8 text-gray-400">
            <div class="animate-spin inline-block w-8 h-8 border-4 border-pink-500 border-t-transparent rounded-full mb-2"></div>
            <p>正在搜索...</p>
          </div>

          <div v-else-if="magnetCandidates.length === 0" class="text-center py-8 text-gray-400">
            暂无结果，请尝试修改关键词
          </div>

          <div v-else class="space-y-2">
            <div 
              v-for="(item, index) in magnetCandidates" 
              :key="index"
              @click="selectMagnet(item)"
              class="bg-gray-800 hover:bg-gray-750 border border-gray-700 hover:border-pink-500 rounded-lg p-4 cursor-pointer transition-all"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="flex-1 min-w-0">
                  <h3 class="text-white font-medium mb-2 break-words">{{ item.title }}</h3>
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
        <p v-if="episodeAction.sort > detailData.current_episode" class="rounded-lg bg-gray-700 p-3 text-center text-gray-300">这一集尚未播出</p>
        <div v-else class="space-y-3">
          <button @click="downloadSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-pink-600 px-4 font-bold text-white">{{ detailData?.downloaded_eps?.includes(episodeAction.sort) ? '已下载到家中电脑' : '下载／选择资源' }}</button>
          <button @click="markSelectedEpisode" class="min-h-[48px] w-full rounded-lg bg-gray-700 px-4 font-bold text-white">{{ detailData?.watched_eps?.includes(episodeAction.sort) ? '取消已观看' : '标记已观看' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { GetLocalFollows, GetFollowAirings, GetAnimeDetail, ToggleEpisodeWatched, SearchEpisodeMagnet, GetAutoSelectMagnet, PlayMagnet, DownloadEpisode, PlayLocalEpisode, DeleteEpisodeData, SearchEpisodeMagnetList, SaveEpisodeMagnet, isWebMode, EventsOn } from '../api';
import { proxyImg } from '../utils/image';

const collection = ref([]);
const airings = ref({});
const loading = ref(true);
const finishedFilterKey = 'ultimateanime-hide-finished';
const hideFinished = ref(localStorage.getItem(finishedFilterKey) === 'true');
const finishedCount = computed(() => collection.value.filter(item => airings.value[item.subject_id]?.release_status === 'FINISHED').length);
const visibleCollection = computed(() => hideFinished.value
  ? collection.value.filter(item => airings.value[item.subject_id]?.release_status !== 'FINISHED')
  : collection.value);
const toggleFinishedFilter = () => {
  hideFinished.value = !hideFinished.value;
  localStorage.setItem(finishedFilterKey, String(hideFinished.value));
};

const formatAirTime = (timestamp) => new Date(timestamp * 1000).toLocaleString('zh-CN', {
  month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'
});

const sortCollection = (items) => [...items].reverse().sort((a, b) => {
  const aToday = airings.value[a.subject_id]?.today_airing_at || 0;
  const bToday = airings.value[b.subject_id]?.today_airing_at || 0;
  if (aToday && !bToday) return -1;
  if (!aToday && bToday) return 1;
  return bToday - aToday;
});

// 详情弹窗相关
const selectedItem = ref(null);
const detailLoading = ref(false);
const detailData = ref(null);
const detailError = ref('');

// 磁力选择弹窗相关
const showMagnetPicker = ref(false);
const episodeAction = ref(null);
const magnetPickerEp = ref(null);
const magnetSearchKeyword = ref('');
const magnetSearching = ref(false);
const magnetCandidates = ref([]);
const episodeClickPending = ref({});

// 代理也加载失败时，回退到原始 URL 再尝试一次（万一用户的网络能直连呢）
const onImgError = (event, originalUrl) => {
  const img = event.target;
  if (!img || !originalUrl) return;
  // 标记一下，防止无限循环回退
  if (img.dataset.fallback === '1') return;
  img.dataset.fallback = '1';
  const httpsUrl = originalUrl.replace(/^http:\/\//i, 'https://');
  if (img.src !== httpsUrl) {
    img.src = httpsUrl;
  }
};

const showDetail = async (item) => {
  episodeAction.value = null;
  selectedItem.value = item;
  detailLoading.value = true;
  detailError.value = '';
  detailData.value = null;
  
  try {
    // 注意：Home 中的 item 是 FollowedItem，ID 字段是 subject_id
    const id = item.subject_id || item.id;
    const res = await GetAnimeDetail(id);
    detailData.value = res;
  } catch (e) {
    detailError.value = e.toString();
  } finally {
    detailLoading.value = false;
  }
};

const getEpisodeClass = (ep) => {
  if (!detailData.value) return '';
  
  // 1. 未放送 (灰色)
  if (ep.sort > detailData.value.current_episode) {
    return 'bg-gray-800 text-gray-600 cursor-not-allowed';
  }

  // 2. 已下载 (蓝色)
  if (detailData.value.downloaded_eps && detailData.value.downloaded_eps.includes(ep.sort)) {
    return 'bg-blue-600 text-white hover:bg-blue-700 border border-blue-400';
  }

  // 3. 已观看 (绿色)
  if (detailData.value.watched_eps && detailData.value.watched_eps.includes(ep.sort)) {
    return 'bg-green-600 text-white hover:bg-green-700';
  }

  // 4. 已放送但未观看 (粉色)
  return 'bg-pink-600 text-white hover:bg-pink-700';
};

const toggleWatched = async (ep) => {
  if (!detailData.value) return;
  
  // 如果未放送，不可点击
  if (ep.sort > detailData.value.current_episode) return;

  try {
    const res = await ToggleEpisodeWatched(detailData.value.subject.id, ep.sort);
    if (res === "Success") {
        // 更新本地状态
        let watched = detailData.value.watched_eps || [];
        if (watched.includes(ep.sort)) {
            watched = watched.filter(s => s !== ep.sort);
        } else {
            watched.push(ep.sort);
        }
        detailData.value.watched_eps = watched;
    }
  } catch (e) {
    console.error(e);
  }
};

const getMagnet = async (ep) => {
  const epKey = ep.sort.toString();
  if (detailData.value.episode_magnets && detailData.value.episode_magnets[epKey]) {
      return detailData.value.episode_magnets[epKey];
  }
  
  try {
      console.log(`正在搜索第 ${ep.sort} 集...`);
      const res = await SearchEpisodeMagnet(detailData.value.subject.id, ep.sort);
      if (typeof res !== 'string' || !res.startsWith('magnet:')) {
          alert(res);
          return "";
      }
      
      // 更新本地数据
      if (!detailData.value.episode_magnets) {
          detailData.value.episode_magnets = {};
      }
      detailData.value.episode_magnets[epKey] = res;
      return res;
  } catch (e) {
      alert("搜索出错: " + e);
      return "";
  }
};

const handleEpisodeClick = async (ep, event) => {
    if (!detailData.value) return;
    
    // 1. 如果已下载，直接播放本地文件
    if (detailData.value.downloaded_eps && detailData.value.downloaded_eps.includes(ep.sort)) {
        if (isWebMode()) {
            alert('这一集已下载到家里的电脑');
            return;
        }
        try {
            console.log("正在请求播放本地文件...");
            const res = await PlayLocalEpisode(detailData.value.subject.id, ep.sort);
            if (res !== "Success") {
                alert(res);
            }
        } catch (e) {
            alert("播放请求失败: " + e);
        }
        return;
    }
    
    // 2. 未下载时按配置选择磁力并开始下载
    await startEpisodeDownload(ep);
};

const usesMobileWebActions = () => isWebMode() && window.matchMedia('(max-width: 1023px)').matches;
const handleEpisodeTap = (ep, event) => {
  if (usesMobileWebActions()) {
    episodeAction.value = ep;
  } else {
    handleEpisodeClick(ep, event);
  }
};
const handleEpisodeContext = (ep) => {
  if (usesMobileWebActions()) {
    episodeAction.value = ep;
  } else {
    toggleWatched(ep);
  }
};
const downloadSelectedEpisode = () => {
  const ep = episodeAction.value;
  episodeAction.value = null;
  if (ep) handleEpisodeClick(ep);
};
const markSelectedEpisode = () => {
  const ep = episodeAction.value;
  episodeAction.value = null;
  if (ep) toggleWatched(ep);
};

const openMagnetPicker = async (ep) => {
    magnetPickerEp.value = ep;
    magnetSearchKeyword.value = '';
    showMagnetPicker.value = true;
    await searchMagnetList();
};

const startEpisodeDownload = async (ep) => {
  const epKey = ep.sort.toString();
  if (episodeClickPending.value[epKey]) return;
  episodeClickPending.value[epKey] = true;
  try {
    const autoSelect = await GetAutoSelectMagnet();
    if (!autoSelect) {
      await openMagnetPicker(ep);
      return;
    }
    const magnet = await getMagnet(ep);
    if (!magnet) {
      await openMagnetPicker(ep);
      return;
    }
    const res = await DownloadEpisode(detailData.value.subject.id, ep.sort, magnet);
    if (res === 'Started') {
      alert(`已自动优选并开始下载第 ${ep.sort} 集`);
    } else {
      alert(res);
    }
  } catch (e) {
    alert('准备下载失败: ' + e);
  } finally {
    delete episodeClickPending.value[epKey];
  }
};

const searchMagnetList = async () => {
  if (!detailData.value || !magnetPickerEp.value) return;
  
  magnetSearching.value = true;
  magnetCandidates.value = [];
  
  try {
    const res = await SearchEpisodeMagnetList(
      detailData.value.subject.id, 
      magnetPickerEp.value.sort, 
      magnetSearchKeyword.value
    );
    magnetCandidates.value = res || [];
  } catch (e) {
    alert("搜索失败: " + e);
  } finally {
    magnetSearching.value = false;
  }
};

const selectMagnet = async (item) => {
  if (!detailData.value || !magnetPickerEp.value) return;
  
  // 保存磁力到本地
  try {
    const saved = await SaveEpisodeMagnet(detailData.value.subject.id, magnetPickerEp.value.sort, item.magnet);
    if (saved !== 'Success') {
      alert('保存磁力链接失败: ' + saved);
      return;
    }
    
    // 更新 UI
    const epKey = magnetPickerEp.value.sort.toString();
    if (!detailData.value.episode_magnets) {
      detailData.value.episode_magnets = {};
    }
    detailData.value.episode_magnets[epKey] = item.magnet;
    
    // 开始下载
    const res = await DownloadEpisode(detailData.value.subject.id, magnetPickerEp.value.sort, item.magnet);
    if (res === "Started") {
      alert("已开始下载: " + item.title);
      showMagnetPicker.value = false;
    } else {
      alert(res);
    }
  } catch (e) {
    alert("操作失败: " + e);
  }
};

const handleEpisodeDelete = async (ep) => {
  if (isWebMode()) return;
  if (!detailData.value) return;
  
  const epKey = ep.sort.toString();
  const hasMagnet = detailData.value.episode_magnets && detailData.value.episode_magnets[epKey];
  const hasLocal = detailData.value.downloaded_eps && detailData.value.downloaded_eps.includes(ep.sort);
  
  if (!hasMagnet && !hasLocal) {
    alert("该集没有磁力链接或本地文件");
    return;
  }
  
  // 构建删除选项提示
  let options = [];
  if (hasMagnet) options.push("磁力链接");
  if (hasLocal) options.push("本地视频");
  
  const confirmMsg = `删除第 ${ep.sort} 集的:\n\n${options.join(" + ")}\n\n确定删除吗？`;
  
  if (!confirm(confirmMsg)) return;
  
  try {
    const res = await DeleteEpisodeData(detailData.value.subject.id, ep.sort);
    if (res === "Success") {
      // 更新本地状态
      if (hasMagnet && detailData.value.episode_magnets) {
        delete detailData.value.episode_magnets[epKey];
      }
      if (hasLocal && detailData.value.downloaded_eps) {
        detailData.value.downloaded_eps = detailData.value.downloaded_eps.filter(s => s !== ep.sort);
      }
      alert("✅ 删除成功");
    } else {
      alert(res);
    }
  } catch (e) {
    alert("删除失败: " + e);
  }
};

const fetchCollection = async () => {
  try {
    if (collection.value.length === 0) loading.value = true;
    const res = await GetLocalFollows();
    if (res) {
      collection.value = sortCollection(res);
      loading.value = false;
      try {
        airings.value = await GetFollowAirings();
        collection.value = sortCollection(res);
      } catch (err) {
        console.error('读取追番排期失败', err);
      }
    }
  } catch (err) {
    console.error(err);
  } finally {
    loading.value = false;
  }
};

let stopDownloadEvents;
let stopDownloadErrors;
let refreshTimer;
onMounted(() => {
  fetchCollection();
  refreshTimer = setInterval(fetchCollection, 60 * 60 * 1000);
  stopDownloadEvents = EventsOn('download-complete', (data) => {
    if (detailData.value?.subject.id === data.subject_id) {
      if (!detailData.value.downloaded_eps) detailData.value.downloaded_eps = [];
      if (!detailData.value.downloaded_eps.includes(data.ep_sort)) detailData.value.downloaded_eps.push(data.ep_sort);
    }
  });
  stopDownloadErrors = EventsOn('download-error', (data) => {
    if (detailData.value?.subject.id === data.subject_id) {
      alert(`第 ${data.ep_sort} 集下载失败：${data.error}`);
    }
  });
});
onUnmounted(() => {
  clearInterval(refreshTimer);
  stopDownloadEvents?.();
  stopDownloadErrors?.();
});
</script>
