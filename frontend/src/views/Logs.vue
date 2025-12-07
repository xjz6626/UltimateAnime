<template>
  <div class="p-6 text-white h-full flex flex-col">
    <h1 class="text-2xl font-bold mb-4">📜 系统日志</h1>
    
    <div class="flex-1 bg-gray-950 rounded-lg border border-gray-800 p-4 overflow-y-auto font-mono text-sm shadow-inner" ref="logContainer">
      <div v-if="logs.length === 0" class="text-gray-600 text-center mt-10">暂无日志...</div>
      <div v-for="(log, index) in logs" :key="index" class="mb-1 hover:bg-gray-900 px-2 rounded">
        <span class="text-gray-500 mr-2">[{{ log.time }}]</span>
        <span :class="getLevelClass(log.level)" class="font-bold mr-2">[{{ log.level }}]</span>
        <span class="text-gray-300 break-all">{{ log.message }}</span>
      </div>
    </div>
    
    <div class="mt-4 flex justify-end">
      <button @click="clearLogs" class="px-3 py-1 text-xs bg-gray-800 hover:bg-gray-700 rounded text-gray-400">清空日志</button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { GetLogs } from '../../wailsjs/go/main/App';

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

onMounted(() => {
  fetchHistory();
  
  // 监听后端日志事件
  EventsOn("log-message", (data) => {
    logs.value.push(data);
    // 自动滚动到底部
    nextTick(() => {
      if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight;
      }
    });
  });
});
</script>