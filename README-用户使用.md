## 🚀 用户使用指南（无需编译）

### 第一步：下载

前往 [Releases 页面](https://github.com/xjz6626/UltimateAnime/releases) 下载对应系统的 zip：

- Windows 10/11 amd64 → `UltimateAnime-v0.3.0-windows-amd64.zip`
- Linux amd64 → `UltimateAnime-v0.3.0-linux-amd64.zip`（需要 GTK 3 和 WebKit2GTK 4.1）

解压到独立目录，例如 Windows 的 `D:\UltimateAnime\` 或 Linux 的 `~/Applications/UltimateAnime/`。Linux 请从该目录启动，使配置和追番文件能被正确读取。

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

3. **MPV 播放器**（本机播放时推荐）
   - Windows 可安装 MPV，并记下 `mpv.exe` 的完整路径，例如 `C:\mpv\mpv.exe`
   - Linux 可安装系统的 `mpv` 包；如果已在 `PATH` 中，`mpv_path` 留空即可

4. **Bangumi Token**（可选，只在想同步在线追番列表时需要）
   - 去 [bgm.tv/dev/app](https://bgm.tv/dev/app) 登录并创建应用
   - 复制 Access Token

### 第三步：配置

1. 首次使用时，把 `config.example.json` 复制为 `config.json`，把 `followed.example.json` 复制为 `followed.json`。已有旧版文件则直接保留，不要用示例文件覆盖。
2. 打开 `config.json`，填入你的信息：

```json
{
  "global_settings": {
    "bangumi_api_token": "（可选）你的 Bangumi Token",
    "pikpak_users": [
      "your-email-1@example.com",
      "your-email-2@example.com"
    ],
    "pikpak_password": "你的 PikPak 密码",
    "proxy": "http://127.0.0.1:7897"
  },
  "local_storage": {
    "anime_dir": "Downloads"
  },
  "player": {
    "mpv_path": "",
    "mpv_args": ""
  }
}
```

⚠️ **`proxy` 字段务必填对你本地代理的端口**，否则图片和 API 都无法加载。

**旧版数据兼容**：v0.1 的 `config.json` 和 `followed.json` 可以直接使用，观看记录、已选磁力与本地文件路径会保留。旧配置中的 `auto_login` 会被忽略；缺少的 `auto_select_magnet` 默认关闭。跨 Windows／Linux 迁移时，请检查代理端口和 `mpv_path`，原平台的绝对本地文件路径不会自动转换。

### 第四步：运行

Windows：在解压目录双击 `UltimateAnime.exe`。如缺少 WebView2 Runtime，请先安装微软官方运行时。

Linux：在解压目录运行：

```bash
chmod +x UltimateAnime
./UltimateAnime
```

启动后会自动：
- 在 `127.0.0.1:54321` 启动图片代理服务（用于显示番剧封面）
- 开始下载时使用已保存的 PikPak 账号登录
- 加载 Bangumi 当季新番列表

### 第五步：开始追番

1. 进入 **"当季新番"** 页面，选你想看的番剧
2. 点 **"💖 追番"** 加入追番列表
3. 在详情弹窗里点剧集按钮 → 选磁力链接 → 自动离线下载到 PikPak

“当季新番”每天刷新 Bangumi 列表。对已追番剧，程序每周查询 AniList 的下一集排期；确认会在新季度播出的番剧会继续留在日历中。无法对应到 AniList 或查不到下一集的番剧不会被猜测为跨季续播。

“我的追番”每天查询当日排期，将今天播出的作品放在最前面，并标出预计下集时间。该时间表示电视播出安排，磁力资源可能稍后才发布。

追番页的“隐藏完结”按钮仅隐藏 AniList 标记为 `FINISHED` 的作品，不会删除追番；停播或排期未知的作品继续显示。

4. 下载完成后剧集变蓝，左键点击调用 MPV 播放

默认需要手动选择磁力。若想点击剧集后自动优选并下载，可在桌面端“系统设置 → 磁力选择”打开开关，或在 `config.json` 的 `torrent_searcher` 中设置 `"auto_select_magnet": true`；网页端也会按这个设置运行。

下载前默认显示 B 站弹幕来源选择窗口。可搜索番剧或普通视频，也可直接输入 BV 号、`ss`／`ep` 番剧链接；点选视频后确认对应集数或分 P。确认后会保存选择并下载弹幕，转换成视频旁的 `视频名.danmaku.ass`。选“仅下载视频”可跳过弹幕；关闭窗口会取消本次下载。已有本地视频可在番剧详情页点击“选择／更换 B 站弹幕”，选择目标集数并保存，程序会重新获取弹幕，无需重新下载视频。下次启动 MPV 时加载更新后的弹幕。

在“系统设置 → B 站弹幕”可关闭整个功能，或开启“自动匹配弹幕来源”。自动模式比较追番中文名和原名，统一季度写法并核对可用的年份，再匹配 B 站标注的正片集数；有多个无法区分的同名结果时跳过，不按列表顺序猜集数。旧配置缺少 `danmaku` 时默认开启弹幕、关闭自动匹配；手动配置可添加 `"danmaku": {"enabled": true, "auto_select": false, "bilibili_cookie": ""}`。

Cookie 是网站识别登录会话的信息。这里使用你自己的 B 站账号登录后的 Cookie；无需手动获取，在设置里点击“扫码登录 B 站”，用 B 站手机 App 扫码并确认即可自动获取并保存到本机 `config.json`，扫码成功立即生效。支持取消、过期后刷新及清除本机登录信息，仍可在高级选项手动填写 Cookie。账号登录只在桌面端设置，远端网页可以选择弹幕来源但不会收到 Cookie。

MPV 自动加载 ASS 弹幕，普通字幕可作为第二字幕同时显示；无需安装转换工具。弹幕搜索或下载失败不会中断视频下载，原因可在日志中查看。弹幕来自当前弹幕池，不保证包含全部历史弹幕；不同版本的片头或剪辑可能导致时间偏移，可用 MPV 的字幕延迟调整。删除本地视频时也会删除对应的 ASS 弹幕文件。

手机远端访问：先在家中电脑启动程序并连接 Tailscale，再在手机上连接同一 tailnet，打开 `http://家中电脑的 Tailscale IPv4:54322`。手机网页使用底部导航；点按剧集后可选择下载或标记观看。网页端不提供账号设置和本地播放器。


详细操作见上方 **使用指南** 章节。

### 常见问题

**Q: 启动后封面图全是灰色的？**  
A: 通常是代理没配对。检查 `config.json` 的 `proxy` 是否填了正确的本地代理端口，并确认代理软件（如 Clash）正在运行。

**Q: 下载时 PikPak 登录失败？**
A: 进入 **"系统设置"** 页面，检查账号密码及代理地址。PikPak 可能要求验证码，请查看运行日志中的具体错误。

**Q: 启动后 PikPak 显示未连接？**
A: 这是正常状态。选择磁力开始下载时，程序才会尝试登录。

**Q: 想升级到新版本？**  
A: 退出旧程序，备份 `config.json` 和 `followed.json`，下载对应系统的新版 zip，并替换程序文件。新包只有示例配置，不会提供真实 `config.json` 或 `followed.json`；原文件保留不动即可直接升级。`Downloads/` 也不要删除。

**Q: 数据存在哪里？**  
A: 从程序所在目录启动时，数据保存在该目录：
- `config.json` - 你的配置
- `followed.json` - 你的追番列表
- `cache/` - 缓存（可删）
- `Downloads/` - 下载的视频
