<template>
  <div class="flex h-screen bg-gray-900 text-white font-sans">
    <div class="w-64 bg-gray-800 flex flex-col border-r border-gray-700">
      <div class="p-6 flex items-center justify-center border-b border-gray-700/50">
        <div class="w-8 h-8 bg-pink-600 rounded-lg flex items-center justify-center mr-3 shadow-lg shadow-pink-500/20">
          <span class="text-white font-bold text-lg">U</span>
        </div>
        <h1 class="text-lg font-bold tracking-wide text-gray-100">Ultimate Anime</h1>
      </div>
      
      <nav class="flex-1 p-4 space-y-2 overflow-y-auto">
        <router-link to="/" class="nav-item group" active-class="active">
          <span class="mr-3 text-xl group-hover:scale-110 transition-transform">📺</span>
          <span class="font-medium">我的追番</span>
        </router-link>
        <router-link to="/discovery" class="nav-item group" active-class="active">
          <span class="mr-3 text-xl group-hover:scale-110 transition-transform">📅</span>
          <span class="font-medium">当季新番</span>
        </router-link>
        <router-link to="/settings" class="nav-item group" active-class="active">
          <span class="mr-3 text-xl group-hover:scale-110 transition-transform">⚙️</span>
          <span class="font-medium">系统设置</span>
        </router-link>
        <router-link to="/logs" class="nav-item group" active-class="active">
          <span class="mr-3 text-xl group-hover:scale-110 transition-transform">📜</span>
          <span class="font-medium">运行日志</span>
        </router-link>
      </nav>

      <div class="p-4 bg-gray-900/50 border-t border-gray-700/50">
        <div class="flex items-center space-x-3">
          <div class="w-2 h-2 rounded-full" :class="pikpakStatus === 'Success' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-red-500'"></div>
          <div class="flex-1 min-w-0">
            <p class="text-xs text-gray-400 font-medium">PikPak 状态</p>
            <p class="text-xs text-gray-300 truncate" :title="pikpakStatus">{{ pikpakStatus === 'Success' ? '已连接' : '未连接' }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto bg-gray-900">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { GetPikPakStatus } from '../wailsjs/go/main/App'

// 这里将来调用后端的 AutoLogin
const pikpakStatus = ref("未登录") 

const checkStatus = async () => {
  try {
    const status = await GetPikPakStatus();
    pikpakStatus.value = status;
  } catch (e) {
    console.error(e);
  }
}

onMounted(() => {
    checkStatus();
    
    // 监听状态变化
    EventsOn("pikpak-status", (status) => {
        pikpakStatus.value = status;
    });
})
</script>

<style>
/* 导航按钮样式 */
.nav-item {
  display: block;
  padding: 12px 16px;
  border-radius: 8px;
  color: #9ca3af;
  transition: all 0.2s;
  text-decoration: none;
}
.nav-item:hover {
  background-color: #374151;
  color: white;
}
.nav-item.active {
  background-color: #ec4899; /* Pink-500 */
  color: white;
  font-weight: bold;
  box-shadow: 0 4px 6px rgba(0,0,0,0.3);
}

/* 页面切换动画 */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>