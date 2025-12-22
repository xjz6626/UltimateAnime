#!/bin/bash

echo "🚀 UltimateAnime 安装脚本"
echo "================================"

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "❌ 未检测到 Go，请先安装 Go 1.23+"
    exit 1
fi

echo "✅ Go 版本: $(go version)"

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo "❌ 未检测到 Node.js，请先安装 Node.js 16+"
    exit 1
fi

echo "✅ Node.js 版本: $(node --version)"

# 检查 Wails CLI 是否安装
if ! command -v wails &> /dev/null; then
    echo "⚠️  未检测到 Wails CLI，正在安装..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
fi

echo "✅ Wails 版本: $(wails version)"

# 安装 Go 依赖
echo ""
echo "📦 正在安装 Go 依赖..."
go mod download

# 安装前端依赖
echo ""
echo "📦 正在安装前端依赖..."
cd frontend
npm install
cd ..

# 创建配置文件
if [ ! -f "config.json" ]; then
    echo ""
    echo "📝 创建配置文件..."
    cp config.example.json config.json
    echo "⚠️  请编辑 config.json 填入你的 PikPak 账号密码"
fi

if [ ! -f "followed.json" ]; then
    cp followed.example.json followed.json
fi

echo ""
echo "✅ 安装完成！"
echo ""
echo "🎯 下一步:"
echo "  1. 编辑 config.json 填入配置（至少需要 PikPak 账号）"
echo "  2. 运行 'wails dev' 启动开发模式"
echo "  3. 或运行 'wails build' 构建生产版本"
echo ""
