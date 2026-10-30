# 本机 Windows / macOS 隔离构建

源码共用；依赖、工具链、缓存、内嵌资源和可执行程序按操作系统分开。
不要在同步目录直接运行 `pnpm install` 或 `cargo build`。
`make build` 现已按平台转调下面的隔离构建脚本。
`.gitignore` 只控制 Git，不会阻止同步软件传输这些目录。

## Windows

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
# 构建后打开原生窗口
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1 -Launch
```

需要 Windows GNU Rust 工具链（`rustup toolchain install stable-x86_64-pc-windows-gnu`）、
w64devkit、Go（以 backend/go.mod 为准）、Node 和 pnpm。
默认 GCC 路径为 `%LOCALAPPDATA%\w64devkit\w64devkit\bin`，可用 `-GccBin` 指定。
默认 Rust 源码为相邻 `rs_WGCNA`，可用 `-WorkerSource` 指定。

前端的 `frontend/.npmrc` 使用 `node-linker=hoisted`，避免 pnpm 在无符号链接权限的
Windows 账户下安装依赖时出现 `ERR_PNPM_EPERM`。无需管理员权限或开启开发者模式。
该配置会随前端源码复制到每次全新的隔离快照，因此无需删除同步目录内的旧依赖。
已有依赖目录不会被脚本复用；请直接重新运行上述构建命令。

脚本显式使用 Windows GNU 工具链和 `-C link-self-contained=yes`，让 Rust 使用
自身携带的 CRT/展开支持库；该设置同时覆盖宿主构建脚本，因此不额外传 `--target`。
这解决当前 w64devkit 缺少 `libgcc_eh` 的链接失败，无需改变系统默认 Rust 工具链。

产物及源文件快照在 `%LOCALAPPDATA%\rsWGCNA-build\windows\runs\...\app`。
`%LOCALAPPDATA%\rsWGCNA-build\windows\latest-app.txt` 保存最新成功构建位置。
每次构建使用新快照，包含尚未提交的工作文件；不写回同步源码，也不复用旧的内嵌引擎。
`build-info.json` 记录源码提交和 Windows EXE / 引擎的 SHA-256。

供用户打开的本地版本统一复制到相邻 `releases/rsWGCNA-current/`，后续覆盖更新该目录，
不再为每次 UI 修改新建发布目录。隔离构建快照仍保留在本机缓存，避免同步编译依赖。
固定发布目录必须保留 `R/` 运行环境（含 ComplexHeatmap 等绘图包）。首次准备时从
现有 Windows 发行包复制完整 `R/`；后续仅覆盖 EXE、worker 和构建信息，不删除 `R/`。
只复制编译生成的三个文件会导致 Overview Figure 等 R 图表无法预览。

## macOS

```bash
bash scripts/build-macos.sh
```

需要 Xcode Command Line Tools、Rust stable、Go、Node 和 pnpm。
产物在 `~/Library/Caches/rsWGCNA-build/macos/run-.../app`，
`latest-app.txt` 保存最新成功构建位置。
可用 `WGCNA_WORKER_SOURCE` 和 `WGCNA_BUILD_ROOT` 指定源目录与本机构建根目录。
此脚本只生成本机程序；带 R.framework 的 DMG 仍由 release workflow 打包。
本轮仅在 Windows 验证，macOS 脚本需要在 Mac 实机验收。

## 绘图与同步

本地程序仍需要 R 及绘图包。可设置 `WGCNA_RSCRIPT` 指向本机 `Rscript`，
或按现有发布布局把 R 放在程序旁；单独构建 EXE 不会自动安装 R。

同步软件应排除 `.git/`、`node_modules/`、`target/`、`dist/`、
`backend/internal/web/dist/`、`backend/internal/worker/worker-bin` 和本机临时产物。
源码、锁文件和脚本可以同步；Git 历史更适合通过远端 push/pull 传递。
当前脚本已将新产物移出同步盘，但不自动修改同步客户端规则或删除历史文件。
如需要共享成品，请分别使用 `windows-x64/`、`macos-arm64/` 等目录。

GitHub Actions 两个平台运行在独立 runner，其现有构建互不覆盖。

## Report 面板

报告会在独立临时快照中自动准备全部模块详情，不要求交互流程先生成
`step5_detail_all.json`，也不修改当前模块选择、任务参数或进度。
生成完成后可直接在 Report 面板内阅读；最近一次成功报告的位置保存在任务的
`report-latest.json`，重新进入或重启后可以恢复。

报告不再包含全局总览网络。常规图表（样本树、软阈值、基因树、模块性状、
GS/MM、单模块表达、富集、建议K和Hub网络）使用离线 ECharts；
只有模块与聚类的综合三联热图使用 ComplexHeatmap。
Hub 网络可缩放、拖动、搜索和悬停查看邻居；每图可导出 PNG。
单模块表达图明确显示 |MM| 最高的150个基因，完整数据仍在 detail.json。
所有表达热图按 `sample_id` 直接升序排列（普通字符串排序），不使用
sample_info 的材料/时间优先级；即使缺少 metadata 也保持相同顺序。
数值列、ME曲线和注释与样本标签一起移动，ComplexHeatmap 不再按元数据切分重排列。
`report/report-charts.js` 与数据一同导出，无需 CDN 或在线后台，
保留整个报告目录即可用 `file://` 离线阅读。报告设置见
`result/00_provenance/report-settings.json`。
没有生成的图不会显示破图；执行问题列在面板及 `export-warnings.json`。

已有输出项目目录不会被覆盖。每次生成请使用新的项目名。
可用 `/report?job=<任务ID>` 重开该任务的报告面板；本机启动时可设置
`WGCNA_START_PATH=/report?job=<任务ID>`，直接进入该面板。

## 上传检查与离群样本

上传区选择文件后调用只读预检，不创建分析任务。TSV/CSV/XLSX 与 XLS 会转为统一的
UTF-8 TSV 分析副本；XLS 通过同版本 worker 的 calamine 转换（因此更新 GUI 时也要更新
worker）。只读取 Excel 第一张表。原始上传文件保存在任务目录的 `*.uploaded.tsv`
（内容保留原始格式，文件后缀不表示实际编码），用户磁盘上的原文件不会被修改。

检查包括行列数、重复/空 ID、有限数值、表达量符号、raw count 整数约束，以及 trait /
sample_info 对表达矩阵的样本匹配。自动清理 BOM、单元格首尾空白和完全空行，保留 ID
内部空格和大小写；有歧义的 ID、缺失数值与任意多余/缺失样本会阻止分析。顺序不同但
集合相同的样本会自动重排；常数基因/性状列给出提示。预检最多展示 100 条详细问题，
同时保留完整的错误/提示计数。实际上传端点再次执行同样的检查，不能绕过前端校验。

成功运行 filter 后，以及运行任何后续分析前，trait.tsv 和 sample_info.tsv 从原始副本
重新生成，严格按 step1_filter.json 的保留样本删除和排序。连续删样累计排除名单；
重跑恢复样本时可重新取回原始行。缺少对应行仍会报错，不自动填值。annotation /
gene2go / gene2kegg 属于基因表，不随样本删除而删除行。旧任务首次同步时先备份现有
样本表。筛选后至少需要 3 个样本，输入变更会启动新的任务，避免新文件名误用旧结果。

## 防御机制审查后的重编译要求（2026-10-30）

本批源码必须同时更新 Dashboard `main` 和 rs-wgcna `master`，并重新编译完整 GUI + worker。
不要用旧 EXE/App 验收新版源码，也不要单独覆盖 web/dist。家中 Windows 的操作与验收
清单见本仓库 [HANDOFF.md](HANDOFF.md)。生成的本地 dist/App/DMG 已排除出 Git。

运行入口现在在计算前校验参数、依赖、输入数值定义域和模块/性状选择；同一任务只允许
一个计算，报告快照期间源数据不可修改。每次运行有 run_id，旧派生 JSON 保存在
`previous-runs/<run_id>/`，失败/取消不会把旧文件当成本次结果。上游参数更改会提示重跑。
