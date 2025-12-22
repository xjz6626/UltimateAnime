# UltimateAnime 项目完整性检查清单

## ✅ 核心文件
- [x] main.go - 程序入口
- [x] app.go - 应用核心逻辑
- [x] go.mod - Go 依赖管理
- [x] wails.json - Wails 配置

## ✅ 后端模块 (pkg/)
- [x] pkg/bangumi/ - Bangumi API 客户端
- [x] pkg/config/ - 配置管理
- [x] pkg/crawler/ - 资源爬虫
- [x] pkg/pikpak/ - PikPak 客户端（包含 client.go, server.go, types.go, utils.go）

## ✅ 前端文件 (frontend/)
- [x] frontend/package.json - 前端依赖
- [x] frontend/src/App.vue - 根组件
- [x] frontend/src/main.js - 入口文件
- [x] frontend/src/views/ - 页面组件（Home, Discovery, Settings, Logs）
- [x] frontend/src/router/ - 路由配置
- [x] vite.config.js - Vite 配置
- [x] tailwind.config.cjs - Tailwind CSS 配置

## ✅ 配置文件
- [x] config.example.json - 配置文件模板
- [x] followed.example.json - 追番列表模板
- [x] .gitignore - Git 忽略规则
- [x] .gitattributes - Git 属性配置

## ✅ 文档
- [x] README.md - 完整文档（包含安装、配置、使用说明）
- [x] LICENSE - MIT 许可证

## ✅ 安装脚本
- [x] install.bat - Windows 快速安装脚本
- [x] install.sh - Linux/macOS 快速安装脚本

## ✅ 构建相关
- [x] build/ - 构建配置（图标、Manifest 等）

## 🔍 克隆后需要的操作

1. 运行安装脚本 (`install.bat` 或 `install.sh`)
   或手动执行：
   ```bash
   go mod download
   cd frontend && npm install && cd ..
   ```

2. 复制配置文件：
   ```bash
   cp config.example.json config.json
   cp followed.example.json followed.json
   ```

3. 编辑 config.json 填入：
   - PikPak 账号密码（必须）
   - Bangumi Token（可选）
   - MPV 路径（可选）
   - HTTP 代理（可选）

4. 运行：
   ```bash
   wails dev  # 开发模式
   # 或
   wails build  # 构建生产版本
   ```

## ⚠️ 注意事项

- `config.json` 和 `followed.json` 已被 .gitignore 忽略（用户数据）
- `Downloads/` 目录会自动创建，用于存放下载的视频
- 首次运行时，应用会自动创建缺失的配置文件
- 至少需要配置一个 PikPak 账号才能使用下载功能

## 🎯 验证安装

克隆后可以通过以下方式验证：

```bash
# 检查 Go 依赖
go mod verify

# 检查前端依赖
cd frontend && npm list && cd ..

# 尝试构建
wails build
```

如果所有步骤都成功，说明项目完整可用！
