# 全量测试与回归报告 · 2026-10-23

本轮测试跨 10 月 22～23 日，针对当前工作区执行，未提交 Git。发现并修复下载任务页的两个问题，新增回归用例后，最终自动化测试和发布构建全部通过。

## 发现与修复

### 1. 首次读取任务列表与进度事件竞争，导致历史任务暂时消失

- 复现：打开下载页，首次 `GetDownloadTasks` 尚未返回时收到某个任务的 `download-task` 事件；随后快照返回多条历史任务。
- 原行为：事件递增请求代次，使整个首次快照被丢弃，页面仅剩收到事件的任务；若刷新总遇到进度事件，其他任务可能持续无法显示。
- 修复：请求代次只用于区分不同快照请求。另行记录事件版本，将请求期间收到的最新任务状态覆盖到完整快照，保留其他历史任务。
- 验证：实际编译、挂载 `Downloads.vue`，断言历史任务仍显示，且旧快照不能把 75% 的新进度回退到 50%。修复前用例失败，修复后通过。

### 2. 任务读取失败时误报“暂无下载任务”

- 复现：`GetDownloadTasks` 因损坏记录或读取错误而失败。
- 原行为：错误提示与“暂无下载任务”同时显示，容易把读取失败误认为没有历史记录。
- 修复：区分加载中、读取失败与确实为空的状态；失败时保留错误提示。
- 验证：实际组件测试模拟读取失败，断言显示错误且不显示空任务提示。修复前用例失败，修复后通过。

修改文件：`frontend/src/views/Downloads.vue`。新增用例位于 `frontend/src/components.test.js`，组件宿主位于 `frontend/src/testUtils/renderComponent.js`。

## 最终结果

| 检查 | 结果 |
| --- | --- |
| Go 全量单元/集成测试 | 8 个包通过，120 个顶层测试函数，144 个测试项（含子测试）；0 失败、0 跳过 |
| Go 竞态检测 | 全量启用 `-race`，没有竞态报告 |
| 缓存与顺序 | 使用 `-count=1 -shuffle=on`，实际重新执行并打乱顺序 |
| Go 静态检查 | `go vet ./...` 通过 |
| 并发/恢复压力回归 | 关键场景随机顺序重复 20 轮，全部通过 |
| 真实 MPV 重复验证 | 正常播完、退出保存、再次启动续播，重复 3 轮通过 |
| 下载跨模块恢复 | 新增应用级集成测试通过：暂停后重新创建 App 与 API 客户端，从磁盘恢复，复用完整分块，不重复创建云端任务，完成后恢复片库索引并清理缓存 |
| 前端单元/组件回归 | 6 个测试文件、27 个用例通过；无失败、跳过或取消 |
| 前端生产构建 | Vite 构建通过 |
| Linux 发布构建 | `production,webkit2_41` 标签构建通过；`ldd` 未发现缺失动态库 |
| Windows 发布构建 | Windows amd64 GUI 子系统交叉构建通过 |
| 文本检查 | `git diff --check` 通过 |

前端组件测试编译实际 Vue SFC，使用 Vue 渲染器和内存宿主验证按钮、提示、事件与异步更新，不等同于浏览器像素布局测试。新增场景还覆盖：暂停过程中禁止继续、操作错误不会被刷新清除、下一集旧快照不会覆盖新倒计时、迟到的取消响应不会清除另一个新倒计时、组件卸载释放监听。

Go 压力回归包括并发片库事务、排队暂停、停止期间拒绝重试、跨实例续传、验证码请求隔离、缓存校验、扫码取消、弹幕来源切换、播放记录保护和搜索取消。

## Go 语句覆盖率

总覆盖率 **62.0%**，由最终一次全量 `-race -coverprofile` 计算。这是现有自动化测试覆盖的语句比例，不代表所有业务场景都已验证。

| 包 | 通过测试项（含子测试） | 语句覆盖率 |
| --- | ---: | ---: |
| `UltimateAnime` | 68 | 54.5% |
| `pkg/bangumi` | 3 | 30.1% |
| `pkg/bilibili` | 26 | 79.4% |
| `pkg/config` | 3 | 50.4% |
| `pkg/crawler` | 21 | 86.4% |
| `pkg/imgproxy` | 2 | 48.4% |
| `pkg/pikpak` | 20 | 77.0% |
| `pkg/storage` | 1 | 63.6% |

## 执行方式

先完成前端构建，再执行 Go 构建/测试，避免 Go 嵌入资源时与 Vite 清理 `dist` 竞争。

```sh
npm test --prefix frontend
npm run build --prefix frontend
go test -race -count=1 -shuffle=on -timeout=3m -coverprofile=/tmp/ultimateanime-coverage.out ./...
go vet ./...
go build -trimpath -tags 'production,webkit2_41' -ldflags '-s -w' -o /tmp/UltimateAnime-linux .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags production -ldflags '-H=windowsgui -s -w' -o /tmp/UltimateAnime-windows.exe .
git diff --check
```

当前 Node 的默认测试输出按文件汇总；本轮另外执行 `node --test --test-isolation=none --test-reporter=tap src/*.test.js`，确认 27 个具体用例均执行通过。

本轮环境：Linux amd64、Go `go1.27.1-X:nodwarf5`、Node `v26.10.0`、MPV `v0.41.0`、GTK `3.24.52`、WebKit2GTK `2.52.6`。沙箱环境中 Go 缓存设为 `/tmp/ultimateanime-go-cache`，本机监听类测试使用获准的执行环境。

原始日志、覆盖率和构建产物保存于 `/tmp/ultimateanime-verification-20261022/`：`go-tests.json`、`coverage.out`、`frontend-tests.tap`、`frontend-build.log`、`stress.log`、`mpv-repeat.log` 及两个平台可执行文件。临时目录可能被系统清理，本报告保留结果与复现方法。

## 警告与未覆盖范围

- Vite 构建提示 Browserslist 的 `caniuse-lite` 数据约 10 个月未更新；构建成功。本轮未为消除提示而升级依赖。
- 未使用真实账号测试 PikPak 下载/云盘清空、B 站扫码登录或在线服务兼容性；相关测试使用模拟 API，避免改动真实账号数据。
- 已验证真实 Linux MPV 的无界面播放；未验证 Wails 完整图形交互、真实片源与字幕/弹幕组合、Windows 原生命名管道运行及 Windows 安装流程。
- Windows 已生成正式发布配置的可执行文件，但交叉构建不等同于 Windows 实机运行。
- 未验证项目声明的最低 Go/Node 版本组合；本报告环境版本见上文。未对所有未覆盖分支作“没有问题”的保证。
