<template>
  <div class="space-y-3 rounded-lg border border-gray-700 bg-gray-900/50 p-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm" :class="account.logged_in ? 'text-green-400' : 'text-gray-300'">{{ account.logged_in ? `已登录：${account.username || account.user_id}` : configured ? '已有登录信息' : '未登录，可匿名获取公开弹幕' }}</p>
      <div class="flex gap-2">
        <button @click="start" :disabled="starting" class="min-h-[44px] rounded bg-pink-600 px-4 text-sm font-bold text-white disabled:bg-gray-700">{{ starting ? '正在获取…' : challenge ? '刷新二维码' : '扫码登录 B 站' }}</button>
        <button v-if="configured || account.logged_in" @click="clearLogin" class="min-h-[44px] rounded bg-gray-700 px-3 text-sm text-gray-200">清除登录信息</button>
      </div>
    </div>
    <p class="text-xs leading-relaxed text-gray-400">用自己的 B 站手机 App 扫码并确认，程序会自动保存当前账号的 Cookie，无需输入账号密码。</p>
    <div v-if="challenge" class="flex flex-col items-center gap-3 py-2">
      <img v-if="qrImage" :src="qrImage" alt="B 站登录二维码" class="h-64 w-64 rounded-lg bg-white p-2" style="image-rendering: pixelated">
      <p role="status" class="text-sm text-pink-300">{{ message }}</p>
      <button @click="stop" class="min-h-[44px] px-4 text-sm text-gray-400">取消扫码</button>
    </div>
    <p v-else-if="message" role="status" class="text-sm text-green-400">{{ message }}</p>
    <p v-if="error" role="alert" class="text-sm text-red-400">{{ error }}</p>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import qrcode from 'qrcode-generator';
import { StartBilibiliLogin, PollBilibiliLogin, CancelBilibiliLogin, GetBilibiliLoginStatus, ClearBilibiliLogin } from '../api';

defineProps({ configured: Boolean });
const emit = defineEmits(['session-updated']);
const account = ref({ logged_in: false }), challenge = ref(null), qrImage = ref('');
const message = ref(''), error = ref(''), starting = ref(false);
let generation = 0, timer;
function stop() {
  generation++; clearTimeout(timer);
  const old = challenge.value;
  challenge.value = null; qrImage.value = ''; starting.value = false; message.value = '';
  if (old) CancelBilibiliLogin(old.key).catch(() => {});
}
async function refreshAccount() {
  const current = generation;
  try {
    const status = await GetBilibiliLoginStatus();
    if (current === generation) account.value = status;
  } catch { /* A temporary connection failure should not erase a saved session. */ }
}
async function start() {
  stop(); error.value = ''; starting.value = true;
  const current = generation;
  try {
    const next = await StartBilibiliLogin();
    if (current !== generation) { CancelBilibiliLogin(next.key).catch(() => {}); return; }
    const qr = qrcode(0, 'M'); qr.addData(next.url); qr.make();
    qrImage.value = qr.createDataURL(6, 24);
    challenge.value = next;
    message.value = '请使用 B 站手机 App 扫码，并在手机上确认登录';
    timer = setTimeout(() => poll(current), 2000);
  } catch (err) { if (current === generation) error.value = String(err); }
  finally { if (current === generation) starting.value = false; }
}
async function poll(current) {
  const active = challenge.value;
  if (!active || current !== generation) return;
  if (Date.now() >= active.expires_at * 1000) {
    CancelBilibiliLogin(active.key).catch(() => {});
    challenge.value = null; qrImage.value = ''; message.value = ''; error.value = '二维码已过期，请重新扫码'; return;
  }
  try {
    const result = await PollBilibiliLogin(active.key);
    if (current !== generation) return;
    message.value = result.message;
    if (result.state === 'success') {
      challenge.value = null; qrImage.value = '';
      emit('session-updated'); await refreshAccount(); return;
    }
    if (result.state === 'expired') { challenge.value = null; qrImage.value = ''; return; }
    timer = setTimeout(() => poll(current), 2000);
  } catch (err) {
    if (current !== generation) return;
    error.value = String(err);
    CancelBilibiliLogin(active.key).catch(() => {});
    challenge.value = null; qrImage.value = ''; message.value = '';
  }
}
async function clearLogin() {
  stop(); error.value = '';
  try {
    const result = await ClearBilibiliLogin();
    if (result !== 'Success') throw new Error(result);
    account.value = { logged_in: false }; message.value = '已清除本机保存的 B 站登录信息'; emit('session-updated');
  } catch (err) { error.value = String(err); }
}
onMounted(refreshAccount);
onUnmounted(stop);
</script>
