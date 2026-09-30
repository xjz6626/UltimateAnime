<template>
  <div class="flex h-screen h-[100dvh] min-w-0 bg-gray-900 text-white font-sans" :class="isWebMode() ? 'flex-col lg:flex-row' : 'flex-row'">
    <div class="w-64 shrink-0 flex-col border-r border-gray-700 bg-gray-800" :class="isWebMode() ? 'hidden lg:flex' : 'flex'">
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
        <router-link v-if="!isWebMode()" to="/settings" class="nav-item group" active-class="active">
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
            <p class="text-xs text-gray-300 truncate" :title="pikpakStatus">{{ pikpakStatus === 'Success' ? '已连接' : '下载时连接' }}</p>
          </div>
        </div>
      </div>
    </div>

    <header v-if="isWebMode()" class="mobile-header flex shrink-0 items-center justify-between gap-3 border-b border-gray-700 bg-gray-800 px-4 py-3 lg:hidden">
      <div class="flex min-w-0 items-center gap-2.5">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-pink-600 font-bold">U</div>
        <span class="truncate font-bold">Ultimate Anime</span>
      </div>
      <span class="shrink-0 text-xs" :class="pikpakStatus === 'Success' ? 'text-green-400' : 'text-gray-400'">
        <span aria-hidden="true">●</span> {{ pikpakStatus === 'Success' ? '已连接' : '下载时连接' }}
      </span>
    </header>

    <main class="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain bg-gray-900">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>

    <nav v-if="isWebMode()" class="mobile-nav grid shrink-0 grid-cols-3 border-t border-gray-700 bg-gray-800 lg:hidden" aria-label="主导航">
      <router-link to="/" class="mobile-nav-item" active-class="active">
        <span aria-hidden="true" class="text-xl">📺</span><span>追番</span>
      </router-link>
      <router-link to="/discovery" class="mobile-nav-item" active-class="active">
        <span aria-hidden="true" class="text-xl">📅</span><span>新番</span>
      </router-link>
      <router-link to="/logs" class="mobile-nav-item" active-class="active">
        <span aria-hidden="true" class="text-xl">📜</span><span>日志</span>
      </router-link>
    </nav>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { EventsOn, GetPikPakStatus, isWebMode } from './api'

const pikpakStatus = ref("未登录") 

const checkStatus = async () => {
  try {
    const status = await GetPikPakStatus();
    pikpakStatus.value = status;
  } catch (e) {
    console.error(e);
  }
}

let stopStatusEvents;
onMounted(() => {
    checkStatus();
    
    // 监听状态变化
    stopStatusEvents = EventsOn("pikpak-status", (status) => {
        pikpakStatus.value = status;
    });
})
onUnmounted(() => stopStatusEvents?.())
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

.mobile-header { padding-top: max(0.75rem, env(safe-area-inset-top)); }
.mobile-nav { padding-bottom: env(safe-area-inset-bottom); }
.mobile-nav-item {
  display: flex;
  min-height: 3.5rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.125rem;
  color: #9ca3af;
  font-size: 0.75rem;
  text-decoration: none;
}
.mobile-nav-item.active { color: #f9a8d4; font-weight: bold; }

/* 页面切换动画 */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
