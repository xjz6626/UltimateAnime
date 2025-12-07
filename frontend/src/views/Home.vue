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
                <h3 class="text-lg font-bold text-white mb-3">剧集列表 <span class="text-xs font-normal text-gray-500 ml-2">(左键: 播放本地/下载，右键: 标记观看)</span></h3>
                <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-2 max-h-40 overflow-y-auto pr-2">
                  <div v-for="ep in detailData.episodes" :key="ep.id" 
                       @click="handleEpisodeClick(ep, $event)"
                       @contextmenu.prevent="toggleWatched(ep)"
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { GetLocalFollows, GetAnimeDetail, ToggleEpisodeWatched, SearchEpisodeMagnet, PlayMagnet, DownloadEpisode, PlayLocalEpisode } from '../../wailsjs/go/main/App';

const collection = ref([]);
const loading = ref(true);

// 详情弹窗相关
const selectedItem = ref(null);
const detailLoading = ref(false);
const detailData = ref(null);
const detailError = ref('');

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
    
    // 2. 如果未下载，则开始下载
    const magnet = await getMagnet(ep);
    if (magnet) {
         const res = await DownloadEpisode(detailData.value.subject.id, ep.sort, magnet);
         if (res === "Started") {
             alert("已开始下载，请留意通知");
         } else {
             alert(res);
         }
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