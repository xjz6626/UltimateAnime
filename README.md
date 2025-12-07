# UltimateAnime 📺

![Wails](https://img.shields.io/badge/Built%20With-Wails-red) ![Go](https://img.shields.io/badge/Backend-Go-blue) ![Vue](https://img.shields.io/badge/Frontend-Vue%203-green) ![License](https://img.shields.io/badge/License-MIT-yellow)

**UltimateAnime** 是一个基于 [Wails](https://wails.io/) 构建的现代化、一站式二次元资源管理桌面应用。

它创造性地集成了 **Bangumi 番组计划**（追番管理）、**Anime Garden**（资源搜索）以及 **PikPak**（云端下载与流式播放），配合本地 **MPV 播放器**，旨在为 ACG 爱好者提供从“发现”到“下载”再到“观看”的无缝闭环体验。

---

## ✨ 核心功能特性

### 1. 📅 Bangumi 深度集成
* **每日放送**：实时同步 Bangumi 每日新番放送列表，不错过每一集更新。
* **追番同步**：登录后可同步你的“在看”列表，管理观看进度。
* **收藏管理**：支持在应用内直接标记“追番”、“看过”或“抛弃”。
* **智能详情**：获取番剧详细信息、评分、别名及剧集列表。

### 2. 🔍 智能资源搜索 (Crawler)
* **聚合搜索**：内置爬虫引擎，对接 Anime Garden 等资源站。
* **智能解析**：自动解析番剧标题，提取集数信息。
* **资源优选**：独创的资源评分算法，自动优先选择 **简中/繁中** 字幕资源，智能降权生肉或纯英文字幕，并在列表页直观展示磁力链接状态。
* **集数匹配**：自动将搜索结果与 Bangumi 的剧集列表（Ep.01, Ep.02...）进行精准匹配。

### 3. ☁️ PikPak 云盘无缝对接
* **离线下载**：一键将搜索到的磁力链接推送到 PikPak 云端实现秒传/离线下载。
* **多账号轮询**：支持配置多个 PikPak 账号。当单日配额耗尽或遇风控时，系统自动切换备用账号，确保持续可用。
* **流式播放**：内置本地 HTTP 代理服务器（Port: 54321），无需等待下载回本地，直接流式播放云盘视频。
* **智能文件识别**：自动处理下载任务中的文件夹结构，智能定位视频主文件。

### 4. 🎬 极致播放体验
* **MPV 深度整合**：调用本地高性能 MPV 播放器，支持 4K/HDR。
* **双模式播放**：
    * **在线流式**：直接播放 PikPak 云端文件。
    * **本地播放**：自动检测已下载到本地的文件，优先本地播放。
* **自定义参数**：支持自定义 MPV 启动参数（如全屏、着色器配置等）。

### 5. 🛠️ 现代化架构与设置
* **暗黑模式 UI**：基于 Vue 3 + TailwindCSS 打造的沉浸式深色界面。
* **系统日志**：内置实时日志控制台，方便排查网络请求与下载状态。
* **灵活配置**：支持 HTTP 代理设置，解决 API 访问受限问题。

---

## 🚀 快速开始

### 前置要求

* **操作体统**：Windows 10/11 (macOS 亦支持，但 MPV 路径需自行适配)
* **环境依赖**：
    * [Go](https://go.dev/) 1.23+
    * [Node.js](https://nodejs.org/) (npm)
    * [MPV Player](https://mpv.io/) (需添加到环境变量或在设置中指定路径)

### 安装与运行

1.  **克隆项目**
    ```bash
    git clone [https://github.com/xjz6626/UltimateAnime.git](https://github.com/xjz6626/UltimateAnime.git)
    cd UltimateAnime
    ```

2.  **安装依赖**
    ```bash
    # 安装前端依赖
    cd frontend
    npm install
    cd ..
    
    # 安装 Wails 依赖 (会自动处理 go.mod)
    go mod tidy
    ```

3.  **开发模式运行** (支持热重载)
    ```bash
    wails dev
    ```

4.  **构建生产版本**
    ```bash
    wails build
    # 构建产物将生成在 build/bin/UltimateAnime.exe
    ```

---

## ⚙️ 配置指南

首次运行后，应用会自动生成 `config.json`。你也可以在应用的 **"系统设置"** 界面进行图形化配置。

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
    "auto_login": true,
    "proxy": "[http://127.0.0.1:7890](http://127.0.0.1:7890)" 
  },
  "local_storage": {
    "anime_dir": "Downloads/Anime"
  },
  "player": {
    "mpv_path": "C:\\Program Files\\MPV\\mpv.exe",
    "mpv_args": "--fullscreen --volume=50"
  }
}
```

* **PikPak 账号**：建议配置多个账号以应对非会员的每日添加限制。
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
│   └── pikpak/             # PikPak 客户端 (登录, 文件列表, 离线任务)
│
├── frontend/               # 前端资源 (Vue 3 + Vite)
│   ├── index.html          # 入口 HTML
│   ├── src/
│   │   ├── main.js         # Vue 入口
│   │   ├── App.vue         # 根组件 (侧边栏导航)
│   │   ├── views/          # 页面视图
│   │   │   ├── Home.vue      # 我的追番 (本地/收藏)
│   │   │   ├── Discovery.vue # 当季新番 (日历/详情/搜索)
│   │   │   ├── Settings.vue  # 设置页面
│   │   │   └── Logs.vue      # 日志页面
│   │   └── style.css       # Tailwind 引入与全局样式
│   └── wailsjs/            # Wails 自动生成的 JS 绑定文件
│
└── build/                  # 构建相关 (图标, Windows Manifest, Installer 配置)
```

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
* 请在下载后的 24 小时内删除，并支持正版番剧。

## 📄 License

[MIT License](LICENSE)