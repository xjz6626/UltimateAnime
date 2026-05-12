## 🚀 用户使用指南（无需编译）

### 第一步：下载

前往 [Releases 页面](https://github.com/xjz6626/UltimateAnime/releases) 下载最新版 zip：

- Windows 10/11 → `UltimateAnime-vX.X.X-windows-amd64.zip`

解压到一个**英文路径**目录，例如 `D:\UltimateAnime\`（避免中文路径偶尔引起的玄学问题）。

### 第二步：准备账号

应用启动前你需要准备：

1. **PikPak 账号**（必须）
   - 没账号去 [pikpak.com](https://mypikpak.com/) 免费注册
   - 建议注册 2~3 个账号，应对免费用户每日下载限额
   - 所有账号建议用**同一个密码**（应用支持多账号轮询，但目前共用密码）

2. **HTTP 代理**（多数用户必须）
   - 国内网络通常无法直连 bgm.tv 和 PikPak
   - 准备一个 HTTP 代理，如 Clash / V2rayN / Mihomo
   - 记下代理监听端口，常见的是 `http://127.0.0.1:7890` 或 `7897`

3. **MPV 播放器**（推荐）
   - 去 [mpv.io](https://mpv.io/installation/) 下载 Windows 版
   - 解压记下 `mpv.exe` 的完整路径，例如 `C:\mpv\mpv.exe`

4. **Bangumi Token**（可选，只在想同步在线追番列表时需要）
   - 去 [bgm.tv/dev/app](https://bgm.tv/dev/app) 登录并创建应用
   - 复制 Access Token

### 第三步：配置

1. 把解压目录里的 `config.example.json` **复制一份**改名为 `config.json`
2. 用记事本/VSCode 打开 `config.json`，填入你的信息：

```json
{
  "global_settings": {
    "bangumi_api_token": "（可选）你的 Bangumi Token",
    "pikpak_users": [
      "your-email-1@example.com",
      "your-email-2@example.com"
    ],
    "pikpak_password": "你的 PikPak 密码",
    "auto_login": true,
    "proxy": "http://127.0.0.1:7897"
  },
  "local_storage": {
    "anime_dir": "Downloads"
  },
  "player": {
    "mpv_path": "C:\\mpv\\mpv.exe",
    "mpv_args": ""
  }
}
```

⚠️ **`proxy` 字段务必填对你本地代理的端口**，否则图片和 API 都无法加载。

### 第四步：运行

双击 `UltimateAnime.exe` 即可。首次启动可能会有 Windows Defender 提示——这是因为 exe 没做代码签名（个人开发者签名很贵），点 **"仍要运行"** 即可。

启动后会自动：
- 在 `127.0.0.1:54321` 启动图片代理服务（用于显示番剧封面）
- 尝试自动登录 PikPak（如果 `auto_login: true`）
- 加载 Bangumi 当季新番列表

### 第五步：开始追番

1. 进入 **"当季新番"** 页面，选你想看的番剧
2. 点 **"💖 追番"** 加入追番列表
3. 在详情弹窗里点剧集按钮 → 选磁力链接 → 自动离线下载到 PikPak
4. 下载完成后剧集变蓝，左键点击调用 MPV 播放

详细操作见上方 **使用指南** 章节。

### 常见问题

**Q: 启动后封面图全是灰色的？**  
A: 通常是代理没配对。检查 `config.json` 的 `proxy` 是否填了正确的本地代理端口，并确认代理软件（如 Clash）正在运行。

**Q: PikPak 自动登录失败？**  
A: 进入 **"系统设置"** 页面，检查账号密码是否正确。PikPak 可能会有验证码风控，多试几次或换网络环境。

**Q: 点击剧集没反应 / 提示"请先登录 PikPak"？**  
A: 等待左下角状态指示灯变绿（PikPak 已连接），再点剧集。

**Q: 想升级到新版本？**  
A: 下载新版 zip，**只替换 `UltimateAnime.exe`** 即可。`config.json` 和 `followed.json` 是你的数据，保留不动。

**Q: 数据存在哪里？**  
A: 所有数据都在 exe 同目录下：
- `config.json` - 你的配置
- `followed.json` - 你的追番列表
- `cache/` - 缓存（可删）
- `Downloads/` - 下载的视频