# rsWGCNA 输出目录方案 · 审阅版 v2

这是待接入 app 的导出约定。`review-v2/` 是独立的设计样张，未改动 app；其中的模拟结果仅用于评审风格。生产导出目录按下面的结构实现，未运行的功能不生成虚假图表或空结果。

## 保存入口

用户选择一个**本地父目录**，填写可选的**项目名称**。未填名称时使用 `rsWGCNA-YYYYMMDD-HHMMSS`，例如 `rsWGCNA-20261027-143052`；使用用户本地时区，在记录中保存 ISO 时间及 UTC 偏移。开始导出时固定名称及分析快照。

导出到 `<用户选定的父目录>/<项目名称>/`，只有 `result/`、`report/` 两个主目录。名称不能含路径分隔符、保留字符或 Windows 保留名称。若同名目录已经存在，自动追加 `-02`、`-03`，不覆盖旧结果。确认所有文件成功写入并编译报告后，再显示导出成功及打开报告入口；失败时明确列出失败项，保留已成功写出的结果。

## 完整结构

```text
<用户选择的父目录>/<项目名称>/
├── result/
│   ├── README.txt                         # 当前导出语言的总索引和读图说明
│   ├── 00_provenance/
│   │   ├── README.txt                     # 参数、环境、视图快照的用途
│   │   ├── analysis_manifest.json         # 项目、job、语言、时间/时区、各步骤状态/运行 ID
│   │   ├── parameters.final.json          # 每步成功运行时真正使用的参数，不是未运行的界面值
│   │   ├── parameters.final.tsv           # 步骤、参数键、可读名称、最终值、单位
│   │   ├── view-state.json                # 每张图对应的完整 app 视图快照
│   │   ├── input_manifest.tsv             # 文件角色、原名、大小、SHA256、行列数
│   │   ├── environment.txt                # app/引擎/R/Quarto/绘图库版本、OS、线程、随机种子（适用时）
│   │   ├── export_manifest.tsv            # 报告图/表编号、结果路径、格式、生成状态、来源 run/view ID
│   │   ├── checksums.sha256               # 导出文件校验（不包含其自身）
│   │   ├── analysis.log                   # 分析日志，去除密钥等敏感配置
│   │   └── export.log                     # 图表导出、Quarto 编译、失败及跳过原因
│   ├── 01_input/
│   │   ├── README.txt
│   │   ├── sample_info.tsv                # 原始上传全部行全部列，保持原 ID/列名/顺序
│   │   ├── sample_info.analysis.tsv       # 最终参与分析的样本表，顺序与分析矩阵一致
│   │   ├── traits.analysis.tsv            # 实际进入相关分析的性状（含编码后的列）
│   │   ├── trait_encoding.tsv             # 原列、类别、编码列、缺失值处理说明
│   │   ├── annotation.tsv                 # 导出实际使用的基因注释
│   │   ├── gene2go.tsv                    # 实际使用的 GO 映射（用户提供/内置来源如实标明）
│   │   ├── gene2kegg.tsv                  # 实际使用的 KEGG 映射
│   │   ├── input_statistics.tsv           # 原始 expmat 样本×基因数、重复/缺失 ID、文件读取统计
│   │   ├── annotation_statistics.tsv      # 总行/唯一基因/有描述基因/匹配表达矩阵的基因数
│   │   ├── background_statistics.tsv      # GO/KEGG 唯一基因、唯一 term、映射、重复、交集统计
│   │   └── unmatched_ids.tsv              # 文件角色、未匹配 ID、原因（若存在）
│   ├── 02_filter/
│   │   ├── README.txt
│   │   ├── expression_filtered.tsv        # 完整最终分析矩阵：gene_id + 样本列，行基因/列样本
│   │   ├── filtering_summary.tsv          # 每步前后样本/基因数，注明表达量尺度与执行顺序
│   │   ├── gene_filter_audit.tsv          # 每基因保留/低表达/零变异/未入 top-N、评分与排名
│   │   ├── removed_samples.tsv            # ID、阶段、用户操作、实际填写的原因；无删除也保留表头
│   │   ├── sample_tree.before.svg         # 删除前树（若已生成，保留 app 对应视图）
│   │   ├── sample_tree.before.png
│   │   ├── sample_tree.final.svg          # 最终保留样本树；报告中明确标出是哪一次视图
│   │   ├── sample_tree.final.png
│   │   └── sample_tree.annotations.tsv    # 样本、叶序、显示名、色带原名/标签、类别、颜色
│   ├── 03_soft_threshold/
│   │   ├── README.txt
│   │   ├── sft.fit.svg / .png / .pdf      # 拟合曲线（各格式按实际渲染支持生成）
│   │   ├── sft.connectivity.svg / .png / .pdf
│   │   ├── sft.connectivity_hist.svg / .png / .pdf
│   │   ├── sft.loglog.svg / .png / .pdf
│   │   ├── sft.candidates.tsv             # 每个候选 β 的拟合/斜率/连通性等原始统计
│   │   └── sft.selection.tsv              # 推荐 β、采用 β、手动/自动、阈值及选择依据
│   ├── 04_network/
│   │   ├── README.txt
│   │   ├── gene_dendrogram.svg / .png / .pdf
│   │   ├── module_sizes.svg / .png / .pdf
│   │   ├── gene_modules.tsv               # gene_id、合并前后模块与色值、注释
│   │   ├── module_summary.tsv             # 基因数、占比、合并对应；grey 单独列出
│   │   └── module_eigengenes.tsv          # 样本 × 模块 ME（不是单个代表基因）
│   ├── 05_module_trait/
│   │   ├── README.txt
│   │   ├── module_trait.svg / .png / .pdf
│   │   ├── eigengene_adjacency.svg / .png / .pdf
│   │   ├── correlations.tsv               # 模块、性状、r、p、实际有效 n、检验/校正信息
│   │   └── trait_order.tsv                # 显示列顺序、标签、编码与 app 设置
│   ├── 06_module_detail/
│   │   ├── README.txt
│   │   └── <module_slug>/                 # 每个实际分析/保存的模块；多模块不得互相覆盖
│   │       ├── README.txt
│   │       ├── genes.tsv                  # 完整模块成员、注释及 kME
│   │       ├── gs_mm.<trait_slug>.svg / .png / .pdf
│   │       ├── gs_mm.<trait_slug>.tsv      # gene_id、kME、GS 及相关 p 值（如已计算）
│   │       ├── expression_heatmap.svg / .png / .pdf
│   │       ├── expression_heatmap.tsv      # 与图相同的基因/样本顺序和转换尺度
│   │       └── enrichment/
│   │           ├── README.txt
│   │           ├── background.go.tsv      # 真正被检验使用的 GO 基因背景
│   │           ├── background.kegg.tsv    # 真正被检验使用的 KEGG 基因背景
│   │           ├── enrichment_settings.json # 方法、校正、阈值、基因集/背景大小、ID 处理
│   │           ├── go.all.tsv             # 全部被检验条目，含 p/padj、命中 ID、分子分母
│   │           ├── kegg.all.tsv
│   │           ├── go.displayed.tsv       # 当前图实际显示的条目、排序
│   │           ├── kegg.displayed.tsv
│   │           ├── go.dotplot.svg / .png / .pdf
│   │           ├── kegg.dotplot.svg / .png / .pdf
│   │           └── enrichment_heatmap.svg / .png / .pdf # 已运行时
│   └── 07_hub/
│       ├── README.txt
│       └── <module_slug>/<trait_slug>/
│           ├── README.txt
│           ├── hub.all.tsv                # 全部符合阈值的候选，不受显示上限截断
│           ├── hub.displayed.tsv          # 经过 top-N/显示数量限制后的候选
│           ├── network.nodes.tsv          # 真正显示的节点、属性、标签、颜色、坐标
│           ├── network.edges.tsv          # 真正显示的边与权重
│           ├── network.cytoscape.json      # 支持导出时：网络及布局，可重新载入
│           ├── hub_network.svg / .png      # app 当前布局、节点/边/标签样式
│           └── selection.json             # 模块、性状、阈值、比较符号、排序/截断、布局设定
└── report/
    ├── index.html                         # 双击打开的报告入口；语言跟随导出时 app
    ├── report.qmd                         # 该次实际分析的报告源文件，可编辑并重新编译
    └── assets/
        ├── report.css                     # 本地样式，无网络字体依赖
        ├── report.js                      # 导航、表格搜索、打印等轻量交互
        ├── figures/                       # 与 result 对应图完全相同的文件副本
        └── vendor/                        # Quarto 依赖（若需要），均本地引用
```

`/ .png / .pdf` 是多格式简写，并非含斜杠的文件名。SVG/PNG 为优先格式；PDF 按已有绘图后端支持导出。不能声称没有生成的格式已经可下载。大矩阵 TOM/adjacency 不默认输出，避免无意产生巨量文件；如后续提供开关，应单独记录是否导出。

完整项目目录含报告及可下载数据。`report/` 可独立阅读图文，因为保存图表副本；报告里的数据下载链接指向 `../result/`，下载完整数据仍需同时分享 `result/`。样张为减少重复使用共享的 `assets/`，所以请复制整个 `review-v2/`，不要只复制 HTML。

## 报告顺序与文案约定

1. **输入与背景**：原始矩阵的样本 × 基因数；所有输入文件；完整 sample info（全部列、全部行，可搜索，不用省略号代替）；注释、GO/KEGG 文件统计；未提供的文件标记“未提供”。
2. **最终参数**：按真实运行步骤展开全部参数；正文讲理由，附录/表格保留技术键和值。记录每步实际成功运行的配置和各自运行 ID，避免把尚未运行的 UI 参数写成结果。
3. **过滤与样本质控**：按引擎实际顺序解释删除样本、转换、低表达过滤、零变异移除及 MAD/方差 top-N；分开统计，不把低变异基因一概称为噪音。保留完整最终矩阵，列出删除 ID、操作及已记录的原因。
4. **软阈值**：候选曲线、连通性、scale-free 检验；自动推荐/手动选择及其最终数值。未达到目标就写“未达到”，不包装为达标。
5. **网络与模块**：基因树、合并前后色带、模块大小、grey 未分配数量；解释 ME 是表达特征向量。
6. **模块与性状**：实际热图、r/p、有效样本数、编码方式、检验及校正信息；不把相关解释为因果。
7. **模块深入与富集**：用户所选模块/性状，GS/MM、基因热图、GO/KEGG 完整结果和实际背景；“缺背景”“未运行”“无显著结果”分开表述。
8. **Hub**：实际筛选规则、全部候选数、展示数、网络边/邻居规则；保留当前网络布局。
9. **文件与复现**：结果索引、说明书、参数、环境与时间/时区。所有图/表编号由 manifest 统一生成，与 README 一一对应。

### 过滤句子的精确性

当前 `rs_WGCNA/src/filter.rs` 的逻辑是：表达值低于 cutoff 的样本数 **> 样本比例 × 保留样本数** 时去除；等于临界值保留。`worker.rs` 当前先移除样本，再转换表达值，再对转换后的矩阵做阈值过滤，最后按 MAD/方差筛选；因此不能无条件把 cutoff 说成 read count。

样张特意选 `normalize_method=raw`，可以写：“如果一个基因在超过 50% 的样本中 read count 小于 10，则因低表达被去除。”17 个样本时至少 9 个低于 10。转换启用时应改写为相应尺度，例如“log10(x+1) 转换后的表达值”，MAD/方差也注明使用的尺度。低表达过滤关闭时直接写“本次未启用低表达过滤”。若不足设定的 top-N，就写实际保留数量。

### 所见即所得的实施约定

- 报告导出开始时冻结一个有效的分析与视图快照；下游过期时标记“需要重算”，不能拼接新参数和旧结果。
- **样本树**：冻结使用的树数据、叶序、样本别名、色带原始列名、显示 label、行顺序、类别→色值映射、图例、字号/尺寸和排除标记。不能只根据 metadata 重新自动选列或配色。
- **其他图**：冻结模块/性状、条目排序/截断、图例、范围、色阶、标签、隐藏项、尺寸及全部绘图选项；网络还需保存节点坐标、边与节点样式。
- 尽量直接导出 app 当前渲染结果作为报告插图；R/PDF 版本若由另一后端生成，必须使用同一视图配置验证一致性，不能把重新计算的图称为所见即所得。
- 保存当前视图是最低范围；用户保存过的其他视图若纳入导出，使用独立 view ID，不能凭空生成未运行的分析。

## README.txt 的内容标准

每个结果子目录至少有一份当前语言的说明：文件名与报告图/表编号、图的目的、横纵轴/行列含义、颜色/点大小含义、统计方法/阈值、关键字段和单位、如何读及局限、该图采用的模块/性状和对应 view ID。无结果时写原因。未知操作原因应标记“未填写”，不编造质量问题。

## 语言、版权与首页

导出开始时读取 app 当前语言并固定：中文界面生成中文报告/README，英文界面生成英文报告/README；gene ID、模块名、原始样本 ID、用户字段名保持原样。样张提供两个独立报告供审阅。

首页设计以暖白纸感、墨色文字、共表达网络和步骤索引为主，取代此前绿色渐变大横幅与功能卡片布局。页脚：`© 2026 Xiao Wang Lab · rsWGCNA`；年份正式接入时随当前年份更新。课题组入口暂时指向用户指定的 [GitHub 仓库](https://github.com/XiaoWang-SXAU-Lab/xiaowang-sxau-lab.github.io)，不切换至尚未确认的域名。
