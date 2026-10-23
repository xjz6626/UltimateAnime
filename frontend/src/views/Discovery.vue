<template>
  <div class="p-4 sm:p-6">
    <h1 class="mb-4 text-xl font-bold text-white sm:text-2xl">📅 当季新番</h1>
    
    <div v-if="loading" class="text-gray-400 animate-pulse">
      <p>正在加载新番列表...</p>
      <div class="mt-6 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="i in 6" :key="i" class="bg-gray-800 p-4 rounded h-32"></div>
      </div>
    </div>
    
    <div v-else-if="error" class="text-red-400">
      <p>加载失败: {{ error }}</p>
      <button @click="fetchCalendar" class="mt-2 px-4 py-2 bg-pink-600 rounded text-white text-sm">重试</button>
    </div>
    
    <div v-else class="space-y-6 pb-10">
      <!-- 星期导航栏 -->
      <div ref="weekdayNav" class="flex gap-2 overflow-x-auto overscroll-x-contain border-b border-gray-700 pb-2">
        <button 
          v-for="day in sortedCalendar" 
          :key="day.weekday.id"
          @click="selectDay(day.weekday.id)"
          :data-weekday-id="day.weekday.id"
          :class="[
            'min-h-[44px] px-4 py-2 rounded-full text-sm font-medium transition-all whitespace-nowrap',
            activeDayId === day.weekday.id 
              ? 'bg-pink-600 text-white shadow-lg scale-105' 
              : 'bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-white'
          ]"
        >
          {{ day.weekday.cn }}
        </button>
      </div>

      <!-- 番剧列表 -->
      <div v-if="activeDayData" class="animate-fade-in">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 lg:grid-cols-3 xl:grid-cols-4">
          <div v-for="item in activeDayData.items" :key="item.id" @click="showDetail(item)" class="bg-gray-800 p-3 rounded hover:bg-gray-700 transition group cursor-pointer border border-transparent hover:border-pink-500/30">
            <div class="flex gap-3">
              <div class="w-20 h-28 flex-shrink-0 bg-gray-700 rounded overflow-hidden relative">
                 <img v-if="item.images" :src="proxyImg(item.images.common || item.images.medium)" @error="onImgError($event, item.images.common || item.images.medium)" referrerpolicy="no-referrer" class="w-full h-full object-cover group-hover:scale-110 transition duration-300" loading="lazy" />
                 <div class="absolute top-0 right-0 bg-pink-600 text-white text-[10px] px-1.5 py-0.5 rounded-bl" v-if="item.rating && item.rating.score >= 8">
                   {{ item.rating.score }}
                 </div>
              </div>
              <div class="flex-1 min-w-0 flex flex-col justify-between py-1">
                <div>
                  <h3 class="text-sm font-bold text-white truncate leading-tight mb-1" :title="item.name_cn || item.name">{{ item.name_cn || item.name }}</h3>
                  <p class="text-xs text-gray-400 truncate" :title="item.name">{{ item.name }}</p>
                  <span v-if="item.followed_continuation || item.next_quarter_continuation" class="inline-block mt-1 rounded bg-amber-900/40 px-2 py-0.5 text-[10px] text-amber-300">
                    {{ item.next_quarter_continuation ? '已追·下季度续播' : '已追·跨季续播' }}<span v-if="item.next_airing_at * 1000 > Date.now()"> · 下集预计 {{ formatAirTime(item.next_airing_at) }}</span>
                  </span>
                </div>
                
                <div class="space-y-2">
                  <div class="flex items-center justify-between text-xs text-gray-500">
                     <span>{{ item.air_date }}</span>
                     <span v-if="item.collection" class="text-pink-400">{{ item.collection.doing || 0 }}人在追</span>
                  </div>
                  
                  <!-- 追番按钮 -->
                  <button 
                    @click.stop="toggleCollection(item)"
                    class="flex min-h-[44px] w-full items-center justify-center gap-1 rounded py-1.5 text-sm font-bold transition-colors sm:min-h-0 sm:text-xs"
                    :class="item.is_collected ? 'bg-gray-700 text-gray-300 hover:bg-red-900/50 hover:text-red-400' : 'bg-pink-600 text-white hover:bg-pink-700'"
                  >
                    <span v-if="item.is_collected">❤️ 已追番</span>
                    <span v-else>🤍 追番</span>
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

    <!-- 详情弹窗 -->
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
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue';
import { GetBangumiCalendar, FollowLocal, UnfollowLocal, GetLocalFollows, EventsOn, isWebMode } from '../api';
import { proxyImg } from '../utils/image';
import { useAnimeDetail } from '../composables/useAnimeDetail';
import { requestDanmakuSource } from '../composables/useDanmakuPicker';

const calendar = ref([]);
const formatAirTime = (timestamp) => new Date(timestamp * 1000).toLocaleString('zh-CN', {
  month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'
});
const localFollows = ref([]); // 本地追番列表
const loading = ref(true);
const error = ref('');
const activeDayId = ref(new Date().getDay() || 7); // 默认为今天 (周日=0 -> 7)
const weekdayNav = ref(null);

const scrollToActiveDay = async () => {
  await nextTick();
  const nav = weekdayNav.value;
  const button = nav?.querySelector(`[data-weekday-id="${activeDayId.value}"]`);
  if (!button) return;
  const offset = button.getBoundingClientRect().left - nav.getBoundingClientRect().left + nav.scrollLeft;
  nav.scrollTo({ left: offset - (nav.clientWidth - button.clientWidth) / 2, behavior: 'smooth' });
};
const selectDay = (id) => {
  activeDayId.value = id;
  scrollToActiveDay();
};

const {
 operationMessage, toggleFollow, selectedItem, detailData, detailLoading, detailError, showDetail, closeDetail,
 showMagnetPicker, episodeAction, magnetPickerEp, magnetSearchKeyword, magnetSearching, magnetCandidates, magnetSearchError,
 getEpisodeClass, available, downloaded, watched, searchMagnetList, selectMagnet, handleEpisodeTap, handleEpisodeContext,
 handleEpisodeDelete, downloadSelectedEpisode, restartSelectedEpisode, markSelectedEpisode, searchSelectedEpisode,
} = useAnimeDetail(undefined, () => fetchCalendar());
const onImgError = (event, originalUrl) => {
 const img = event.target;
 if (!img || !originalUrl || img.dataset.fallback === '1') return;
 img.dataset.fallback = '1'; img.src = originalUrl.replace(/^http:\/\//i, 'https://');
};

// 排序后的日历数据
const sortedCalendar = computed(() => {
  return [...calendar.value].sort((a, b) => a.weekday.id - b.weekday.id);
});

// 当前选中的星期数据
const activeDayData = computed(() => {
  if (!calendar.value) return null;
  const day = calendar.value.find(d => d.weekday.id === activeDayId.value);
  if (!day) return null;

  // 注入 is_collected 状态
  const itemsWithStatus = day.items.map(item => {
    const isFollowed = localFollows.value.some(f => f.subject_id === item.id);
    return { ...item, is_collected: isFollowed };
  });

  return { ...day, items: itemsWithStatus };
});

const fetchCalendar = async () => {
  try {
    loading.value = true;
    error.value = '';
    
    // 并行获取日历和本地追番列表
    const [calRes, followRes] = await Promise.all([
        GetBangumiCalendar(),
        GetLocalFollows()
    ]);

    if (followRes) {
        localFollows.value = followRes;
    }

    if (calRes) {
        calendar.value = calRes;
        if (!calRes.find(d => d.weekday.id === activeDayId.value) && calRes.length > 0) {
            activeDayId.value = calRes[0].weekday.id;
        }
    }
  } catch (err) {
    console.error(err);
    error.value = err.toString();
  } finally {
    loading.value = false;
    scrollToActiveDay();
  }
};

const toggleCollection = async (item) => {
  try {
    if (item.is_collected) {
        // 取消追番
        const res = await UnfollowLocal(item.id);
        if (res === "Success") {
            // 更新本地列表
            localFollows.value = localFollows.value.filter(f => f.subject_id !== item.id);
        } else {
            alert("取消失败: " + res);
        }
    } else {
        // 添加追番
        const res = await FollowLocal(item);
        if (res === "Success") {
            // 重新获取列表以确保同步
            const newList = await GetLocalFollows();
            localFollows.value = newList;
            await fetchCalendar();
        } else if (res === "Already followed") {
             // ignore
        } else {
            alert("添加失败: " + res);
        }
    }
  } catch (err) {
    alert("错误: " + err);
  }
};

let stopDownloadEvents;
let refreshTimer;
onMounted(() => {
  fetchCalendar();
  refreshTimer = setInterval(fetchCalendar, 60 * 60 * 1000);

  // 监听下载完成事件，实时更新 UI
  stopDownloadEvents = EventsOn("download-complete", (data) => {
      console.log("收到下载完成事件:", data);
      // data: { subject_id, ep_sort, path }
      
      // 如果当前正在查看该番剧详情，更新详情数据
      if (detailData.value && detailData.value.subject.id === data.subject_id) {
          if (!detailData.value.downloaded_eps) {
              detailData.value.downloaded_eps = [];
          }
          if (!detailData.value.downloaded_eps.includes(data.ep_sort)) {
              detailData.value.downloaded_eps.push(data.ep_sort);
          }
      }
  });

});
onUnmounted(() => {
    clearInterval(refreshTimer);
    stopDownloadEvents?.();
  });
</script>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.3s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
