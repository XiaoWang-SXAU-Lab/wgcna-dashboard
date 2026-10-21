package ai

// KnowledgeDocs is the curated knowledge base: WGCNA principles, the pipeline
// guide, parameter semantics, and troubleshooting. These are chunked + embedded
// into the local vector store at startup.
func KnowledgeDocs() map[string]string {
	return map[string]string{
		"wgcna-principles": `
WGCNA (Weighted Gene Co-expression Network Analysis) 是一种从表达矩阵中发现共表达
基因模块并关联性状的方法。核心步骤：
1. 计算基因间 Pearson 相关矩阵 cor。
2. 软阈值 beta 把相关矩阵幂次化为邻接矩阵 adjacency = |cor|^beta（unsigned）或
   ((1+cor)/2)^beta（signed）。beta 越大网络越"无标度"，但会牺牲连通性。
3. 拓扑重叠矩阵 TOM 衡量两个基因共享的"邻居"重叠度，比原始相关更能反映生物学
   上的模块结构。dissimilarity = 1 - TOM。
4. 对 dissimilarity 做层次聚类（UPGMA/average），再用 dynamic tree cut 切出模块。
5. 每个模块用第一主成分（module eigengene, ME）概括表达谱。
6. 模块 eigengene 与性状相关，得到模块-性状关联。
7. 基因显著性 GS = |cor(gene, trait)|，模块隶属度 MM = |cor(gene, ME)|，二者结合
   可筛选 hub 基因。
`,

		"pipeline": `
本工具是 7 步管线：
0 导入质检：上传表达矩阵（基因×样本 TSV），自动检测格式。
1 清洗过滤：归一化（FPKM/TPM 用 log10(x+1)）、低表达过滤、按 MAD 取 top-N 高变异基因。
2 软阈值 beta：扫描各 beta 的 scale-free 拟合 R²，选 R² 达标的推荐 beta。
3 网络与模块：邻接矩阵→TOM→层次聚类→切模块→合并相似模块。产出模块颜色 + 树状图。
4 模块-性状：上传性状表，算模块 eigengene 与性状的相关及 p 值（热图）。
5 模块深入：选一个模块+一个性状，看 GS/MM 散点与模块基因表达热图。
6 hub 基因：按 kME、GS 阈值筛 hub 基因，导出 Cytoscape 网络。

支持 one-step（一键顺序跑完，软阈值处暂停确认）与 step-by-step（分步跑，
改某步参数只重算该步及下游）。
`,

		"params": `
关键参数含义：
- n_keep（保留基因数）：MAD top-N，默认 8000。基因多计算慢，建议 5000~10000。
- normalize_method：FPKM/TPM 用 log10p1，count 用 log，代谢/蛋白用 raw 或 log。
- soft_threshold beta：通常取推荐值；无标度拟合 R²≥0.85 且 mean connectivity 不过低。
- min_module_size：最小模块基因数，默认 30。
- merge_cut_height：模块合并树高阈值（0~1），默认 0.25，越大合并越激进（模块越少）。
- deep_split：切树灵敏度 0~4，默认 2。
- network_type：unsigned（推荐，|cor|）或 signed（区分正负相关）。
- kme_cutoff / gs_cutoff：hub 基因的 MM/GS 阈值，默认 0.8 / 0.5。
`,

		"troubleshooting": `
常见问题：
- "read trait.tsv" 报错：模块-性状步之前没上传性状表。先在步骤 4 上传样本×性状的 TSV。
- 结果只有 grey 模块或模块极少：beta 太大、merge_cut_height 太大、或 min_module_size 太大。
- 树状图/热图不显示：确认对应步骤已跑完（done_steps 含该步），再刷新。
- 内存不足/卡死：基因数太大（>12000）。当前版本是单块 dense 计算，内存 O(G²)，
  建议把 n_keep 降到 8000 以下，或减小数据。
- 无标度拟合 R² 一直不达标：检查是否漏了归一化，或 beta 搜索范围不够。
`,
	}
}
