# R-EAN 全量落地方案（R_EAN_PLAN）

> 历史方案（2026-11-02）。2026-11-10 决定将已实现部分作为模块网络下方的可选边稳定性诊断，暂停差异层、桥接层和独立面板。下文保留原规划供查阅。
> 关键决定：R-EAN 独立成 panel（module overview 之后、report 之前）；module overview 去掉基因级整体网络；**UMAP 默认不做**。

## 1. 定位（一句话）

R-EAN（Reliability- and Evidence-Aware Network）是独立分析面板，介于模块总览与报告之间；对「选中模块」的网络，把**强度 / 可靠性 / 差异 / 桥接**作为相互独立的证据层编码进同一张网络，可切换、可追溯。

## 2. 与 hdWGCNA / UMAP 的关系（诚实边界）

- 底层分析思想（bootstrap 稳定性、差异共表达、桥接基因、多层网络）与 hdWGCNA 等已有工作**重叠，不宣称首创**；rsWGCNA 的原创点只在「多证据层分开编码、可切换、可追溯的网络表示方法」本身。
- 可视化策略（环形布局 ringTopN）**借鉴 hdWGCNA 的思想**，但网络抽取 / 稀疏化 / 稳定性用**完全不同的算法**独立实现（Rust 诱导子图 + 确定性稀疏化 + provenance + SplitMix64 bootstrap）。详见 `ATTRIBUTION.md`。
- **UMAP 默认不做**：UMAP 的价值在揭示单细胞群体等「别处不可见的结构」；bulk WGCNA 的模块结构已由树状图 + TOM 热图 + eigengene（PCA）忠实呈现，非线性 2D 嵌入是冗余的有损重投影。仅当审稿人/用户明确要求，再作为可选可视化补上，不占主线。

## 3. 面板序列（最终）

```
1  Data Import     数据导入
2  SFT             软阈值
3  Network         网络构建 & 模块识别
4  Module-Trait    模块-性状关联
5  Module Overview 模块级总览（热图/eigengene/树状图/聚类；去基因级整体网络）
6  Module Explorer 模块内网络抽取（hub/seed + 诱导子图 + 三指标，只做强度层）
7  R-EAN           证据感知网络（独立 panel：多证据层 + 可切换可视化）
8  Report          报告
```

依赖约束：R-EAN（`rean`）的 bootstrap 依赖 module-explorer（`hub`）抽出的网络边，故 R-EAN 必须在 Module Explorer 之后。

## 4. 四个证据层

| 层 | 字段 | 定义 | 状态 |
|---|---|---|---|
| 强度 Strength | `weight` | 候选池 TOM | 已实现（hub 步骤） |
| 可靠性 Reliability | `edge_stability` | bootstrap 出现频率（**非 p 值**） | 已实现（rean 步骤） |
| 差异 Differential | `edge_diff` | 两条件下 TOM 权重差 | 未实现（需多条件数据） |
| 桥接 Bridge | `bridge_score` | 基因跨模块连接度（participation coefficient） | 未实现 |

## 5. 计算层改动（worker / 后端）

- `rean` 步骤**保留并增强**：当前只出 `edge_stability`；将来在同一 scope（candidate pool）补差异层、桥接层输出。
- 新增（M3/M4）：`diff`（两个条件组分别算候选池 TOM → 边权重差 + 符号）；`bridge`（基因对多模块 connectivity 分布 → participation coefficient）。
- `StepOrder`：`rean` 之后，`report` 依赖 R-EAN 输出。
- **去掉** `overview_network` 辅助步骤（基因级整体网络）；保留 `cluster` / `cluster_elbow`（聚类对比归 module overview）。

## 6. 前端 R-EAN 面板（2.5，核心卖点，低风险先做）

1. **证据层切换器**：顶部单选「强度 / 可靠性 / 差异 / 桥接」（未实现的层置灰 + 说明）。
2. **边着色通道** 绑定当前证据层（`NetworkGraph.vue` 现有 `weight | stability` 扩成四通道）。
3. **点击边/节点 → 多维证据弹窗**：一次展示该边/节点的所有已实现证据层数值（这是 R-EAN 区别于 hdWGCNA「一张图一个证据」的核心）。
4. **色阶 = 当前层实际 min/max**（相对刻度，图例注明，跨层不可直接比颜色）——沿用现有诚实设计。
5. **显示层零回写**：切换证据层前后 `step6_hub.json` / `step7_rean.json` 字节不变（沿用现有验收）。

## 7. 数据文件（JSON schema）

- `step6_hub.json`：不变（网络抽取 + provenance）。
- `step7_rean.json`：现在含 `edges[].stability`；将来每条边扩展 `weight / stability / diff / bridge` 四个可选字段，缺字段时前端回退、禁用对应层（沿用现有缺值语义）。

## 8. 里程碑（可落地顺序）

| 里程碑 | 内容 | 验收 |
|---|---|---|
| **M0** | 拆 R-EAN 路由/组件成独立 panel；删 module-overview 基因级整体网络（后端 aux 步骤 + 前端引用） | 路由可达、overview 无毛线球、旧测试不新增失败 |
| **M1** | 迁移现有 reliability 层到 R-EAN panel + report 增加 R-EAN 小节 | 稳定性表/图在新 panel 可用，report 能出 R-EAN 结果 |
| **M2** | 2.5 可切换证据可视化（strength+reliability 双层切换 + 点击多维证据弹窗） | 切换不回写、弹窗显示两层数值 |
| **M3** | 差异层（2.3，需多条件数据） | 两组数据下 `edge_diff` 可算、可视化可用 |
| **M4** | 桥接层（2.4）+ 重采样收敛性评估（2.2） | 桥接分数、收敛曲线 |
| **M5** | report 全量集成 + 双平台验收 | 完整报告含 R-EAN 已实现层 + 诚实标注 |

## 9. 诚实性红线（写进方案，别丢）

- 底层思想（bootstrap/差异/桥接）**不宣称首创**；只原创「多证据层表示方法」。
- 未实现的层**必须在 UI/report 明确标「未实现/设计意图」**，不许灰着还暗示存在。
- `edge_stability` 是采样频率**不是 p 值**；阈值过低时可能全≈1（无区分度），需在面板提示。

## 10. 实现锚点（关键文件）

- worker：`rs_WGCNA/src/bin/worker.rs`（`run_module_detail` / `run_hub` / `run_rean` / `run_overview_network`）
- 后端：`backend/internal/job/job.go`（`StepOrder`、`RunAuxStep`）、`backend/internal/api/report.go`（`detectSteps`、报告小节）
- 前端：`frontend/src/router.ts`、`views/ModuleExplorerView.vue`、`components/NetworkGraph.vue`、`stores/project.ts`、`api/projects.ts`
