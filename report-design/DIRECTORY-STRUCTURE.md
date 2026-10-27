# rsWGCNA 报告输出目录结构

用户在「报告」页选择一个本地目录 + 项目名（缺省 `rsWGCNA-YYYYMMDD-HHMMSS`），
rsWGCNA 在该目录下创建如下结构：

```
<用户选择的目录>/<项目名>/
│
├── report/                            # 🌐 网页报告（可分享、可打印）
│   ├── index.html                     # 报告主页（Quarto 编译产物）
│   ├── report.qmd                     # 报告源文件（可重新编译、二次编辑）
│   ├── styles.css                     # 自定义样式
│   └── (scripts.js 可选)              # 自定义交互脚本
│
├── result/                            # 📊 所有图表 + 数据 + 说明
│   ├── README.txt                     # 图表总说明（帮用户看懂每张图）
│   │
│   ├── 01_input/                      # 第 1 步 · 数据输入与质控
│   │   ├── sample_info.tsv            # 样本信息表
│   │   ├── sample_info.png            # 样本信息表截图（可选）
│   │   └── annotation_stats.txt       # 注释 + GO/KEGG 背景统计
│   │
│   ├── 02_filter/                     # 第 2 步 · 清洗过滤
│   │   ├── expression_filtered.tsv    # 过滤后表达矩阵（下载用）
│   │   ├── removed_samples.txt        # 被删除的样本（如有）
│   │   ├── filtering_summary.txt      # 过滤前后基因数/样本数统计
│   │   └── sample_tree.png            # 样本聚类树（色块标签=APP 所选）
│   │
│   ├── 03_soft_threshold/             # 第 3 步 · 软阈值
│   │   ├── sft_r2.png / .pdf / .svg   # 无标度拟合 R² 曲线
│   │   ├── sft_connectivity.png       # 平均连通性曲线
│   │   └── sft_power.txt              # 选定的 β + R² 值
│   │
│   ├── 04_network/                    # 第 4 步 · 网络与模块
│   │   ├── dendrogram.png             # 模块聚类树 + 模块色带
│   │   ├── module_sizes.png           # 模块大小条形图
│   │   ├── module_colors.tsv          # 基因 → 模块颜色映射
│   │   └── modules_summary.txt        # 模块数 / 合并统计
│   │
│   ├── 05_module_trait/               # 第 5 步 · 模块-性状
│   │   ├── mtrait_heatmap.png         # 模块-性状相关热图
│   │   ├── eigengene_heatmap.png      # 特征基因邻接热图
│   │   └── mtrait_cor.tsv             # 相关系数 + p 值矩阵
│   │
│   ├── 06_module_detail/              # 第 6 步 · 模块深入 + 富集
│   │   ├── gsmm_<module>.png          # GS×MM 散点（每个模块一张）
│   │   ├── geneheatmap_<module>.png   # 模块基因表达热图
│   │   ├── go_<module>.png            # GO 富集气泡图
│   │   ├── kegg_<module>.png          # KEGG 富集气泡图
│   │   ├── go_<module>.tsv            # GO 富集结果表
│   │   └── kegg_<module>.tsv          # KEGG 富集结果表
│   │
│   └── 07_hub/                        # 第 7 步 · Hub 基因
│       ├── hub_<module>_<trait>.tsv   # Hub 基因列表
│       ├── hub_network.png            # Hub 基因网络图（可选）
│       └── hub_summary.txt            # Hub 筛选阈值与统计
│
└── (可选) analysis.log                # 全程运行日志
```

## 设计要点

1. **两个主目录**：`report/`（网页报告）+ `result/`（图表数据），职责分离。
2. **`result/README.txt`**：用「人话」逐图解释每张图怎么看、横纵轴是什么、结论怎么读，配合报告中「说人话」的参数说明。
3. **编号对应**：报告里的「图 1/图 2…」与 `result/` 下的文件名一一对应（README 里列出映射）。
4. **WYSIWYG**：报告里所有图表（尤其 sample tree 的色块 label、mtrait 热图的行列标签）都直接复用 APP 里用户当前选择，保证「所见即所得」。
5. **过滤后的表达矩阵**单独放 `02_filter/expression_filtered.tsv`，报告里提供下载链接。
