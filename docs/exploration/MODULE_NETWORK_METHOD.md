# 模块内网络（Intra-module Network）方法说明

> ⚠️ **状态：草稿 / 待核实，勿作为最终依据、勿推送。** 本文多处（尤其 R-EAN 的归属与描述）尚未完全核实，待作者彻底搞清楚后再定稿。

> 本文描述 rsWGCNA「模块解析 / Module Explorer」中围绕 hub 基因抽取、可视化与边稳定性评估的完整方法。
> 范围：`module_detail` → `hub` → `rean` 三步，以及前端 Module Explorer 的显示层。
> 更新：2026-11-01。实现位置：`rs_WGCNA/src/bin/worker.rs`（`run_module_detail`/`run_hub`/`run_rean`）、`wgcna-dashboard/frontend/src/{views/ModuleExplorerView.vue, components/NetworkGraph.vue}`。

---

## 1. 背景与问题

WGCNA 给出的「模块」通常是几百到几千个基因，模块内两两 TOM/adjacency 都很高，直接画成网络是一个近乎完全图，既看不出结构、也无法区分 hub 与普通成员。标准 WGCNA 只有两个总结性指标——模块内连通性 **kWithin**（`intramodularConnectivity`，完整模块内 adjacency 非对角项之和）和模块成员度 **kME**（基因与模块 eigengene 的相关系数），它不定义「一个模块内网络该怎么抽出来画、怎么评估每条边有多可靠」。

rsWGCNA 要解决的是三件事：

1. **抽**：从大模块里抽出一个有生物学意义、可解释、可画的「模块内网络」；
2. **标**：给每个节点/每条边一个科学上诚实、不会混淆的指标；
3. **证**：给边一个可复现的「稳定性」估计，而不是只画一张静态图。

这三件事分别对应本文的 hub 网络抽取、指标分层、R-EAN 边稳定性。

---

## 2. 方法总览

整条链路按「科学计算 → 网络抽取 → 显示」分三层，边界明确、互不污染：

| 层 | 步骤 | 输出 |
|---|---|---|
| 科学计算 | `module_detail` | 每个模块基因的 kME（=GS/MM 基础）、eigengene |
| 网络抽取 | `hub` | hub 表、候选池、节点集、诱导子图边、三套节点指标、provenance |
| 稳定性 | `rean` | 每条原网络边的 edge_stability（bootstrap） |
| 显示（纯前端） | Module Explorer | hub-centric / sparse / full 三种视图，只改画法、不改科学数据 |

关键原则：**显示层只做"画什么"，绝不回写或重算科学指标**。切换显示模式前后，`step6_hub.json` 的字节不发生变化（有验收验证），因此图上的稀疏化/着色永远不会悄悄改掉底层的网络或指标。

---

## 3. Hub 判定：smart hub + 两层标记

### 3.1 smart hub（科学 hub）

对每个模块基因同时算两个量（`run_hub`）：

- **kME** = |cor(gene, module eigengene)|，衡量「基因代表模块的程度」；
- **GS** = |cor(gene, 所选性状)|，衡量「基因与表型的关联」。

科学 hub 判定为：

```
|kME| > kme_cutoff  (默认 0.8)
   且
（仅当"所选性状"与模块显著相关时）|GS| > gs_cutoff
```

其中「所选性状与模块显著相关」用启发式阈值 `|cor| > 0.6 且 p < 0.05` 判断。

两个明确的取舍（已写入代码注释与 provenance）：

- 0.6/0.05 是**探索性启发式**，不是 WGCNA 的通用 hub 标准，也没有多重校正——只用于筛选候选 hub，不作严格显著性结论；
- 显著性门控**只看所选性状**，不再看「模块对任一性状显著」。这是修复过的语义：选择性状 B 时，即使性状 A 显著，也不应该拿 B 的 GS 去过滤。

### 3.2 两层标记：is_hub vs is_seed

早期实现里「hub」只有一个含义，导致表格和图互相矛盾。现在拆成两个正交标记：

| 字段 | 含义 | 数量 |
|---|---|---|
| `is_hub` | **科学 hub**：通过上面判定（kME + 条件 GS）的所有基因 | 可能上百个 |
| `is_seed` | **网络种子**：科学 hub 中按 kME 降序的前 `max_hubs`（默认 30）个 | 最多 30 |

网络是围绕 **seed**（种子）构建的；`is_hub` 只用于科学标注（表格徽章、tooltip 的 `hub` 字段）。这样「图上画成 hub 的节点」一定是「表格里的 hub 前几名」，两者严格一致；同时又不丢失「还有更多基因其实也过了科学阈值」这个信息。

---

## 4. 候选池 TOM 近似（`tom_scope = candidate_pool`）

这是本方法最关键的工程取舍之一。

**问题**：完整模块的 TOM 是 O(n²) 内存、O(n³) 时间的 Nowick 求和（对 2000 基因的模块意味着百万级两两相关 + 更重的求和）。对交互式分析不可行。

**做法**：只在 **候选池（candidate pool）** 上重算 TOM，池子由「种子 + 按 kME 降序的其余模块基因」构成，上限 **400**：

```
candidate_pool = seeds ∪ top-kME 基因（去重，≤ 400）
TOM ← tom_from_cor(correlation over candidate_pool, power, network_type, tom_type, tom_denom)
```

**诚实性要求**（关键）：这不是「完整模块 TOM 的子矩阵」，而是在一个**更小的基因集合上重新算**的 TOM。因为 TOM 的分子分母都依赖基因集合，从大矩阵抠一块不等于小集合上重算。所以输出里显式记录：

- `tom_scope = "candidate_pool"`、`tom_pool_size`、`candidate_gene_ids`（完整池基因列表）；
- `power`、`network_type`、`tom_type`、`tom_denom`、`used_threshold`（边阈值）。

这些 provenance 字段让下游（尤其 R-EAN）能**在完全相同的池子和参数上复现**，也避免任何人把候选池 TOM 误当成全模块 TOM。

---

## 5. 网络抽取：节点集 + 诱导子图

在候选池 TOM 上做两步抽取（`run_hub`）：

1. **节点集** = 种子 ∪ 每个种子按 TOM 权重的 top-N（默认 20）邻居；
2. **边** = 该节点集内部的**诱导子图**：所有满足 `TOM > weight_threshold`（默认 0.075）的无向边，去重、无自环。

所以最终的 `step6_hub.json` 网络是「**种子 + 强邻居之间的阈值化诱导子图**」，既包含 hub–neighbor 边，也包含 neighbor–neighbor 边。它保留了局部结构，而不是把 hub 压成孤立的星形。

`degree` 定义为这个阈值化诱导子图里的度数——这是**抽取范围 + 阈值双重决定**的量，与「全模块 connectivity」是两个概念（见 §6）。

---

## 6. 三个指标的分层（命名诚实）

历史上最容易出错的就是把三个不同东西都叫「degree/kWithin」。现在严格区分：

| 字段 | 定义 | 范围 | 是否标准 |
|---|---|---|---|
| `degree` | 诱导子图内超过阈值的边数 | 抽取节点集 | 自定义（阈值化子图度数） |
| `extracted_tom_strength` | 抽取节点集内所有非对角 TOM 权重之和 | 抽取节点集 | **不是**标准 kWithin（此前误命名 kwithin） |
| `module_kwithin` | 完整模块内 adjacency 非对角项之和 | 完整模块 | **是** WGCNA 标准 kWithin |

补充：

- `extracted_tom_strength` 用 **TOM**（不是 adjacency），且在**抽取节点集**上求和，还包含了低于边阈值的权重——所以它和标准 kWithin 在「矩阵（TOM vs adjacency）」「节点范围（抽取集 vs 完整模块）」「是否阈值化」三个维度都不同。改名就是为了不再冒充 kWithin。
- `module_kwithin` 严格按 WGCNA `intramodularConnectivity` 的定义：完整模块内 adjacency 之和，用 `adjacency()`（按 network_type 做 |cor|^power 或 ((1+cor)/2)^power）逐对累加，与 TOM 无关。

前端里两个指标**并存**、标签区分（"TOM strength" vs "kWithin"），旧结果缺字段时自动回退、禁用，绝不把「未知」当 0。

---

## 7. 显示层 vs 科学层

前端 Module Explorer 提供三种显示模式，**都只是对同一份 `step6_hub.json` 边做筛选/着色**，不回写：

| 模式 | 规则 | 用途 |
|---|---|---|
| Hub-centric（默认） | 每个 seed 的 top-K 条边（K = top_n_neighbors） | 看清 hub 的最强连接（hub-and-spoke） |
| Sparse | 边预算内 hub 边优先 + 最多 30% 非 hub 边预留 | 中等规模快速浏览 |
| Full | 诱导子图全部边 | 完整结构（常是近完全图） |

配套的诚实性设计：

- **displayDegree**（当前视图度数）与 worker 的 `degree`（诱导子图度数）分开，tooltip 标注「hub / seed」两态；
- 色阶是**当前视图相对刻度**（图例已注明），跨模式不可直接比颜色/大小；
- 缺失指标：大小落到中点、颜色落到低色，选项在无该指标时禁用 + 自动回退 kME。

---

## 8. R-EAN：可靠性层 edge_stability（原创概念）

### 8.1 概念来源

R-EAN（**Reliability- and Evidence-Aware Network**）是 rsWGCNA **自己提出的原创方法**（暂定名），不是从任何论文借来的概念。其目标是把「连接强度（TOM）、重采样稳定性、条件差异」作为**相互独立的证据层**编码进同一张网络，而不是混成一个不透明分数；并进一步支持模块间桥梁基因发现（participation coefficient、跨模块连接稳定性等）。

需要明确边界：其**底层分析思想**（bootstrap 稳定性、差异共表达、桥梁基因、多层网络）在 hdWGCNA 等已有工作中已经存在，因此**不能宣称这些分析概念是 rsWGCNA 首创**；rsWGCNA 的原创点在于「**把多个证据层分开编码、可切换、可追溯的网络表示方法**」本身。

本文当前实现的 `edge_stability` 只是 R-EAN 三层证据中的「**可靠性层**」的一个确定性实现，不是 R-EAN 的全部。

### 8.2 实现

```
对 b = 1..n_bootstraps（默认 100，seed=42）：
    boot = 样本集有放回重采样（SplitMix64 确定性 RNG）
    TOM_b ← 在"完整 candidate pool"上重算 TOM（复用 hub 的 power/type/denom）
    对每条原网络边 (i,j)：
        if TOM_b[i,j] > used_threshold:  count[i,j] += 1
stability[i,j] = count[i,j] / n_bootstraps
```

三个刻意设计：

1. **同 scope**：bootstrap 在**完整 candidate pool**（400 基因）上重算 TOM，而不是在最终节点集（106）上——保证稳定性评估的 TOM 与产生原网络边的 TOM 是同一范围。`scope = "candidate_pool"`。
2. **同参数**：`power`、`network_type`、`tom_type`、`tom_denom`、`used_threshold` 全部从 `step6_hub.json` 继承，不由 `rean` 单独定义，杜绝参数分叉。
3. **确定性可复现**：用无外部依赖的 SplitMix64，固定 seed 两次运行输出 SHA-256 完全一致（已验收）。

### 8.3 语义与边界

`stability` = 「在 bootstrap 重采样下，这条边在原网络阈值下出现的频率」。它衡量的是**边的采样稳定性**，不是 p 值、也不是"边一定存在"。已知边界：

- 阈值沿用原网络阈值（0.075 等），若阈值很低、候选池很密，可能所有边 stability≈1（无区分度）；这是阈值选择问题，不是算法错误。
- 旧版曾以 `hub_node_set` 为 scope，与新网络范围不一致，已废弃并改为 `candidate_pool`。

---

## 9. Provenance 与可复现

`step6_hub.json` 记录：`tom_scope`、`tom_pool_size`、`candidate_gene_ids`、`network_type`、`tom_type`、`tom_denom`、`power`、`used_threshold`、`selected_trait`、`selected_trait_significant`、`raw_edge_count`、`edge_set_completeness`。

`step7_rean.json` 记录：`scope`、`threshold`、`power`、`network_type`、`tom_type`、`tom_denom`、`n_bootstraps`、`seed`。

配合引擎哈希失效机制：worker 二进制变化 → 旧步骤全部标「engine changed」，必须按序重跑，保证任何结果都能追溯它的计算参数。

---

## 10. 借鉴 vs 自研对照

| 能力 | 来源 | 说明 |
|---|---|---|
| 模块识别、kME、kWithin | WGCNA（Langfelder & Horvath 2008） | 忠实重实现，`module_kwithin` 即标准 kWithin |
| 环形模块网络图（ringTopN 布局） | hdWGCNA（Morabito et al. 2023，GPLv3） | 仅借鉴布局，见 `ATTRIBUTION.md` |
| 多层证据感知网络表示（R-EAN） | —（自研，暂定名） | 原创表示方法；底层 bootstrap/差异/桥接思想与已有方法重叠 |
| **smart hub 判定** | 自研 | kME + 条件 GS + 所选性状门控 |
| **候选池 TOM 近似 + provenance** | 自研 | 交互式规模下的诚实近似，全参数落盘 |
| **is_hub / is_seed 两层标记** | 自研 | 分离科学判定与网络种子 |
| **extracted_tom_strength vs module_kwithin 分层** | 自研 | 命名诚实，避免冒充标准指标 |
| **显示/科学层分离 + 相对刻度 + 缺值语义** | 自研 | 显示层零回写、可验收 |
| **确定性 edge_stability（SplitMix64 bootstrap）** | 自研 | 固定 seed 可复现 |

---

## 11. 局限与后续

- **候选池 TOM 是近似**：边排序/邻居选择在 400 基因池上计算，与全模块 TOM 的差异未量化；对超大模块需评估 400 上限是否够。
- **阈值敏感**：`weight_threshold`（默认 0.075）偏低时诱导子图接近完全图（实测 blue 模块 106 节点 5565 边 = 完全图），需用户按权重分布调高。
- **hub 启发式**：0.6/0.05 与 `kme_cutoff`/`gs_cutoff` 是探索参数，未做多重校正，不作严格显著性。
- **R-EAN 仅实现「可靠性层」**：强度层已有（TOM）、差异层（differential edge weight）与桥接基因（node bridge score / participation coefficient）尚未实现。
- **module_kwithin 成本**：完整模块 adjacency 是 O(n²) 两两相关，超大模块会变慢，需按需评估（当前只求和、不落矩阵）。
