# rsWGCNA 报告与首页设计样张 v2

独立本地审阅稿，未改动 app。使用 Sites 技能的本地预览流程，没有部署。

- `index.html`：新版软件首页视觉稿。
- `export-preview.html`：保存目录、项目名和缺省名称的交互概念稿；不访问本地文件。
- `zh/report/index.html`、`en/report/index.html`：实际使用 Quarto 编译的中英文报告。
- `DIRECTORY-STRUCTURE.md`：完整生产输出目录与导出约定。
- `build_preview.py`：生成模拟内容与 Quarto 源文件，无第三方 Python 依赖。
- `assets/`：离线 CSS、JS 和 SVG。请保留整个 review-v2 目录结构。

重新生成：

```sh
python3 build_preview.py
(cd zh/report && quarto render report.qmd --output index.html)
(cd en/report && quarto render report.qmd --output index.html)
```

报告中的数据、图表、参数、输入统计都是明确标注的模拟示例，不是对工作区数据重新分析的结果。矩阵下载只含 8 行示例数据。完整生产目录尚未实现，等待样式审阅后再接入 app。此样张不宣称已实现真实 app 的所见即所得导出。
