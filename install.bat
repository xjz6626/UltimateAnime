@echo off
chcp 65001 >nul
echo 🚀 UltimateAnime 安装脚本
echo ================================
echo.

:: 检查 Go 是否安装
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 未检测到 Go，请先安装 Go 1.23+
    echo 下载地址: https://go.dev/dl/
    pause
    exit /b 1
)

for /f "tokens=*" %%i in ('go version') do set GO_VERSION=%%i
echo ✅ Go 版本: %GO_VERSION%

:: 检查 Node.js 是否安装
where node >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 未检测到 Node.js，请先安装 Node.js 16+
    echo 下载地址: https://nodejs.org/
    pause
    exit /b 1
)

for /f "tokens=*" %%i in ('node --version') do set NODE_VERSION=%%i
echo ✅ Node.js 版本: %NODE_VERSION%

:: 检查 Wails CLI 是否安装
where wails >nul 2>&1
if %errorlevel% neq 0 (
    echo ⚠️  未检测到 Wails CLI，正在安装...
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
)

for /f "tokens=*" %%i in ('wails version') do set WAILS_VERSION=%%i
echo ✅ Wails 版本: %WAILS_VERSION%

:: 安装 Go 依赖
echo.
echo 📦 正在安装 Go 依赖...
go mod download

:: 安装前端依赖
echo.
echo 📦 正在安装前端依赖...
cd frontend
call npm install
cd ..

:: 创建配置文件
if not exist "config.json" (
    echo.
    echo 📝 创建配置文件...
    copy config.example.json config.json >nul
    echo ⚠️  请编辑 config.json 填入你的 PikPak 账号密码
)

if not exist "followed.json" (
    copy followed.example.json followed.json >nul
)

echo.
echo ✅ 安装完成！
echo.
echo 🎯 下一步:
echo   1. 编辑 config.json 填入配置（至少需要 PikPak 账号）
echo   2. 运行 'wails dev' 启动开发模式
echo   3. 或运行 'wails build' 构建生产版本
echo.
pause
