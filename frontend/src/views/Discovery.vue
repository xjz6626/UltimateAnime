<template>
  <div class="p-6">
    <h1 class="text-2xl font-bold mb-4 text-white">📅 当季新番 (Discovery)</h1>
    
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
      <div class="flex space-x-2 overflow-x-auto pb-2 border-b border-gray-700">
        <button 
          v-for="day in sortedCalendar" 
          :key="day.weekday.id"
          @click="activeDayId = day.weekday.id"
          :class="[
            'px-4 py-2 rounded-full text-sm font-medium transition-all whitespace-nowrap',
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
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
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
                </div>
                
                <div class="space-y-2">
                  <div class="flex items-center justify-between text-xs text-gray-500">
                     <span>{{ item.air_date }}</span>
                     <span v-if="item.collection" class="text-pink-400">{{ item.collection.doing || 0 }}人在追</span>
                  </div>
                  
                  <!-- 追番按钮 -->
                  <button 
                    @click.stop="toggleCollection(item)"
                    class="w-full py-1.5 rounded text-xs font-bold transition-colors flex items-center justify-center gap-1"
                    :class="item.is_collected ? 'bg-gray-700 text-gray-300 hover:bg-red-900/50 hover:text-red-400' : 'bg-pink-600 text-white hover:bg-pink-700'"
                  >
                    <span v-if="item.is_collected">❤️ 已追番</span>
                    <span v-else>🤍 追番</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 番剧详情弹窗 -->
      <div v-if="selectedItem" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black opacity-50" @click="selectedItem = null"></div>
        <div class="bg-gray-900 rounded-lg overflow-hidden shadow-lg max-w-lg w-full z-10">
          <!-- 加载状态 -->
          <div v-if="detailLoading" class="p-6 text-center">
            <p class="text-gray-400">正在加载番剧详情...</p>
          </div>
          
          <!-- 错误提示 -->
          <div v-else-if="detailError" class="p-6 text-red-400">
            <p>加载失败: {{ detailError }}</p>
            <button @click="showDetail(selectedItem)" class="mt-2 px-4 py-2 bg-pink-600 rounded text-white text-sm">重试</button>
          </div>
          
          <!-- 番剧详情内容 -->
          <div v-else class="p-6">
            <h2 class="text-xl font-bold text-white mb-4">{{ detailData.name }}</h2>
            
            <div class="flex gap-4 mb-4">
              <div class="w-32 h-48 bg-gray-700 rounded overflow-hidden relative">
                 <img v-if="detailData.images" :src="proxyImg(detailData.images.common || detailData.images.medium)" @error="onImgError($event, detailData.images.common || detailData.images.medium)" referrerpolicy="no-referrer" class="w-full h-full object-cover" loading="lazy" />
              </div>
              
              <div class="flex-1 min-w-0">
                <p class="text-sm text-gray-400 mb-2" v-if="detailData.aka && detailData.aka.length">别名: <span class="text-white">{{ detailData.aka.join(', ') }}</span></p>
                <p class="text-sm text-gray-400 mb-2">类型: <span class="text-white">{{ detailData.type }}</span></p>
                <p class="text-sm text-gray-400 mb-2">状态: <span class="text-white">{{ detailData.status }}</span></p>
                <p class="text-sm text-gray-400 mb-2">首播: <span class="text-white">{{ detailData.air_date }}</span></p>
                <p class="text-sm text-gray-400 mb-2">评分: <span class="text-white">{{ detailData.rating ? detailData.rating.score : '暂无' }}</span></p>
                
                <!-- 追番按钮 -->
                <div class="mt-4">
                  <button 
                    @click.stop="toggleCollection(detailData)"
                    class="w-full py-2 rounded text-sm font-bold transition-colors flex items-center justify-center gap-2"
                    :class="detailData.is_collected ? 'bg-gray-700 text-gray-300 hover:bg-red-900/50 hover:text-red-400' : 'bg-pink-600 text-white hover:bg-pink-700'"
                  >
                    <span v-if="detailData.is_collected">❤️ 已追番</span>
                    <span v-else>🤍 追番</span>
                  </button>
                </div>
              </div>
            </div>
            
            <div class="text-sm text-gray-400" v-if="detailData.intro">
              <h3 class="text-white font-semibold mb-2">简介</h3>
              <p>{{ detailData.intro }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <div v-if="selectedItem" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm" @click="selectedItem = null">
      <div class="bg-gray-900 rounded-xl max-w-4xl w-full max-h-[90vh] overflow-y-auto shadow-2xl border border-gray-700" @click.stop>
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
          <div class="h-48 overflow-hidden relative">
             <img :src="proxyImg(detailData.subject.images.large)" @error="onImgError($event, detailData.subject.images.large)" referrerpolicy="no-referrer" class="w-full object-cover opacity-30 blur-sm transform scale-110">
             <div class="absolute inset-0 bg-gradient-to-b from-transparent to-gray-900"></div>
             <button @click="selectedItem = null" class="absolute top-4 right-4 bg-black/50 text-white rounded-full w-8 h-8 flex items-center justify-center hover:bg-black/80">✕</button>
          </div>
          
          <div class="px-8 pb-8 -mt-20 relative flex flex-col md:flex-row gap-6 items-start">
            <!-- 封面图 -->
            <div class="w-48 flex-shrink-0 rounded-lg overflow-hidden shadow-2xl border-4 border-gray-800 bg-gray-800">
              <img :src="proxyImg(detailData.subject.images.large || detailData.subject.images.common)" @error="onImgError($event, detailData.subject.images.large || detailData.subject.images.common)" referrerpolicy="no-referrer" class="w-full h-auto block">
            </div>
            
            <!-- 信息区域 -->
            <div class="flex-1 pt-4 md:pt-20 text-left">
              <h2 class="text-3xl font-bold text-white mb-1">{{ detailData.subject.name_cn || detailData.subject.name }}</h2>
              <p class="text-gray-400 text-sm mb-4">{{ detailData.subject.name }}</p>
              
              <div class="flex flex-wrap gap-4 mb-6 text-sm">
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
                <h3 class="text-lg font-bold text-white mb-3">剧集列表 <span class="text-xs font-normal text-gray-500 ml-2">(左键: 播放/下载，右键: 标记观看，中键: 删除)</span></h3>
                <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-2 max-h-40 overflow-y-auto pr-2">
                  <div v-for="ep in detailData.episodes" :key="ep.id" 
                       @click="handleEpisodeClick(ep, $event)"
                       @contextmenu.prevent="toggleWatched(ep)"
                       @mousedown.middle.prevent="handleEpisodeDelete(ep)"
                       class="px-2 py-1.5 rounded text-center text-xs truncate cursor-pointer transition-colors border border-transparent relative"
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
    <div v-if="showMagnetPicker" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showMagnetPicker = false">
      <div class="bg-gray-900 rounded-xl shadow-2xl max-w-4xl w-full max-h-[80vh] overflow-hidden border border-gray-700">
        <div class="p-6 border-b border-gray-700">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-xl font-bold text-white">选择磁力链接 - 第 {{ magnetPickerEp?.sort }} 集</h2>
            <button @click="showMagnetPicker = false" class="text-gray-400 hover:text-white text-2xl">&times;</button>
          </div>
          
          <!-- 搜索关键词输入 -->
          <div class="flex gap-2">
            <input 
              v-model="magnetSearchKeyword" 
              @keyup.enter="searchMagnetList"
              class="flex-1 bg-gray-800 border border-gray-700 rounded px-4 py-2 text-white focus:border-pink-500 focus:outline-none"
              placeholder="输入关键词搜索（留空使用默认）"
            />
            <button 
              @click="searchMagnetList" 
              :disabled="magnetSearching"
              class="bg-pink-600 hover:bg-pink-700 disabled:bg-gray-600 text-white px-6 py-2 rounded font-bold transition"
            >
              {{ magnetSearching ? '搜索中...' : '🔍 搜索' }}
            </button>
          </div>
        </div>

        <!-- 候选列表 -->
        <div class="p-6 overflow-y-auto max-h-[calc(80vh-200px)]">
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
              <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0">
                  <h3 class="text-white font-medium mb-2 break-words">{{ item.title }}</h3>
                  <div class="flex flex-wrap gap-3 text-xs text-gray-400">
                    <span>📦 {{ item.size }}</span>
                    <span>👤 {{ item.source }}</span>
                    <span>📅 {{ item.publish_date }}</span>
                  </div>
                </div>
                <button class="ml-4 bg-pink-600 hover:bg-pink-700 text-white px-4 py-2 rounded text-sm font-bold flex-shrink-0">
                  选择
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue';
import { GetBangumiCalendar, FollowLocal, UnfollowLocal, GetLocalFollows, GetAnimeDetail, ToggleEpisodeWatched, SearchEpisodeMagnet, PlayMagnet, DownloadEpisode, PlayLocalEpisode, DeleteEpisodeData, SearchEpisodeMagnetList, SaveEpisodeMagnet } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

const calendar = ref([]);
const localFollows = ref([]); // 本地追番列表
const loading = ref(true);
const error = ref('');
const activeDayId = ref(new Date().getDay() || 7); // 默认为今天 (周日=0 -> 7)

// 详情弹窗相关
const selectedItem = ref(null);
const detailLoading = ref(false);
const detailData = ref(null);
const detailError = ref('');

// 磁力选择弹窗相关
const showMagnetPicker = ref(false);
const magnetPickerEp = ref(null);
const magnetSearchKeyword = ref('');
const magnetSearching = ref(false);
const magnetCandidates = ref([]);

// 把 bgm.tv 的图片走后端代理（绕开 Webview 直连被切的问题）
// 后端代理服务在 127.0.0.1:54321，PikPak 登录后自动启动
const proxyImg = (url) => {
  if (!url) return '';
  // 先升级 http -> https，避免混合内容警告
  const httpsUrl = url.replace(/^http:\/\//i, 'https://');
  // 已经是本地代理的不重复包装
  if (httpsUrl.startsWith('http://127.0.0.1:')) return httpsUrl;
  // 非 http(s) 直接返回（base64、data: 等）
  if (!/^https?:\/\//i.test(httpsUrl)) return httpsUrl;
  return `http://127.0.0.1:54321/img?u=${encodeURIComponent(httpsUrl)}`;
};

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
  selectedItem.value = item;
  detailLoading.value = true;
  detailError.value = '';
  detailData.value = null;
  
  try {
    const res = await GetAnimeDetail(item.id);
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

  // 如果未追番，提示先追番
  if (!detailData.value.is_followed) {
    // 自动追番? 或者提示
    if (confirm("需要先追番才能记录进度，是否立即追番？")) {
        await toggleCollection(selectedItem.value);
        // 刷新详情以获取最新的 is_followed 状态
        const res = await GetAnimeDetail(selectedItem.value.id);
        detailData.value = res;
    } else {
        return;
    }
  }

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
    } else {
        // alert("操作失败: " + res);
    }
  } catch (e) {
    console.error(e);
  }
};

const getMagnet = async (ep) => {
  if (!detailData.value.is_followed) {
      alert("请先追番");
      return "";
  }
  
  const epKey = ep.sort.toString();
  if (detailData.value.episode_magnets && detailData.value.episode_magnets[epKey]) {
      return detailData.value.episode_magnets[epKey];
  }
  
  try {
      console.log(`正在搜索第 ${ep.sort} 集...`);
      const res = await SearchEpisodeMagnet(detailData.value.subject.id, ep.sort);
      if (res.startsWith("Error")) {
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
        try {
            console.log("正在请求播放本地文件...");
            const res = await PlayLocalEpisode(detailData.value.subject.id, ep.sort);
            
            if (res === "Error: FileMissing") {
                // 文件丢失，自动触发下载
                console.log("文件丢失，尝试重新下载...");
                
                // 更新 UI 状态 (移除已下载标记)
                detailData.value.downloaded_eps = detailData.value.downloaded_eps.filter(s => s !== ep.sort);
                
                // 打开磁力选择弹窗
                magnetPickerEp.value = ep;
                magnetSearchKeyword.value = '';
                showMagnetPicker.value = true;
                await searchMagnetList();
                return;
            }

            if (res !== "Success") {
                alert(res);
            }
        } catch (e) {
            alert("播放请求失败: " + e);
        }
        return;
    }
    
    // 2. 如果未下载，打开磁力选择弹窗
    magnetPickerEp.value = ep;
    magnetSearchKeyword.value = '';
    showMagnetPicker.value = true;
    await searchMagnetList();
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
    await SaveEpisodeMagnet(detailData.value.subject.id, magnetPickerEp.value.sort, item.magnet);
    
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

onMounted(() => {
  fetchCalendar();

  // 监听下载完成事件，实时更新 UI
  EventsOn("download-complete", (data) => {
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