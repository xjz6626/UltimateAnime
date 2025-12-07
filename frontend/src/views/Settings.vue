<template>
  <div class="p-6 text-white">
    <h1 class="text-2xl font-bold mb-6">⚙️ 系统设置</h1>
    
    <div class="space-y-6 max-w-2xl">
      <!-- PikPak 设置 -->
      <div class="bg-gray-800 p-6 rounded-lg shadow-lg">
        <h2 class="text-lg font-semibold mb-4 text-pink-500 flex items-center">
          <span class="mr-2">☁️</span> PikPak 账号
        </h2>
        <div class="space-y-4">
          <div>
            <label class="block text-sm text-gray-400 mb-2">账号列表</label>
            <div v-for="(user, index) in config.global_settings.pikpak_users" :key="index" class="flex gap-2 mb-2 items-center">
              <input v-model="config.global_settings.pikpak_users[index]" type="text" class="flex-1 bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors" placeholder="输入账号 (邮箱/手机)">
              
              <!-- 封禁状态控制 -->
              <button 
                v-if="user"
                @click="toggleBlockStatus(user)" 
                class="px-3 py-2.5 rounded transition text-sm font-bold whitespace-nowrap"
                :class="isBlocked(user) ? 'bg-red-600 text-white hover:bg-red-700' : 'bg-green-900/30 text-green-400 hover:bg-green-900/50'"
                :title="isBlocked(user) ? '点击解封 (已封禁: ' + getBlockDate(user) + ')' : '点击手动封禁'"
              >
                {{ isBlocked(user) ? '🚫 已封禁' : '✅ 正常' }}
              </button>

              <button @click="removeUser(index)" class="px-3 bg-red-900/50 text-red-400 rounded hover:bg-red-900 hover:text-white transition h-[42px]">🗑️</button>
            </div>
            <button @click="addUser" class="text-sm text-pink-400 hover:text-pink-300 flex items-center mt-2">
              <span class="mr-1">+</span> 添加账号
            </button>
          </div>
          
          <div class="pt-2 border-t border-gray-700">
            <label class="block text-sm text-gray-400 mb-1">统一密码 <span class="text-xs text-gray-500">(所有账号共用)</span></label>
            <input v-model="config.global_settings.pikpak_password" type="password" class="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors" placeholder="输入密码">
          </div>

          <div class="pt-2 flex items-center justify-between">
            <label class="text-sm text-gray-400">启动时自动登录</label>
            <div class="relative inline-block w-12 mr-2 align-middle select-none transition duration-200 ease-in">
                <input type="checkbox" v-model="config.global_settings.auto_login" class="toggle-checkbox absolute block w-6 h-6 rounded-full bg-white border-4 appearance-none cursor-pointer transition-transform duration-200 ease-in-out" :class="{'translate-x-6 border-pink-600': config.global_settings.auto_login, 'border-gray-300': !config.global_settings.auto_login}"/>
                <label class="toggle-label block overflow-hidden h-6 rounded-full bg-gray-700 cursor-pointer" :class="{'bg-pink-900': config.global_settings.auto_login}"></label>
            </div>
          </div>
        </div>
      </div>

      <!-- 播放器设置 -->
      <div class="bg-gray-800 p-6 rounded-lg shadow-lg">
        <h2 class="text-lg font-semibold mb-4 text-blue-500 flex items-center">
          <span class="mr-2">🎬</span> 播放器设置 (MPV)
        </h2>
        <div class="space-y-4">
           <div>
            <label class="block text-sm text-gray-400 mb-1">MPV 路径 <span class="text-xs text-gray-500">(mpv.exe 的完整路径)</span></label>
            <div class="flex gap-2">
                <input v-model="config.player.mpv_path" type="text" class="flex-1 bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors" placeholder="例如: C:\Program Files\MPV\mpv.exe">
                <!-- 暂时不做文件选择器，让用户手动填 -->
            </div>
          </div>
           <div>
            <label class="block text-sm text-gray-400 mb-1">启动参数 <span class="text-xs text-gray-500">(可选)</span></label>
            <input v-model="config.player.mpv_args" type="text" class="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors" placeholder="例如: --fullscreen --volume=50">
          </div>
        </div>
      </div>

      <!-- 其他设置 -->
      <div class="bg-gray-800 p-6 rounded-lg shadow-lg">
        <h2 class="text-lg font-semibold mb-4 text-gray-400 flex items-center">
          <span class="mr-2">🛠️</span> 高级设置
        </h2>
        <div class="space-y-4">
           <div>
            <label class="block text-sm text-gray-400 mb-1">HTTP 代理 <span class="text-xs text-gray-500">(例如 http://127.0.0.1:7890)</span></label>
            <input v-model="config.global_settings.proxy" type="text" class="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors" placeholder="留空则不使用代理">
          </div>
           <div>
            <label class="block text-sm text-gray-400 mb-1">下载目录</label>
            <input v-model="config.local_storage.anime_dir" type="text" class="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-white focus:border-pink-500 focus:outline-none transition-colors">
          </div>
      </div>
      </div>

      <!-- 保存按钮 -->
      <div class="flex justify-end pt-4">
        <button @click="saveConfig" :disabled="saving" class="bg-pink-600 hover:bg-pink-700 disabled:bg-gray-600 text-white px-6 py-3 rounded-lg font-bold shadow-lg transition-all transform hover:scale-105 flex items-center">
          <span v-if="saving" class="animate-spin mr-2">⏳</span>
          {{ saving ? '保存中...' : '💾 保存配置' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { GetAppConfig, SaveAppConfig, GetBlockedAccounts, SetAccountBlockStatus } from '../../wailsjs/go/main/App';

const saving = ref(false);
const config = ref({
  global_settings: { pikpak_users: [''], pikpak_password: '', proxy: '', auto_login: false },
  local_storage: { anime_dir: '' },
  player: { mpv_path: '', mpv_args: '' }
});
const blockedAccounts = ref({});

const loadConfig = async () => {
  try {
    const res = await GetAppConfig();
    // 确保数组存在
    if (!res.global_settings.pikpak_users) res.global_settings.pikpak_users = [''];
    if (res.global_settings.pikpak_users.length === 0) res.global_settings.pikpak_users.push('');
    // 确保 player 存在
    if (!res.player) res.player = { mpv_path: '', mpv_args: '' };
    
    config.value = res;
  } catch (err) {
    console.error("加载配置失败", err);
  }
};

const loadBlockedAccounts = async () => {
  try {
    blockedAccounts.value = await GetBlockedAccounts();
  } catch (err) {
    console.error("加载封禁列表失败", err);
  }
};

const isBlocked = (user) => {
  return !!blockedAccounts.value[user];
};

const getBlockDate = (user) => {
  return blockedAccounts.value[user] || '';
};

const toggleBlockStatus = async (user) => {
  if (!user) return;
  const currentStatus = isBlocked(user);
  // 如果当前是封禁状态，则解封(false)；如果是正常状态，则封禁(true)
  const newStatus = !currentStatus;
  
  try {
    await SetAccountBlockStatus(user, newStatus);
    // 刷新列表
    await loadBlockedAccounts();
  } catch (err) {
    alert("操作失败: " + err);
  }
};

const addUser = () => {
  config.value.global_settings.pikpak_users.push('');
};

const removeUser = (index) => {
  config.value.global_settings.pikpak_users.splice(index, 1);
  if (config.value.global_settings.pikpak_users.length === 0) {
    config.value.global_settings.pikpak_users.push('');
  }
};

const saveConfig = async () => {
  saving.value = true;
  try {
    const jsonStr = JSON.stringify(config.value);
    const res = await SaveAppConfig(jsonStr);
    if (res === "Success") {
      alert("配置已保存！部分设置可能需要重启生效。");
    } else {
      alert("保存失败: " + res);
    }
  } catch (err) {
    alert("保存异常: " + err);
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  loadConfig();
  loadBlockedAccounts();
});
</script>

<style scoped>
.toggle-checkbox:checked {
  right: 0;
  border-color: #db2777;
}
.toggle-checkbox:checked + .toggle-label {
  background-color: #831843;
}
.translate-x-6 {
    transform: translateX(100%);
}
</style>