

# UltimateAnime 📺

![Wails](https://img.shields.io/badge/Built%20With-Wails-red) ![Go](https://img.shields.io/badge/Backend-Go-blue) ![Vue](https://img.shields.io/badge/Frontend-Vue%203-green) ![License](https://img.shields.io/badge/License-MIT-yellow)

**UltimateAnime** 是一个基于 [Wails](https://wails.io/) 构建的现代化、一站式二次元资源管理桌面应用。

它创造性地集成了 **Bangumi 番组计划**（追番管理）、**Anime Garden**（资源搜索）以及 **PikPak**（云端下载与流式播放），配合本地 **MPV 播放器**，旨在为 ACG 爱好者提供从“发现”到“下载”再到“观看”的无缝闭环体验。

---

## ✨ 核心功能特性

### 1. 📅 Bangumi 深度集成
* **每日放送**：每天刷新 Bangumi 放送列表；每周通过 AniList 核对已追番剧的下一集时间，确认跨季播出的已追番剧继续显示在新季度日历中。
* **追番筛选**：在“我的追番”中可切换隐藏／显示已完结番剧。仅将 AniList 标记为 `FINISHED` 的作品视为完结；停播和排期未知的作品仍会显示。筛选选择只保存在当前浏览器中，不删除追番记录。
* **追番优先**：每天查询已追番剧的当日播出排期，在“我的追番”中将今日播出的作品置顶。排期是预计电视播出时间，不代表磁力资源已经发布。
* **ID 对照**：Bangumi 与 AniList 条目关联使用 [bangumi-data](https://github.com/bangumi-data/bangumi-data) 的数据（CC BY 4.0）。对照表缓存在 `cache/airing_mappings.json`，通常每 30 天更新一次；更新失败时继续使用旧表。未匹配的条目不会被自动判断为跨季续播。
* **追番同步**：登录后可同步你的“在看”列表，管理观看进度。
* **收藏管理**：支持在应用内直接标记“追番”、“看过”或“抛弃”。
* **智能详情**：获取番剧详细信息、评分、别名及剧集列表（本地缓存 24 小时）。

### 2. 🔍 智能资源搜索 (Crawler)
* **聚合搜索**：内置爬虫引擎，对接 Anime Garden 等资源站。
* **智能解析**：自动解析番剧标题，提取集数信息。
* **资源优选**：独创的资源评分算法，自动优先选择 **简中/繁中** 字幕资源，智能降权生肉或纯英文字幕，并在列表页直观展示磁力链接状态。
* **集数匹配**：自动将搜索结果与 Bangumi 的剧集列表（Ep.01, Ep.02...）进行精准匹配。
* **手动选择模式** ⭐：点击集数时弹出资源选择窗口，显示所有候选磁力链接，支持手动选择最合适的版本。
* **自定义搜索**：支持修改搜索关键词重新搜索，解决自动搜索不准确的问题。

### 3. ☁️ PikPak 云盘无缝对接
* **离线下载**：一键将搜索到的磁力链接推送到 PikPak 云端实现秒传/离线下载。
* **多账号轮询**：支持配置多个 PikPak 账号。当单日配额耗尽或遇风控时，系统自动切换备用账号，确保持续可用。
* **智能空间管理** ⭐：空间不足时先永久清空当前 PikPak 账号的云盘并重试；若离线任务次数也已耗尽，再切换账号。
* **账号管理** ⭐：可视化管理多个账号状态，支持手动封禁/解封账号，支持一键清空指定账号的云盘空间。
* **流式播放**：内置本地 HTTP 代理服务器（Port: 54321），无需等待下载回本地，直接流式播放云盘视频。
* **智能文件识别**：自动处理下载任务中的文件夹结构，智能定位视频主文件。

### 4. 🎬 极致播放体验
* **MPV 深度整合**：调用本地高性能 MPV 播放器，支持 4K/HDR。
* **双模式播放**：
    * **在线流式**：直接播放 PikPak 云端文件。
    * **本地播放**：自动检测已下载到本地的文件，优先本地播放。
* **自定义参数**：支持自定义 MPV 启动参数（如全屏、着色器配置等）。
* **集数管理** ⭐：
    * **鼠标左键**：播放已下载的集数；未下载时按设置手选磁力或自动优选并下载。
    * **鼠标右键**：标记/取消标记为已观看。
    * **鼠标中键**：删除该集的本地文件和/或磁力链接记录。

### 5. 🛠️ 现代化架构与设置
* **暗黑模式 UI**：基于 Vue 3 + TailwindCSS 打造的沉浸式深色界面。
* **系统日志**：内置实时日志控制台，方便排查网络请求与下载状态。
* **灵活配置**：支持 HTTP 代理设置，解决 API 访问受限问题。
* **可视化设置** ⭐：图形化配置界面，支持多账号管理、账号状态监控、一键清空云盘等高级功能。

---

## 🚀 快速开始

### 前置要求

* **发布版**：Windows 10/11 amd64 或 Linux amd64。Windows 需要 WebView2 Runtime；Linux 需要 GTK 3 和 WebKit2GTK 4.1。MPV 只在本机播放时需要。
* **源码构建**：[Go](https://go.dev/) 1.23+、[Node.js](https://nodejs.org/) 18+ (npm)。`wails dev` 还需要 [Wails CLI](https://wails.io/docs/gettingstarted/installation)。

### 使用发布版

从 [Releases](https://github.com/xjz6626/UltimateAnime/releases) 下载对应平台的 `UltimateAnime-v0.2.0-*-amd64.zip`，解压后按 [用户使用指南](README-用户使用.md) 首次配置。升级时只替换程序及文档，保留原来的 `config.json`、`followed.json` 和下载目录；v0.1 的两个 JSON 文件可以直接继续使用。

### 安装与运行

#### 方式一：快速安装（推荐）

**Windows 用户**：
```bash
# 克隆项目后，直接运行安装脚本
install.bat
```

**Linux/macOS 用户**：
```bash
# 克隆项目后，运行安装脚本
chmod +x install.sh
./install.sh
```

安装脚本会自动：
- ✅ 检测环境依赖
- ✅ 安装 Wails CLI（如果未安装）
- ✅ 下载 Go 模块和前端依赖
- ✅ 创建配置文件模板

#### 方式二：手动安装

1.  **克隆项目**
    ```bash
    git clone https://github.com/xjz6626/UltimateAnime.git
    cd UltimateAnime
    ```

2.  **安装 Wails CLI** (如果未安装)
    ```bash
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    ```

3.  **安装依赖**
    ```bash
    # 安装 Go 依赖
    go mod download
    
    # 安装前端依赖
    cd frontend
    npm install
    cd ..
    ```

4.  **配置应用**
    ```bash
    # 复制配置文件示例
    cp config.example.json config.json
    cp followed.example.json followed.json
    
    # 编辑 config.json，填入你的 PikPak 账号密码等信息
    # 至少需要配置 pikpak_users 和 pikpak_password
    ```

5.  **开发模式运行** (支持热重载)
    ```bash
    wails dev
    ```

6.  **在 Linux 上构建双平台发布包**
    ```bash
    ./scripts/build-release.sh v0.2.0
    # dist/ 下生成 Linux、Windows 压缩包及 SHA256 校验文件
    ```

    Linux 构建使用 `production,webkit2_41` 标签；Windows amd64 使用无控制台 GUI 子系统。发布包不包含 `config.json`、`followed.json`、缓存和下载内容。

### 首次使用配置

首次运行后，建议进入 **"系统设置"** 页面进行配置：

1. **PikPak 账号**（必须）
   - 添加至少一个 PikPak 账号和密码
   - 建议添加多个账号以应对限额

2. **Bangumi Token**（可选，追番同步需要）
   - 访问 [Bangumi 开发者设置](https://bgm.tv/dev/app)
   - 创建应用获取 Access Token
   - 在设置中填入 Token

3. **MPV 播放器**（推荐）
   - 下载并安装 [MPV](https://mpv.io/installation/)
   - 在设置中填入 mpv.exe 的完整路径
   - 或将 mpv 添加到系统环境变量

4. **HTTP 代理**（可选）
   - 如果无法访问 Bangumi 或 Anime Garden
   - 在设置中填入代理地址，如 `http://127.0.0.1:7890`

---

## 🌐 通过 Tailscale 使用远端网页

应用运行时会在家中电脑的 `127.0.0.1:54322` 和当前 Tailscale IPv4 地址的 `54322` 端口提供网页版。它复用桌面端的 Vue 页面，可查看追番和当季新番、选择磁力链接、启动家中电脑的下载，并查看下载日志。网页端不提供账号设置、本地文件删除和播放器操作。

手机浏览器使用底部导航。打开番剧详情后点按集数，可选择下载资源或标记已观看；磁力搜索和候选列表可在整屏弹窗中滚动操作。

1. 在家中电脑上连接 Tailscale，然后启动 UltimateAnime。先在桌面端配置好 PikPak 账号和密码；开始下载时才会连接 PikPak。
2. 在家中电脑上运行 `tailscale ip -4`，记下显示的 IPv4 地址。
3. 在上班电脑上连接同一个 tailnet，用浏览器打开 `http://<家中电脑的 Tailscale IPv4>:54322`。家中电脑和 UltimateAnime 需要保持运行。

网页版服务只监听本机回环地址和 Tailscale 地址，不需要路由器端口转发，也不需要 Tailscale Serve。若应用启动后才连接 Tailscale，请重启应用以绑定该地址。请在 tailnet 访问控制规则中限制可访问家中设备的用户。

## 📖 使用指南

### 基本流程

1. **发现新番**
   - 进入"当季新番"页面，查看 Bangumi 放送日历
   - 点击感兴趣的番剧查看详情
   - 点击"💖 追番"添加到本地收藏

2. **下载集数**
   - 点击剧集按钮，默认弹出磁力选择窗口；开启自动优选后会直接开始下载
   - 浏览所有候选资源，查看文件大小、来源等信息
   - 选择合适的版本（推荐选择简中/1080P）
   - 如果没有满意的结果，修改搜索关键词重新搜索
   - 点击"选择"按钮开始下载

3. **播放观看**
   - 已下载的集数会显示为蓝色
   - 左键点击蓝色集数，自动调用 MPV 播放本地文件
   - 右键点击集数，标记为已观看（显示为绿色）
   - 中键点击集数，删除本地文件或磁力记录

4. **账号管理**
   - 进入"系统设置"页面
   - 添加多个 PikPak 账号应对限额
   - 查看账号状态（✅ 正常 / 🚫 已封禁）
   - 手动封禁/解封账号，或一键清空指定账号的云盘空间

### 操作技巧

* **集数按钮交互**：
  - 🖱️ **左键**：播放/下载（按磁力选择设置手选或自动优选）
  - 🖱️ **右键**：标记观看状态
  - 🖱️ **中键**：删除本地文件和磁力记录

* **颜色含义**：
  - 🔴 **粉色**：已放送未观看
  - 🔵 **蓝色**：已下载到本地
  - 🟢 **绿色**：已标记为观看
  - ⚪ **灰色**：未放送

* **蓝点指示器**：集数按钮右上角的蓝点表示已缓存磁力链接

---

## ⚙️ 配置指南

首次使用时从 `config.example.json` 复制出 `config.json`，并从 `followed.example.json` 复制出 `followed.json`。升级时保留原文件；新增配置项使用默认值，旧版 `auto_login` 字段会被忽略，因为 PikPak 现在只在开始下载时连接。桌面端也可通过 **“系统设置”** 修改配置。

### 关键配置项 (`config.json`)

```json
{
  "global_settings": {
    "bangumi_api_token": "你的_Bangumi_Access_Token",
    "pikpak_users": [
      "account1@email.com",
      "account2@email.com"
    ],
    "pikpak_password": "统一的PikPak密码",
    "proxy": "http://127.0.0.1:7890"
  },
  "local_storage": {
    "anime_dir": "Downloads/Anime"
  },
  "torrent_searcher": {
    "auto_select_magnet": false
  },
  "player": {
    "mpv_path": "C:\\Program Files\\MPV\\mpv.exe",
    "mpv_args": "--fullscreen --volume=50"
  }
}
```

* **PikPak 账号**：建议配置多个账号以应对非会员的每日添加限制。
* **auto_select_magnet**：默认 `false`，点击剧集后手选磁力；设为 `true` 后优先复用已保存的磁力，否则按字幕和画质排序选第一条并开始下载。自动搜索失败时仍可手选。
* **Proxy**：如果你所在的网络环境无法直接访问 Bangumi 或 Anime Garden，请务必配置 HTTP 代理。

---

## 📂 项目结构概览

```
UltimateAnime/
├── app.go                  # 后端核心逻辑 (App Struct, 生命周期, 暴露给前端的方法)
├── main.go                 # 程序入口，Wails 初始化
├── wails.json              # Wails 项目配置
├── go.mod                  # Go 依赖管理
│
├── pkg/                    # 后端功能模块
│   ├── bangumi/            # Bangumi API 客户端 (User-Agent, Token管理)
│   ├── config/             # 配置管理 (JSON读写, 多线程安全)
│   ├── crawler/            # 资源爬虫 (Anime Garden API, 集数正则解析)
│   └── pikpak/             # PikPak 客户端 (登录, 文件管理, 离线任务, 流式代理)
│       ├── client.go       # 核心客户端 (API 请求, 文件操作, 账号管理)
│       ├── server.go       # HTTP 流式代理服务器
│       ├── types.go        # 数据结构定义
│       └── utils.go        # 工具函数 (签名计算, MD5等)
│
├── frontend/               # 前端资源 (Vue 3 + Vite)
│   ├── index.html          # 入口 HTML
│   ├── src/
│   │   ├── main.js         # Vue 入口
│   │   ├── App.vue         # 根组件 (侧边栏导航)
│   │   ├── views/          # 页面视图
│   │   │   ├── Home.vue      # 我的追番 (本地/收藏)
│   │   │   ├── Discovery.vue # 当季新番 (日历/详情/搜索)
│   │   │   ├── Settings.vue  # 设置页面 (账号管理/配置)
│   │   │   └── Logs.vue      # 日志页面
│   │   └── style.css       # Tailwind 引入与全局样式
│   └── wailsjs/            # Wails 自动生成的 JS 绑定文件
│
└── build/                  # 构建相关 (图标, Windows Manifest, Installer 配置)
```

## 🎯 核心特性亮点

### 🔥 磁力选择系统
传统自动下载器常遇到版本不对、字幕组不符等问题，本项目创新性地引入**可视化磁力选择**：
- 并行搜索 AnimeGarden 和蜜柑计划，按磁力哈希合并去重；单个来源暂时不可用时仍展示另一来源的结果
- 搜索后展示所有候选资源的详细信息
- 支持自定义关键词重新搜索
- 一键对比文件大小、来源、发布时间
- 彻底解决"下错版本"的困扰

### 🔄 多账号智能轮询
PikPak 免费用户每日限额有限，本项目实现：
- 自动检测配额耗尽/空间不足/风控
- 即时切换到下一个可用账号
- 后台自动清理空间，循环利用
- 手动管理账号状态（封禁/解封/清空）

### 🎬 无缝播放体验
- 云端文件通过本地代理流式播放，无需等待下载完成
- 自动检测本地已下载文件，优先本地播放
- 集成 MPV 高性能播放器，支持 HDR/杜比
- 鼠标操作即可完成"播放-标记-删除"全流程

## 🤝 贡献

欢迎提交 Issue 或 Pull Request！

1.  Fork 本仓库
2.  创建你的特性分支 (`git checkout -b feature/AmazingFeature`)
3.  提交你的修改 (`git commit -m 'Add some AmazingFeature'`)
4.  推送到分支 (`git push origin feature/AmazingFeature`)
5.  开启一个 Pull Request

## ⚠️ 免责声明

* 本项目仅供技术学习与交流使用。
* 项目本身不提供任何视频资源存储服务，所有资源搜索结果均来源于第三方公开网络。
* 用户需自行承担使用本软件所产生的法律责任。
* 请在下载后的 24 小时内删除，并支持正版番剧。

---

## 📝 更新日志

### v1.1.0 (2025-12-22)
- ✨ 新增磁力选择系统，支持手动选择资源版本
- ✨ 新增自定义搜索关键词功能
- ✨ 新增集数管理功能（中键删除）
- ✨ 新增 PikPak 多账号可视化管理
- ✨ 新增智能云盘空间清理功能
- ✨ 优化账号轮询逻辑，支持空间不足自动清理
- 🐛 修复文件夹删除失败问题
- 🐛 修复下载进度统计不准确问题

### v1.0.0
- 🎉 初始版本发布
- ✨ 实现 Bangumi 追番同步
- ✨ 实现 Anime Garden 资源搜索
- ✨ 实现 PikPak 离线下载
- ✨ 实现 MPV 播放器集成

## 📄 License

[MIT License](LICENSE)
