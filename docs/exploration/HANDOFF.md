## 最新（2026-11-11 上午，Claude @ 办公室 Mac：macOS 包 + CI 依赖修复）

- 办公室 Mac 拉取 dashboard `explore/project-copilot@be49821`（含 `9a53b38`）、worker `explore/project-copilot@10ae0be`，两仓库工作树干净。
- macOS arm64 包构建通过（`scripts/build-macos.sh` 隔离快照）：`.app` 内置 R.framework 4.5.1，R 库裁剪到 75 个包（7 个绘图包 + 递归依赖 + base/recommended，约 229 MB），DMG 207 MB。SHA-256 `5fded609762fd6f742251d2eb0d43b74096aaaa3ef65201f2140f195a2ddd42b`。暂存在本机 `~/Bio-info-PRJ/_rsWGCNA-local/release-staging/v0.6.0-explore-be49821-10ae0be/`。
- 验证：`codesign --verify --deep --strict`；从挂载的只读 DMG headless 启动 `/api/health` 返回 ok；包内 R 在挂载镜像内输出 PNG、PDF、SVG；ComplexHeatmap 热图正常。
- CI 修复：`.github/workflows/release.yml` 的 macOS 和 Windows job 都补装 `igraph`、`svglite`（基因网络图和 SVG 导出必需）；macOS 从 `brew install --cask r` 改为按架构安装 CRAN R 4.5.1 官方 pkg，并断言 `getRversion() == "4.5.1"`。
- 未验证：原生窗口 GUI 交互、完整 WGCNA 分析流程、第二台 Mac 的安装/卸载、Apple 公证（当前 ad-hoc 签名）。
- 下一步：在 GitHub 上跑一次 macOS job，验证固定 R 4.5.1 的下载/安装路径；确认后把 DMG 按发布规则复制到同步盘 `releases/`。

## 最新（2026-11-10 深夜，Claude @ 家里 Windows：边稳定性降为可选诊断，构建通过）

- 用户决定将 R-EAN 从默认流程撤下：保留实现与历史结果，作为模块网络下方的可选诊断；暂停差异层与桥接层。
- 已完成（未提交）：`frontend/src/views/ModuleExplorerView.vue` 主图默认只显示连接强度；稳定性参数/结果移入默认折叠的「边稳定性（可选）」；移除「强但不稳」横幅和稳定性着色，历史结果仍可查看。同步更新中英文 README、NEWS、`R_EAN_PLAN.md` 状态和 `ToolkitGuideView.vue` 帮助。
- 验证：Windows 隔离构建 `scripts/build-windows.ps1` 通过（Rust worker release、`vue-tsc -b`、`vite build`、Go GUI EXE）。最终产物 `%LOCALAPPDATA%\rsWGCNA-build\windows\runs\20261111-000420-4642fd\app\rsWGCNA.exe`。本次构建未找到 `~/Bio-info-PRJ/dist-windows/R`，未嵌入 R 运行时；不影响 UI 构建，但绘图/GUI 交互验收未做。
- 性能修复（同轮，未提交）：用户报告卡顿。通过 WebView2 调试端口和 `document.getAnimations()` 定位到 165 Hz 屏上常驻 CSS 动画导致整窗每帧重绘：导航栏「工具箱」的 box-shadow 脉冲、猫尾巴/眨眼。已改为静态样式（保留 hover/thinking/happy 短动画）。实测 Module Explorer 空闲 CPU 91% → 6%，首页 34% → <1%；NEWS 已记录。
- 表格性能（同轮，未提交）：`SimpleTable` 改为窗口化虚拟滚动，只渲染可见约 30 行，搜索/排序/CSV 仍用全量数据；报告页候选表、报告摘要、module overview 表也改用同一组件。实测 module-trait 的 8,000 × 7 kME 表：DOM 72,307 → 570，空闲 CPU 79% → 0.2%，滚动到中段/底部、搜索过滤、清空搜索均通过。报告页和候选优先级表因结果 stale 未实际展开（需重跑 priority 才能验收）。
- 已本地提交 `9a53b38`（feat(ui): demote R-EAN to optional and virtualize large tables），**未推送**。下一步：用户确认后授权推送。

## 最新（2026-11-10，Codex：Windows 探索版安装包）

- Dashboard `explore/project-copilot@d873ade`、worker `explore/project-copilot@10ae0be` 与远端分支一致。构建使用干净源码快照。本轮只更新交接文档，未改应用源码或发布脚本。
- 按 GitHub Release 的 Inno Setup 配置生成 `rsWGCNA-setup-0.6.0.exe`，内嵌 Rust worker、R 4.5.1 x64 和绘图依赖。SVG 导出需要 `svglite`，基因网络图需要 `igraph`，本次打包均已补齐。构建、测试与成品都留在 Windows 本机构建目录。未上传 GitHub。
- 验证通过：前端类型检查和生产构建、Go GUI EXE、WebView2 运行时启动、内嵌 worker 哈希和示例四步分析。使用包内 R 并隔离用户库后，软阈值图、模块性状热图及 igraph 网络图均通过 PNG、PDF、SVG 导出。
- 安装器编译成功，未在现有稳定版环境运行安装器。若要发布，下一步是在独立 Windows 环境验证安装、启动和卸载。未测试 Mac 或运行全量测试套件。

## 最新（2026-11-05 上午，Claude @ 办公室 Windows：candidate priority 注释/预算 + README/NEWS + 推送）

- **收尾状态**：用户结束当天工作，要求 handoff、push、README/NEWS。dashboard `explore/project-copilot`；功能提交 **`3fe2b53`** 已推送 `origin/explore/project-copilot`；本文件、`DSH_PROMPT.md`、`STATE.md` 为随后收尾提交。worker `explore/project-copilot@10ae0be` = origin，工作树干净；`master` @ `fb74752` 落后 `origin/master` 1（未合并）。
- **本轮完成**：
  - Candidate priority 的 plain summary 和 technical details 表增加 **`Symbol/name`、`Description`**，按当前 candidate-priority 模块加载 `annotation.tsv`；表格搜索和 CSV 导出包含注释；raw worker TSV 明确标注“无注释”。
  - 工程预算 UX：`g²×B > 1e9` 时禁用 Run，显示最大 B，并提供 **Set B=100**。blue 2,405×200 = 1,156,805,000 被拒绝；B=100 = 578,402,500 成功；Test-01 blue B=100 **valid 100/100**。
  - `sample_percentage` 改为 0–1 slider、旧项目 `50 → 0.5` 迁移、Go 后端 [0,1] 守卫（上一节详情）。
  - **README.md / README.zh-CN.md / NEWS.md** 已更新：项目工作区、candidate priority/离线证据、R-EAN reliability、0–1 过滤、注释列；NEWS 新增 `[Unreleased] — 2026-11-05`。
  - Test-01 本地观察（分析项目不在仓库）：blue candidate priority top 核心为 NFXL1、PDC2、HSP20-like、HSP101、DNAJ、NAC13 等 early heat-response 基因；108 个 memory 基因中最好的是 CUL2（第 21 名）、DREB2C（第 31 名），大多数靠后；**全 blue 的 candidate priority 不是 memory 选择器**。平衡 trait 后 module-trait 仍显示 blue 是早期热响应、green 的 28h 信号非 memory-specific。
- **验证**：`vue-tsc -b` PASS；`go test ./...`（`CGO_ENABLED=1` + w64devkit gcc）PASS；`go vet ./internal/project ./internal/api` PASS；Windows 隔离构建 PASS（`scripts/build-windows.ps1`，未 `-Launch`），build-info worker `10ae0be`、dashboard `a9ebb04` + working files。Codex 本轮未重跑。
- **Git/推送**：功能提交 `3fe2b53` 已推送；收尾文档随后提交/推送。未合并 `main`；worker 无改动，无需推送。`.dsh-acl-probe.tmp` 仍未跟踪、未提交；未覆盖未知本地改动。
- **下一步**：① 建 108/156 个 memory 基因的 memory-focused 网络并跑 candidate priority/differential；② 做 GO/KEGG 注释一致性 PoC（只作 prior/排序，不作方向；TF–target 后置）；③ 继续 R-EAN differential/bridge 设计；不接在线 LLM、不把全模块 candidate priority 说成 memory。

## 上一轮（2026-11-05 凌晨，Claude @ 办公室 Windows：sample_percentage 0–1 UI + 后端守卫 + 构建）

- **用户要求**：把 UI 的 sample_percentage 滑块范围改成 0–1，并与后端匹配；确认后端 rs_WGCNA 与 GitHub 最新同步。
- **改动（dashboard，未提交）**：
  - `frontend/src/views/DataImportView.vue`：number input 改为 range slider（`min=0 max=1 step=0.01`），显示两位小数；旧项目中 >1 的百分比（如 50）自动迁移为 0.5（1 保留为 1.0）；补充“0 = 关闭，0.5 = 至少一半样本高于阈值”说明。
  - `frontend/src/stores/wgcna.ts`：移除 legacy `samplePercentage / 100`，直接发送 0–1（旧 store 目前无 UI 消费者，但避免未来误用）。
  - `backend/internal/project/run.go`：新增 `validateStepDraft`，`filter.sample_percentage` 不在 [0,1] 时在任何 worker 启动前拒绝，错误提示使用 0.5 表示 50%；`backend/internal/project/store_test.go` 新增拒绝 50 的回归测试。
- **同步核对**：已 `git fetch --prune origin`。worker `rs_WGCNA` 的 `explore/project-copilot` = `origin/explore/project-copilot` @ `10ae0be`，工作树干净；`master` @ `fb74752` 落后 `origin/master` 1（未合并）。dashboard 仍为 `explore/project-copilot`，有未提交改动；未 commit/push/merge。
- **验证**：前端 `vue-tsc -b` PASS；后端 `go test ./...`（CGO_ENABLED=1）PASS；`go vet ./internal/project ./internal/api` PASS；Windows 隔离构建 PASS（`scripts/build-windows.ps1`，无 `-Launch`）。构建产物在本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/runs/.../app/rsWGCNA.exe`，`build-info.json` 显示 dashboard `a9ebb04` + working files、worker `10ae0be`。
- **下一步**：关闭当前旧 rsWGCNA（项目锁），启动新构建；打开 Test-01，Data Import 应显示 0.50；重跑 filter → soft_threshold → network 后再试 trait 上传。构建是工作区未提交源码，不能当作已提交版本或发布版。

## 上一轮（2026-11-04 晚间，Claude @ 办公室 Windows：evidence P1/P2 修复 + Codex 终局 PASS with residual risks）

- **现场**：办公室 Windows 为唯一写入者；家里 Mac 已停止写入；wgcna 自动监控保持 PAUSED。本机网络未通；dashboard 本地无 `origin/explore/project-copilot` 引用，未能 fetch。本地 dashboard `a9ebb04`、worker `10ae0be` 与交接记录一致。未 commit/push/merge。
- **Codex 复核已在本机跑通**：Codex CLI `codex-cli 0.166.0-alpha.5.1`（`%LOCALAPPDATA%\OpenAI\Codex\bin\...\codex.exe`）。主 `~/.codex/.sandbox-bin` 归 Administrators 所有、缺少用户 `WRITE_DAC`，会报 `helper_sandbox_lock_failed`；改用隔离 `CODEX_HOME=%LOCALAPPDATA%\rsWGCNA-build\windows\codex-home-review`（含 `auth.json` 副本，勿同步/提交），并由用户从管理员终端一次性运行 `codex sandbox setup --elevated --current-user`。之后每次在同步盘外一次性快照 `%LOCALAPPDATA%\rsWGCNA-build\windows\review-20261104*` 中执行 `codex exec --sandbox workspace-write`，工作区源码保持只读参考。
- **Codex 轮次**：首轮复核 P1 #6 判 FAIL（`validateGenes` 用 TrimSpace 后值校验、原值入库/渲染，首尾空白可绕过）；修复后连续 6 轮收紧 P2 一致性，最终 round `rean-evidence-p2-20261104-21` 判 **PASS with residual risks**，无本轮必修项。
- **本轮改动（未提交）**：
  - `backend/internal/evidence/evidence.go`：直接校验实际存储的 gene/module 原值；`sum(mean_top_k)=top_k`；按 worker 稳定降序 + bounded-span tie 重算 kWithin→rank；`mean_top_k=0/1` 与 P05/P95 约束；valid=1 三分位数相等、单轮 tie 可实现性、membership 重建；valid=2 `P05+P95=2*median`、median 总和 `n(n+1)/2`、反解 midrank ∈[1,n]；`strata/gene/sample_fingerprint` 与 `resource_note` 类型/必填/事实；`ContractError` 稳定错误码 + 固定安全文案 + 本地 `Diagnostic()`；32 MiB / 100k gene 行预算；envelope/result 严格拒绝未知字段；M3「序列化 bundle 不可信」注释。
  - `backend/internal/evidence/evidence_test.go`、`public_api_test.go`：补齐上述回归/反例与公开路径测试；fixture 的 kWithin/rank/mean_top_k 改为生产自洽值。
  - `backend/internal/api/report_test.go`、`report_html.go`：修复既有 `report_test.go:44` 基线；测试改为验证新的交互式模块富集契约；`overviewTabGroupHTML` 恢复对 `module_overview_*` / `cluster_overview_*` 的 `Assets` 检查，缺图显示「Figure not generated」而不是破图。
  - 真实 worker e2e（非同步盘）：从 `rs_WGCNA@10ae0be` 源码快照构建 `rs-wgcna-worker.exe`；对 demo `black` 模块跑 `hub_priority`（B=20、K=5、iid、design_confirmed=true）。结果 valid 20/20、184 基因、1864 facts、`sum(mean_top_k)=5`；`BuildBundle` 接受真实 envelope，`Evaluate` 成功渲染候选 `AT1G02910`。产物在 `%LOCALAPPDATA%\rsWGCNA-build\windows\e2e-worker`。
  - M3 最小切片（未提交）：`evidence.OfflineSuggestion()` 确定性离线建议（midrank 最低、mean_top_k 次之、gene ID 最终；零有效不推荐；空数组契约）；`GET /api/projects/{id}/evidence/hub-priority` 只从服务器当前原子 `CandidateReportEnvelope` 重建 bundle，not-current 返回固定安全文案（不回显底层路径），`full=1` 才返回 facts/candidates；报告页新增“离线证据解读”卡片（仅当前执行、切换/迟到响应清空、只显示安全 code/message、无在线模型/自由文本/自动执行）。测试：`internal/api/evidence_test.go`、OfflineSuggestion/跨 execution/mean 次级排序/空数组；真实 worker envelope 通过。
  - `docs/exploration/HANDOFF.md`、`DSH_PROMPT.md`、`STATE.md`（dashboard 与工作区根）：本交接。
  - 新建 workspace skill `.claude/skills/codex-code-review/SKILL.md`：复用本机 Codex 复核流程（隔离 home、快照、manifest、中性 prompt、结果回读）；不在两仓库 git 内。
- **实际验证**：evidence **51/51 顶层**通过（90 子测试，0 失败）；API **12/12** 顶层通过（2 个 env-gated skip）；`go vet`、`go build ./internal/...`、`go test ./...`（CGO_ENABLED=1）全通过（`report_test.go:44` 已修）；真实 worker `hub_priority` e2e 与真实 envelope API 测试通过；前端 `vue-tsc --noEmit` 通过（vitest 本机无依赖/无网络未跑）。Codex 独立 probe 复核 evidence；M3 首轮 FAIL 的 P2 已修，第二轮 PASS with residual risks；报告/e2e 未经 Codex 动态复核。
- **Codex M3 复核**：首轮 FAIL（not-current 错误回显底层路径）→ 改为固定安全文案并补测试；round `rean-m3-evidence-20261104-02` 判 **PASS with residual risks**。Codex 快照内 HTTP 项目测试受 Windows `Access is denied` 阻塞，本机真实工作区 `go test ./...` 与 not-run API 测试通过；UI 可视验收未做。
- **残余风险/未跑**：valid≥3 的复杂可实现性、完整桌面/GUI/Mac 可视验收、在线 LLM/API/UI、候选理由持久化、报告 HTML/PDF；Codex 快照内 HTTP 集成受 sandbox 权限阻塞。
- **下一步**：手动 GUI/浏览器验收离线解读面板；决定是否接在线模型 adapter（当前未接）；候选理由持久化；完整桌面/真实 worker 全流程 e2e。仍必须从服务器当前原子 `CandidateReportEnvelope` 重建，持久化建议展示前复验，UI 只输出 `ContractError.Code/Message`，不输出 `Diagnostic()`；提交/推送需用户明确授权。
- **保留不动**：`SidebarDrawer.vue`、`style.css`、`report_assets/design.css`、`DSH_REVIEW_PROMPT.md` 的 CRLF-only 本地改动；`.dsh-acl-probe.tmp`。Codex 复核产物/提示词在 `%LOCALAPPDATA%\rsWGCNA-build\windows\review-20261104*` 与 `review-loop\ready.json`（非同步盘）。

## 上一轮（2026-11-04 09:39，Claude：交接，准备换办公地点）

- **交接意图**：本机停止写入，用户准备去另一个办公地点继续。下一地点先确认唯一写入者，再决定是否恢复任何自动循环；wgcna 自动监控保持 PAUSED。未授权 commit/push/merge/发布。
- **仓库状态**：
  - dashboard `explore/project-copilot` @ `a9ebb04`（完整 `a9ebb042...`，与 `origin/explore/project-copilot` 一致，本次已 fetch 核对）。本地 `main` `c9409e3` 落后 `origin/main` 2（`238170c`、`9a8b1f3`），未合并。
  - worker `explore/project-copilot` @ `10ae0be`（完整 `10ae0be0831...`，与 origin 一致，工作树干净）。本地 `master` `fb74752` 落后 `origin/master` 1（`4012702`），未合并。
- **dashboard 未提交清单**（无 staged）：
  - 本轮 evidence P1 修复：`backend/internal/evidence/evidence.go`、`evidence_test.go`、新增 `public_api_test.go`。
  - 本轮交接文档：`STATE.md`、`docs/exploration/HANDOFF.md`、`docs/exploration/DSH_PROMPT.md`。
  - 其它来源、保留不动的本地改动：`frontend/src/components/SidebarDrawer.vue`、`frontend/src/style.css`、`backend/internal/api/report_assets/design.css`、`docs/exploration/DSH_REVIEW_PROMPT.md`。已用 `git diff --ignore-cr-at-eol` 核对为纯 CRLF 行尾变化，无内容差异；不要 revert 或覆盖。
- **本轮完成**：
  1. Codex 深度审查 `rean-evidence-fix-20261103-14` 判“不通过”，列 5 个 P1；已在 evidence 包修复：私有 sealed 状态、完整结果不变量、strata/method/tie/trait 校验、provenance（含 result digest）绑定、explanation 同候选事实绑定、导出状态不可伪造。
  2. Codex 复核中断前又命中 4 个反例，已修复并加测试：strata 计数整数溢出、kme_abs 负值/kme>1 容差、gene/module 标签渲染注入、重复候选/fact。
  3. 本地验证：`go test -count=1 ./internal/evidence/` **38/38**（34 内部 + 4 外部）；`go vet ./internal/evidence/`、`go build ./internal/...`、`go test ./internal/project/`、`go test ./internal/api/ -run 'TestProject|TestHubPriority|TestCandidateReport'` 通过。前端 `vue-tsc -b` 与 Vite 构建随 macOS 构建通过。完整 `go test ./internal/api/` 仍在既有 `report_test.go:44` 失败（非本轮引入，未修）。
  4. 已构建 macOS 测试 DMG：arm64、ad-hoc 签名、22MB、使用本机系统 R（R 4.5.0 + jsonlite/ggplot2/patchwork/circlize/ComplexHeatmap + pandoc）。DMG 校验通过；从挂载镜像 headless 启动 `/api/health` 返回 ok；内嵌 worker 含 `hub_priority`。产物在本机桌面/构建缓存（不在同步盘），未覆盖 releases。evidence 尚未接入 API/UI，应用界面测试不会直接体现 P1 修复。
- **Codex 复核状态**：第一次因账户额度耗尽中断；额度恢复后重试在探针阶段被 Codex 内容安全过滤器拦截（“possible cybersecurity risk”），未产生最终 verdict。其已运行现有 38 项测试全部通过，但**不得声称 Codex 复核通过**。相关日志/提示词在本机 rsWGCNA-build/macos/review-loop（不在同步盘）。
- **未跑/未验证**：完整 Rust/桌面真实 worker e2e、手动 GUI/Mac 验收（仅 headless health）、在线 LLM/API/UI、M3 adapter/UI、报告 HTML/PDF、候选理由持久化。P2 项（生产 schema 字段对齐、fixture 质量、错误路径泄露、build size/unknown fields）未修。
- **当前进行中的操作**：无。构建、Codex 复核、headless app 进程均已结束；本机已停止写入。
- **交接验收场景**：1) `go test ./internal/evidence/` 38/38；2) 5 个 P1 + 4 个追加反例逐项 PASS；3) 未覆盖或 revert 未知未提交/CRLF-only 文件；4) 下一地点 DMG（或重建包）能启动并对 demo 项目跑通 `hub_priority` 与报告；5) 跨项目/旧 execution/伪造 fact/篡改导出字段均被拒绝。
- **下一步（下一地点）**：
  1. 先核对现场与唯一写入者；不要同时在本机与下一地点写入，不自动恢复 wgcna 循环。
  2. 核对 `git status`/branch/HEAD 与上面一致；保留未知未提交改动。worker 可重新 fetch `10ae0be`。
  3. 重跑 evidence 38 项与 `go vet`；若计数变化先查原因。
  4. 用中性措辞重跑 Codex P1 逐项复核（逐项 PASS/FAIL：结果不变量、strata/method/tie、provenance、explanation 绑定、sealed bundle，以及 4 个追加反例）。避免在 prompt 中使用“绕过/注入/攻击”等触发安全过滤的表述；只要求验证契约不变量。PASS 后再 review 14 轮剩余 P2，再决定 M3 adapter/UI。
  5. DMG 仅在本机可用；若下一地点是另一台机器，重新执行隔离构建（macOS 用 `scripts/build-macos.sh`，Windows 用 `scripts/build-windows.ps1`；需对应 Rust/Go/Node/pnpm 工具链，R 用于报告）。不要复制依赖/缓存/R 环境。
- **本地记录**（不在同步盘）：本机 `rsWGCNA-build/macos/` 下有 build 日志、latest-app、DMG、review-loop 审核日志；另有 `HANDOFF-LOCAL.md` 记录本机代理/工具链/精确路径。密钥、账户、端口、代理配置不写入共享文档。

## 上一轮（2026-11-03 10:57，Claude：evidence P1 修复完成，Codex 复核未完成）

- 起因：Codex 深度审查 `rean-evidence-fix-20261103-14` 判定“不通过”，列出 5 个 P1：结果不变量可伪造、strata/method/tie/trait 仅表面校验、facts/suggestion 未绑定 provenance、explanation 未绑定本候选事实、导出的可变 bundle 可绕过校验。
- 本轮已修（未提交）：`backend/internal/evidence/evidence.go`、`evidence_test.go`、`public_api_test.go`：
  1. 私有 `sealedBundle` 作为 Validate/Evaluate/render 唯一可信状态；导出字段仅快照；零值/伪造 bundle 拒绝；allow-list 非导出。
  2. 完整 result 不变量：计数非负、n_bootstraps/attempted 一致、top_k 范围、P05≤median≤P95、invalid_reasons 合计、zero-valid 统计 null、kme 边界与 |kme| 一致、tie group 完整划分、kme/strata 溢出防护。
  3. strata/method/tie/trait：stratified/iid 分支校验，method_version 仅 `hub_priority.v1`，trait_source 仅 `none`，direction_undetermined ≤ valid，limited 一致。
  4. provenance：envelope 强制 `step=hub_priority`、params 非空对象、inputs/upstream 非空；fact ID/`Fact.Provenance` 绑定 project/step/execution/engine/params/inputs/upstream/result digest；`Suggestion.provenance` 必填并校验。
  5. explanation：`ranking_stability`/`top_k_frequency`/`tie_indistinguishable` 必须引用本候选对应 fact；重复候选/fact 拒绝。
  6. 注入修复：gene/module 标签限制为安全 ASCII 标识符；strata 计数逐项 ≤ sample_count 且加法和不溢出。
- 本地验证：`go test -count=1 ./internal/evidence/` **38/38** 通过（34 内部 + 4 外部）；`go vet ./internal/evidence/`、`go build ./internal/...`、`go test ./internal/project/`、`go test ./internal/api/ -run 'TestProject|TestHubPriority|TestCandidateReport'` 通过。全量 `go test ./internal/api/` 仍在既有 `report_test.go:44` 基线失败（未修复、非本轮引入）。
- Codex 复核：11:41 额度恢复前无法完成。中断前已命中并已修复 4 个新反例（strata 计数溢出、kme_abs 负值/kme>1 容差、gene/module 渲染注入、重复候选）；其对 provenance/explanation/sealed 的既有测试复核通过但未给出最终 verdict。**不得声称 Codex 复核通过。**
- 未跑：完整 Rust/桌面/真实 worker e2e/GUI/Mac、在线 LLM、M3 adapter/UI、报告 HTML/PDF、候选理由持久化。
- 下一步：Codex 额度恢复后用本机 review-loop 的 `round14-p1-verification-prompt.md` 重跑复核；PASS 后再 review 14 轮剩余项并接 M3。仍不 commit/push/merge/合并主线；自动循环保持 PAUSED。

## 上一轮交接（2026-11-03，Codex @ Mac）

请先读本节；下方「办公室交接」节与 [OFFICE_HANDOFF.md](OFFICE_HANDOFF.md) 保留为来源记录。用户从办公室回到家里，授权本轮测试与代码修复；未授权提交/推送/合并/发布。两仓库使用 `explore/project-copilot`：worker HEAD `10ae0be`、dashboard HEAD `a9ebb04`（均已与 origin/explore/project-copilot 核对一致）。远端主线已前进，本次未合并/覆盖。家里 wgcna 自动任务保持 PAUSED，本机（Mac）为当前唯一源码写入者，办公室已停止写入。

本轮修复办公室留下的两个 evidence 反例（14 轮 review 的未闭合项）：
1. **presence/null/type**：`Envelope.Validity` 由普通 `string` 改为 `*string`。`BuildBundle` 现在显式拒绝缺失/null（指针为 nil）与 stale（非空），空字符串正常通过；非字符串类型在 JSON 解码即报错。修正原 `TestMissingValidityRejected` 的假阳性（旧 fixture 因 `gene_count=0` 才失败），新增 `TestNullValidityRejected`。
2. **公开渲染边界**：`RenderSuggestion` 改为非导出 `renderSuggestion`，`Evaluate`（Decode→Validate→Render）成为唯一公开渲染入口；外部无法再跳过 Validate 渲染伪造候选。新增内部 `TestEvaluateRejectsForgedGene`，并新增外部 `package evidence_test` 的 `TestPublicEvaluateRejectsForgedGene` 与 `TestPublicBuildBundleRejectsMissingOrNullValidity`（真实公开 API 调用路径）。

验证（本机 Mac，缓存/产物在同步盘外）：
- `go test ./internal/evidence/...` **22/22** 通过（20 内部 + 2 外部）。
- `go test ./internal/project/...` 通过。
- `go test ./internal/api/... -run 'TestProject|TestHubPriority|TestCandidateReport'` 通过。
- 前端 `vue-tsc --noEmit -p tsconfig.json` 0 错误。
- 前端 vitest `src/stores/project.test.ts` + `src/composables/useCandidateReport.test.ts` **13/13** 通过（vitest 用 `pnpm dlx` 临时拉取到 pnpm 缓存，未写入项目 node_modules）。
- evidence 包 `gofmt` / `go vet` clean。

未跑（保留为缺口）：完整 Rust/桌面/真实 worker e2e/GUI/Mac；`report_test.go:44` 既有基线；报告 HTML/PDF 与候选理由持久化；M3 adapter/UI 接线；在线 LLM/API/UI。

本机环境注意：
- Git 全局配置 `http.proxy=http://127.0.0.1:7897`（Windows 遗留），本机无该代理进程；fetch/ls-remote 用 `git -c http.proxy= -c https.proxy=` 直连，未修改全局配置。
- Go 模块代理 `proxy.golang.org` 本机超时；用 `GOPROXY=https://goproxy.cn,direct` 临时覆盖，未修改 go env。
- dashboard 本地未提交（保留）：`design.css`、`DSH_REVIEW_PROMPT.md`、`SidebarDrawer.vue`、`style.css`（本轮前已存在，未动）；本轮新增/修改 `evidence.go`、`evidence_test.go`、`public_api_test.go` 及本文档。worker 工作树干净。`.dsh-acl-probe.tmp` 为未知本地临时探针，保留不提交。

下一步：先由 Codex 复核本轮两项修复与边界测试；通过后再 review 14 轮其余来源/设计/完整性/候选 fact 关联与严格 JSON 契约，然后才接 M3 adapter/UI。不切 provider、不使用旧 Navigator 权限、不声明防幻觉边界闭合或 M0-M4 验收；不合并远端主线；若需恢复自动循环，先由用户在本机明确恢复，避免双机写入。

## 办公室交接（2026-11-03，Codex，当前唯一接续入口）

请先读 [OFFICE_HANDOFF.md](OFFICE_HANDOFF.md)。用户授权本轮测试及GitHub检查点推送；两仓库使用explore/project-copilot，worker已推10ae0be，dashboard为包含本段的检查点提交。主分支远端已前进，本次未合并/覆盖。家里自动任务wgcna已PAUSED，办公室单写入者确认后再明确恢复。

本轮独立Go evidence/project和相关API、前端typecheck、13项实际store/composable全部通过；额外两反例失败（缺validity的合法envelope被接受、公开RenderSuggestion绕过校验）。14已收尾但未最终通过review，不进入在线AI。完整复现、版本、未运行项/基线、依赖隔离、日志位置和可复制prompt都在OFFICE_HANDOFF.md；下方全部为历史。
## 最新（2026-11-03 03:45，DSH：evidence 包契约收紧（14），awaiting_review）

- **review_id**：`rean-evidence-fix-20261103-14`。上一轮（额度中断）已重写 `backend/internal/evidence/evidence.go` + `evidence_test.go`，本轮完成收尾验证。未接 API/UI/在线模型、不改 worker。
- **修复 4 个反例**：
  1. `BuildBundle` 收紧不变量：`gene_count == len(genes)`（截断行不再称完整）、`design_confirmed` 必须 true、`scope == fixed_full_module`、`resampling ∈ {iid,stratified}`、`attempted == valid+invalid`、gene ID 非空唯一、数值有限 + rank/midrank/mean_top_k 范围校验；`n_bootstraps` 读原字段（不再用 attempted 冒充）；补 invalid_reasons/strata/method_version/stratum_count 入事实。
  2. fact 显式绑定 `Gene`/`Semantics`（不靠解析含 gene 文本的路径）；`ValidateSuggestion` 要求每个候选至少关联本 gene 的事实；`tie_indistinguishable` 必须有并列事实支持；数值 claims 必须引用已列入 fact_ids 的事实。
  3. `DecodeSuggestion` 严格解码：恰好一个非 null JSON 对象 + EOF（拒绝尾随 JSON/垃圾/null）；`Evaluate` 作为唯一安全入口 Decode→Validate→Render。
  4. Envelope 纳入 params/inputs/upstream/validity 并校验（validity 非空拒绝、来源缺失拒绝）；Suggestion 显式绑定 project/execution 并与 bundle 校验（旧执行/跨项目无引用也拒绝）。
- **测试**：18/18 通过（原 12 + 4 新反例 + 缺失 validity + tie 条件）。`go test ./internal/evidence/...` ✅。
- **验证**：`go build ./internal/evidence/...` ✅；`go test ./internal/evidence/...` 18/18 ✅。**未跑**：完整 `go build ./...`（webview_go 构建约束预存在）、Rust/桌面、在线 LLM、UI/GUI/Mac。
- **状态**：`awaiting_review`（ready.json + manifest 本机 review-loop，最后采哈希）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核 14；之后 M3 adapter/UI 接线（在线 LLM 与应用执行仍未做）。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：DSH第14轮因额度耗尽中断，部分改动未验收）

- GUI已直接确认“处理失败 / 本轮运行失败 当前请求的额度已用尽 / QUOTA”。这不是Codex额度保护；DSH上一轮14已收到，但中途失败。未切provider、未重发请求、未代办充值/认证。
- DSH可见轨迹：已重写evidence builder/校验/测试，曾运行测试并修fixture rank_upper与gene_count不一致；没有新ready，也无完整完成/验证结论。现有ready仍13-fix，不能拿其旧manifest复核当前部分写入源码，不能声称14修复通过。Codex本次不跑构建测试、不接手修改源码。
- 精确接续：额度恢复/用户指定处理方式后，让同DSH会话继续DSH_REVIEW_PROMPT顶部14，先核对保留的部分修改，完成全部失败反例与生产路径测试；所有文件/STATE/双交接收尾后最后采新manifest并写新round_id awaiting_review，再由Codex复核。不要重复实现已写部分，不进入API/UI/在线模型。
- 最近独立结论仍为13快照原12通过、新4失败，日志本机review-loop/codex13-evidence-probe.txt；不能套用到当前未完成修复。报告GUI/Mac/HTML/PDF/候选理由持久化等缺口保留，report_test.go:44既有基线未重跑。
- 两仓库仍dashboard explore/project-copilot@c9409e3、worker master@fb74752，未知未提交保留。唯一授权源码写入者仍DSH，当前图形轮次已失败/停止；现场分支完整HEAD/工作区、进程与当前evidence文件hash在本机review-loop/dsh-quota-block-latest.json，state.json已记blocked_external_quota。机器密钥/配置、依赖缓存/产物留各自机器同步盘外，不将本机路径用于另一主机。
## 最新（2026-11-03，Codex：AI离线契约review发现4反例，派发14）

- rean-ai-contract-20261103-13-fix manifest稳定，DSH GUI空闲。独立本机隔离运行当前evidence两源码+probe：原12通过、新4失败（截断1/1000称全量、g2事实支持g1、尾随第二JSON动作被接受、设计未确认可推荐）。日志review-loop/codex13-evidence-probe.txt，probe源码同名目录；未改仓库源码。
- 下一步执行DSH_REVIEW_PROMPT顶部rean-evidence-fix-20261103-14：收紧builder完整性/设计/数值、候选fact语义绑定、完整JSON+唯一安全渲染入口、来源输入参数上游与bundle身份、实际字段和分母。保持仅离线evidence包，不接UI/API/在线LLM、不改worker。
- 原“12tests防幻觉通过”为对方测试报告/现有案例覆盖，不能宣称完整防幻觉已完成。Codex独立结果12通过4失败；完整Rust/桌面/GUI/Mac未跑，report_test.go:44基线保留。两仓库dashboard explore/project-copilot@c9409e3、worker master@fb74752，未知未提交保留，DSH唯一源码写入者；本机review-loop记录轮次/manifest/派发/验证，依赖产物配置密钥各机同步盘外。
## 最新（2026-11-03 03:40，DSH：M3 离线 EvidenceBundle + 校验器，awaiting_review）

- **review_id**：`rean-ai-contract-20261103-13`。本轮新增独立 Go 包 `backend/internal/evidence/`（evidence.go + evidence_test.go），未改 worker/算法/UI/现有 api，未做在线 LLM 调用、不切 provider、不读密钥、不新增 shell/文件/HTTP 工具。
- **实现**：`BuildBundle` 从 Project 报告 envelope 构造离线事实集——每个 fact 有稳定 `fact_id`（SHA256(project+execution+field_path)）、field_path/value/unit/scope/quality/source；含模块规模/设计/valid·attempted·invalid/方向未知/K·B·seed/方法局限；候选 ID 必须在完整结果中、full_coverage=true；Summary 由确定代码算。
- **只读建议 schema**：`Suggestion`（gene_ids/fact_ids/有限枚举 explanations·limitations·plan_preview/claims），无执行参数/运行指令/tool；`ValidateSuggestion` 校验 fact 存在/候选匹配/数值 claims 精确一致/枚举允许/零 valid 禁推荐；`RenderSuggestion` 用代码渲染已批准模板 + **始终注入确定性局限**（条件化说明不可被模型省略）；`DecodeSuggestion` 严格解码（拒绝非 JSON/未知字段/超大输入，隐藏执行字段无法存活）。`OfflineStatus` 返回 deterministic_fallback（未配置/远程关闭，不悄悄换 provider）。
- **测试**（生产路径，导入实际 builder/validator/renderer，mock 模型输入为 Suggestion）：12 例——合法引用通过、伪造 gene/fact/value、跨项目/旧 execution、子集标记、无表型给 GS 解释、零 valid 推荐、invalid 条件始终渲染、频率当真实概率、恶意"忽略规则"注释、隐藏执行字段/非 JSON/未知字段/超大输入、离线降级、不改变 envelope/数值。`go test ./internal/evidence/...` **12/12 通过**。
- **验证**：`go build ./internal/evidence/...` ✅；`go test ./internal/evidence/...` ✅。**未跑**：完整 `go build ./...`（webview_go 构建约束预存在，需 CGO/Windows 标签，非本轮引入）；完整 Rust/桌面构建；在线 LLM（未配置）；UI；GUI/Mac。
- **状态**：`awaiting_review`（ready.json + manifest 本机 review-loop，最后采哈希）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **明确未做**：在线 LLM 实测、UI、应用建议执行权限、报告 HTML/PDF、候选理由持久化；adapter 仅 mock/确定性降级。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：12定向13/13通过，派发13离线AI事实契约）

- 已复核rean-report-state-20261103-12-fix，manifest稳定，DSH同一wgcna会话空闲。独立实际store/composable13/13通过，production composable快照哈希匹配，日志本机review-loop/codex12-report-review.txt。跨项目loading/真实reject回归已通过；不等于全并发/GUI/M0/M2全部验收。
- 下一步DSH_REVIEW_PROMPT顶部rean-ai-contract-20261103-13：M3离线backend EvidenceBundle/结构化只读响应validator/确定性模板与降级、生产路径防幻觉测试。不在线调用LLM、不改provider、不做UI或参数自动执行，不用自由文本提示冒充校验。
- 当前报告HTML/PDF、候选理由持久化、可视GUI/Mac、完整报告Go独立复跑仍缺；report_test.go:44基线未重跑。13阶段只代码契约，后续在线/适配/UI需要下一轮review派发。
- dashboard explore/project-copilot@c9409e3、worker master@fb74752未知未提交保留，DSH唯一源码写入者；本机review-loop/state/ready/manifest与诊断日志记录状态，机器配置密钥/依赖缓存R/产物各机同步盘外。
## 最新（2026-11-03 03:28，DSH：报告请求状态边界修复，awaiting_review）

- **review_id**：`rean-report-state-20261103-12`。本轮只改前端 `composables/useCandidateReport.ts` + `useCandidateReport.test.ts`，未改 worker/算法/Go/AI。
- **P2 修复**：`useCandidateReport` 的项目切换 watch 现在同时 `loading.value = false`（此前只清 report/error + bump reportReq，导致挂起请求切项目后新项目按钮永远 loading 而无法生成报告）。
- **测试**：`useCandidateReport.test.ts` 新增 2 例——「跨项目切换释放挂起 loading」+「真正 reject 的迟到错误不清新报告」（把原「superseded late error」改为真实 reject，不再用成功 resolve 冒充错误分支）；`ProjectResponse` 类型改从 `api/projects` 导入（原从 `api/client` 错导入）。**本机外置 vitest 快照实际 store+composable 13/13 通过**（7 store + 6 composable，原 11 保留）。
- **验证**：vue-tsc ✅；vitest 13/13 ✅。**未跑**：完整 Rust/Go/桌面构建（本轮仅前端 composable+测试）；GUI/Mac。
- **状态**：`awaiting_review`（ready.json + 21 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 继续 10/12 复核；M3 受约束 AI assistant。报告 HTML/PDF 导出与候选理由持久化仍未做。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：11快照一致，报告状态probe失败，派发12）

- rean-manifest-20261103-11-fix全部manifest前后匹配，DSH GUI空闲。10轮锁内envelope与前端报告自身project/execution绑定已源码落地，尚有UI请求状态缺口。
- 独立实际store+composable测试原11通过，新第12例失败：报告加载中切项目，reportReq增加但loading永不复位，新项目生成按钮卡死。日志本机review-loop/codex10-report-loading.txt，追加probe及.codex10-backup在store-test/src/composables目录；production composable哈希匹配当前源码。
- 下一步DSH_REVIEW_PROMPT顶部rean-report-state-20261103-12：修项目/会话/执行请求状态边界，正式纳入跨项目loading反例，真正reject的迟到error测试（原同名用例只resolve成功），纠正测试ProjectResponse导入/类型检查；仅前端定向验证，不推进AI/不改算法/不整套构建。
- 本轮Go仅源码审查未独立重跑；DSH报告Go定向/typecheck/11tests通过与Codex独立结果分开。report_test.go:44基线未重跑，GUI/Mac未验收；HTML/PDF/候选理由持久化未做。两仓库HEAD仍dashboard explore/project-copilot@c9409e3、worker master@fb74752，未知未提交保留。DSH唯一源码写入者；本机review-loop记录ready/state/manifest与诊断，机器依赖配置密钥产物不同步。
## 最新（2026-11-03 03:22，DSH：仅重写 manifest 快照，awaiting_review）

- **review_id**：`rean-manifest-20261103-11`。10 轮复核暂缓的唯一原因：上一轮 manifest 里 `STATE.md` 的 SHA256 是在后续编辑 STATE.md 之前采的，已过期。本轮**不改任何源码/算法/UI，不重跑构建**；仅确认所有文件收尾后重新采集 21 文件哈希，写新 round_id `rean-manifest-20261103-11-fix` 的 ready/manifest。
- 10 轮报告身份 envelope + 生产路径测试（Go envelope 测试 + 前端 useCandidateReport 测试，vitest 11/11）的结论**尚未判通过**，等快照一致后 Codex 继续复核；不进入 M3。
- 未改文件内容（HANDOFF/DSH_PROMPT 本段除外）；两仓库 HEAD 仍 worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- 单写入者：DSH；未 commit/push/merge。

## Codex：10轮复核暂缓（2026-11-03）

ready round rean-report-fix-20261103-10-fix已报告完成，但manifest包含的STATE.md实际SHA256不匹配（其余文件当前匹配）。依自动review协议先停止本轮结论，不把未完成复核称通过、不派M3。请确认收尾后不再写文件，重写完整manifest并生成新round_id的awaiting_review；本轮仅修交接快照，不改算法/UI或重跑构建。snapshot修好后继续10轮报告身份与生产路径测试复核。11/11为DSH报告，Codex本轮尚未独立运行。机器hash/记录留本机review-loop；既有report_test.go:44、GUI/Mac未验收保留。
## 最新（2026-11-03 03:19，DSH：M2 报告身份 envelope + 生产路径测试，awaiting_review）

- **review_id**：`rean-report-fix-20261103-10`。本轮改 Go `access.go`/`projects.go`/`store_test.go` + 前端 `client.ts`/`api/projects.ts`/新增 `composables/useCandidateReport.ts(+test)`/`ProjectReportView.vue`，未改 worker/AI。
- **P1 后端 envelope**：`project.CandidateReportEnvelope(engine)` 在**锁内一次一致读取**中 re-check validity + 返回 `{project_id, execution_id, engine, params, inputs, upstream, validity, result}`；`candidateReport` 返回该 envelope（stale/not-run → 409）。不再返回裸 worker JSON、不分别 Snapshot/Validity/Result 拼装。`readResultLocked` 复用锁内实现，`Result` 重构调用它。
- **P1 前端身份**：抽 `useCandidateReport` composable，报告自身携带 project_id/execution_id/upstream/params/inputs；`reportCurrent` 要求报告身份与当前 project/execution 一致 + `currentResult('hub_priority')`；upstream 从报告 envelope 快照读取（不猜实时 store）；TSV 链接仅 `reportCurrent` 时显示；报告 JSON 下载含来源/参数/全部候选；请求计数器 + project/execution 落地守卫（迟到旧请求/迟到 error 不覆盖新报告）；项目切换 watch 清报告。`load` 按钮在 draftDirty/saving 时禁用。
- **测试**：Go `TestCandidateReportEnvelope`（not-run/stale 拒绝 + envelope 身份/provenance/result）；前端 `useCandidateReport.test.ts`（身份不匹配撤销、项目切换清空、两请求逆序、迟到 error 不清新结果）。**本机外置 vitest 快照实际 store + 实际 composable 11/11 通过**（7 store + 4 report）。
- **验证**：vue-tsc ✅；go build ✅；go test（TestCandidateReport|TestHubPriority）✅；vitest 11/11 ✅。**未跑**：完整 Rust（worker 未改）；完整桌面构建（reviewer 要求不重复）；GUI；Mac。
- **状态**：`awaiting_review`（ready.json + 21 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；M3 受约束 AI assistant。报告 HTML/PDF 导出与候选选择/理由持久化仍**未做**（明确缺口）。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：09报告复核未通过，派发10身份修复）

- 09任务已确认送达并完成；旧delivery_unknown已解除。round rean-report-20261103-09-fix的18文件manifest稳定，DSH同一wgcna GUI完成且空闲。
- 主要阻塞：ProjectReportView保存原JSON却用实时store.currentResult判断当前、实时上游做来源，A报告在B重跑/跨项目后可再次被称当前并下载B TSV；后端candidateReport仅原始结果，缺project/execution/input/params来源快照与可验证包。只有not-run新测试，未验收正常/失效/跨项目/逆序路径。
- 下一步执行DSH_REVIEW_PROMPT顶部rean-report-fix-20261103-10：报告envelope一致快照、前端绑定报告身份/历史与下载、实际生产路径定向测试、完整机器可读JSON下载；不改worker、不进入AI、不重复完整桌面/Rust构建。
- Codex本轮独立仅源码/哈希/GUI；DSH报告09 typecheck/Go核心/构建通过，未独立重跑报告测试，GUI/Mac未验证；report_test.go:44既有基线未重跑。08实际store独立7/7不替代报告验收。
- dashboard explore/project-copilot@c9409e3，worker master@fb74752，未知未提交保留；DSH唯一源码写入者。本机review-loop/state/ready/manifest存派发与待审记录，依赖配置/密钥/产物各机同步盘外。
## 最新（2026-11-03 03:10，DSH：M2 候选证据报告最小切片，awaiting_review）

- **review_id**：`rean-report-20261103-09`（09 派发已收到并完成）。本轮改 Go `projects.go`/`projects_test.go` + 前端 `api/projects.ts`/`router.ts`/新增 `ProjectReportView.vue` + `STATE.md`，未改 worker/算法/AI。
- **后端**：新增 `GET /api/projects/{id}/report/hub-priority` → `candidateReport`，**服务端重校验** `s.validity = s.p.Validity(h.engine)`（不信任前端 boolean），stale/not-run → 409 带清楚原因；有效才返回 committed step8_hub_priority.json。不把 project id 当 job id、不复用 legacy Job 报告。
- **前端**：`ProjectReportView.vue`（接替 `/report` 占位）：客户端 `currentResult('hub_priority')` 门控 + 请求回包 project/session/execution 代际校验；展示方法版本/scope/power/network_type/K/B/seed/重采样/strata/valid/invalid/方向未知/资源说明/上游 filter·network execution；完整候选表（安全转义 `{{ }}`）+ 完整 TSV 下载（复用白名单端点）；确定性解释与局限（无表型→仅网络候选优先级、midrank 区间非置信区间/显著性、并列不可区分不强制唯一、不合成总分不称真正 hub、valid=0 禁止推荐）。
- **未做（明确缺口）**：报告 HTML/PDF 导出、候选选择/理由持久化（选中 UI 状态未冒充已持久化）、AI 总结（M3）、自由生成文献/功能注释。
- **验证**：vue-tsc ✅；go build ✅；go test（TestCandidateReportNotRun：not-run→409；TestHubPriority）✅；go test project ✅；隔离桌面构建 ✅（696 模块，新报告入口 bundle）。**未跑**：vitest（store 本轮未改，上轮 7/7）；完整 Rust；GUI 可视点击；Mac。
- **状态**：`awaiting_review`（ready.json + 18 文件 manifest 本机 review-loop）。新 exe `…\runs\20261103-071128-d0e413\app\rsWGCNA.exe`。
- **下一步**：等 Codex 复核；M3 受约束 AI assistant。M0 各子项逐项记录、未整体标关闭。
- 单写入者：DSH；未 commit/push/merge。

> 09发送状态补充：review与prompt已完成，但本机DSH图形输入未能确认生效；再次抓图仍无09草稿/消息气泡，重新激活后焦点仍无法验证。为避免重复发送，本机state标delivery_unknown并停止输入。下次先查同review_id是否已收到；没有收到且桌面恢复可操作时再发送。没有通过内部接口或其他会话绕过。08独立7/7通过不受此派发阻塞影响。
## 最新（2026-11-03，Codex：额度恢复，08实际store7/7通过，推进09报告切片）

- 已复核rean-state-race-20261103-08-fix，15文件源码manifest前后稳定，DSH同一wgcna会话空闲无草稿。Codex独立核对当前store/测试与本机快照SHA256一致，实际store vitest7/7通过，含之前refresh-during-run反例；日志本机review-loop/codex08-store-review.txt。不宣布所有并发/整个M0/GUI已验收。
- 已解除额度保护等待，下一步执行DSH_REVIEW_PROMPT顶部rean-report-20261103-09：M2 Project当前候选证据报告只读预览/全量导出、后端一致execution/validity检查、前端dirty与回包代际、确定性解释/局限/转义；不把project id当legacy job id，不改worker，不上AI。
- 本轮未完整Rust/Go/桌面构建或GUI点击；DSH此前typecheck通过为对方报告，report_test.go:44已有基线未重跑。Mac未验证。M2局部图/候选理由持久化若未做继续列缺口，M3待下轮审查后派发。
- 两仓库dashboard explore/project-copilot@c9409e3、worker master@fb74752未变，所有未知未提交保留。DSH唯一源码写入者；本机review-loop/state/ready/manifest/诊断日志记录派发与写入者，依赖缓存R产物在各自机器同步盘外；旧quota-handoff-latest.json为低额度历史快照，不代表当前暂停。
## 最新（2026-11-03，Codex：短周期剩余20%，仅交接，08待复核）

- 触发低额度保护，本轮不审查新实现、不运行测试/构建、不再派发或发送DSH指令、不切provider。用户授权的review循环保留，额度恢复后先复核，不自动跳过。
- DSH ready 报告新轮次 rean-state-race-20261103-08-fix 已完成：run/import/demo/cancel响应落地后补stateGen；实际store正式测试7/7、typecheck通过。**这些是DSH报告，Codex尚未复核08源码/独立重跑**。上一轮Codex独立结果仍为07快照6通过1失败，不可把两者混写成已独立验收通过。
- 精确下一步：读DSH_REVIEW_PROMPT顶部08要求，核对ready和15文件source manifest、GUI同一wgcna会话确已完成且无草稿、前后源码稳定；针对新增mutation落地代际与第7例检查并在本机隔离快照必要重跑实际store测试。通过后再规划M2报告/候选解释；AI为M3。可视GUI点击验收仍未跑，不能宣布M0全关闭。report_test.go:44已有基线失败未重跑；不重复整套Rust/Go/桌面构建。
- 两仓库现场：dashboard explore/project-copilot@c9409e3c02a0eaec50dd73e78d7fb3f78e91342f；worker master@fb74752d61b98eeeb3ef2b8ffbadf9eaac73c2dc。dashboard源码/前端tsconfig/project测试/后端project/API/docs/scripts等大量未提交及未跟踪保留，worker.rs/lib.rs与新增hub_priority/kwithin/resampling保留；没有提交推送。
- DSH保持唯一源码写入者；ready为awaiting_review报告，本轮低额度仅核对进程存在，未再次GUI确认空闲。准确未提交清单、活跃进程/PID与写入者说明已存本机review-loop/quota-handoff-latest.json；ready.json/state.json及本轮manifest也在本地，不共享机器配置/密钥。依赖/R/缓存/产物继续留各机器同步盘外，不假设另一机器路径。
## 最新（2026-11-03 03:03，DSH：mutation 落地代际 + 第 7 例反例修复，awaiting_review）

- **review_id**：`rean-state-race-20261103-08`。本轮只改 `frontend/src/stores/project.ts` + `project.test.ts`，未改 worker/Go/UI/AI。
- **P1 修复**：`run`/`importFile`/`loadDemo`/`cancel` 在 mutation 响应**落地**后再 bump 一次 `stateGen`（此前只在开始 bump）。这样 run POST 挂起期间发起的 `refresh`（其 `ver` 捕获的是 run 开始后的版本）在 run 落地后因 stateGen 再次前进而被丢弃，不再用旧快照回滚 revision/execution/busy。保留 run 开始时的 bump（使更早的读失效）+ 落地 bump（使 mutation 期间的读失效）。
- **测试**：`project.test.ts` 新增第 7 例「run 进行中发起 refresh 不得回滚已落地的 run」（open rev1 → run POST 挂起 → refresh GET 挂起 → run resolve rev2/新 execution → refresh resolve 旧 rev1 → 断言 rev 仍 2、execution 仍新）。**本机外置 vitest 快照运行实际 store，7/7 通过**（保留原 6 例不弱化）。
- **验证**：vue-tsc ✅；vitest 实际 store 7/7 ✅。**未跑**：完整 Rust/Go/桌面构建（本轮 worker/Go 未改，reviewer 要求不重复）；GUI 点击验收（DSH 无 GUI 能力，缺口保留）。
- **状态**：`awaiting_review`（ready.json + 15 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；M2 报告/候选解释未完成，AI 为 M3。M0 前端各子项逐项记录、未整体标关闭。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：07独立实测6通过1失败，派发08修mutation期间读）

- 已审rean-state-fix-20261103-07-fix，15文件manifest稳定、DSH空闲。实际store测试来源哈希等于当前源码；Codex在本机隔离快照独立运行原6例+新反例，6通过1失败。
- P1：run开始后、响应落地前发起refresh；run先提交新revision/execution，refresh晚返回旧状态仍被接受。stateGen仅run开始增加、落地不增加，refresh又允许working期间发起。日志本机review-loop/codex07-refresh-during-run.txt；追加测试store-test/src/stores/project.test.ts，原件备份同目录.codex07-backup。不是模拟测试，不改仓库算法/UI源码。
- 下一步执行DSH_REVIEW_PROMPT顶部rean-state-race-20261103-08：修mutation进行期间读/响应落地代际，纳入正式deferred回归测试；只前端定向检查，不扩报告/AI、不重复整套构建。selection身份和双current overlay已源码确认，不重复改。
- GUI点击/完整Rust/Go/桌面本轮未跑；report_test.go:44基线未重跑。dashboard explore/project-copilot@c9409e3、worker master@fb74752，未知未提交保留，DSH唯一源码写入者；本机review-loop/ready/state/manifest记录派发与现场，各机依赖配置产物在同步盘外。
## 最新（2026-11-03 02:55，DSH：M0 refresh stateGen + 实际 store deferred 测试，awaiting_review）

- **review_id**：`rean-state-fix-20261103-07`。本轮只改前端 `stores/project.ts` + `ModuleExplorerView.vue` + 新增 `stores/project.test.ts` + `tsconfig.json`，未改 worker/Go/AI。
- **P1 refresh**：新增 `stateGen`（覆盖所有状态写入/操作边界：run/import/demo/cancel/flush PUT/hydrate/close/refresh start）；`refresh` 落地前校验 `sessionGen + stateGen + id`，无参数 run / 迟到 refresh / 保存交错都使旧读失效，不再回滚 execution/busy/revision。
- **P2 selection 身份**：`selectedElement` watch 绑定 hub/rean 的 execution_id + currentResult，任一变动即清除；`stabilityMap` 叠加要求 `currentResult('hub') && currentResult('rean')`（rean 经 backend validity 校验 rean→hub upstream execution 关联）+ module 匹配。
- **实际 store deferred 测试**（`project.test.ts`，导入真实 `useProjectStore`、仅 mock `projectRequest` 传输）：6 用例覆盖无参数 run vs 迟到 refresh、两次 refresh 逆序、A/B 结果逆序、同项目重开、draft 保存交错、缺失 validity。**在本机外置快照 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-loop/store-test` 用 vitest 5.0.5 + vue 3.5.44 + pinia 4.1.0 运行，6/6 通过**（非守卫模拟重写）。
- **tsconfig**：主 typecheck 排除 `src/**/*.test.ts`（vitest 依赖只在本机外置快照，不进主构建）。
- **验证**：vue-tsc ✅；vitest 实际 store 6/6 ✅。**未跑**：完整 Rust/Go/桌面构建（本轮 worker/Go 未改，reviewer 要求不重复）；可视化 GUI 点击验收（DSH 无 GUI 能力，缺口保留）。
- **状态**：`awaiting_review`（ready.json + 15 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；M2 报告/候选解释未完成，AI 为 M3。M0 前端各子项逐项记录、未整体标关闭。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：06修复复核，剩余refresh竞争，派发07）

- rean-ui-fix-20261103-06-fix的13文件manifest稳定，DSH空闲；resultMeta/session/batch守卫、strict validity和历史分母未知已实现。
- 尚有P1：project.ts:117-130的refresh未隔离无参数修改的run/import或另一refresh；session/generation不变时迟到读可回滚execution/busy。模拟测试重写守卫而非导入实际store，不能算production回归测试。另selectedElement仅watch hub boolean，需绑定rean/hub证据身份。
- 下一步执行DSH_REVIEW_PROMPT顶部rean-state-fix-20261103-07：只修状态请求竞争，实际store deferred反例，不改worker、不上AI、不重复完整构建。GUI验收DSH无能力，明确保留后续Codex/用户；报告/解释M2尚未完成，AI为M3。
- 本轮Codex仅源码/manifest/GUI复核；typecheck/3模拟案例/构建为DSH报告，未独立测试。report_test.go:44已有基线未跑。两仓库HEAD仍dashboard explore/project-copilot@c9409e3、worker master@fb74752，未知未提交保留，DSH唯一源码写入者；本机review-loop存ready/state/manifest/派发记录，依赖进程配置与产物在各机同步盘外。
## 最新（2026-11-03 02:50，DSH：M0 store/UI 异步代际与证据边界修复，awaiting_review）

- **review_id**：`rean-ui-fix-20261103-06`。本轮只改 `frontend/src/stores/project.ts` + `frontend/src/views/ModuleExplorerView.vue`，未改 worker/Go/AI。
- **P1 loadResults**：新增 `sessionGen`（hydrate/close 会话代际）+ `resultsGen`（批次）+ `resultMeta`（step→execution_id）；每次响应落地前校验会话/批次/execution_id 一致，迟到旧 execution/同项目重开请求不覆盖新结果。
- **P1 refresh**：响应落地前校验 sessionGen + generation；迟到的 refresh 不覆盖保存后的 revision/validity。
- **P1 currentResult**：`validity[step] !== ''` 严格判空（缺失不当有效）；要求 `resultMeta[step] === 当前 execution_id`（结果确已加载且身份一致）。
- **P2 UI**：`hubStale` 改用 `!currentResult('hub')`；`selectedElement` 在 hub 证据撤销时清除（watch）；stabilityMap 增加 module 关联校验（不只按 gene-pair join）；rean 卡加「历史」徽标 + 旧 schema 缺 valid/attempted 显示「未知」（不推断 n_bootstraps）。
- **验证**：vue-tsc ✅；本机外置守卫逻辑模拟测试 3 场景通过（A/B 逆序、关闭重开、refresh 交错）；隔离桌面构建 ✅（bundle `index-DaZCj91O.js`）。**未跑**：可视化 GUI 点击验收（DSH 无 GUI 截图/点击能力，非人工步骤；新 exe 已更新桌面快捷方式）。**未重跑**：完整 Rust/Go（本轮 worker/Go 未改）。
- **状态**：`awaiting_review`（ready.json + 13 文件 manifest 本机 review-loop）。新 exe `…\runs\20261103-024419-cf2238\app\rsWGCNA.exe`。
- **下一步**：等 Codex 复核；M2 报告/候选解释缺口仍列明未完成，AI 为 M3。M0 前端各子项逐项记录、未整体标关闭。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：UI review 未通过，派发 rean-ui-fix-20261103-06）

- 已审 round rean-ui-20261103-05-fix，13 文件源码 manifest 匹配，DSH 同一 wgcna 会话完成/空闲。候选表/默认设计确认/TSV 与 worker Project fixture 已接入。
- 阻塞：store.loadResults 只有 project id 守卫，迟到旧 execution/同项目重开请求可覆盖新结果；currentResult 未绑定实际加载结果 execution，validity 缺失还会按有效。refresh 迟到可覆盖保存后的 revision/validity。旧 selectedElement 与 rean 卡/导出仍可能保留过期证据。详见 DSH_REVIEW_PROMPT 顶部精确行号、反例与验收。
- 下一步仅 store/UI 代际与当前/历史边界修复 + deferred 请求反例测试 + 必要可视 fixture 验收；不做 AI，不改 worker 算法，不重复完整 Rust/Go 构建。总体 AI 为 M3；M2 报告/解释缺口先列明，不按 DSH“下一步AI”直接跳过。
- Codex 本轮只源码/哈希/GUI核对，未独立重跑测试/构建；DSH 报告64 Rust tests、Go project/API、真实 worker Project e2e和隔离桌面构建通过；可视点击验收未跑。已知 report_test.go:44 富集基线失败未重跑；Codex此前 API 独立缓存缺 excelize2.11，不能声称独立通过。
- dashboard explore/project-copilot@c9409e3，worker master@fb74752，HEAD 未变；大量未提交（含新未跟踪文件）保留。DSH唯一源码写入者；本机 review-loop/state.json/ready.json/source manifest 记录轮次/派发/写入者，依赖进程配置与产物在各机同步盘外，不把本机绝对路径写为另一机器路径。
# 探索分支交接（每次工作结束更新）

## 最新（2026-11-03 02:45，DSH：M0 候选优先级 UI + currentResult，awaiting_review）

- **review_id**：`rean-ui-20261103-05`。本轮改 worker 小修 + 前端 + store_test.go，未做 AI，未覆盖用户项目/稳定版。
- **worker 小修**：`read_sample_strata` 拒绝纯空白分层值；输出补 `strata_counts`（每层数量）+ `strata_fingerprint` + `resource_note`（g²×B≤1e9 工程预算说明，默认 B=200 拒绝 g≳2236）。
- **前端**：
  - `api/projects.ts`：注册 `hub_priority` StepName + defaultParams（`design_confirmed:false`、`trait_source:'none'`）；`Params` 类型支持 bool；新增 `projectOutputUrl`。
  - `api/client.ts`：`HubPriorityResult`/`HubPriorityGene`/`HubPriorityInvalidReason` 类型。
  - `stores/project.ts`：拆分 `draftDirty`（仅 draft 变更，revoke 当前证据）vs `dirty`（draft+card+ui 变更，仅触发保存）；`currentResult(step)` 统一 selector（done + validity 空 + !saving + !draftDirty），重命名替换旧 `resultCurrent`；flush 里用 `draftGeneration` 正确清除 draftDirty。
  - `ModuleExplorerView.vue`：新增「候选优先级」tab：门控仅 filter/network 有效 + 模块已选 + design_confirmed 显式勾选（默认未勾）；iid/stratified + 分层列（来自 sample_info）；工程预算提示（超 1e9 仅提示不截断）；结果表格分页（默认 50/页）+ 完整 TSV 下载（走精确白名单端点）；invalid/全无效/direction_undetermined/过期警示；midrank 并列分数定义不称概率。
- **测试**：`store_test.go` 新增 `TestRealWorkerHubPriority`（真实 Rust worker 经 Project 跑 filter→sft→network→hub_priority，校验 JSON+TSV 提交）。
- **验证**：vue-tsc ✅；cargo test 64 ✅；go test project/api 核心 ✅；**真实 worker e2e PASS**（0.48s）；隔离桌面构建 ✅（前端 bundle 含候选优先级 tab，`index-DfAlT5Cs.js`）；完整 `go test ./internal/api/...` 仅 report_test.go:44 基线失败（无新回归）。**未跑**：可视化 UI 点击验收（手动，桌面快捷方式已更新到新 exe）。
- **状态**：`awaiting_review`（ready.json + 13 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。新 exe `…\runs\20261103-023255-d9b026\app\rsWGCNA.exe`，worker sha256 `95616bb4…`。
- **下一步**：等 Codex 复核；通过后 M2 AI assistant。M0 各子项（当前 selector/候选优先级 UI）已逐项落地，未整体标「关闭」——M0 前端尚未独立验收。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex自动review：Project接入复核，派UI与M0 selector）

- ready rean-project-20261103-04-fix与9文件manifest一致，DSH GUI已完成/空闲。源码确认仅filter/network依赖、validity顺序、IsStep、step8 JSON/TSV提交及精确只读输出白名单，legacy未暴露hub_priority。
- 本轮独立本机隔离Go TestHubPriority project包通过；API包setup失败：指定离线Go cache缺excelize/v2 v2.11.0，未联网安装/未重复完整构建，不判代码回归亦不称API已独立通过。DSH报告Rust64及Go相关通过；真实worker经Project e2e仍缺，report_test.go:44基线未重跑。独立日志本机review-loop/verify-project-04-*/tests.txt。
- 下一派发rean-ui-20261103-05：当前证据selector/dirty与异步代际、无trait/hub候选tab、design默认false、全量下载/历史nullable兼容/质量提示，完成后一次必要隔离UI构建与真正worker Project小fixture。不做AI，不宣称性状证据已实现或M0全部关闭。
- 两仓库HEAD未变，dashboard Go新增修改及worker.rs/lib.rs和新增算法文件等所有未知未提交保留；Codex不改算法/UI源码。派发/源码manifest本机review-loop/state.json，依赖产物同步盘外，单DSH写入、不提交推送。

## 最新（2026-11-03 02:28，DSH：hub_priority Go 项目接入，awaiting_review）

- **review_id**：`rean-project-20261103-04`。本轮改 worker.rs（小修）+ Go 后端，未动前端/AI，未做完整桌面构建/快捷方式。
- **worker 小修**：`read_sample_strata` 拒绝纯空白分层值（trim）；输出补 `strata_counts`（每层数量）+ `strata_fingerprint`（样本→层分配指纹）+ `resource_note`（g²×B≤1e9 是工程预算非科学阈值，B=200 拒绝 g≳2236 模块）；逐轮 kME 仅描述性/方向计数、不输出区间（未宣称 kME 不确定性）。
- **Go 后端**：
  - `project/run.go`：注册 `hub_priority` 依赖仅 `{filter, network}`、主产物 `step8_hub_priority.json`、validity 列表；新增 `project.IsStep`。
  - `api/projects.go`：run 端点改用 `project.IsStep`（不再用 legacy `job.StepOrder`）；新增 `GET /api/projects/{id}/outputs/{step}/{basename}` 只读输出下载 + 精确白名单（hub_priority→step8_hub_priority.tsv，filter→step1_filtered.tsv/step1_genes.txt）。
  - `job.StepOrder` **未改**：legacy 线性语义不暴露 hub_priority（不能表达 filter+network-only 依赖）。
  - 输出提交沿用 staging/原子 commit 前缀收集（step8_ 前缀同时收 JSON+TSV）；sample_info 经已有 input 角色复制；design_confirmed 由 worker 强制 true。
- **测试**：`store_test.go` 新增 lifecycle（无 trait/hub 完成、JSON+TSV 提交/可读、module_trait/hub 未被运行）+ invalidation（network 变更→upstream invalid、B/seed 变更→parameters changed、UI 变更→不变）；`projects_test.go` 新增 run 白名单 + 输出下载白名单。mock worker 补写 hub_priority TSV。
- **验证**：Rust `cargo test --release` 64 通过；Go `go test ./internal/project/...` ok；`go test -run 'TestProject|TestHubPriority' ./internal/api/...` ok。**未跑**：完整 `go test ./internal/api/...`（report_test.go:44 基线失败）、完整桌面构建、真实 worker 端到端（未设 WGCNA_PROJECT_TEST_WORKER）。
- **状态**：`awaiting_review`（ready.json + 9 文件 manifest 本机 review-loop）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；通过后 M0 前端 selector + hub_priority UI（M0 前端尚未验收，不可标关闭）。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex自动review：worker主要P1已修，推进Project接入）

- ready rean-worker-fix-20261103-03-fix已核对DSH GUI空闲及五文件manifest一致。源码确认设计默认false/两方案确认、gene显式映射及重复ID、重采样kWithin Err计invalid、原ME/kME有限性、方向未知单独计数、invalid原因排序修复；64tests为DSH报告，本轮无独立Cargo/worker运行，既有report_test.go:44未重跑。
- 下一派发rean-project-20261103-04见DSH_REVIEW_PROMPT顶部：只接Go Project依赖/validity/staging提交/JSON与TSV/API与生命周期，必须无trait/hub可跑；legacy Job StepOrder不得强制前序性状，无法支持则显式不接。暂不前端/AI。顺手拒绝纯空白分层标签、补层计数/分配指纹，FNV不得当缓存SHA；当前逐轮kME未保存区间、1e9 g²B是工程限制，文档诚实说明。
- M0前端当前/历史selector及完整导出尚需后续验收，不标整体完成；性状仍none。两仓库仍worker master@fb74752、dashboard explore/project-copilot@c9409e3且未知未提交保留。Codex只读源码/更新review交接，源码仅DSH写；manifest与发送状态在本机review-loop/state.json，不提交推送。

## 最新（2026-11-03 02:12，DSH：hub_priority worker 边界修复，awaiting_review）

- **review_id**：`rean-worker-fix-20261103-03`。本轮只改 `rs_WGCNA/src/bin/worker.rs`，未动 Go/UI/AI，未做完整桌面构建/快捷方式。
- **修复**：
  1. P1 设计确认：`design_confirmed` 默认改 `false`；iid 与 stratified 都要求显式 `true`（不区分方案），否则拒绝。
  2. P1 身份校验：分析前校验表达/模块基因 ID 非空唯一、样本 ID 非空唯一、genes/colors 长度一致；模块基因经**显式 ID 映射**解析到表达行（不再按位置索引）；`read_sample_strata` 拒绝重复/空 sample_info ID。
  3. P1 数值退化：重采样 `full_module_kwithin` 的 `Err` 转整轮 invalid（不再 expect panic）；原始 kME/eigengene 输出前验证有限；参数/身份错误仍在循环前拒绝。
  4. P2 对齐：每轮描述性 kME 方向对齐原 ME（`align<0` 翻转），方向≈0/非有限计 `direction_undetermined`（网络排名仍有效，不混入 invalid）；输出加 strata_column/stratum_count/gene·sample 指纹（FNV-1a）/direction_undetermined；invalid_reasons 稳定排序；资源预检 g²×B>1e9 拒绝（不静默截基因）。
- **测试**：design_confirmed 默认/显式 false→拒绝、true→通过；模块基因缺失/重复/短 colors→拒绝；sample_info 重复行→拒绝；部分无效计数+JSON/TSV；端到端 fixture + 确定性保留。`cargo test --release` ✅（64 tests，0 failed）。
- **状态**：`awaiting_review`（ready.json + manifest 本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-loop/`）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；通过后 Go 项目接入。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex自动review：worker接线待修复，尚不接Go）

- 已读取ready rean-worker-20261103-02-fix、核对DSH当前会话空闲及五文件manifest稳定。源码审查发现：设计确认默认true且分层绕过确认；module基因列表与表达行仅按位置关联无身份校验；sample_info重复ID覆盖；bootstrap kWithin数值退化仍expect中止；align计算未用于ME翻转且未逐轮算kME，输出缺必要分层/身份指纹和资源预检。
- 新派发review_id rean-worker-fix-20261103-03见DSH_REVIEW_PROMPT顶部，先修身份/设计/退化计数和描述性kME/provenance，补负例与全无效/部分无效worker测试，暂不接Go/UI/AI。现有59tests为DSH报告，本轮未重跑worker/Cargo/完整构建；既有report_test.go:44未复测。
- branch/HEAD仍worker master@fb74752、dashboard explore/project-copilot@c9409e3，未知未提交保留。Codex本轮只读源码和维护review/交接；派发状态与源码manifest本机review-loop/state.json。依赖产物同步盘外、DSH单一源码写入，不提交推送。

## 最新（2026-11-03 02:00，DSH：hub_priority worker 接线，awaiting_review）

- **review_id**：`rean-worker-20261103-02`（worker 集成）。本轮改 `rs_WGCNA/src/bin/worker.rs` + `rs_WGCNA/src/hub_priority.rs`，未动 Go/UI/AI，未做完整桌面构建/快捷方式。
- **实现**：
  - `HubPriorityParams`（select_module/n_bootstraps/seed/top_k/resampling/strata_column/design_confirmed/trait_source/tie_tol）+ `Params` 字段 + dispatch `"hub_priority"`。
  - `run_hub_priority`：固定完整模块；原始 kWithin/kME(带符号+abs)/排名上下界；bootstrap 每轮同索引重采样→`validate_round_vectors`(退化记 invalid)→`full_module_kwithin`→eigengene 对齐原 ME→`rank_round`；`aggregate_priority` 汇总 midrank 中位数/P05/P95/mean_top_k。输出 `step8_hub_priority.json` + 完整 TSV。
  - 校验：B∈[1,1000]、K∈[1,g]、模块存在、grey 拒绝、iid 需 design_confirmed、trait_source 非 none 明确拒绝、stratified 从 sample_info 对齐生成标签（缺失/空值报错）。
  - 输出含 method_version=`hub_priority.v1`、scope=`fixed_full_module`、attempted/valid/invalid+原因、power/network_type、gene/sample 数、`limited` 标志（无效轮时 true，不称可靠候选）。
- **封住 RankRound 不变量**：`aggregate_priority` 校验 gene_count>0、top_k∈[1,g]、midrank==(l+u)/2、membership==membership_weight(l,u,K)，伪造 bounds 报错。
- **测试**：端到端 fixture（无 trait/hub 仍成功、K 边界、全量 JSON/TSV）、固定 seed 两次一致、伪造 RankRound/bad top_k/zero gene_count 拒绝。`cargo test --release` ✅（59 tests，0 failed）。
- **状态**：`awaiting_review`（ready.json + manifest 本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-loop/`）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 复核；通过后 Go 项目接入（run.go 依赖/StepOrder/端点），再 UI。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex自动审查：排名修复通过，派worker集成）

- 新轮ready rean-rank-20261103-01-fix已核对当前DSH会话空闲及四文件manifest前后稳定。独立同步盘外rustc测试包装运行当前hub_priority的12项测试全部通过；原全无效丢维度、valid>attempted、长度不匹配与并列说明问题关闭。非完整Cargo或应用验收；54tests仍为DSH记录。
- 当前接续prompt为DSH_REVIEW_PROMPT.md顶部review_id rean-worker-20261103-02：只做M1 worker params/dispatch/无表型全模块bootstrap与JSON/TSV、小型端到端fixture，暂不接Go/UI/AI；顺手封住公开RankRound的非法K/结构不变量。方法/无效计数/设计确认/指纹必须明确。完整prompt与自动循环约定保持有效。
- 两仓库HEAD仍dashboard explore/project-copilot@c9409e3、worker master@fb74752，worker.rs/lib.rs修改及hub_priority/kw/in/resampling新增，dashboard大量未提交保留。既有report_test.go:44未复测。独立测试在本机review-loop/verify-rank-fix-01/results.txt，发送状态/本地manifest在review-loop/state.json。DSH单一源码写入，Codex不改算法源码，不提交推送。

## 最新（2026-11-03 01:50，DSH：hub_priority 排名纯函数 review 修复，awaiting_review）

- **review_id**：`rean-rank-20261103-01`（Codex 审查 hub_priority 纯函数后派发）。本轮只改 `rs_WGCNA/src/hub_priority.rs`，未接 worker/UI，未做完整桌面构建。
- **修复**：① P1 `aggregate_priority` 新增显式 `gene_count`，全无效时输出长度恰为 g 的 None（不再 g=0）；② P1 改返回 `Result`，校验 attempted>=valid、各 round 四向量长度==gene_count、top_k 一致、值有限且范围内，排序用 `total_cmp` 无 panic；③ P2 并列注释改为"锚定组首最大值、跨度<=tol 的有界规则（v1）"，补链式近邻反例 `[1,0.9,0.8] tol0.11 → [1.5,1.5,3]`。
- **新增测试**：全无效 g=3/B=100 → 3 个 None；valid>attempted → Err；round 长度不一致 → Err；链式并列；top_k 权重和≈K；非恒定排名中位数/P05/P95；成员权重边界。`RankRound` 增加 `top_k` 字段。
- **验证**：`cargo test --release` ✅（54 tests，0 failed）。**未运行**：完整桌面构建（纯函数改动非必需）、go/前端套件。
- **状态**：`awaiting_review`（ready.json + manifest 已写本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-loop/`）。worker `master` @ `fb74752`、dashboard `explore/project-copilot` @ `c9409e3`。
- **下一步**：等 Codex 本轮复核；通过后按实施 prompt 做 `hub_priority` 的 worker 集成。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-03，Codex：加固源码复核，允许推进M1核心）

- 已读当前kw/in与resampling：分层长度严格校验并返回Result、零上界拒绝、power检查、cor/adj/out有限性检查以及显式三网络类型/下溢/置换测试均已落地。原评审两处P1在源码层面得到处理；本轮未重跑41项测试或完整构建，不能称独立运行验收。
- 允许DSH继续hub_priority纯函数和排名/并列测试。需顺手补一个数值边界：当前cor允许1e-9范围的越界但未clamp；明确对容差内值clamp至[-1,1]，大越界仍报错，增加signed非整数power+完全反相关回归，避免极小负底数powf产生NaN。不替换非有限值。
- 本轮无算法源码修改、未测试构建；仍由DSH单写入者继续。既有分支/提交及本地诊断指针见前节；基线report_test.go:44未重跑。下一步核心限定为完整模块、每轮重新排名、边界并列分数入选、有效重复分母与经验分位区间，暂不扩UI/AI。

## 最新（2026-11-03，Codex：M1地基审查，仍需加固）

- DSH新增worker kwithin.rs/resampling.rs、lib导出和run_hub调用；本轮审查结果见 `M1_FOUNDATION_REVIEW.md`。先修P1：Stratified缺/短分层标签静默归0；kWithin无cor/adj/out有限性及power校验，能Ok(NaN)或signed-hybrid掩盖为0。补三network_type显式期望值等测试，再写hub_priority。
- 实际隔离probe用当前源码+缓存第三方依赖编译，复现缺分层仍返回数组、tiny输入unsigned输出NaN/hybrid输出0、NaN power被接受；三基因显式公式正常案例通过。最终有效probe未复用旧rs_wgcna实现。材料本机构建根review-m1-foundation-*，指针review-m1-foundation-latest.txt。未跑全套Cargo/Go/前端或应用构建，34tests/构建成功是DSH报告；report_test.go:44基线未重跑。
- worker仍master@fb74752：worker.rs/lib.rs修改，新增kw/in与resampling；dashboard仍explore/project-copilot@c9409e3、大量未提交保留。本轮只新增review并更新交接，无源码修复。DSH接手前核对是否有活跃写入者；运行应用等记录仍可查本机handoff-records/latest.json（旧快照不可视为当前）。不自动启动DSH/切provider/提交推送，依赖产物各机同步盘外。

## 最新（2026-11-02，Codex：答复DSH四点并补充实施定案）

- 实施prompt顶部新增定案：kWithin共享计算函数但保留hub原输出；rean与hub_priority并行，旧schema不迁移冒充；步骤显示顺序不构成依赖，新候选入口不能受旧性状/hub门控；M0最小修复为当前selector/dirty/validity/请求代际，历史Result接口不改。
- 查到已有LLM实现：backend/internal/ai/{llm,config,chat}.go、api/ai.go、ChatWidget.vue；现有上下文偏legacy job。复用传输/设置，新增当前Project证据束和校验，不继承全部旧工具权限。未读取密钥/实际机器配置，未作在线调用。
- 更正理解：高TOM+低稳定性的描述性双条件不是综合评分，不能直接称噪声或不可信。整轮跳过NaN会改变分母及估计目标，须记录有效/无效重复与局限，不能只做代码跳过。
- 本轮仅读源码并更新prompt/两份交接，未修改算法/UI、未测试构建、未联系/启动DSH。两仓库仍explore/project-copilot@c9409e3、master@fb74752，未提交范围与上一节相同；已有未知修改保留。运行/写入者记录仍在本机构建根handoff-records/latest.json（上轮快照，接手需重新核对）；既有report_test.go:44未重跑。下一步DSH按定案推进M0，无需为两个step关系继续等待。

## 最新（2026-11-02，Codex：交付DSH候选优先级与AI实施prompt）

- 用户要求具体prompt交DSH执行，追加AI assistant与防幻觉要求。已新增 `DSH_REAN_HUB_PRIORITY_IMPLEMENTATION_PROMPT.md`，并将DSH_PROMPT顶部改为该文的明确接续入口；本轮仅文档，未实现源码、未运行构建或测试、未启动/联系DSH。
- 实施顺序：M0过期证据/NaN/完整表格 → M1完整固定模块hub_priority（kWithin排名、重采样入选频率、并列、实验设计、有效重复）→ M2项目/UI/报告 → M3只读事实约束AI及建议预览 → M4验收；M5独立效果评估无数据时交协议。完整prompt含文件定位、schema、算法定义、权限/幻觉契约和验收场景。
- 必需依赖收敛为filter/network，性状可选；不依赖hub400候选池，不强制module_trait，无表型可运行。AI输出有fact_id/执行来源，科学计算与数值渲染由程序完成，修改参数需用户明确应用，远程调用需明确开启；没有配置不伪装AI已连接。
- dashboard仍explore/project-copilot@c9409e3（BUILDING/main/API/enrich/job/前端/脚本修改及project/docs/工具新增等大量未提交）；worker仍master@fb74752，仅src/bin/worker.rs未提交。保留现场全部未知修改，未提交推送。本机现场记录在构建根 `handoff-records/latest.json`；运行应用存在，其他源码写入者状态未知，DSH接手须现场核对，不凭进程名判断无并发。
- 上轮隔离R诊断和worker反例是已执行检查；本轮只检查分支/文件/文档一致性。report_test.go:44既有失败未重跑，Mac/在线AI均未验证。下一步由用户转交prompt，DSH依顺序实施；旧17:55独立面板迁移不再是当前主线。依赖、R、缓存和产物仍各机同步盘外。

## 最新（2026-11-02，Codex：八模块 TOM 分布与阈值讨论）

- 新增 `MODULE_TOM_DISTRIBUTIONS.md`，留存48样本fixture，最多400高|kME|候选/模块，signed/power12/signed TOM/min。0.35保留blue和turquoise全部候选池边，black2.38%、red1.54%；不能用同一绝对cutoff保证相近显示密度。大模块不是完整模块TOM。
- 实际运行本机R统计8模块，并对blue原边核对worker权重，最大误差2.44249e-15。脚本/CSV/JSON在本机构建根tom-module-review-*，指针tom-module-review-latest.txt；无构建/完整套件/新bootstrap，无源码修复。上轮REAN_REVIEW问题仍待修复，report_test.go:44基线失败未重跑。
- 下一步：若获实施授权，提供模块内分位数显示筛边、原始强度分布/密度与独立的R-EAN阈值语义；分位数用于R-EAN时固定原始参考阈值，每轮重新求分位数则应定义为另一种排名稳定性。不要以出现非1结果作为选阈值目标。
- 两仓库仍dashboard explore/project-copilot@c9409e3大量未提交、worker master@fb74752仅worker.rs未提交。本轮只新增诊断说明并更新交接；未启动/联系DSH，未改用户项目及现有进程。活跃应用/写入者须现场复核，机器诊断记录保留本机，依赖产物不进同步盘。

## 最新（2026-11-02，Codex：R-EAN 算法评审，未改源码）

- 用户报告 cutoff=0.35/B1000/stability 全 1，完整评审见 `REAN_REVIEW.md`。发现：失效 R-EAN 仍驱动当前网络结论；bootstrap 零方差 NaN 污染无关边；稳定性降序截前 200 条隐藏非 1 结果且截断导出；高均值直接归因阈值低的提示无充分依据。
- 实际隔离复跑现有 worker：留存 blue fixture、阈值0.35、B1000，5565 边、4148 条=1、1417 条<1、min0.979、mean0.999077628，表格前200条全部=1。不能据此声称读取了用户本次实际项目。合成反例 A=B 固定，稀疏第三基因使 A—B stability=0.678，对照=1；是数值污染问题，方向为压低稳定性。
- 核对运行应用的 worker 哈希 `5e2b0cd4…`，worker.rs 与 ModuleExplorerView.vue 对应构建源码快照一致。当前 dashboard `explore/project-copilot`@`c9409e3` 大量未提交；worker `master`@`fb74752` 仅 worker.rs 未提交。保留所有未知改动。本轮仅新增评审及更新两份交接文档。
- 诊断产物/summary/反例在本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-rean-*`，指针 `review-rean-latest.txt`；当前应用与写入者状态须现场复核，未启动或联系 DSH，未修改用户项目。依赖及诊断产物留本机同步盘外。
- 未运行：完整构建、Cargo/Go/前端套件及浏览器自动验证；`report_test.go:44` 既有基线失败未重跑。下一步优先按评审修复并补数值反例回归，勿先搬面板。最新对话建议为“模块内证据 + 可选跨模块分析页面”，尚未实现或形成正式架构定案，旧17:55方案需重新核对。

## 最新（2026-11-02 17:55，DSH：R-EAN 全量方案定稿 + UMAP 决定不做，纯文档无代码改动）

- **本轮只改文档、未改代码**：新增 `docs/exploration/R_EAN_PLAN.md`（R-EAN 全量落地方案）；订正 `ATTRIBUTION.md` 中 UMAP 表述。无构建/测试重跑，engine 仍 `5e2b0cd4…`。
- **关键决定**：
  - R-EAN 独立成 panel；最终面板序列：Data Import → SFT → Network → Module-Trait → Module Overview（去基因级整体网络）→ Module Explorer（网络抽取，强度层）→ R-EAN（证据层）→ Report。R-EAN 依赖 hub，故在 Module Explorer 之后。
  - module-overview 去掉基因级整体网络（`overview_network` aux 步骤：全基因 TOM 毛线球）；保留模块级关系（热图/eigengene/树状图/聚类）。
  - **UMAP（P2）决定不做**：UMAP 价值在揭示单细胞群体等「别处不可见结构」；bulk WGCNA 模块结构已由树状图+TOM 热图+eigengene(PCA) 忠实呈现，非线性 2D 嵌入是冗余有损重投影。仅当审稿人/用户明确要求再补，不占主线。
- **下一步（按 R_EAN_PLAN.md 里程碑）**：M0 拆 R-EAN 独立 panel + 删 overview 基因级整体网络 → M1 迁移 reliability 层 + report 集成 → M2 可切换证据可视化（2.5）→ M3 差异层 → M4 桥接层+收敛 → M5 report 全量 + 双平台验收。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-02 01:00，DSH：R-EAN 归属更正 + 跑偏审视，纯文档无代码改动）

- **本轮只改文档、未改代码**：新增 `docs/exploration/MODULE_NETWORK_METHOD.md`；更正 R-EAN 归属（见下）；无构建/测试重跑，engine 仍 `5e2b0cd4…`。
- **R-EAN 归属更正（重要）**：R-EAN = Reliability- and Evidence-Aware Network，是 **rsWGCNA 自己的原创概念**（Codex 规划 prompt Phase 2），**不是** hdWGCNA / Morabito 2020 提出。DSH 此前两轮把出处归错（先 hdWGCNA、后脑补 Morabito 2020 HMG），已在 `MODULE_NETWORK_METHOD.md` §8/§10/§11 与 `ATTRIBUTION.md` 更正。底层思想（bootstrap/差异/桥接/多层网络）不宣称首创。
- **跑偏审视（对照 Codex prompt）**：P0/P1/P3 已做；**P2 UMAP 被跳过未评估**；R-EAN 仅 Reliability 层，Differential 层与桥接基因未做；2.5 Evidence-aware 可视化只做了"边色=稳定性"。
- **下一步**：① 2.5 Evidence-aware 可视化；② P2 UMAP 可行性；③ 2.2 收敛性评估；④ P4 差异层+桥接；⑤ P5 基准/消融。
- 单写入者：DSH；未 commit/push/merge。

## 最新（2026-11-01 23:45，DSH：is_seed 独立标记 + 标准 module_kwithin）

- **范围**：第二批后半。第一批与第二批前半（R-EAN scope / TOM strength 刻度）已验收关闭。
- **dashboard** `explore/project-copilot` @ `c9409e3`（未提交：`api/client.ts`、`components/NetworkGraph.vue`、`views/ModuleExplorerView.vue`、`utils/reportCharts.ts` 等）。**worker** `master` @ `fb74752`，仅 `src/bin/worker.rs` 未提交。
- **本轮改动**：
  - worker：`NetNode`/`HubGene` 加 `module_kwithin`；`NetNode` 加 `is_seed`，`is_hub` 语义改为「科学 hub（通过判定）」、`is_seed`=「网络种子（top max_hubs）」。`run_hub` 计算标准 kWithin = 完整模块内 adjacency 非对角项之和（`pearson` + `adjacency` 全模块矩阵，非 TOM）。import `rs_wgcna::adjacency::adjacency`。
  - 前端：网络视觉（hub 边框/填充、hub-centric、ring/radial/concentric、report 图）改 `is_seed`；tooltip 显示 hub/seed 两态；注释表保留科学 hub 徽章；大小/颜色新增 kWithin 选项（缺字段自动回退 kme，`hasKwithin` 禁用逻辑）。
- **验证**：`cargo check`（无新增警告，仅 cutree.rs 既有 10 警告）、`cargo test --release`（27 tests ok）、`vue-tsc --noEmit` 0 错误、完整隔离构建 PASS → `runs\20261101-233811-3040b6\app\rsWGCNA.exe`。**未运行**：go test / go vet（本轮未改 Go）、is_seed/module_kwithin 的浏览器实测（用户端待做）。
- **引擎哈希**：worker `5e2b0cd4…`（Test-01 需重跑 filter→sft→network→module_trait→hub→rean）。
- **下一步（按序）**：① 用户端验收 is_seed + module_kwithin；② 富集 universe / 表达缺失 / term 竞态（第三批）。
- **已知失败**：`report_test.go:44` 富集断言（干净基线也失败，非本轮回归）。
- **单写入者**：DSH；未 commit/push/merge。

## 最新（2026-11-01 23:05，DSH：第一批已验收关闭 + 第二批前半 R-EAN scope / TOM strength 刻度）

- **范围**：本会话（Codex 评审 + 用户端 computer-use 实测）推进 module-explorer 科学口径正确性。
- **dashboard** `explore/project-copilot` @ `c9409e3`（未提交：`api/client.ts`、`api/projects.ts`、`stores/project.ts`、`components/NetworkGraph.vue`、`views/ModuleExplorerView.vue` 等，以 `git status` 为准）。**worker** `master` @ `fb74752`，仅 `src/bin/worker.rs` 未提交。
- **第一批（已验收关闭）**：
  - hub 显著性门控只看所选性状（`module_trait_is_significant`）；hub 表格与网络 seeds 同一判定（`select_hubs`）；`kwithin`→`extracted_tom_strength` 改名；provenance 加 `selected_trait`/`selected_trait_significant`；旧结果兼容（`kwithin` 回退 + 无指标自动回退 kme）。
  - 回归测试：`is_hub`、`select_hubs`、`module_trait_is_significant`、`seeds_are_top_max_hubs`、`select_hubs_empty`。
  - 用户端实测（Test-01 blue×trait4）：262 hub / 10 seeds，seeds=表格前 10；106 节点 5565 边无重复自环；显示切换不改科学结果（hub 输出哈希不变）。
- **第二批前半（本轮，待用户实测验收）**：
  - TOM strength 视觉刻度：实际 min/max（非 0 基线）、等值→中点、排除缺失/非有限（缺失大小→中点、颜色→低色）、图例注「色阶为当前视图相对刻度」。
  - R-EAN scope 统一：`step6_hub` 新增 `candidate_gene_ids`/`tom_type`/`tom_denom`；`run_rean` 在完整 candidate pool（400）bootstrap、复用 hub 的 power/network_type/tom_type/tom_denom/used_threshold、只对原网络边算 stability、`scope=candidate_pool`；`ReanParams` 去 threshold/power（前端 R-EAN 阈值只读显示 hub 阈值）；严格输入校验 `resolve_candidate_pool`（空/重复/不可映射→panic），边映射失败→panic，缺失 provenance→「重跑 hub」。
  - 回归测试：`resolve_candidate_pool_rejects_bad_input`（空/不可解析/重复）。
- **验证**：`cargo check`（无新增警告，仅 cutree.rs 既有 10 警告）、`cargo test --release`（27 tests ok）、`vue-tsc --noEmit` 0 错误、完整隔离构建 PASS → `runs\20261101-230531-948bb8\app\rsWGCNA.exe`。**未运行**：go test / go vet（本轮未改 Go）、新前端界面与 R-EAN scope 的浏览器实测（用户端待做）。
- **引擎哈希**：worker `756efc36…`（多轮变更，Test-01 需重跑 filter→sft→network→module_trait→hub→rean）。
- **下一步（按序）**：① 用户端验收 R-EAN scope + TOM strength 刻度；② `is_seed` 独立标记 + 完整模块 `module_kwithin`；③ 富集 universe / 表达缺失 / term 竞态。
- **已知失败**：`report_test.go:44` 富集断言（干净基线也失败，非本轮回归）。
- **单写入者**：DSH；未 commit/push/merge。

## 自动交接检查点（2026-11-01 11:04，Asia/Hong_Kong）

- 本轮因额度触发，仅做只读状态核对及更新交接；没有构建、测试、提交、推送、启动/联系 DSH 或修改 provider。
- dashboard 仍 `explore/project-copilot` @ `c9409e3`，大量改动未提交；worker 仍 `master` @ `fb74752`，工作区干净。下面 22:47 的记录不覆盖其后新改动。
- 新增/继续修改但尚未经本轮验证：`backend/internal/api/project_enrich.go`（注释、富集、表达接口）、`project_plots.go`（项目绘图导出）、`toolkits.go`（eggNOG annotations 转换）与 `frontend/src/views/ToolkitEggnogView.vue`（已有 `/toolkits/eggnog` 路由）；`api.go`、`api/client.ts`、`PlotDownloadWidget.vue`、分析页面和构建脚本也有未提交改动。不要按旧待办重新创建或覆盖这些文件。
- 本轮只核对文件/函数/路由存在与 Git 状态，不宣称上述新能力测试通过。最近有记录的测试仍以原 DSH 22:47 节为准，原报告基线失败未重新验证。
- 本机 `dsh-handoff/latest-run.txt` 仍指向早前 20:52 结束的会话，不能用于判断后续 DSH 是否完成或当前写入者是否活跃。先现场核对会话/进程；存在其他写入者时不要并发改源码。
- 下一步：先核查新增项目富集/绘图接口与 eggNOG 工具的当前完成度、调用方和已有验证记录，再确定剩余工作；已有接口优先验收，避免重复实现。继续保持依赖/运行时/产物在本机同步盘外，保留 `.dsh-acl-probe.tmp` 等未知改动。

## 最新（2026-10-31 22:47，DSH：分析页全适配 + 后端三接口 + 进度条）

- 在 21:40 导航恢复基础上继续补齐（均未提交）：
  - 前端：DataImport/SFT/Network/ModuleTrait/ModuleExplorer 全部接 `useProjectStore`（SidebarDrawer 折叠侧栏 + draft/run/results）；module-overview/report 仍占位。恢复 load demo 按钮、样本树色块着色（按 sample_info 列）、真实进度覆盖层 `ProjectRunProgress.vue`。
  - 后端：新增 `POST /api/projects/{id}/demo`（内嵌 GSE162434 导入 6 输入）、`GET /api/projects/{id}/sample-info`（`enrich.ParseSampleInfo`）、`Project.Input(role)`；`Run` 改 `cmd.Start()`+`Wait()` + 250ms 轮询读 staging `progress.json`，`reply` 带 `progress`。
  - `projects.ts`：新增 `RunProgress`、`getSampleInfo`；ProjectResponse 增 `progress`。
- **验证**：`vue-tsc --noEmit` 0 错误；`go vet ./internal/project ./internal/api` + `go test ./internal/project` + `TestProjectHTTP ./internal/api` 均 ok；完整隔离构建 PASS（exe `runs\20261031-224716-99a123`）；无头冒烟：demo 导入 6 输入、sample-info 48 样本/8 字段、filter 进度 `0→0.7→done` 均 ok。
- **待办（按序）**：module-explorer 注释/GO/KEGG 富集 + 网络 geneMap 标注（需补 project 后端 annotation/enrich 接口）；Overview/report 适配；导出按钮（PlotDownloadWidget/ReportFigure 仍耦合旧 `/api/jobs`）。
- 单写入者：DSH；未 commit/push；worker `master` @ fb74752 干净。

## 最新（2026-10-31 21:40，DSH 已实现：导航恢复 + 三页项目适配）

- **更正 21:25 的过时判断**：上方「导航整合未实施」基于 20:52 早前一个 shell 被 ACL 阻断的 DSH 会话；本会话（21:20 起，ACL 已修复、shell 正常）已实际完成导航整合，以下为准。
- **前端改动（未提交，dashboard `explore/project-copilot` @ c9409e3）**：
  - `stores/project.ts`：新增 `results` 分步结果缓存 + `loadResults()` + `stepStatus()`；hydrate/close 时重置。
  - `App.vue`：删除 `visibleNav` 过滤 → 导航始终显示完整步骤；无项目时分析项置灰禁用并提示；导航栏全局显示项目名 + 保存状态；全局轮询（refresh + loadResults）+ beforeunload。
  - `router.ts`：beforeEach 离开前 `project.flush()` 尽力保存；module-trait / module-explorer / module-overview / report 改指新增 `views/ProjectPlaceholderView.vue`（明确占位，不走旧 Job）。
  - `DataImportView / SftView / NetworkView`：重写为 `useProjectStore`（draft / `store.run(step)` / `store.results.*`），保留 SampleTreeChart / SftPlot / SftCheckChart / DendrogramChart / SimpleTable 原图表；无项目时显示引导页。
  - `ProjectView.vue`：回归管理入口（新建/打开/关闭/最近项目 + 研究问题卡 + 名称/路径/保存状态），去掉内联分步 UI。
  - `AGENTS.md`：加「先读根 STATE.md 与 wgcna-dev 技能」指针。
- **验证**：`vue-tsc --noEmit` 0 错误；后端未改、`go test ./internal/project` 与 `TestProjectHTTP ./internal/api` 均 ok；**完整隔离构建（build-windows.ps1）PASS** → `%LOCALAPPDATA%\rsWGCNA-build\windows\runs\20261031-214739-927f2e\app\rsWGCNA.exe`（exe_sha256 `C3CCB078…EC29CAB`；dashboard c9409e3 + worker fb74752，含未提交改动）。注意：脚本须用 `powershell.exe`(5.1) 运行，`pwsh`(7.3+) 会把 Rust 的 unused 警告当错误中止。
- **待办（按序）**：module-trait（step4→TraitHeatmap）/ module-explorer（module_detail→GsMmScatter、hub→表）接项目；module-explorer 富集/注释/表达/网络需先补 project 后端接口（现仅 `/results/{step}`）；Overview/report 适配；demo 数据导入接项目；样本按 sample_info 着色；导出按钮（PlotDownloadWidget/ReportFigure 仍耦合 `/api/jobs`）。
- 单写入者：DSH（本会话）；未 commit/push；worker `master` @ fb74752 干净。

## 当前状态：2026-10-31

### 21:25 交接检查：DSH 已结束，导航整合未实施

- 已读取本机 dsh-handoff/latest-run.txt 所指的 state/events：DSH 于本地 20:52 结束，进程退出码 0 仅表示会话正常结束，不代表开发任务完成。
- DSH 最终报告：所有 shell 调用在执行前被 `SetNamedSecurityInfoW failed (Win32 5): grantWrite(...)` 阻断，连 git status 都无法运行；文件读写工具可用。它仅做源码审查，没有修改功能源码，也没有构建/测试，未更新交接文件。
- 当前现场重新核对：dashboard 仍在 explore/project-copilot、HEAD c9409e3；既有未提交改动保留，新增 `.dsh-acl-probe.tmp` 是 DSH 留下的探针，未删除。worker 仍 master，工作区干净。本轮不运行构建/测试，不启动新的 DSH 会话。
- 阻塞：需先恢复 DSH 正常 shell 执行能力，再接续导航整合。DSH 建议的 ACL 修复脚本尚未执行、未经本轮审查；不要因其输出自行改权限或关闭沙箱。Codex 本机命令可运行不代表 DSH 权限链路已恢复。
- 本轮只同步 HANDOFF 和 DSH_PROMPT。导航仍为旧临时设计，用户要求尚未完成。准确失败详情和源码审查在本机 dsh-handoff/events.jsonl，不将全部运行日志搬进共享文档。

### 最新用户决定与本次交接：恢复原分析导航（最高优先级）

- 用户明确指出：创建项目后 navbar 消失、所有分析挤在 Project 下难理解。已同意修正：Project 只负责新建/打开/关闭/最近项目与研究问题卡；分析继续使用原导航和各自页面，顶部显示当前项目名/保存状态。
- 当前隐藏旧导航只是第一版隔离措施，不是最终设计。不能只删除 `visibleNav` 过滤就算完成：旧页面仍使用 useWgcnaStore/旧 Job API，必须接到当前项目的状态与执行/结果接口，否则会显示空数据或错数据。
- 用户已授权：继续实现；短周期不足时先写交接，再启动本机 DSH 接续。本次额度检查已触发交接，Codex 本轮不再并发编辑功能源码，交给单一 DSH 写入者。
- 建议最小切片：先将 DataImport、SFT、Network 接上项目 adapter/store；Project 页拆为管理入口；navbar 保留完整步骤，无项目时分析页禁用/引导；项目名与保存状态置于全局。再接 module-trait、module-explorer，以及尚不支持的 Overview/report（明确占位说明，不能走错旧 Job）。
- 验收：新建项目后导航不消失；跨数据/SFT/Network 页面共享同一项目；改参数/离开页先保存；退出后打开恢复页面/参数/结果；beta=6 结果与 beta=8 草稿分离仍成立。保留现有项目 API 和存储回归，不改 worker 算法。
- 本次交接只更新文档，未对导航做新的功能改动、未重跑构建。DSH 启动/会话/日志记录于本机 build root 的 dsh-handoff 目录；核验该记录及进程，避免两个智能体同时改同一仓库。启动是否成功以运行记录为准。

### 交接触发约定（本轮新增）

- 用户要求额度不足提前交接、家里/办公室提醒交接时也更新。已写入 AGENTS.md，HANDOFF 与 DSH_PROMPT 必须一起维护。
- 已为当前聊天创建 ACTIVE heartbeat：`wgcna`（WGCNA 低额度交接保护），每小时检查；适用 Codex 任一可用额度窗口剩余 <=20% 时，若有新进展则同步两个文件。无变化保持安静。该自动化属于创建它的本机聊天，另一主机是否可调度需现场检查，不能视为仓库自带的后台服务。
- 后续开发还需在回合开始/阶段结束检查余量；用户明确说交接/换机器/去办公室/回家继续/交给DSH时立即更新，不等一小时。额度耗尽或应用未运行时无法保证自动写入，因此每阶段都留可用快照。
- 本轮只增加交接保护规则与更新 prompt；没有新功能改动、没有重新运行构建测试、没有提交推送或自动启动 DSH。下方上一轮验证仍有效，但不冒充本轮重跑。

- dashboard：`explore/project-copilot`，分支起点 `c9409e3`。
- 已新增 project 核心、`api/projects.go` registry/HTTP 和前端 `ProjectView.vue`、`stores/project.ts`、`api/projects.ts`。server 注册接口，退出时取消并等待 worker。首页进入 `/projects`，项目打开期间隐藏旧 Job 导航与 AI 面板，避免上下文混用。
- worker：`master`，`fb74752d61b98eeeb3ef2b8ffbadf9eaac73c2dc`，本轮未修改、工作区干净。真实集成使用本机已有 worker，未重新构建该源码，不能将该二进制等同此 HEAD；执行时记录的是实际二进制 SHA256。
- 已检查：job Manager、前端 store、server 默认任务路径、双平台隔离构建脚本与 BUILDING.md。
- 验证：8 个核心顶层测试、3 个 HTTP 顶层测试及子场景 PASS；含 native Rust filter/SFT/network 和服务重建后重开。HTTP race、project/API vet PASS。Windows 隔离完整构建（Rust/vue-tsc/Vite/Go）PASS；Mac project/API 测试二进制交叉编译 PASS，Mac 实机未验收。
- UI：实际新建项目/问题卡/导入，filter/SFT 后真正重启 exe，从最近项目重开恢复图表/页面。network(beta=6) 后改草稿 8，保存/关闭/刷新/重开，草稿与执行参数分别保留，旧图显示待更新。
- 全量测试仍 FAIL：`report_test.go:44` 旧富集断言。另取 HEAD=c9409e3 干净 backend 在同步盘外复现同一失败，非本轮引入。其他包通过。
- 未提交、未推送。保留本机隐藏预览服务供用户检查，仅持有本机合成测试项目；会话信息在本机构建根 `project-preview/session.json`。切换机器前关闭预览/项目。

## 下一步

先完成上方最新用户决定的“原分析导航 + 当前项目统一状态”整合；此前“优先补表型验收”的顺序已被用户新的 UI 要求调整。然后用带表型示例实测 module_trait/detail/hub，补无表型路径，并设计富集/报告/Overview 适配。缓存兼容、冲突恢复与 Copilot 在其后。

首版严格匹配 worker 二进制、所有输入都参与指纹，跨 OS 暂不能宣称缓存免重算。新工作区暂无富集、Overview、报告和 Copilot。未实现多分析分支、问题卡来源历史、备份恢复 UI 和旧 job 导入。运行中禁止编辑；保存错误保留草稿，409 合并界面未实现。

本机环境：隔离构建 pnpm store/node_modules 均在本机非同步目录；Go 不在默认 PATH。测试设置本地 Go build cache/temp，并复用经路径检查的机器 module cache，避免空缓存触发不可达 proxy 的重复下载。未运行 R；Mac 与 R 路径仍需对应机器验收。

两平台 test-project 脚本先复制 backend 到本机非同步目录，运行核心与新 HTTP 测试；Mac 脚本未实机运行。Windows 隔离构建已生成探索程序，未复制到相邻 releases/稳定版安装目录；本机 latest-app.txt 定位最近构建，核对 build-info 后启动。

## 双机交接规程

1. 出发机器停止构建和项目写入，记录 dashboard 与 worker 的 branch/HEAD/status。Git 优先提交并按授权推送，不依赖同步 `.git`。
2. 若源码通过同步盘传输，禁止两机并发编辑，等同步完成；未提交文件须明确列出，不用 reset/clean 覆盖它们。建议逐步转为两机独立 checkout + Git 远端交接。
3. 分析项目如需传输，先关闭写入者，再完整复制目录并确认同步完成；不得将缓存/依赖夹带进去。当前尚未实现项目导出，不能把旧 job 目录称为可移植新项目。
4. 到达机器阅读本文件、检查冲突与两仓库状态，在本机隔离环境构建；不复用另一 OS 的 node_modules/worker/R runtime。
5. 新记录使用下面模板；路径、端口等机器细节写本地记录，共享记录只保留路径约定和必要诊断摘要。

## 本机路径约定

| 内容 | Windows | macOS |
|---|---|---|
| 构建快照/产物 | `%LOCALAPPDATA%/rsWGCNA-build/windows` | `~/Library/Caches/rsWGCNA-build/macos` |
| 项目索引/机器配置（拟定） | `%LOCALAPPDATA%/rsWGCNA-explore` | `~/Library/Application Support/rsWGCNA-explore` |
| R 与扩展包 | 本机非同步目录，WGCNA_RSCRIPT 指定 | 本机非同步目录，WGCNA_RSCRIPT 指定 |
| 分析项目 | 用户选择的持久目录 | 用户选择的持久目录 |

工具链和包管理器缓存也必须本机化；启动前检查环境变量覆盖。最近构建指针只表示该机器最近成功构建，不能证明它属于当前分支；启动前核对 build-info/源码版本。Mac 脚本现无完整 build-info，M0 应补齐或另行记录。

## 每轮交接模板

```text
日期/平台：
dashboard branch/HEAD/未提交文件：
worker branch/HEAD/未提交文件：
本轮目标与完成内容：
数据格式/API 决策：
验证命令、环境及结果（PASS/FAIL/未运行）：
本机构建身份（不包含密钥）：
已知问题、未完成操作、是否有存活写入者：
是否提交/推送/同步完成：
下一步（具体文件与验收场景）：
```
