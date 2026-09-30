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

              <!-- 清空云盘按钮 -->
              <button 
                v-if="user"
                @click="clearStorageForUser(user)" 
                :disabled="clearingAccounts[user]"
                class="px-3 py-2.5 rounded transition text-sm font-bold whitespace-nowrap bg-orange-900/30 text-orange-400 hover:bg-orange-900/50 disabled:bg-gray-600 disabled:text-gray-400"
                title="清空该账号的云盘空间"
              >
                <span v-if="clearingAccounts[user]" class="animate-spin">⏳</span>
                <span v-else>🗑️ 清空</span>
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

          <p class="text-sm text-gray-400 pt-2">PikPak 会在开始下载时登录。空间不足时会永久清空当前账号云盘并重试；次数也耗尽时才切换账号。</p>

          <!-- 去掉原来的手动清空云盘按钮区域 -->
        </div>
      </div>

      <!-- 磁力选择方式 -->
      <div class="bg-gray-800 p-6 rounded-lg shadow-lg">
        <h2 class="text-lg font-semibold mb-4 text-pink-500">🧲 磁力选择</h2>
        <label class="flex items-center justify-between gap-4 cursor-pointer">
          <span>
            <span class="block font-medium">自动优选并下载</span>
            <span class="block text-sm text-gray-400 mt-1">开启后点击未下载的剧集，会按字幕和画质排序选择首条资源并开始下载；关闭时显示候选列表供你手选。</span>
          </span>
          <input v-model="config.torrent_searcher.auto_select_magnet" type="checkbox" class="w-5 h-5 accent-pink-500 flex-shrink-0">
        </label>
        <p class="text-xs text-gray-400 mt-3">已保存的磁力链接会优先复用。自动搜索失败时会打开手选窗口。</p>
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

      <div class="bg-gray-800 p-6 rounded-lg shadow-lg">
        <h2 class="text-lg font-semibold mb-3 text-blue-400">🌐 远端网页</h2>
        <p class="text-sm text-gray-300 mb-3">桌面应用运行时，可在本机打开 <a href="http://127.0.0.1:54322" target="_blank" rel="noopener noreferrer" class="text-blue-400 hover:underline">http://127.0.0.1:54322</a>。</p>
        <p class="text-sm text-gray-300 mb-2">要通过 Tailscale 从其他电脑访问，请先连接 Tailscale，并在家中电脑运行：</p>
        <code class="block bg-gray-950 border border-gray-700 rounded px-3 py-2 text-sm text-pink-300 select-all">tailscale ip -4</code>
        <p class="text-xs text-gray-400 mt-3">在另一台已登录同一 tailnet 的电脑上打开 http://显示的 IP:54322。家中电脑和本应用需要保持运行。</p>
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
import { GetAppConfig, SaveAppConfig, GetBlockedAccounts, SetAccountBlockStatus, ClearPikPakStorage } from '../api';

const saving = ref(false);
const clearingAccounts = ref({});
const config = ref({
  global_settings: { pikpak_users: [''], pikpak_password: '', proxy: '' },
  local_storage: { anime_dir: '' },
  torrent_searcher: { auto_select_magnet: false },
  player: { mpv_path: '', mpv_args: '' }
});
const blockedAccounts = ref({});

const loadConfig = async () => {
  try {
    const res = await GetAppConfig();
    // 确保数组存在
    if (!res.global_settings.pikpak_users) res.global_settings.pikpak_users = [''];
    if (res.global_settings.pikpak_users.length === 0) res.global_settings.pikpak_users.push('');
    if (!res.torrent_searcher) res.torrent_searcher = { auto_select_magnet: false };
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

const clearStorageForUser = async (username) => {
  if (!username || username.trim() === '') {
    alert("请先输入账号");
    return;
  }
  
  if (!confirm(`⚠️ 确定要清空账号 ${username} 的云盘吗？\n\n此操作将永久删除该账号的所有文件，无法恢复！`)) {
    return;
  }
  
  clearingAccounts.value[username] = true;
  try {
    const res = await ClearPikPakStorage(username);
    if (res === "Started") {
      alert(`✅ 账号 ${username} 的清空任务已启动！\n\n请在日志页面查看清理进度。`);
    } else if (res.startsWith("Error:")) {
      alert("❌ " + res);
    }
  } catch (err) {
    alert("操作失败: " + err);
  } finally {
    // 延迟 2 秒再恢复按钮，防止误点
    setTimeout(() => {
      clearingAccounts.value[username] = false;
    }, 2000);
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
const clearStorage = async () => {
  if (!confirm("⚠️ 确定要清空云盘吗？\n\n此操作将永久删除当前账号的所有文件，无法恢复！")) {
    return;
  }
  
  clearing.value = true;
  try {
    const res = await ClearPikPakStorage();
    if (res === "Started") {
      alert("✅ 清空任务已启动！\n\n请在日志页面查看清理进度。");
    } else if (res.startsWith("Error:")) {
      alert("❌ " + res);
    }
  } catch (err) {
    alert("操作失败: " + err);
  } finally {
    // 延迟 2 秒再恢复按钮，防止误点
    setTimeout(() => {
      clearing.value = false;
    }, 2000);
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
