package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

//go:embed report_assets/design.css
var reportCSS []byte

//go:embed report_assets/design.js
var reportJS []byte

// reportData carries the real analysis facts used to build the report.
type reportData struct {
	KeyModules    []keyModule
	FigureSources map[string]string
	ProjectName   string
	Timestamp     string
	Version       string
	Lang          string // "zh" | "en"
	Samples       int
	Genes         int
	Modules       []string
	ModuleSizes   map[string]int
	GreyGenes     int
	Params        map[string]any
	HubCount      int
	HasTrait      bool
	EnrichOK      bool
	EnrichType    string
	Warnings      []string
	Assets        map[string]bool
	Networks      []reportNetworkSummary
	ClusterMethod string
	ClusterK      int
	HubModule     string
	HubTrait      string
	HubGenes      []reportHubGene
	Bridges       []netNode
	ModuleTraits  map[string]string
}

type reportHubGene struct {
	Gene string  `json:"gene"`
	Kme  float64 `json:"kme"`
	GS   float64 `json:"gs"`
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
		d.L("模块与聚类总览", "Module & clustering overview"),
		d.L("关键模块深入", "Key-module detail"),
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
	if len(d.Warnings) > 0 {
		b.WriteString(`<details class="notice" open><summary>` + d.L("导出提示：部分项目未生成或需核对", "Export notes: some items were not generated or need review") + `</summary><ul>`)
		for _, warning := range d.Warnings {
			b.WriteString(`<li>` + html.EscapeString(warning) + `</li>`)
		}
		b.WriteString(`</ul></details>`)
	}

	// ---- step 1: 输入与背景 ----
	b.WriteString(stepHead(d, 1, nav[0], d.L("先记录分析从哪里开始。", "Start with the source data.")))
	b.WriteString(tableHTML([]string{d.L("内容", "Contents"), d.L("数值", "Value")}, [][]string{
		{d.L("样本数", "Samples"), fmt.Sprint(d.Samples)},
		{d.L("进入网络分析的基因数", "Genes entering network"), fmt.Sprint(d.Genes)},
		{d.L("共表达模块数（不含 grey）", "Modules, excluding grey"), fmt.Sprint(nMod)},
		{d.L("grey 未分配基因数", "Unassigned grey genes"), fmt.Sprint(d.GreyGenes)},
	}))
	if d.EnrichOK {
		b.WriteString(`<div class="reading">` + d.L("已提供所选 "+strings.ToUpper(d.EnrichType)+" 富集背景；各模块结果见下文，无显著条目时只保留结果表，执行失败会列在导出提示中。", "The selected "+strings.ToUpper(d.EnrichType)+" background was provided. Module results are listed below; no significant terms yields a table without a plot. Failures appear in export notes.") + `</div>`)
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
	b.WriteString(filterStory(d))
	_ = d.L("", "")
	sampleTreeCap := d.L("根据这 "+fmtInt(d.Genes)+" 个基因的表达矩阵，样本聚类结果如下（色带为所选性状列）。", "Sample clustering based on the "+fmtInt(d.Genes)+"-gene matrix (annotation band = the selected trait column).")
	b.WriteString(`<figure><img src="../result/02_filter/sample_tree.png" alt="sample tree"><figcaption><span><b>` + d.L("图 1.", "Figure 1.") + `</b> ` + sampleTreeCap + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 4: 软阈值 ----
	b.WriteString(stepHead(d, 4, nav[3], d.L("先观察候选值，再记录最终采用值。", "Inspect candidates, then record the chosen power.")))
	{
		netType := pstr(d.Params, "soft_threshold", "network_type", "unsigned")
		rsq := pfloat(d.Params, "soft_threshold", "rsquared_cut", 0.85)
		power := pfloat(d.Params, "network", "power", 6)
		b.WriteString(`<div class="story"><p>` + d.L("软阈值搜索使用 "+html.EscapeString(netType)+" 网络，参考拟合阈值 R²="+fmtNum(rsq)+"；实际网络采用参数中的 <strong>β = "+fmtNum(power)+"</strong>。采用值可能由用户调整，应结合拟合曲线与连通性判断。", "The search uses a "+html.EscapeString(netType)+" network and reference fit threshold R²="+fmtNum(rsq)+". The actual network uses <strong>β = "+fmtNum(power)+"</strong> from the parameters, which may have been adjusted by the user. Interpret it alongside fit and connectivity.") + `</p></div>`)
	}
	b.WriteString(`<figure><img src="../result/03_soft_threshold/sft.png" alt="soft threshold"><figcaption><span><b>` + d.L("图 2.", "Figure 2.") + `</b> ` + d.L("无标度拟合 R² 与平均连通性曲线。", "Scale-free fit R² and mean connectivity.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 5: 网络与模块 ----
	b.WriteString(stepHead(d, 5, nav[4], d.L("将表达模式相似的基因分组。", "Group genes with similar expression patterns.")))
	{
		minSize := pint(d.Params, "network", "min_module_size", 30)
		mergeCut := pfloat(d.Params, "network", "merge_cut_height", 0.25)
		deepSplit := pint(d.Params, "network", "deep_split", 2)
		nMod := len(d.Modules)
		zh := fmt.Sprintf("根据拓扑重叠关系（TOM）做层次聚类与动态树切割，最小模块大小设为 %d，模块合并距离阈值 %s，deepSplit=%d。最终得到 <strong>%d 个模块</strong>，另有 %d 个 grey 未分配基因（不计入模块总数）。", minSize, fmtNum(mergeCut), deepSplit, nMod, d.GreyGenes)
		en := fmt.Sprintf("Genes are clustered by topological overlap (TOM) using hierarchical clustering and dynamic tree cutting. Minimum module size is %d; the merge distance threshold is %s; deepSplit=%d. This yields <strong>%d modules</strong> and %d unassigned grey genes (excluded from the module count).", minSize, fmtNum(mergeCut), deepSplit, nMod, d.GreyGenes)
		b.WriteString(`<div class="story"><p>` + d.L(zh, en) + `</p></div>`)
	}
	b.WriteString(`<figure><img src="../result/04_network/dendrogram.png" alt="dendrogram"><figcaption><span><b>` + d.L("图 3.", "Figure 3.") + `</b> ` + d.L("基因聚类树与模块色带。", "Gene clustering and module assignments.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 6: 模块-性状 ----
	b.WriteString(stepHead(d, 6, nav[5], d.L("用模块特征向量概括模块表达。", "Summarize module expression using its eigengene.")))
	{
		traitCol := html.EscapeString(pstr(d.Params, "module_trait", "trait_column", ""))
		traitSrc := html.EscapeString(pstr(d.Params, "module_trait", "trait_source", ""))
		var story string
		if traitCol == "" {
			story = d.L("模块特征向量（ME）概括每个模块的主要表达模式。热图中红偏正相关、蓝偏负相关，颜色越深绝对相关越强。", "A module eigengene (ME) summarizes each module's main expression pattern. In the heatmap, red is positive and blue is negative correlation; deeper colors indicate larger absolute correlations.")
		} else {
			story = d.L("模块特征向量（ME）概括每个模块的主要表达模式。本次的性状来自 "+traitSrc+" 的列 <strong>"+traitCol+"</strong>。热图中红偏正相关、蓝偏负相关，颜色越深绝对相关越强。", "A module eigengene (ME) summarizes each module's main expression pattern. Traits come from the <strong>"+traitCol+"</strong> column of "+traitSrc+". In the heatmap, red is positive and blue is negative correlation; deeper colors indicate larger absolute correlations.")
		}
		b.WriteString(`<div class="story"><p>` + story + `</p></div>`)
	}
	b.WriteString(`<figure><img src="../result/05_module_trait/mtrait.png" alt="module-trait"><figcaption><span><b>` + d.L("图 4.", "Figure 4.") + `</b> ` + d.L("模块-性状相关热图。", "Module–trait correlation heatmap.") + `</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 7: 模块总览 ----
	b.WriteString(stepHead(d, 7, nav[6], d.L("一张图看懂所有模块的表达趋势与功能。", "All modules' expression patterns and functions at a glance.")))
	b.WriteString(`<figure><img src="../result/module_overview.png" alt="module overview"><figcaption><span><b>` + d.L("图 5.", "Figure 5.") + `</b> ` + d.L("模块总览：左侧 eigengene 折线 + 中间 z-score 热图（按模块切分）+ 右侧富集术语。", "Module overview: eigengene sparklines + z-scored heatmap (split by module) + enrichment terms.") + `</span></figcaption></figure>`)
	b.WriteString(`<h3>` + d.L("聚类对照", "Clustering comparison") + `</h3>`)
	if d.ClusterMethod != "" {
		b.WriteString(`<p>` + html.EscapeString(d.ClusterMethod) + fmt.Sprintf(" · K=%d", d.ClusterK) + `</p><p>` + d.L("聚类结果独立于 WGCNA 模块，用于比较表达分组；如已计算肘部/有效性指标，可下载对应 JSON 查看。", "Clustering is independent of WGCNA module assignments and provides a comparison of expression groups. Elbow/validity metrics, when computed, are included as JSON.") + `</p><figure><img src="../result/08_clustering/cluster_overview.png" alt="cluster overview"><figcaption>` + d.L("聚类表达趋势、热图与富集总览。", "Cluster trends, heatmap and enrichment overview.") + `</figcaption></figure>`)
		b.WriteString(`<a href="../result/08_clustering/clusters.json" download>` + d.L("下载聚类数据", "Download clustering data") + `</a>`)
		if d.Assets["08_clustering/elbow.json"] {
			b.WriteString(` · <a href="../result/08_clustering/elbow.json" download>` + d.L("下载建议 K 指标", "Download K-selection metrics") + `</a>`)
		}
	} else {
		b.WriteString(`<div class="notice">` + d.L("本次未运行聚类对照，未自动补跑。", "Clustering comparison was not run for this analysis.") + `</div>`)
	}
	b.WriteString(`</section>`)

	// ---- step 8: 各模块深入 ----
	b.WriteString(stepHead(d, 8, nav[7], d.L("展示至少一个性状相关检验 p < 0.05 的模块；p 为未校正值，每个模块采用显著性状中 |cor| 最大者。", "Modules with at least one unadjusted trait-association p < 0.05; each uses its strongest significant trait.")))
	if len(d.KeyModules) == 0 {
		b.WriteString(`<div class="notice">` + d.L("未检测到满足 p < 0.05 的关键模块，因此仅展示全部模块总览。", "No key modules meet p < 0.05; the complete module overview is retained.") + `</div>`)
	}
	for _, keyModule := range d.KeyModules {
		m := keyModule.Module
		b.WriteString(`<h3>` + d.L("模块", "Module") + ` ` + html.EscapeString(m) + `</h3>`)
		if trait := d.ModuleTraits[m]; trait != "" {
			b.WriteString(`<p>` + d.L("GS 使用性状：", "GS trait: ") + html.EscapeString(trait) + d.L("（在显著关联性状中按 |cor| 最大者选择）。", " (the strongest significant module–trait association).") + `</p>`)
		}
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/gs_mm.png" alt="` + m + ` GS/MM"><figcaption><span><b>` + m + `.</b> ` + d.L("GS×MM 散点。", "GS vs module membership.") + `</span></figcaption></figure>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/expression_heatmap.png" alt="` + m + ` heatmap"><figcaption><span><b>` + m + `.</b> ` + d.L("模块基因表达热图。", "Module gene expression heatmap.") + `</span></figcaption></figure>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/hub_network.png" alt="` + m + ` hub network"><figcaption>` + d.L("关键模块 Hub 网络。", "Key-module Hub network.") + `</figcaption></figure>`)
		if d.EnrichOK {
			stem := "06_module_detail/" + m + "/enrichment/" + d.EnrichType
			if d.Assets[stem+".png"] {
				b.WriteString(`<figure><img src="../result/` + stem + `.png" alt="` + m + ` enrichment"><figcaption>` + strings.ToUpper(d.EnrichType) + `</figcaption></figure>`)
			} else {
				b.WriteString(`<p>` + d.L("无富集图：无显著条目或该项未生成；请查看结果表及导出提示。", "No enrichment figure: no significant terms, or the item was not generated. Check the results table and export notes.") + `</p>`)
			}
			if d.Assets[stem+".tsv"] {
				b.WriteString(`<a href="../result/` + stem + `.tsv" download>` + d.L("下载富集结果表", "Download enrichment table") + `</a>`)
			}
		}
	}
	b.WriteString(`</section>`)

	// ---- step 9: Hub 基因 ----
	b.WriteString(stepHead(d, 9, nav[8], d.L("候选筛选与网络展示。", "Candidate selection and network display.")))
	if d.HubModule == "" {
		b.WriteString(`<div class="notice">` + d.L("本次没有保存的 Hub 结果。", "No saved Hub result is available for this analysis.") + `</div>`)
	} else {
		b.WriteString(`<p>` + d.L("当前 Hub 结果的模块 / 性状：", "Module / trait for the saved Hub result: ") + html.EscapeString(d.HubModule) + " / " + html.EscapeString(d.HubTrait) + `</p>`)
		{
			kme := pfloat(d.Params, "hub", "kme_cutoff", 0.8)
			gs := pfloat(d.Params, "hub", "gs_cutoff", 0.5)
			if d.HasTrait {
				b.WriteString(`<div class="story"><p>` + d.L("当前选中模块存在显著强相关性状（p &lt; 0.05 且 |cor| &gt; 0.6），候选表需满足 <strong>|kME| &gt; "+fmtNum(kme)+"</strong> 且所选性状 <strong>|GS| &gt; "+fmtNum(gs)+"</strong>。", "The selected module has a significantly strong trait association (p &lt; 0.05 and |cor| &gt; 0.6). The candidate table requires <strong>|kME| &gt; "+fmtNum(kme)+"</strong> and <strong>|GS| &gt; "+fmtNum(gs)+"</strong> for the selected trait.") + `</p></div>`)
			} else {
				b.WriteString(`<div class="story"><p>` + d.L("未检测到显著强相关的性状，Hub 基因仅按 <strong>|kME| &gt; "+fmtNum(kme)+"</strong> 筛选（不看表型数据）。", "No significantly strong trait was found; hubs are selected by <strong>|kME| > "+fmtNum(kme)+"</strong> alone (phenotype data not used).") + `</p></div>`)
			}
		}
		var hubRows [][]string
		for _, gene := range d.HubGenes {
			hubRows = append(hubRows, []string{gene.Gene, fmt.Sprintf("%.4f", gene.Kme), fmt.Sprintf("%.4f", gene.GS)})
		}
		b.WriteString(tableHTML([]string{d.L("候选基因", "Candidate gene"), "|kME|", "|GS|"}, hubRows))
		b.WriteString(`<p>` + d.L("此处统计候选表条目；交互 Hub 网络的种子节点按 kME 与数量上限选择，二者不应混为同一个计数。", "This count refers to the candidate table. Seed nodes in the interactive Hub network use kME and the seed-count cap; the two counts are distinct.") + `</p>`)
		if d.Assets["07_hub/hub_network.png"] {
			b.WriteString(`<figure><img src="../result/07_hub/hub_network.png" alt="hub network"><figcaption>` + d.L("当前模块的 Hub 种子与邻居网络快照。", "Snapshot of Hub seeds and neighbors for the selected module.") + `</figcaption></figure>`)
		}
	}
	b.WriteString(`</section>`)

	// ---- step 10: 文件与复现 ----
	b.WriteString(stepHead(d, 10, nav[9], d.L("回到原始图表、数据和分析设定。", "Keep figures, data and analysis settings within reach.")))
	b.WriteString(`<div class="download-row"><a class="button" href="../result/README.txt" download>` + d.L("下载读图说明", "Download reading guide") + `</a></div>`)
	b.WriteString(`<p><a href="../result/00_provenance/parameters.final.json" download>` + d.L("分析参数", "Analysis parameters") + `</a> · <a href="../result/00_provenance/report-settings.json" download>` + d.L("报告设置", "Report settings") + `</a> · <a href="../result/00_provenance/export_manifest.tsv" download>` + d.L("文件清单", "File manifest") + `</a> · <a href="../result/00_provenance/export-warnings.json" download>` + d.L("导出提示", "Export notes") + `</a></p>`)
	b.WriteString(`</section>`)

	b.WriteString(`<footer class="footer"><span>© 2026 Xiao Wang Lab · rsWGCNA</span></footer></main>`)
	body := b.String()
	if len(d.KeyModules) == 0 {
		body = strings.Replace(body, "</section><!-- key-module-placeholder -->", "</section>", 1)
	}
	for key, origin := range d.FigureSources {
		note := d.L("分析过程快照", "Analysis snapshot")
		if origin == "existing-app-component" {
			note = d.L("缺少快照，使用 App 原绘图组件补绘", "No saved snapshot; rendered with the original App component")
		}
		marker := `src="../result/` + key + `"`
		body = strings.ReplaceAll(body, marker, marker+` data-figure-source="`+origin+`" title="`+note+`"`)
	}
	return availableReportFigures(body, d)
}

func networkModeName(d reportData, mode string) string {
	if mode == "intra" {
		return d.L("模块内（Within）", "Within modules")
	}
	if mode == "inter" {
		return d.L("模块间（Between）", "Between modules")
	}
	return d.L("模块内 + 模块间", "Within + Between")
}

var reportFigurePattern = regexp.MustCompile(`(?s)<figure><img src="\.\./result/([^"]+)".*?</figure>`)

func availableReportFigures(body string, d reportData) string {
	return reportFigurePattern.ReplaceAllStringFunc(body, func(figure string) string {
		match := reportFigurePattern.FindStringSubmatch(figure)
		if d.Assets[match[1]] {
			return figure
		}
		return `<div class="notice">` + d.L("此图未生成：", "Figure not generated: ") + html.EscapeString(match[1]) + `</div>`
	})
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
		"overview":       {"总览网络", "Overview network"},
		"cluster":        {"聚类对照", "Clustering comparison"},
	}
	var b strings.Builder
	for _, group := range []string{"filter", "soft_threshold", "network", "module_trait", "module_detail", "hub", "overview", "cluster"} {
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
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, k := range keys {
			val := m[k]
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

// ---- parameter extraction helpers ----

func pstr(params map[string]any, group, key, def string) string {
	if g, ok := params[group].(map[string]any); ok {
		if v, ok := g[key].(string); ok {
			return v
		}
	}
	return def
}

func pfloat(params map[string]any, group, key string, def float64) float64 {
	if g, ok := params[group].(map[string]any); ok {
		if v, ok := g[key].(float64); ok {
			return v
		}
	}
	return def
}

func pint(params map[string]any, group, key string, def int) int {
	return int(pfloat(params, group, key, float64(def)))
}

// normText describes the normalization method in plain language.
func normText(m, lang string) string {
	zh := map[string]string{
		"raw": "不做数值转换（保留原始值）", "vst": "Rust 方差稳定变换（VST）",
		"log10p1": "log10(x+1) 转换", "log": "log(x) 转换", "log10": "log10(x) 转换",
		"log2": "log2(x) 转换", "log2p1": "log2(x+1) 转换",
	}
	en := map[string]string{
		"raw": "no transformation (raw values)", "vst": "Rust variance-stabilizing transform (VST)",
		"log10p1": "log10(x+1)", "log": "log(x)", "log10": "log10(x)",
		"log2": "log2(x)", "log2p1": "log2(x+1)",
	}
	if lang == "en" {
		if v, ok := en[m]; ok {
			return v
		}
	} else if v, ok := zh[m]; ok {
		return v
	}
	return m
}

// filterMethodText describes MAD/VAR in plain language.
func filterMethodText(m, lang string) string {
	if lang == "en" {
		if m == "var" {
			return "variance (VAR)"
		}
		return "median absolute deviation (MAD)"
	}
	if m == "var" {
		return "方差（VAR）"
	}
	return "中位数绝对离差（MAD）"
}

// filterStory generates the plain-language filtering narrative from the actual
// filter parameters.
func filterStory(d reportData) string {
	p := d.Params
	norm := pstr(p, "filter", "normalize_method", "log10p1")
	cutoff := pfloat(p, "filter", "cutoff", 10)
	pct := pfloat(p, "filter", "sample_percentage", 0.5)
	method := pstr(p, "filter", "filter_method", "mad")
	nKeep := pint(p, "filter", "n_keep", 8000)

	normDesc := normText(norm, d.Lang)
	methodDesc := filterMethodText(method, d.Lang)

	story1 := d.L(
		"表达矩阵先进行归一化：本分析采用 "+normDesc+"。",
		"The expression matrix is first normalized: this analysis uses "+normDesc+".",
	)
	var story2 string
	if pct > 1 {
		story2 = d.L("样本比例参数为 "+fmtNum(pct)+"，超出引擎要求的0–1范围。按本次引擎行为，该阈值不会删除低表达基因；请核对参数并重新分析。", "The sample-percentage parameter is "+fmtNum(pct)+", outside the engine's 0–1 range. With this value, the engine does not remove low-expression genes. Review the parameter and rerun the analysis.")
	} else if pct > 0 {
		story2 = d.L(
			"低表达过滤：如果一个基因在超过 "+fmtPercent(pct)+" 的样本中表达值低于 "+fmtNum(cutoff)+"，就被当作低表达噪音基因去除。",
			"Low-expression filtering: a gene is removed as noise if its expression is below "+fmtNum(cutoff)+" in more than "+fmtPercent(pct)+" of samples.",
		)
	} else {
		story2 = d.L(
			"本次未启用低表达过滤（样本比例阈值设为 0）。",
			"Low-expression filtering was not enabled (the sample-percentage threshold is 0).",
		)
	}
	story3 := d.L(
		"高变基因选择：剩余基因按 "+methodDesc+" 从大到小排序，选取前 "+fmtInt(nKeep)+" 个基因，作为后续分析的最终表达矩阵。",
		"Variable-gene selection: the remaining genes are ranked by "+methodDesc+" and the top "+fmtInt(nKeep)+" enter downstream analysis.",
	)
	return `<div class="story"><p>` + story1 + `</p></div><div class="story"><p>` + story2 + `</p></div><div class="story"><p>` + story3 + `</p></div>`
}

func fmtPercent(v float64) string {
	return fmt.Sprintf("%g%%", v*100)
}

func fmtNum(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%g", v)
}

func fmtInt(v int) string {
	return fmt.Sprintf("%d", v)
}
