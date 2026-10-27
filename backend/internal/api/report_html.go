package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

//go:embed report_assets/design.css
var reportCSS []byte

//go:embed report_assets/design.js
var reportJS []byte

// reportData carries the real analysis facts used to build the report.
type reportData struct {
	ProjectName string
	Timestamp   string
	Version     string
	Lang        string // "zh" | "en"
	Samples     int
	Genes       int
	Modules     []string
	ModuleSizes map[string]int
	GreyGenes   int
	Params      map[string]any
	HubCount    int
	HasTrait    bool
	EnrichOK    bool
}

// L picks the string for the report language.
func (d reportData) L(zh, en string) string {
	if d.Lang == "en" {
		return en
	}
	return zh
}

// generateReportHTML builds the report body (rail + hero + steps) using the
// Codex v2 visual language, populated with real analysis data.
func generateReportHTML(d reportData) string {
	if d.Version == "" {
		d.Version = "v0.5.0"
	}
	if d.Lang == "" {
		d.Lang = "zh"
	}
	var b strings.Builder

	// ---- rail ----
	nav := []string{
		d.L("输入与背景", "Inputs & backgrounds"),
		d.L("最终参数", "Final parameters"),
		d.L("过滤与样本质控", "Filtering & sample QC"),
		d.L("软阈值选择", "Soft threshold"),
		d.L("网络与模块", "Network & modules"),
		d.L("模块-性状", "Module–trait"),
		d.L("模块总览", "Module overview"),
		d.L("各模块深入", "Per-module detail"),
		d.L("Hub 基因", "Hub genes"),
		d.L("文件与复现", "Files & reproducibility"),
	}
	b.WriteString(`<aside class="rail"><a class="brand">rs<em>WGCNA</em><span style="font-size:10px;letter-spacing:0;margin-left:8px">REPORT</span></a><div class="eyebrow">` + d.L("分析记录", "Analysis record") + `</div><nav>`)
	for i, n := range nav {
		cls := ""
		if i == 0 {
			cls = ` class="active"`
		}
		b.WriteString(fmt.Sprintf(`<a href="#step-%d"%s><span>%02d</span>%s</a>`, i+1, cls, i+1, n))
	}
	b.WriteString(`</nav><div class="rail-bottom">` + d.L("rsWGCNA 分析报告", "rsWGCNA analysis report") + `<a href="https://github.com/XiaoWang-SXAU-Lab/xiaowang-sxau-lab.github.io" target="_blank" rel="noopener">Xiao Wang Lab ↗</a></div></aside><main class="report">`)

	// ---- topline + hero ----
	b.WriteString(`<div class="topline"><div class="eyebrow">` + d.L("加权基因共表达网络分析", "Weighted gene co-expression network analysis") + `</div><div class="actions"><button data-print>` + d.L("打印 / 保存 PDF", "Print / Save PDF") + `</button></div></div>`)
	b.WriteString(`<header class="hero"><div class="eyebrow">` + html.EscapeString(d.ProjectName) + `</div><h1>` + d.L("从表达矩阵，<br>到共表达网络。", "From expression,<br>to connection.") + `</h1><p>` + d.L("一份可追溯的分析记录。从输入文件、数据筛选到模块解析，逐步说明本次分析做了什么，以及每张图应该怎么看。", "A traceable analysis record. Follow the inputs, filtering decisions and module analysis, with a practical guide to reading every figure.") + `</p><div class="project-line">` + html.EscapeString(d.ProjectName) + ` · ` + d.Timestamp + ` · rsWGCNA ` + d.Version + `</div><div class="metrics">`)
	nMod := len(d.Modules)
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.Samples, d.L("样本", "samples")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.Genes, d.L("进入网络的基因", "network genes")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, nMod, d.L("共表达模块（不含 grey）", "modules, excluding grey")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div></div></header>`, d.HubCount, d.L("Hub 候选", "hub candidates")))

	// ---- step 1: 输入与背景 ----
	b.WriteString(stepHead(d, 1, nav[0], d.L("先记录分析从哪里开始。", "Start with the source data.")))
	b.WriteString(tableHTML([]string{d.L("内容", "Contents"), d.L("数值", "Value")}, [][]string{
		{d.L("样本数", "Samples"), fmt.Sprint(d.Samples)},
		{d.L("进入网络分析的基因数", "Genes entering network"), fmt.Sprint(d.Genes)},
		{d.L("共表达模块数（不含 grey）", "Modules, excluding grey"), fmt.Sprint(nMod)},
		{d.L("grey 未分配基因数", "Unassigned grey genes"), fmt.Sprint(d.GreyGenes)},
	}))
	if d.EnrichOK {
		b.WriteString(`<div class="reading">` + d.L("已提供 GO/KEGG 背景文件，各模块均已执行功能富集分析。", "GO/KEGG backgrounds were provided; enrichment was performed for every module.") + `</div>`)
	} else {
		b.WriteString(`<div class="notice">` + d.L("未提供 GO/KEGG 背景文件，本次未执行功能富集分析。", "No GO/KEGG backgrounds were provided; enrichment was skipped.") + `</div>`)
	}
	b.WriteString(`</section>`)

	// ---- step 2: 最终参数 ----
	b.WriteString(stepHead(d, 2, nav[1], d.L("这里记录实际运行使用的最终参数。", "Final parameters used by successful runs.")))
	b.WriteString(paramsHTML(d.Params, d.Lang))
	b.WriteString(`</section>`)

	// ---- step 3: 过滤 ----
	b.WriteString(stepHead(d, 3, nav[2], d.L("明确区分样本删除、低表达过滤和高变基因选择。", "Separate sample exclusion, low-expression filtering and variable-gene selection.")))
	b.WriteString(`<div class="story"><p>` + d.L("表达矩阵经归一化、低表达过滤和 MAD/VAR 高变基因选择后，保留前 ", "After normalization, low-expression filtering and MAD/VAR variable-gene selection, the top ") + fmt.Sprint(d.Genes) + d.L(" 个基因进入网络分析。", " genes enter network analysis.") + `</p></div>`)
	b.WriteString(`<figure><img src="../result/02_filter/sample_tree.png" alt="sample tree"><figcaption><span><b>` + d.L("图 1.", "Figure 1.") + `</b> ` + d.L("样本聚类树；色带为用户所选标签。", "Sample dendrogram with the selected annotation rows.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 4: 软阈值 ----
	b.WriteString(stepHead(d, 4, nav[3], d.L("先观察候选值，再记录最终采用值。", "Inspect candidates, then record the chosen power.")))
	b.WriteString(`<figure><img src="../result/03_soft_threshold/sft.png" alt="soft threshold"><figcaption><span><b>` + d.L("图 2.", "Figure 2.") + `</b> ` + d.L("无标度拟合 R² 与平均连通性曲线。", "Scale-free fit R² and mean connectivity.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 5: 网络与模块 ----
	b.WriteString(stepHead(d, 5, nav[4], d.L("将表达模式相似的基因分组。", "Group genes with similar expression patterns.")))
	b.WriteString(`<figure><img src="../result/04_network/dendrogram.png" alt="dendrogram"><figcaption><span><b>` + d.L("图 3.", "Figure 3.") + `</b> ` + d.L("基因聚类树与模块色带。", "Gene clustering and module assignments.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 6: 模块-性状 ----
	b.WriteString(stepHead(d, 6, nav[5], d.L("用模块特征向量概括模块表达。", "Summarize module expression using its eigengene.")))
	b.WriteString(`<figure><img src="../result/05_module_trait/mtrait.png" alt="module-trait"><figcaption><span><b>` + d.L("图 4.", "Figure 4.") + `</b> ` + d.L("模块-性状相关热图。", "Module–trait correlation heatmap.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 7: 模块总览 ----
	b.WriteString(stepHead(d, 7, nav[6], d.L("一张图看懂所有模块的表达趋势与功能。", "All modules' expression patterns and functions at a glance.")))
	b.WriteString(`<figure><img src="../result/module_overview.png" alt="module overview"><figcaption><span><b>` + d.L("图 5.", "Figure 5.") + `</b> ` + d.L("模块总览：左侧 eigengene 折线 + 中间 z-score 热图（按模块切分）+ 右侧富集术语。", "Module overview: eigengene sparklines + z-scored heatmap (split by module) + enrichment terms.") + `</span></figcaption></figure>`)
	b.WriteString(`<figure><img src="../result/overview_network.svg" alt="network"><figcaption><span><b>` + d.L("图 6.", "Figure 6.") + `</b> ` + d.L("全基因共表达网络（igraph DrL 布局，节点色=模块色、大小=degree、边色=weight）。", "Gene co-expression network (igraph DrL layout; color = module, size = degree, edge color = weight).") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 8: 各模块深入 ----
	b.WriteString(stepHead(d, 8, nav[7], d.L("每个模块的基因表现与功能线索。", "Gene-level patterns and functional context for every module.")))
	for _, m := range d.Modules {
		b.WriteString(`<h3>` + d.L("模块", "Module") + ` ` + html.EscapeString(m) + `</h3>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/gs_mm.png" alt="` + m + ` GS/MM"><figcaption><span><b>` + m + `.</b> ` + d.L("GS×MM 散点。", "GS vs module membership.") + `</span></figcaption></figure>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/expression_heatmap.png" alt="` + m + ` heatmap"><figcaption><span><b>` + m + `.</b> ` + d.L("模块基因表达热图。", "Module gene expression heatmap.") + `</span></figcaption></figure>`)
		if d.EnrichOK {
			b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/enrichment/go.png" alt="` + m + ` enrichment"><figcaption><span><b>` + m + `.</b> ` + d.L("GO 富集。", "GO enrichment.") + `</span></figcaption></figure>`)
		}
	}
	b.WriteString(`</section>`)

	// ---- step 9: Hub 基因 ----
	b.WriteString(stepHead(d, 9, nav[8], d.L("候选筛选与网络展示。", "Candidate selection and network display.")))
	if d.HasTrait {
		b.WriteString(`<div class="story"><p>` + d.L("存在与模块显著强相关（p &lt; 0.05 且 |cor| &gt; 0.6）的性状，Hub 基因同时参考 |GS| 与 |kME| 阈值。", "A significantly strong trait (p < 0.05 and |cor| > 0.6) exists; hubs use both |GS| and |kME| thresholds.") + `</p></div>`)
	} else {
		b.WriteString(`<div class="story"><p>` + d.L("未检测到显著强相关的性状，Hub 基因仅按 |kME| 阈值筛选。", "No significantly strong trait was found; hubs use |kME| alone.") + `</p></div>`)
	}
	b.WriteString(`</section>`)

	// ---- step 10: 文件与复现 ----
	b.WriteString(stepHead(d, 10, nav[9], d.L("回到原始图表、数据和分析设定。", "Keep figures, data and analysis settings within reach.")))
	b.WriteString(`<div class="download-row"><a class="button" href="../result/README.txt" download>` + d.L("下载读图说明", "Download reading guide") + `</a></div>`)
	b.WriteString(`</section>`)

	b.WriteString(`<footer class="footer"><span>© 2026 Xiao Wang Lab · rsWGCNA</span></footer></main>`)
	return b.String()
}

func stepHead(d reportData, i int, title, intro string) string {
	return fmt.Sprintf(`<section class="step" id="step-%d"><div class="step-head"><span class="step-no">%02d</span><h2>%s</h2></div><p class="step-intro">%s</p>`, i, i, title, intro)
}

func tableHTML(head []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<div class="table-wrap"><table><thead><tr>`)
	for _, h := range head {
		b.WriteString(`<th>` + html.EscapeString(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		b.WriteString(`<tr>`)
		for _, c := range row {
			b.WriteString(`<td>` + html.EscapeString(c) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func paramsHTML(params map[string]any, lang string) string {
	groupNames := map[string][2]string{
		"filter":         {"清洗过滤", "Filtering"},
		"soft_threshold": {"软阈值搜索", "Soft-threshold search"},
		"network":        {"网络构建", "Network construction"},
		"module_trait":   {"模块与性状", "Module–trait"},
		"module_detail":  {"模块深入", "Module detail"},
		"hub":            {"Hub 筛选与网络", "Hub selection & network"},
	}
	var b strings.Builder
	for _, group := range []string{"filter", "soft_threshold", "network", "module_trait", "module_detail", "hub"} {
		v, ok := params[group]
		if !ok {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		name := group
		if gn, ok := groupNames[group]; ok {
			if lang == "en" {
				name = gn[1]
			} else {
				name = gn[0]
			}
		}
		b.WriteString(`<details class="parameter" open><summary>` + name + `</summary>`)
		var rows [][]string
		for k, val := range m {
			j, _ := json.Marshal(val)
			rows = append(rows, []string{k, string(j)})
		}
		head := []string{"参数", "最终值"}
		if lang == "en" {
			head = []string{"Parameter", "Final value"}
		}
		b.WriteString(tableHTML(head, rows))
		b.WriteString(`</details>`)
	}
	return b.String()
}
