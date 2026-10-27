rsWGCNA 设计预览 — 全部为模拟示例，不能用于生物学结论。

先打开 report/index.html。报告与本文件按相同步骤组织。
01_input/sample_info.tsv：全部 18 个输入样本；genotype 为基因型，condition 为条件，replicate 为重复。
00_provenance/parameters.final.json：本样张的全部示例参数。正式报告记录实际运行参数。
00_provenance/view-state.json：样本树色带、类别颜色及模块/性状选择的示例快照。
02_filter/expression_filtered.preview.tsv：只有 8 个模拟基因 × 17 个样本的下载示例，不是完整过滤结果。行是基因，列是样本。
02_filter/removed_samples.tsv：被删除样本和操作记录。原因未填写时如实标明，不自动推测。
图 1 / 02_filter/sample-tree.svg：样本聚类树布局示意；分支表示相似程度，condition/genotype 色带顺序固定；红色样本被手动排除。
图 2 / 03_soft_threshold/soft-threshold.svg：横轴 β，纵轴拟合 R²；目标线 0.85，选择 β=8（示例）。
图 3 / 04_network/modules.svg：基因树和模块色带。模块色是名称，不表示显著性；grey 为未分配。
图 4 / 05_module_trait/module-trait.svg：相关热图布局示意；红正蓝负，无真实 p 值，不作显著性结论。
图 5 / 06_module_detail/turquoise/gs-mm.svg：|kME| 与 |GS| 的模拟散点，虚线是筛选阈值。
图 6 / 06_module_detail/turquoise/enrichment.svg：GO 气泡图示意，非真实富集结果。
图 7 / 07_hub/turquoise/Drought/network.svg：网络排版示意，不是候选基因的真实网络。

完整生产目录见 ../../DIRECTORY-STRUCTURE.md；本预览只提供审阅所需文件。
