<template>
  <div class="p-6">
    <h1 class="text-2xl font-bold mb-4 text-white">📺 我的追番 (本地)</h1>
    
    <div v-if="loading" class="text-gray-400">加载中...</div>
    <div v-else-if="collection.length === 0" class="text-gray-500 text-center mt-10">
      <p>你还没有正在追的番剧哦~</p>
      <p class="text-sm mt-2">去 <router-link to="/discovery" class="text-pink-500 hover:underline">当季新番</router-link> 看看吧！</p>
    </div>

    <div v-else class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
      <div v-for="item in collection" :key="item.subject_id" @click="showDetail(item)" class="bg-gray-800 rounded-lg overflow-hidden hover:scale-105 transition cursor-pointer shadow-lg group relative">
        <div class="relative aspect-[2/3]">
          <img :src="item.image" class="w-full h-full object-cover"/>
          <div class="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-40 transition flex items-center justify-center">
            <span class="text-white opacity-0 group-hover:opacity-100 text-4xl">▶</span>
          </div>
        </div>
        <div class="p-3">
          <div class="font-bold text-gray-200 truncate text-sm" :title="item.name_cn || item.name">
            {{ item.name_cn || item.name }}
          </div>
          <div class="text-xs text-gray-500 mt-1 truncate">{{ item.name }}</div>
          <div class="text-xs text-gray-600 mt-1">添加于: {{ item.added_at.split(' ')[0] }}</div>
        </div>
      </div>
    </div>

    <!-- 详情弹窗 (复用 Discovery 的逻辑) -->
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
             <img :src="detailData.subject.images.large" class="w-full object-cover opacity-30 blur-sm transform scale-110">
             <div class="absolute inset-0 bg-gradient-to-b from-transparent to-gray-900"></div>
             <button @click="selectedItem = null" class="absolute top-4 right-4 bg-black/50 text-white rounded-full w-8 h-8 flex items-center justify-center hover:bg-black/80">✕</button>
          </div>
          
          <div class="px-8 pb-8 -mt-20 relative flex flex-col md:flex-row gap-6 items-start">
            <!-- 封面图 -->
            <div class="w-48 flex-shrink-0 rounded-lg overflow-hidden shadow-2xl border-4 border-gray-800 bg-gray-800">
              <img :src="detailData.subject.images.large || detailData.subject.images.common" class="w-full h-auto block">
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
import { ref, onMounted } from 'vue';
import { GetLocalFollows, GetAnimeDetail, ToggleEpisodeWatched, SearchEpisodeMagnet, PlayMagnet, DownloadEpisode, PlayLocalEpisode, DeleteEpisodeData, SearchEpisodeMagnetList, SaveEpisodeMagnet } from '../../wailsjs/go/main/App';

const collection = ref([]);
const loading = ref(true);

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

const showDetail = async (item) => {
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

const fetchCollection = async () => {
  try {
    loading.value = true;
    const res = await GetLocalFollows();
    if (res) {
      // 按添加时间倒序
      collection.value = res.reverse();
    }
  } catch (err) {
    console.error(err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchCollection();
});
</script>