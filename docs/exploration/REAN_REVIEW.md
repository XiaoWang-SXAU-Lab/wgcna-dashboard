# R-EAN 实现评审（2026-11-02，Codex）

范围：用户报告 weight cutoff=0.35、1000 次 bootstrap 后 stability 仍全为 1。只评审及运行隔离诊断，未修改算法或 UI 源码。

## 已确认发现

1. **P1：失效的 R-EAN 结果仍参与当前网络的证据解读。** `ModuleExplorerView.vue:33` 直接读取 `store.results.rean`，不检查 `validity.rean`；`stabilityMap`、证据发现和绘图继续使用它。`project.ts:116` 起的结果缓存按 steps 表而非 draft/validity 刷新；服务端 Result 明确允许读取历史结果。修改 bootstrap 次数/seed 后，甚至重跑 hub 后但尚未重跑 rean，旧 stability 仍可显示或按基因对混入新网络。修复：保留历史结果入口，但当前证据必须校验 rean validity、hub execution/provenance；草稿变化立即标记，禁止旧结果驱动当前结论。

2. **P1：bootstrap 零方差产生 NaN，污染无关边并算作不稳定。** `worker.rs:1392` 的 Pearson 返回 NaN 后未经检查送入 TOM；`tom.rs` 的 A*A 传播 NaN，`worker.rs:1401` 的 NaN > threshold 为 false，分母仍是全部 B。隔离反例：5 样本，A=B=[1,2,3,4,5]，C=[0,0,0,0,1]，signed/signed TOM/power6/cutoff0.35/B1000/seed42，A—B stability=0.678；仅将 C 改为 [1,2,3,4,5]，结果=1。A、B 未变，第三基因的退化导致数值污染。修复前先定义退化重复的策略、有效重复数及诊断；不得静默替换 NaN 或把计算失败当科学上的边消失。这个缺陷方向为压低 stability，不解释全 1。

3. **P2：表格隐藏非 1 的边。** `ModuleExplorerView.vue:164–168` 按 stability 降序后截前 200 行；SimpleTable 的导出接收的也只是这 200 行。现有 blue fixture 的隔离复跑：cutoff=0.35、B=1000、seed42，5565 边中 4148 条=1，1417 条<1，范围 [0.979,1]，平均 0.999077628；表格前 200 条必然全 1。修复：完整分页/完整导出，显示 min/分位数/精确计数，默认允许从低到高排序，明确总数和展示范围。

4. **P2：把高 stability 归因于阈值过低，证据不足。** `ModuleExplorerView.vue:626` 从 mean>0.99 推断所有边≈1且阈值低/网络密，并建议提高阈值。均值不足以证明所有边接近 1；提高阈值还会改变被评估的原始边集合，可能只留下更强的边。应显示阈值穿越频率定义、权重分布和计算设置，提供诊断而非强制追求区分度。

## 算法语义与本次症状

每轮确实按样本列联合有放回抽样、重新算 Pearson/TOM；RNG 状态跨轮推进。阈值来自已提交 hub.used_threshold，而非当前滑块草稿；前端/后端依赖检查要求先重跑失效 hub，再跑 rean。未发现 stability 硬编码为 1。

当前指标是固定候选池、固定 power/网络类型、固定已选模块与原网络边条件下的 TOM 超阈值频率。没有重新选择模块、hub 或候选池，不能称为完整 WGCNA 流程/模块/hub 选择稳定性，也不是连接真实概率。只输出原先选中的边，天然集中于强连接。留存 blue 网络的原始权重范围为 0.5315–0.7946；0.35 仍低于所有这些原始边的权重，但这本身不能证明每轮都通过。

增加 B 提高频率估计精度，不保证结果产生差异。建议补充 TOM bootstrap 分布/区间、有效重复数及阈值穿越计数；按实验设计考虑分层或按独立实验单位重采样。不得为制造非 1 结果而调整随机性。

## 实际验证与限制

- 使用正在运行应用对应的现有 worker 二进制，SHA256 `5e2b0cd49915c9a1716cb99b9739c33ff4651f4dccdc0deb259e6a4ab06c32b5`；核对 worker.rs 和 ModuleExplorerView.vue 与该构建源码快照 SHA256 一致。
- blue fixture 来自既有审计目录；复制到本机隔离目录，只改 hub 阈值为 0.35、B 为 1000。其原网络边均高于 0.35。没有重跑 hub/候选选择，不声称这是用户当前项目的同一执行。
- 运行上述零方差合成反例和对照；结果分别 0.678、1。
- 原始诊断、summary.json、合成 fixture 位于本机 `%LOCALAPPDATA%/rsWGCNA-build/windows/review-rean-*`；本机指针 `review-rean-latest.txt`。不将诊断产物同步到仓库。
- 未重跑构建、完整 Cargo/Go/前端测试，未做浏览器自动验证，未读取到用户本次实际项目的 1000 次执行结果。已有 `report_test.go:44` 基线失败未重跑。
- 没有运行或联系 DSH、切 provider、提交推送、修改用户项目。初始合成 TSV 格式错误后已修正并完成有效运行；系统 Python 的本地目录虚拟化访问失败，最终诊断通过 PowerShell 和实际 worker 完成。

## 修复顺序

先修当前/历史证据有效性与完整表格/导出；再为 bootstrap 退化重复制定明确数值策略并加入反例回归；之后补充权重分布和指标口径。UI 架构新建议（模块内证据 + 可选跨模块面板）尚未实施，不应直接按旧 R_EAN_PLAN 的独立单模块面板计划推进。
