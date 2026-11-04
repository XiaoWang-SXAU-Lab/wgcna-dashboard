# rsWGCNA 接续状态 — 2026-11-05 上午（办公室 Windows，已推送功能提交）

当前唯一写入者：办公室 Windows。家里 Mac 已停止写入；wgcna 自动监控保持 PAUSED（用户未要求恢复）。网络已恢复：已 fetch dashboard/worker；worker `explore/project-copilot@10ae0be` = origin，工作树干净，`master` 落后 origin/master 1。dashboard 功能提交 **`3fe2b53`** 已推送 `origin/explore/project-copilot`；本 STATE 与 HANDOFF/DSH_PROMPT 为随后收尾提交。未合并 main。

**2026-11-05 新改动**：
- Data Import `sample_percentage` 改为 0–1 range slider（step 0.01）；旧项目 `50` 自动迁移 `0.5`；legacy `wgcna.ts` 去掉 `/100`；`project.Run` 增加 [0,1] 守卫与回归测试。
- Candidate priority 表（plain summary + technical details）加入 `Symbol/name`、`Description`；按当前 priority 模块加载 annotation，表格搜索/CSV 含注释；raw TSV 标明无注释。
- 工程预算 UX：`g²×B > 1e9` 时禁用 Run、显示最大 B、提供 Set B=100；Test-01 blue 2,405×200 被拒绝，B=100 通过（valid 100/100）。
- README.md / README.zh-CN.md / NEWS.md 更新（NEWS `[Unreleased] — 2026-11-05`）。
- 验证：`vue-tsc -b`、`go test ./...`（CGO_ENABLED=1）、`go vet`、Windows 隔离构建均 PASS；Codex 本轮未重跑。

**Test-01 本地观察**（分析项目不在仓库）：blue candidate priority 的稳定核心是 NFXL1/PDC2/HSP20/HSP101/DNAJ/NAC13 等 early heat-response 基因；108 个 memory 基因中最好为 CUL2（第 21）/DREB2C（第 31），大多数靠后。全 blue 的 candidate priority 不是 memory 选择器；下一步应建 108/156 memory-focused 网络。

仓库：dashboard `explore/project-copilot`；worker `explore/project-copilot@10ae0be`（= origin，工作树干净）。`.dsh-acl-probe.tmp` 仍未跟踪、未提交；未覆盖未知本地改动。

本轮完成：Codex CLI 独立复核（隔离 `CODEX_HOME`，7 轮）最终 **PASS with residual risks**（round `rean-evidence-p2-20261104-21`）。evidence 包：
- P1 #6：gene/module 首尾空白/换行不再经 trim 校验后进入 facts/渲染。
- P2：`sum(mean_top_k)=top_k`；kWithin→rank 稳定降序/bounded-span tie 重算；`mean_top_k=0/1` vs P05/P95；valid=1 可实现性；valid=2 分位数对称、median 总和 `n(n+1)/2`、反解 midrank ∈[1,n]；生产 fingerprint/resource_note 入事实；`ContractError` 安全错误码/文案；32 MiB + 100k gene 行预算；严格未知字段；M3 序列化 bundle 不可信注释。
- 新增 workspace skill：`.claude/skills/codex-code-review/SKILL.md`（本机 Codex 复核流程，不在两仓库 git 内）。
- 报告基线：修复 `report_test.go:44`；`overviewTabGroupHTML` 对 `module_overview_*` / `cluster_overview_*` 恢复 `Assets` 检查，缺图显示说明而非破图；测试改为验证交互式模块富集。
- 真实 worker e2e（非同步盘）：`rs_WGCNA@10ae0be` 源码快照构建 worker；demo `black` 模块 `hub_priority`（B=20、K=5）valid 20/20、184 基因、1864 facts、`sum(mean_top_k)=5`；真实 envelope `BuildBundle` 接受，`Evaluate` 渲染候选 `AT1G02910`。
- M3 最小切片：`OfflineSuggestion` 确定性离线建议；`GET /api/projects/{id}/evidence/hub-priority` 从服务器当前原子 envelope 重建 bundle，not-current 固定安全文案，`full=1` 才返回 facts/candidates；报告页“离线证据解读”卡片（仅当前执行、迟到响应丢弃、只显示安全 code/message、无在线模型/自由文本/自动执行）。

验证：evidence **51/51** 顶层（90 子测试，0 失败）；API **12/12**（2 个 env-gated skip）；go vet、`go build ./internal/...`、`go test ./...`（CGO_ENABLED=1）全通过（报告基线已修）；真实 worker e2e 与真实 envelope API 测试通过；前端 `vue-tsc` 通过（vitest 本机无依赖/无网络未跑）。Codex evidence 复核 PASS with residual risks；Codex M3 首轮 FAIL（错误路径泄露）→ 已修 → round `rean-m3-evidence-20261104-02` **PASS with residual risks**；Codex 快照内 HTTP 测试受 Windows 权限阻塞，本机真实工作区通过。

残余风险/未跑：valid≥3 复杂可实现性、完整桌面/GUI/Mac 可视验收、在线 LLM/API/UI、报告 HTML/PDF、候选理由持久化、Codex 快照内 HTTP 集成。

下一步：手动 GUI/浏览器验收离线解读面板；决定是否接在线模型 adapter；候选理由持久化；完整桌面/真实 worker 全流程 e2e。仍必须从当前原子 `CandidateReportEnvelope` 重建，持久化建议展示前复验，UI 只展示 `ContractError.Code/Message`，不展示 `Diagnostic()`。Codex 产物在 `%LOCALAPPDATA%\rsWGCNA-build\windows\review-20261104*` 与 `review-loop\ready.json`（非同步盘）。提交/推送需用户明确授权。
