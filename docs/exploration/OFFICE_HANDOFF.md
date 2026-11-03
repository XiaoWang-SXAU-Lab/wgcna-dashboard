# 办公室接续检查点 — 2026-11-03

本检查点是开发快照，不是已完成/已发布版本。用户已明确授权此次测试、GitHub提交与跨机器交接；没有合并主分支、发布或切换provider。

## 从哪里继续

两仓库都使用 `explore/project-copilot`：

| 仓库 | GitHub | 接续版本 |
|---|---|---|
| dashboard | https://github.com/XiaoWang-SXAU-Lab/wgcna-dashboard | 包含本文件的分支最新提交；拉取后以 `git rev-parse HEAD` 与 `origin/explore/project-copilot` 相等为准 |
| worker | https://github.com/XiaoWang-SXAU-Lab/rs-wgcna | `10ae0be`，固定完整模块候选优先级引擎 |

本机旧基线为 dashboard `c9409e3` / worker `fb74752`。远端主分支已分别推进到 `238170c` / `4012702`，此次没有合并它们；办公室先复现本探索分支，后续另行评估主线整合。worker原工作树master上的改动已提交到新探索分支，不推master。

办公室若已有本地修改，先检查并保留，禁止直接reset/覆盖；推荐单独clone到新源码目录。新clone的可复制命令（不假设办公室绝对路径）：

```sh
git clone --branch explore/project-copilot https://github.com/XiaoWang-SXAU-Lab/wgcna-dashboard.git wgcna-dashboard
git clone --branch explore/project-copilot https://github.com/XiaoWang-SXAU-Lab/rs-wgcna.git rs_WGCNA
```

已有干净clone可各自 `git fetch origin`、切探索分支并 `git pull --ff-only`。确认两仓库HEAD和分支，再开始写入。`wgcna-dashboard/STATE.md` 是这次纳入Git的状态入口；工作区根STATE原来在两仓库外，不能假设办公室通过Git拿得到。

## 实际完成与验证

- 固定完整模块 `hub_priority`：完整模块adjacency kWithin排名、确定性iid/stratified重采样、并列分数top-K、有效/无效轮统计、step8 JSON/TSV。依赖仅filter/network，无性状/hub也可运行。
- Project持久化、运行/validity、候选tab、全量导出；store/execution异步守卫；报告envelope锁内一致读取与来源、报告状态修复。
- M3仅离线 `backend/internal/evidence` 的builder/结构化建议校验/确定性渲染。第14轮已恢复收尾并写新ready；此前“DSH额度中断”是历史状态，不再代表代码尚未收尾。仍未接API/UI/在线LLM。
- Codex本次独立在机器本地新快照执行：Go `./internal/evidence ./internal/project`通过；API `-run 'TestProject|TestHubPriority|TestCandidateReport'`通过；前端 `vue-tsc --noEmit -p tsconfig.json`通过；实际store/composable vitest **13/13通过**。evidence现有**18项测试通过**。
- **额外两个定向反例失败**，因此14尚未最终验收，不能称防幻觉边界闭合。见下节。
- 本次未重跑完整Rust套件、完整桌面构建、实际GUI点击、Mac或真实worker端到端。此前DSH报告Rust64、真实worker Project e2e和隔离构建通过是历史验证，不混作本次重跑。
- 已知 `report_test.go:44` 富集断言基线失败本次未重跑；webview完整构建需对应CGO/平台环境，不用普通 `go build ./...` 代替桌面构建流程。

## 办公室第一优先：两个未修复问题

1. `evidence.go` 的Envelope.Validity是普通string，BuildBundle只检查 `env.Validity != ""`。缺失/JSON null会成为空串并接受。现有 `TestMissingValidityRejected` 同时传入gene_count=0，失败来自其他无效字段，属于假阳性。应使用其余字段全部合法的fixture，只删除validity字段来测；显式presence/类型校验，缺失/null/stale都拒绝，空字符串有效值正常通过。
2. `Evaluate`虽Decode→Validate→Render，`RenderSuggestion`仍为公开方法且不校验；直接传入 `FORGED_GENE` 可输出候选，绕过约束。应封闭公开渲染入口（例如内部私有renderer + 仅Evaluate暴露验证结果），补公开调用路径反例，不能只写“调用者保证已验证”。

独立反例（复用包内现有测试helper）：

```go
func TestHandoffMissingValidityOtherwiseValid(t *testing.T) {
    raw := envelopeJSON("p1", "e1", 200, 0, 0, true,
        gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
    var v map[string]any
    json.Unmarshal(raw, &v)
    delete(v, "validity")
    b, _ := json.Marshal(v)
    if _, err := BuildBundle(b); err == nil { t.Fatal("missing validity accepted") }
}
// 当前公开API可直接调用：
// bundle.RenderSuggestion(&Suggestion{GeneIDs: []string{"FORGED_GENE"}})
// 实测输出含FORGED_GENE；期望该公开绕过路径不可用或返回错误。
```

这两个反例只写在本地诊断快照中，未改算法源码/正式测试以制造通过。办公室先加真正失败的生产路径测试再修；之后复核第14轮其余来源、设计、完整性、候选fact关联与严格JSON契约，再接M3 adapter/UI。不要直接执行旧prompt中的“下一步在线AI”。

## 剩余范围与科学边界

没有在线模型实测、API/UI助手或应用建议执行；没有报告HTML/PDF、候选选择理由持久化、GUI/Mac验收。尚不能标M0-M4全部关闭。候选优先级不是“真正hub”/因果/显著性，top-K频率不是正确概率；无性状路径没有GS证据；invalid结果需条件化解释。局部绘图不改变科学排名。

## 机器隔离、写入者和自动任务

- 家里自动任务 `wgcna` 已**PAUSED**，不再自动给DSH派发。办公室现场确认单一写入者后，明确要求恢复监控再启用；本机调度不会随Git自动迁移到另一台机器。
- 不结束用户应用/项目进程；DSH最新轮已声明awaiting_review，本轮只测试/文档/Git，没有向DSH派新任务。切办公室后家里不再改共享源码。
- Windows本地根 `%LOCALAPPDATA%/rsWGCNA-build/windows`；Mac `~/Library/Caches/rsWGCNA-build/macos`。Node/pnpm/Cargo/Go/R环境、缓存、node_modules、target、编译产物、进程配置、API密钥全部留各自机器同步盘外。
- 本次日志与源码快照在本机 `handoff-records/office-*`，入口指针 `review-loop/office-handoff-root.txt`，含go-core-tests.txt、go-api-tests.txt、frontend-typecheck.txt、frontend-tests.txt、evidence-extra-counterexamples.txt；不假设这些日志办公室存在，重要结论已写本文。
- 旧review-loop ready/state/manifest与worker引擎哈希是本机记录，提交/拉取可能改行尾，办公室须重新采快照，不能照搬本机ready判当前有效。旧引擎项目失效按正常流程重跑，不绕过指纹。
- dashboard剩余 `.dsh-acl-probe.tmp` 为未知本地临时探针，保留但**不提交**；依赖/二进制/用户数据/机器密钥不随这次Git提交。其余提交为当前探索源码、文档、锁文件和小型测试。

## 给办公室DSH/Codex的接续prompt

```text
请从 wgcna-dashboard/docs/exploration/OFFICE_HANDOFF.md 和 wgcna-dashboard/STATE.md 开始。先核对两仓库 explore/project-copilot 与远端HEAD，保留本机未知修改；确认家里监控已暂停、办公室为唯一源码写入者，按本机Windows/Mac在同步盘外准备依赖。
当前14轮离线evidence现有18tests通过，但Codex独立发现两个未修复反例：合法envelope仅删除validity仍接受（现有缺字段测试因gene_count=0假阳性）；公开RenderSuggestion绕过Validate输出伪造gene。先写真实反例并修presence/null类型与公开渲染边界，再运行evidence定向检查和相关Project/API/前端实际测试。当前未接在线LLM/API/UI，不切provider、不使用旧Navigator权限，不声明已完成防幻觉或M0-M4验收。测试与GUI/Mac/历史基线缺口按本文分别记录。
修复后先review再推进M3适配/UI；不要重复重做已实现hub_priority，不把绘图阈值当科学排序，不擅自合并远端主线/提交推送/发布。完成所有文件/STATE/双交接后最后采新的本机manifest和ready，并等待review。若需恢复自动循环，请先由用户明确在当前机器恢复，避免双机写入。
```