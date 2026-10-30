# 防御机制修复交接 · 2026-10-30

**到家后必须重新编译 GUI 和 Rust worker。旧 EXE/App 不包含这批修复，不能继续用旧程序验收，也不能只更新前端。**

## 源码与分支

- Dashboard：`XiaoWang-SXAU-Lab/wgcna-dashboard` 的 `main`，本文件所在提交及后续提交。
- 引擎：`XiaoWang-SXAU-Lab/rs-wgcna` 的 `master`，至少包含 `4012702`。
- 两个仓库应相邻：`wgcna-dashboard/` 与 `rs_WGCNA/`。
- 保留本机尚未提交的工作；先检查 `git status`，不要用 reset/clean 覆盖家中改动。

在两个工作区确认分支后拉取：

```powershell
git -C ..\rs_WGCNA switch master
git -C ..\rs_WGCNA pull --ff-only origin master
git switch main
git pull --ff-only origin main
```

## Windows 必须完整重编译

在 `wgcna-dashboard` 中执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

此脚本从当前源码创建全新隔离快照，依次编译 worker、前端和内嵌 GUI；不要手动复用同步目录内的 target、node_modules、旧 worker 或旧 web/dist。

构建完成后，读取最新成功产物位置：

```powershell
$app = (Get-Content "$env:LOCALAPPDATA\rsWGCNA-build\windows\latest-app.txt" -Raw).Trim()
Get-Content (Join-Path $app 'build-info.json')
```

核对构建信息与本次源码提交，再按 BUILDING.md 更新相邻 `releases/rsWGCNA-current/` 的 `rsWGCNA.exe`、`rs-wgcna-worker.exe` 和 `build-info.json`。
**保留该目录完整的 `R/` 运行环境及绘图包。** 不要删除 R，也不要继续运行其他历史 release 目录的同名 EXE。
退出旧程序后，显式从新产物路径启动；UI 版本号仍是 0.6.0，请用源码提交/构建信息核对新旧，不能只看标题版本号。

macOS 使用 `bash scripts/build-macos.sh`。本轮在 Mac 完成编译与验证，Windows 原生验收需要家中机器执行。

## 本轮 code review 找到并修复的问题

| 优先级 | 原问题 | 现在的行为 |
|---|---|---|
| P1 | 同一任务可并发执行，参数文件与输出互相覆盖 | HTTP 返回前原子预留任务；重复启动返回 409；计算及报告读取期间锁定源数据 |
| P1 | 约 5 分钟后停止等待，失败/重跑也可能读回旧输出 | 每次运行有 run_id；持续跟踪本次状态、自动重连；只在成功后取结果，旧派生 JSON 移入 previous-runs |
| P1 | 修改上游参数仍使用旧结果 | 成功步骤保存实际参数快照；后端拒绝失配依赖，前端显示“参数已更改，需重跑”及相关步骤链接 |
| P1 | 数字 min/max 未生效，空值/小数/非法范围能进入引擎 | 后端统一校验类型、整数约束、范围、枚举、线程数、步骤依赖和模块/性状选择 |
| P1 | log(0)、log(负数)、筛选后无基因等到聚类才 panic | 启动前检查变换定义域及筛选可用性；worker 再验证有限数值和至少 2 个基因 |
| P1 | 离群样本删除只更新表达矩阵 | trait/sample_info 从原始副本按保留样本重排和删减；累计删样，可恢复；不删除基因注释映射表 |
| P2 | 仅有 sample_info 也被“未上传性状表”阻止 | 按 trait_source 判断就绪；允许合法 metadata one-hot；拒绝缺值、单一类别及常数数值性状 |
| P2 | 上传格式错误、空白/重复 ID、NaN 等后置失败 | 只读上传预检 + 真正上传端的同等校验；问题弹窗列出 ID/行列；安全清理 BOM/首尾空白/空行 |
| P2 | 旧任务显示新选择的文件名却继续运行旧数据 | 更换/移除输入会创建新输入集；有结果的任务不允许直接覆盖输入，保留原结果 |
| P2 | 缺失统计量显示为零，某些 null 会使表格崩溃 | 结果表格使用“—”表示不可用统计量 |
| P2 | 取消能力不足，辅助任务靠猜测文件是否生成 | 主流程与辅助分析共用运行状态、取消及 run_id 等待；聚类 K/kmax 与基因数提前校验 |
| P2 | 缺少概览图时显示破图，预览失败后一直转圈 | 报告按资产清单输出缺图说明；图形预览停止等待并提示，可手动重试；部分报告也加载离线图表运行库 |
| P2 | macOS 下载/文件选择无响应 | 保留 WKWebView 原生代理，补上传选择和下载保存/取消/安全覆盖的生命周期 |

主要实现位置：`backend/internal/job/{guards,run,inputs}.go`、`backend/internal/inputcheck/`、`backend/internal/api/{run,inputs}.go`、`frontend/src/stores/wgcna.ts`、`frontend/src/utils/pollJob.ts`。

`params.json` 损坏时 worker 不再静默使用默认参数。XLS 转换依赖新 worker 的 `--convert-workbook`；因此两个仓库必须一起更新并重编译。

## 验证与验收

本轮已通过：

- Go 全量测试，以及 input/job/API race 测试。
- 前端 TypeScript、生产构建、`node scripts/test-pipeline-guards.mjs`：长任务 750 次轮询、断线重连、运行 ID、不读取失败旧结果、metadata-only 性状、参数失效提示。
- Rust worker `defensive_tests` 3 项和 `trait_alignment` 3 项。
- 原来一直失败的 `report_test.go` 已更新为当前交互富集报告断言，同时补上真实缺图及离线运行库回归。
- 前一轮原生 UI 验证：NaN 行列提示、缺失/多余样本提示、BOM/空白/空行提示；输入错误会禁用分析。

最后一轮真实引擎/GUI 联调与产物记录在本机隔离构建的 validation 文件、`dist/build-info.json`；本地 App/DMG 属生成物，不提交 Git。

家中编译后重点验收：

1. Download demo 可保存完整 ZIP；六个上传项能打开文件选择框。
2. 上传带 BOM/首尾空白/空行的有效文件可继续；重复样本、NaN、缺失/多余样本会弹窗，并允许移除错误文件。
3. n_keep=0/空值、无效参数不会启动 worker；参数拒绝后原成功结果仍在。
4. 连续删两批样本后 trait/sample_info 同步，模块–性状可继续；恢复全部样本后可重新过滤。
5. 不上传 trait、只上传合法 sample_info，选择有两个以上类别的列，能生成性状；空值/单一类别会提前提醒。
6. 改 filter / soft-threshold / network 参数后出现重跑提示，不能用旧依赖直接继续。
7. 运行中重复启动被拦截，取消后不显示旧结果；聚类重跑不把上次 JSON 当作本次完成。
8. Report/Overview 缺图或 R 环境问题会给出提示，预览不无限转圈；完整 R/ 绘图环境仍须保留。

没有自动安装 R 包，也没有把用户数据做缺失值填充或猜测 ID。超大数据的资源预估及流式上传不在本批实现范围，后续可单独优化。
