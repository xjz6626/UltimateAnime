<template>
  <div class="flex h-full min-h-0 flex-col p-4 text-white sm:p-6">
    <h1 class="mb-4 text-xl font-bold sm:text-2xl">📜 系统日志</h1>
    
    <div class="min-h-0 flex-1 overflow-y-auto rounded-lg border border-gray-800 bg-gray-950 p-3 text-left font-mono text-xs shadow-inner sm:p-4 sm:text-sm" ref="logContainer">
      <div v-if="logs.length === 0" class="text-gray-600 text-center mt-10">暂无日志...</div>
      <div v-for="(log, index) in logs" :key="index" class="mb-1 hover:bg-gray-900 px-2 rounded">
        <span class="text-gray-500 mr-2">[{{ log.time }}]</span>
        <span :class="getLevelClass(log.level)" class="font-bold mr-2">[{{ log.level }}]</span>
        <span class="text-gray-300 break-all">{{ log.message }}</span>
      </div>
    </div>
    
    <div class="mt-4 flex justify-end">
      <button @click="clearLogs" class="min-h-[44px] rounded bg-gray-800 px-4 py-2 text-sm text-gray-300 hover:bg-gray-700">清空日志</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue';
import { EventsOn, GetLogs } from '../api';

const logs = ref([]);
const logContainer = ref(null);

const getLevelClass = (level) => {
  switch (level) {
    case 'INFO': return 'text-blue-400';
    case 'WARN': return 'text-yellow-400';
    case 'ERROR': return 'text-red-500';
    case 'SUCCESS': return 'text-green-400';
    default: return 'text-gray-400';
  }
};

const clearLogs = () => {
  logs.value = [];
};

const fetchHistory = async () => {
  try {
    const history = await GetLogs();
    if (history) {
      logs.value = history;
      nextTick(() => {
        if (logContainer.value) {
          logContainer.value.scrollTop = logContainer.value.scrollHeight;
        }
      });
    }
  } catch (e) {
    console.error("获取日志历史失败", e);
  }
};

let stopLogEvents;
onMounted(() => {
  fetchHistory();
  
  // 监听后端日志事件
  stopLogEvents = EventsOn("log-message", (data) => {
    logs.value.push(data);
    // 自动滚动到底部
    nextTick(() => {
      if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight;
      }
    });
  });
});
onUnmounted(() => stopLogEvents?.());
</script>
