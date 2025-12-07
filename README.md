# UltimateAnime

UltimateAnime 是一个基于 [Wails](https://wails.io/) 构建的现代化二次元资源管理桌面应用。它集成了 Bangumi 番组计划、资源搜索（Anime Garden）以及 PikPak 网盘功能，旨在为用户提供一站式的追番体验。

## ✨ 功能特性

*   **资源搜索**: 内置资源爬虫，支持从 Anime Garden 等源搜索动漫磁力链接。
    *   支持智能集数解析。
    *   **智能优选**: 自动优先选择简中/繁中字幕资源，过滤纯生肉或英文字幕。
*   **PikPak 集成**:
    *   支持多账号管理。
    *   一键添加磁力链接到 PikPak 云端。
    *   查看云盘文件及状态。
*   **Bangumi 同步**:
    *   集成 Bangumi API，同步追番进度。
    *   查看今日放送列表。
*   **本地播放**:
    *   集成 MPV 播放器支持。
    *   支持自动检测本地 MPV 路径或自定义路径。
    *   支持自定义播放参数。
*   **现代化 UI**: 基于 Vue 3 + TailwindCSS 构建的清爽界面。
*   **配置管理**: 支持自定义代理、API 地址等全局设置。

## 🛠️ 技术栈

*   **后端**: Go (1.23+)
*   **前端**: Vue 3, Vite, TailwindCSS
*   **框架**: [Wails v2](https://wails.io/)
*   **状态管理**: Pinia
*   **路由**: Vue Router

## 🚀 快速开始

### 前置要求

*   [Go](https://go.dev/dl/) (1.18+)
*   [Node.js](https://nodejs.org/) (npm)
*   [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### 安装依赖

```bash
# 安装 Wails CLI (如果尚未安装)
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 开发模式运行

```bash
# 在项目根目录下运行
wails dev
```

### 构建发布版本

```bash
# 构建 Windows 版本
wails build
```

构建产物将位于 `build/bin` 目录下。

## ⚙️ 配置说明

首次运行后，程序会在同级目录下生成 `config.json` 配置文件。你可以在设置界面进行修改，也可以直接编辑该文件。

### 基础配置

*   `bangumi_api_token`: Bangumi API Token (用于同步追番进度)
*   `pikpak_accounts`: PikPak 账号列表 (支持多账号轮询)
*   `proxy`: HTTP 代理地址 (例如 `http://127.0.0.1:7890`)
*   `torrent_api_url`: 资源搜索 API 地址 (默认使用 Anime Garden)

### 播放器配置 (MPV)

程序支持调用本地 MPV 播放器直接播放下载好的视频文件。

*   **自动检测**: 程序会自动尝试在以下路径寻找 MPV:
    *   环境变量 `PATH` 中的 `mpv`
    *   程序同级目录下的 `mpv.exe`
    *   `bin/mpv.exe`
*   **手动配置**: 你也可以在设置中指定 `mpv_path` 为 MPV可执行文件的完整路径。
*   **自定义参数**: 支持通过 `mpv_args` 设置启动参数，例如 `--fullscreen --volume=50`。

## 📝 开发计划

- [x] 基础资源搜索与展示
- [x] 磁力链接智能优选 (优先中文)
- [x] PikPak 离线下载对接
- [x] 本地播放器集成 (MPV)
- [ ] 自动追番订阅功能
- [ ] 更多播放器支持 (PotPlayer, VLC)

## ⚠️ 免责声明

本项目仅供学习交流使用，请勿用于非法用途。所有资源均来源于网络，本项目不存储任何文件。
