package api

import (
	_ "embed"
	"encoding/json"
	"encoding/base64"
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

//go:embed report_assets/report-charts.js
var reportChartsJS []byte

//go:embed report_assets/echarts.min.js
var echartsJS []byte

//go:embed report_assets/rsWGCNA_icon.svg
var reportIcon []byte

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
	SampleTreeJSON string // raw {samples, tree, bands} for the interactive sample tree
	SftJSON        string // raw soft-threshold data for the interactive SFT figures
	ModulesJSON    string // raw {genes, colors, module_sizes, dendrogram} for the network section
	ModuleTraitJSON string // raw module-trait data (modules/traits/cor/p/eigengenes/genes/kme + trait_source/column)
	PairPanelsJSON  string // significant module–trait pairs with genes/MM/GS for the tab panels
	HeatmapDataURLs map[string]string // key "module_trait" -> base64 data URL for the z-score heatmap
	ModuleEnrichJSON string // per-module GO/KEGG enrichment + gene annotation for function exploration

	// input-file summary (project introduction)
	RawSamples     int
	RawGenes       int
	TraitCount     int
	SampleInfoCols []string
	AnnoGenes      int
	GoGenes        int
	GoEntries      int
	KeggGenes      int
	KeggEntries    int
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
		d.Version = "v0.6.0"
	}
	if d.Lang == "" {
		d.Lang = "zh"
	}
	var b strings.Builder

	// ---- rail (toc) ----
	nav := []string{
		d.L("项目介绍", "Project introduction"),
		d.L("第一步 · 数据清洗", "Step 1 · Data cleaning"),
		d.L("软阈值筛选", "Soft-threshold selection"),
		d.L("网络构建与模块划分", "Network construction & modules"),
		d.L("模块-性状", "Module–trait"),
		d.L("模块总览", "Module overview"),
	}
	b.WriteString(`<aside class="rail"><a class="brand" href="#"><img class="report-brand-icon" alt="" src="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString(reportIcon) + `">rs<em>WGCNA</em><span style="font-size:10px;letter-spacing:0;margin-left:8px">REPORT</span></a><div class="eyebrow">` + d.L("分析记录", "Analysis record") + `</div><nav>`)
	for i, n := range nav {
		cls := ""
		if i == 0 {
			cls = ` class="active"`
		}
		b.WriteString(fmt.Sprintf(`<a href="#step-%d"%s><span>%02d</span>%s</a>`, i+1, cls, i+1, n))
	}
	b.WriteString(`</nav><div class="rail-bottom">` + d.L("rsWGCNA 分析报告", "rsWGCNA analysis report") + `<a href="https://xwang-lab-sxau.cn/" target="_blank" rel="noopener">Xiao Wang Lab · Personal laboratory ↗</a></div></aside><main class="report">`)

	// ---- hero ----
	b.WriteString(`<div class="topline"><div class="eyebrow">` + d.L("加权基因共表达网络分析", "Weighted gene co-expression network analysis") + `</div><div class="actions"><button data-print>` + d.L("打印 / 保存 PDF", "Print / Save PDF") + `</button></div></div>`)
	b.WriteString(`<header class="hero"><div class="eyebrow">` + html.EscapeString(d.ProjectName) + `</div><h1>` + d.L("基因共表达网络分析报告", "Gene co-expression network analysis report") + `</h1><p>` + d.L("本报告汇总输入数据、预处理参数、网络构建、模块识别及功能分析结果，并提供交互图表与结果表格。", "This report summarizes input data, preprocessing parameters, network construction, module detection and functional analysis, with interactive figures and result tables.") + `</p><div class="project-line">` + html.EscapeString(d.ProjectName) + ` · ` + d.Timestamp + ` · rsWGCNA ` + d.Version + `</div><div class="metrics">`)
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.RawSamples, d.L("样本", "samples")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.RawGenes, d.L("原始基因", "raw genes")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, len(d.Modules), d.L("共表达模块", "modules")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div></div></header>`, d.HubCount, d.L("Hub 候选", "hub candidates")))

	// ---- step 1: 项目介绍 ----
	b.WriteString(stepHead(d, 1, nav[0], d.L("本次分析使用的输入数据及注释资源。", "Input files used in this analysis.")))
	introRows := [][]string{
		{d.L("表达量矩阵", "Expression matrix"), d.L(fmt.Sprintf("%d 个样本 × %d 个基因", d.RawSamples, d.RawGenes), fmt.Sprintf("%d samples × %d genes", d.RawSamples, d.RawGenes))},
	}
	if d.TraitCount > 0 {
		introRows = append(introRows, []string{d.L("表型数据矩阵", "Trait matrix"), d.L(fmt.Sprintf("%d 个样本 × %d 个 trait", d.RawSamples, d.TraitCount), fmt.Sprintf("%d samples × %d traits", d.RawSamples, d.TraitCount))})
	}
	if len(d.SampleInfoCols) > 0 {
		introRows = append(introRows, []string{d.L("样本信息表", "Sample info"), d.L(fmt.Sprintf("%d 个样本，字段：%s", d.RawSamples, strings.Join(d.SampleInfoCols, ", ")), fmt.Sprintf("%d samples, fields: %s", d.RawSamples, strings.Join(d.SampleInfoCols, ", ")))})
	}
	if d.AnnoGenes > 0 {
		introRows = append(introRows, []string{d.L("基因注释", "Annotation"), fmt.Sprintf("%d %s", d.AnnoGenes, d.L("个基因", "genes"))})
	}
	if d.GoEntries > 0 {
		introRows = append(introRows, []string{"Gene2GO", fmt.Sprintf("%d %s · %d %s", d.GoGenes, d.L("个基因", "genes"), d.GoEntries, d.L("条对应条目", "entries"))})
	}
	if d.KeggEntries > 0 {
		introRows = append(introRows, []string{"Gene2KEGG", fmt.Sprintf("%d %s · %d %s", d.KeggGenes, d.L("个基因", "genes"), d.KeggEntries, d.L("条对应条目", "entries"))})
	}
	b.WriteString(tableHTML([]string{d.L("输入文件", "Input file"), d.L("内容", "Contents")}, introRows))
	b.WriteString(`</section>`)

	// ---- step 2: 第一步数据清洗 ----
	b.WriteString(stepHead(d, 2, nav[1], d.L("低表达过滤与高变基因选择。", "Low-expression filtering and variable-gene selection.")))
	p := d.Params
	dataType := pstr(p, "filter", "data_type", "count")
	norm := pstr(p, "filter", "normalize_method", "vst")
	cutoff := pfloat(p, "filter", "cutoff", 10)
	pct := pfloat(p, "filter", "sample_percentage", 0.5)
	method := pstr(p, "filter", "filter_method", "mad")
	nKeep := pint(p, "filter", "n_keep", 8000)
	dataTypeLabel := map[string]string{
		"count": d.L("Count（原始计数）", "Count (raw counts)"),
		"expected_count": d.L("Expected count（期望计数）", "Expected count"),
		"normalized_count": d.L("Normalized count（已归一化计数）", "Normalized count"),
		"peak_area": d.L("Peak area（峰面积）", "Peak area"),
		"protein_abundance": d.L("Protein abundance（蛋白丰度）", "Protein abundance"),
	}[dataType]
	if dataTypeLabel == "" {
		dataTypeLabel = dataType
	}
	paramRows := [][]string{
		{d.L("Data type", "Data type"), dataTypeLabel, d.L("输入表达矩阵的数据类型", "Input expression data type")},
		{d.L("Genes to keep", "Genes to keep"), fmtInt(nKeep), d.L("数据清洗后保留的基因数量", "Genes kept after filtering")},
		{d.L("Filter method", "Filter method"), method, d.L("选择中位数绝对离差（MAD）作为筛选标准", "Median absolute deviation used to rank genes")},
		{d.L("Sample %", "Sample %"), fmtPercent(pct), d.L("低表达基因在样本中的比例阈值", "Sample-percentage threshold for low-expression removal")},
		{"cutoff", fmtNum(cutoff), d.L("表达量小于该阈值被认为噪音", "Expression below this is treated as noise")},
		{"Normalization", norm, d.L("低表达过滤之后应用的归一化方法", "Normalization applied after low-expression filtering")},
	}
	b.WriteString(tableHTML([]string{d.L("参数名称", "Parameter"), d.L("使用参数", "Value"), d.L("参数解释", "Meaning")}, paramRows))
	b.WriteString(filterStory(d))
	if d.SampleTreeJSON != "" {
		sampleTreeCap := d.L("样本聚类树：点击上方 sample_info 字段（如 group）可切换对应的注释色带。", "Sample clustering: click a sample_info field above to toggle its annotation band.")
		b.WriteString(`<figure><div id="sample-tree-controls" class="st-controls"></div><div id="sample-tree" style="height:560px;min-height:320px"></div><script type="application/json" id="sample-tree-data">` + d.SampleTreeJSON + `</script><figcaption><span><b>` + d.L("图 1.", "Figure 1.") + `</b> ` + sampleTreeCap + `</span></figcaption></figure>`)
	}
	b.WriteString(`</section>`)

	// ---- step 3: 软阈值筛选 ----
	if d.SftJSON != "" {
		b.WriteString(stepHead(d, 3, nav[2], d.L("软阈值搜索与无标度拓扑检验。", "Soft-threshold search and scale-free topology check.")))
		nt := pstr(d.Params, "soft_threshold", "network_type", "unsigned")
		rsqCut := pfloat(d.Params, "soft_threshold", "rsquared_cut", 0.85)
		power := pfloat(d.Params, "network", "power", 6)
		sftRows := [][]string{
			{d.L("Network type", "Network type"), nt, d.L("网络类型（signed / unsigned / signed hybrid）", "Network type (signed / unsigned / signed hybrid)")},
			{d.L("R² cutoff", "R² cutoff"), fmtNum(rsqCut), d.L("无标度拟合阈值", "Scale-free fit threshold")},
			{d.L("Power", "Power"), fmtNum(power), d.L("最终采用的软阈值", "Final soft-threshold power")},
		}
		b.WriteString(tableHTML([]string{d.L("参数名称", "Parameter"), d.L("使用参数", "Value"), d.L("参数解释", "Meaning")}, sftRows))
		b.WriteString(sftStory(d))
		b.WriteString(`<div class="row g-3"><div class="col-md-6"><div id="sft-rsq" style="height:380px"></div></div><div class="col-md-6"><div id="sft-mean-k" style="height:380px"></div></div><div class="col-md-6"><div id="sft-hist" style="height:380px"></div></div><div class="col-md-6"><div id="sft-topology" style="height:380px"></div></div></div><script type="application/json" id="sft-data">` + d.SftJSON + `</script>`)
		b.WriteString(sftConclusion(d))
		b.WriteString(`</section>`)
	} else {
		b.WriteString(stepHead(d, 3, nav[2], d.L("软阈值筛选。", "Soft-threshold selection.")))
		b.WriteString(`<div class="notice">` + d.L("未执行软阈值筛选，无法生成此章节。", "Soft-threshold selection was not run.") + `</div>`)
		b.WriteString(`</section>`)
	}

	// ---- step 4: 网络构建模块划分 ----
	if d.ModulesJSON != "" {
		b.WriteString(stepHead(d, 4, nav[3], d.L("网络构建与模块划分。", "Network construction and module assignment.")))
		power := pfloat(d.Params, "network", "power", 6)
		nt := pstr(d.Params, "network", "network_type", "unsigned")
		tomType := pstr(d.Params, "network", "tom_type", "signed")
		tomDenom := pstr(d.Params, "network", "tom_denom", "min")
		minSize := pint(d.Params, "network", "min_module_size", 30)
		mergeCut := pfloat(d.Params, "network", "merge_cut_height", 0.25)
		deepSplit := pint(d.Params, "network", "deep_split", 2)
		detectCut := pfloat(d.Params, "network", "detect_cut_height", 0.995)
		netRows := [][]string{
			{"Network type", nt, d.L("网络类型（signed / unsigned / signed hybrid）", "Network type (signed / unsigned / signed hybrid)")},
			{"Power", fmtNum(power), d.L("软阈值幂次", "Soft-threshold power")},
			{"TOM type", tomType, d.L("拓扑重叠矩阵类型", "Topological overlap matrix type")},
			{"TOM denominator", tomDenom, d.L("TOM 分母（min / mean）", "TOM denominator (min / mean)")},
			{d.L("Min module size", "Min module size"), fmtInt(minSize), d.L("最小模块基因数", "Minimum module size")},
			{d.L("Merge cut height", "Merge cut height"), fmtNum(mergeCut), d.L("模块合并距离阈值", "Module merge distance threshold")},
			{d.L("Deep split", "Deep split"), fmtInt(deepSplit), d.L("动态树切割灵敏度", "Dynamic tree-cut sensitivity")},
			{d.L("Detect cut height", "Detect cut height"), fmtNum(detectCut), d.L("模块检测切割高度", "Module detection cut height")},
		}
		b.WriteString(tableHTML([]string{d.L("参数名称", "Parameter"), d.L("使用参数", "Value"), d.L("参数解释", "Meaning")}, netRows))
		b.WriteString(moduleBadgesHTML(d))
		b.WriteString(`<div id="gene-dendrogram" style="height:520px"></div><script type="application/json" id="modules-data">` + d.ModulesJSON + `</script>`)
		b.WriteString(geneModuleTableHTML(d))
		b.WriteString(`</section>`)
	} else {
		b.WriteString(stepHead(d, 4, nav[3], d.L("网络构建与模块划分。", "Network construction and modules.")))
		b.WriteString(`<div class="notice">` + d.L("未执行网络构建，无法生成此章节。", "Network construction was not run.") + `</div>`)
		b.WriteString(`</section>`)
	}

	// ---- step 5: 模块-性状 ----
	if d.ModuleTraitJSON != "" {
		b.WriteString(stepHead(d, 5, nav[4], d.L("模块与表型性状的相关性。", "Module–trait correlations.")))
		b.WriteString(moduleTraitStatement(d))
		b.WriteString(moduleTraitTableHTML(d))
		b.WriteString(`<div id="mtrait-heatmap" style="height:460px"></div><script type="application/json" id="mtrait-data">` + d.ModuleTraitJSON + `</script>`)
		b.WriteString(significantCorrHTML(d))
		b.WriteString(pairPanelsHTML(d))
		b.WriteString(mmTableHTML(d))
		b.WriteString(moduleEnrichHTML(d))
		b.WriteString(`</section>`)
	} else {
		b.WriteString(stepHead(d, 5, nav[4], d.L("模块-性状。", "Module–trait.")))
		b.WriteString(`<div class="notice">` + d.L("未执行模块-性状分析，无法生成此章节。", "Module–trait analysis was not run.") + `</div>`)
		b.WriteString(`</section>`)
	}

	// ---- step 6: 模块总览 ----
	b.WriteString(stepHead(d, 6, nav[5], d.L("所有模块的表达趋势、热图与功能富集总览；用 ALL/MF/BP/CC/KEGG 切换富集视角。", "Expression trends, heatmap and enrichment overview for all modules; switch the enrichment view with ALL/MF/BP/CC/KEGG.")))
	b.WriteString(moduleOverviewHTML(d))
	b.WriteString(`</section>`)

	// ---- inline chart init (calls the shared reportCharts bundle) ----
	b.WriteString(reportChartsInlineJS(d))

	return b.String()
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
			"低表达过滤：如果一个基因在超过 "+fmtPercent(pct)+" 的样本中表达值低于 "+fmtNum(cutoff)+"，则按低表达过滤条件移除。",
			"Low-expression filtering: a gene is excluded by the low-expression filter if its expression is below "+fmtNum(cutoff)+" in more than "+fmtPercent(pct)+" of samples.",
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

// sftStory generates the plain-language soft-threshold narrative.
func sftStory(d reportData) string {
	nt := pstr(d.Params, "soft_threshold", "network_type", "unsigned")
	rsqCut := pfloat(d.Params, "soft_threshold", "rsquared_cut", 0.85)
	power := pfloat(d.Params, "network", "power", 6)
	powerEstimate := power
	powerSource := "estimate"
	var pe map[string]any
	if json.Unmarshal([]byte(d.SftJSON), &pe) == nil {
		if v, ok := pe["power_estimate"].(float64); ok && v > 0 {
			powerEstimate = v
		}
		if s, ok := pe["power_source"].(string); ok {
			powerSource = s
		}
	}
	if powerSource == "experience" {
		return `<div class="story"><p>` + d.L(
			"本次分析使用 "+nt+" 网络。分析结果表明，没有任何 soft threshold 的 R² 超过阈值线 "+fmtNum(rsqCut)+"，因此使用 WGCNA 经验推荐 power = "+fmtNum(powerEstimate)+"。",
			"This analysis used a "+nt+" network. No soft threshold reached the R² cutoff of "+fmtNum(rsqCut)+", so the WGCNA experience-recommended power = "+fmtNum(powerEstimate)+" was used.",
		) + `</p></div>`
	}
	return `<div class="story"><p>` + d.L(
		"本次分析使用 "+nt+" 网络。当 power = "+fmtNum(powerEstimate)+" 时 R² 超过阈值线 "+fmtNum(rsqCut)+"，因此推荐 power = "+fmtNum(powerEstimate)+"。",
		"This analysis used a "+nt+" network. At power = "+fmtNum(powerEstimate)+", R² exceeded the cutoff "+fmtNum(rsqCut)+", so power = "+fmtNum(powerEstimate)+" is recommended.",
	) + `</p></div>`
}

// sftConclusion evaluates the scale-free topology at the chosen power from the
// log-log fit R² and slope.
func sftConclusion(d reportData) string {
	var sc struct {
		Power float64
		Slope float64
		AdjR2 float64
	}
	var pe map[string]any
	if json.Unmarshal([]byte(d.SftJSON), &pe) != nil {
		return ""
	}
	if sfc, ok := pe["scale_free_check"].(map[string]any); ok {
		if v, ok := sfc["power"].(float64); ok {
			sc.Power = v
		}
		if v, ok := sfc["slope"].(float64); ok {
			sc.Slope = v
		}
		if v, ok := sfc["adj_r2"].(float64); ok {
			sc.AdjR2 = v
		}
	}
	if sc.Slope == 0 {
		return ""
	}
	gamma := -sc.Slope
	var verdict string
	if sc.AdjR2 >= 0.8 && gamma >= 0.5 && gamma <= 3 {
		verdict = d.L("当前拟合指标及斜率与设定的无标度拓扑筛选条件相符。该诊断不能单独证明网络服从幂律分布或具有生物学合理性。", "the fit and slope meet the specified scale-free topology screening criteria. This diagnostic alone does not establish a power-law distribution or biological validity.")
	} else if sc.AdjR2 >= 0.5 {
		verdict = d.L("R² 或斜率未同时满足设定的筛选条件，需结合数据质量与下游模块结果进行评估。", "R² and the slope do not jointly meet the specified screening criteria. Assess these diagnostics together with data quality and downstream module results.")
	} else {
		verdict = d.L("拟合较弱，网络拓扑可能不完全符合无标度假设，建议增大 power 或检查数据。", "the fit is weak; the topology may not fully satisfy the scale-free assumption — consider a higher power or checking the data.")
	}
	return `<div class="story"><p>` + d.L(
		fmt.Sprintf("无标度拓扑检验：当 power = %s 时，log10(p(k)) 与 log10(k) 的拟合 scale R² = %s、slope = %s（γ ≈ %s）。", fmtNum(sc.Power), fmt.Sprintf("%.2f", sc.AdjR2), fmt.Sprintf("%.2f", sc.Slope), fmt.Sprintf("%.2f", gamma)),
		fmt.Sprintf("Scale-free topology check: at power = %s, the log10(p(k))–log10(k) fit has scale R² = %s and slope = %s (γ ≈ %s).", fmtNum(sc.Power), fmt.Sprintf("%.2f", sc.AdjR2), fmt.Sprintf("%.2f", sc.Slope), fmt.Sprintf("%.2f", gamma)),
	) + verdict + `</p></div>`
}

// reportChartsInlineJS emits the inline <script> that initialises every chart by
// calling the shared reportCharts bundle (same functions the App uses).
func reportChartsInlineJS(d reportData) string {
	var b strings.Builder
	b.WriteString(`<script>(function(){`)
	// sample tree
	b.WriteString(`var stEl=document.getElementById('sample-tree-data');if(stEl){var st=JSON.parse(stEl.textContent);var stChart=echarts.init(document.getElementById('sample-tree'));var shown=st.bands.map(function(){return true});function renderST(){var active=st.bands.filter(function(_,i){return shown[i]}).map(function(b){return {name:b.name,values:b.values}});stChart.setOption(reportCharts.sampleTreeOption(st.samples,st.tree,active),true)}renderST();var ctrl=document.getElementById('sample-tree-controls');if(ctrl&&st.bands.length){function renderCtrl(){ctrl.innerHTML='';st.bands.forEach(function(band,i){var b=document.createElement('button');b.type='button';b.textContent=band.name;b.className='st-key'+(shown[i]?' on':'');b.onclick=function(){shown[i]=!shown[i];renderST();renderCtrl()};ctrl.appendChild(b)})}renderCtrl()}window.addEventListener('resize',function(){stChart.resize()})}`)
	// sft
	b.WriteString(`var sftEl=document.getElementById('sft-data');if(sftEl){var sft=JSON.parse(sftEl.textContent);var c1=echarts.init(document.getElementById('sft-rsq'));c1.setOption(reportCharts.scaleIndependenceOption(sft.fit_indices,sft.rsquared_cut,'` + d.L("无标度独立性", "Scale independence") + `'));var c2=echarts.init(document.getElementById('sft-mean-k'));c2.setOption(reportCharts.meanConnectivityOption(sft.fit_indices,'` + d.L("平均连通性", "Mean connectivity") + `'));var c3=null,c4=null;if(sft.scale_free_check){c3=echarts.init(document.getElementById('sft-hist'));c3.setOption(reportCharts.sftHistogramOption(sft.scale_free_check,'` + d.L("连通性直方图", "Connectivity histogram") + `','` + d.L("频数", "Frequency") + `'));c4=echarts.init(document.getElementById('sft-topology'));c4.setOption(reportCharts.sftLogLogOption(sft.scale_free_check,'` + d.L("无标度拓扑检验", "Scale-free topology check") + `'))}window.addEventListener('resize',function(){[c1,c2,c3,c4].forEach(function(c){c&&c.resize()})})}`)
	// gene dendrogram (network/module)
	b.WriteString(`var modEl=document.getElementById('modules-data');if(modEl){var mod=JSON.parse(modEl.textContent);var gd=echarts.init(document.getElementById('gene-dendrogram'));gd.setOption(reportCharts.geneDendrogramOption(mod.genes,mod.colors,mod.dendrogram));window.addEventListener('resize',function(){gd.resize()})}`)
	// module-trait heatmap
	b.WriteString(`var mtEl=document.getElementById('mtrait-data');if(mtEl){var mt=JSON.parse(mtEl.textContent);var mh=echarts.init(document.getElementById('mtrait-heatmap'));mh.setOption(reportCharts.moduleTraitHeatmapOption(mt.modules,mt.traits,mt.cor,mt.p));window.addEventListener('resize',function(){mh.resize()})}`)
	b.WriteString(`})();</script>`)
	return b.String()
}

// wgcnaHex returns the hex color for a WGCNA module name (matches the App's
// MODULE_COLORS map), falling back to a neutral grey.
func wgcnaHex(name string) string {
	m := map[string]string{
		"turquoise": "#1f9e89", "blue": "#3182bd", "brown": "#8c510a", "yellow": "#e6ab02",
		"green": "#66a61e", "red": "#e41a1c", "black": "#252525", "pink": "#f781bf",
		"magenta": "#984ea3", "purple": "#6a51a3", "greenyellow": "#a6d854", "tan": "#b89b74",
		"salmon": "#fa8072", "cyan": "#00ced1", "midnightblue": "#191970", "grey": "#999999",
		"grey60": "#999999", "lightgreen": "#90ee90", "lightyellow": "#ffffe0", "royalblue": "#4169e1",
		"darkred": "#8b0000", "darkgreen": "#006400", "darkturquoise": "#00ced1", "darkgrey": "#a9a9a9",
		"orange": "#ffa500", "darkorange": "#ff8c00", "skyblue": "#87ceeb", "saddlebrown": "#8b4513",
		"steelblue": "#4682b4", "paleturquoise": "#afeeee", "violet": "#ee82ee", "darkolivegreen": "#556b2f",
		"darkmagenta": "#8b008b", "sienna3": "#cd5c5c", "yellowgreen": "#9acd32", "skyblue3": "#6ca6cd",
		"plum1": "#ffbbff", "orangered4": "#801700", "mediumpurple3": "#8968cd", "lightsteelblue1": "#cae1ff",
		"lightcyan1": "#e0ffff", "ivory": "#fffff0", "floralwhite": "#fffaf0", "darkorange2": "#ee7600",
		"brown4": "#8b2323", "bisque4": "#8b7d6b", "darkslateblue": "#483d8b", "plum2": "#eeaeee",
		"thistle2": "#eed2ee", "lightcyan": "#e0ffff", "white": "#ffffff",
	}
	if v, ok := m[name]; ok {
		return v
	}
	return "#cccccc"
}

// moduleBadgesHTML renders the module–gene-count badges (colored dot + name + count).
func moduleBadgesHTML(d reportData) string {
	var mod struct {
		ModuleSizes map[string]int `json:"module_sizes"`
	}
	if json.Unmarshal([]byte(d.ModulesJSON), &mod) != nil {
		return ""
	}
	type row struct {
		name  string
		count int
	}
	rows := make([]row, 0, len(mod.ModuleSizes))
	for name, count := range mod.ModuleSizes {
		if name == "grey" || name == "grey60" {
			continue
		}
		rows = append(rows, row{name, count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].count > rows[j].count })
	var b strings.Builder
	b.WriteString(`<div class="d-flex flex-wrap gap-2 my-3">`)
	for _, r := range rows {
		hex := wgcnaHex(r.name)
		b.WriteString(`<span style="display:inline-flex;align-items:stretch;border-radius:4px;overflow:hidden;font-size:12px;line-height:1;box-shadow:0 1px 2px rgba(0,0,0,.12)">` +
			`<span style="background:` + hex + `;color:` + contrastText(hex) + `;padding:5px 9px;font-weight:600">` + html.EscapeString(r.name) + `</span>` +
			`<span style="background:rgba(0,0,0,.30);color:#fff;padding:5px 9px">` + fmtInt(r.count) + `</span>` +
			`</span>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// contrastText picks white or near-black text for a given background hex, so
// module names stay readable on light colors (yellow, pink, grey).
func contrastText(hex string) string {
	var r, g, bb int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &bb); err != nil {
		return "#fff"
	}
	// perceived luminance (sRGB)
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bb)
	if lum > 160 {
		return "#1a1a1a"
	}
	return "#fff"
}

// geneModuleTableHTML renders the Gene → Module table (scrollable).
func geneModuleTableHTML(d reportData) string {
	var mod struct {
		Genes  []string `json:"genes"`
		Colors []string `json:"colors"`
	}
	if json.Unmarshal([]byte(d.ModulesJSON), &mod) != nil {
		return ""
	}
	rows := make([][]string, 0, len(mod.Genes))
	for i, g := range mod.Genes {
		c := ""
		if i < len(mod.Colors) {
			c = mod.Colors[i]
		}
		if c == "grey" || c == "grey60" {
			continue
		}
		rows = append(rows, []string{g, c})
	}
	return tableHTML([]string{d.L("Gene", "Gene"), d.L("Module", "Module")}, rows)
}

// moduleTraitStatement explains where the traits came from.
func moduleTraitStatement(d reportData) string {
	var mt struct {
		TraitSource string   `json:"trait_source"`
		TraitColumn string   `json:"trait_column"`
		Traits      []string `json:"traits"`
	}
	if json.Unmarshal([]byte(d.ModuleTraitJSON), &mt) != nil {
		return ""
	}
	if mt.TraitSource == "sample_info" {
		return `<div class="story"><p>` + d.L(
			"表型性状来源：本次使用样本信息表（sample_info）中的 "+mt.TraitColumn+" 列生成表型矩阵，共 "+fmtInt(len(mt.Traits))+" 个水平："+strings.Join(mt.Traits, "、")+"。",
			"Trait source: the trait matrix was generated from the "+mt.TraitColumn+" column of the sample-info table, with "+fmtInt(len(mt.Traits))+" levels: "+strings.Join(mt.Traits, ", ")+".",
		) + `</p></div>`
	}
	return `<div class="story"><p>` + d.L(
		"表型性状来源：本次使用用户上传的表型矩阵（trait table）。",
		"Trait source: the user-uploaded trait table.",
	) + `</p></div>`
}

// moduleTraitTableHTML renders the sample → trait-value table (when available).
func moduleTraitTableHTML(d reportData) string {
	var mt struct {
		TraitTable  []map[string]string `json:"trait_table"`
		TraitColumn string              `json:"trait_column"`
	}
	if json.Unmarshal([]byte(d.ModuleTraitJSON), &mt) != nil || len(mt.TraitTable) == 0 {
		return ""
	}
	head := []string{d.L("样本", "Sample"), mt.TraitColumn}
	rows := make([][]string, 0, len(mt.TraitTable))
	for _, t := range mt.TraitTable {
		rows = append(rows, []string{t["sample"], t["value"]})
	}
	return `<h3>` + d.L("表型表格", "Trait table") + `</h3>` + tableHTML(head, rows)
}

// significantCorrHTML lists the significant module–trait relationships.
func significantCorrHTML(d reportData) string {
	var mt struct {
		Modules []string    `json:"modules"`
		Traits  []string    `json:"traits"`
		Cor     [][]float64 `json:"cor"`
		P       [][]float64 `json:"p"`
	}
	if json.Unmarshal([]byte(d.ModuleTraitJSON), &mt) != nil {
		return ""
	}
	head := []string{d.L("模块", "Module"), d.L("性状", "Trait"), "cor", "p-value"}
	var rows [][]string
	for i, mod := range mt.Modules {
		for j, tr := range mt.Traits {
			cor := mt.Cor[i][j]
			p := mt.P[i][j]
			if (cor > 0.65 || cor < -0.65) && p < 0.05 {
				rows = append(rows, []string{mod, tr, fmt.Sprintf("%.3f", cor), fmt.Sprintf("%.4f", p)})
			}
		}
	}
	if len(rows) == 0 {
		return `<div class="notice">` + d.L("没有 |cor| > 0.65 且 p < 0.05 的显著相关关系。", "No significant relationships with |cor| > 0.65 and p < 0.05.") + `</div>`
	}
	return `<h3>` + d.L("显著相关关系（|cor| > 0.65，p < 0.05）", "Significant relationships (|cor| > 0.65, p < 0.05)") + `</h3>` + tableHTML(head, rows)
}

// mmTableHTML renders the gene → module membership (MM / kME) table.
func mmTableHTML(d reportData) string {
	var mt struct {
		Modules []string    `json:"modules"`
		Genes   []string    `json:"genes"`
		Kme     [][]float64 `json:"kme"`
	}
	if json.Unmarshal([]byte(d.ModuleTraitJSON), &mt) != nil {
		return ""
	}
	var mod struct {
		Colors []string `json:"colors"`
	}
	json.Unmarshal([]byte(d.ModulesJSON), &mod)
	modIdx := map[string]int{}
	for i, m := range mt.Modules {
		modIdx[m] = i
	}
	head := []string{d.L("基因", "Gene"), d.L("模块", "Module"), "MM (kME)"}
	rows := make([][]string, 0, len(mt.Genes))
	for i, g := range mt.Genes {
		modName := ""
		if i < len(mod.Colors) {
			modName = mod.Colors[i]
		}
		if modName == "grey" || modName == "grey60" {
			continue
		}
		mm := ""
		if i < len(mt.Kme) {
			if idx, ok := modIdx[modName]; ok && idx < len(mt.Kme[i]) {
				mm = fmt.Sprintf("%.4f", mt.Kme[i][idx])
			}
		}
		rows = append(rows, []string{g, modName, mm})
	}
	return `<h3>` + d.L("模块隶属度（MM / kME）", "Module membership (MM / kME)") + `</h3>` + tableHTML(head, rows)
}

// pairPanelsHTML renders the significant module–trait pair tab panels (GS×MM
// scatter + z-score/eigengene heatmap, with the hub network as a follow-up).
func pairPanelsHTML(d reportData) string {
	var pairs []struct {
		Module string    `json:"module"`
		Trait  string    `json:"trait"`
		Cor    float64   `json:"cor"`
		P      float64   `json:"p"`
		Genes  []string  `json:"genes"`
		MM     []float64 `json:"mm"`
		GS     []float64 `json:"gs"`
	}
	if json.Unmarshal([]byte(d.PairPanelsJSON), &pairs) != nil || len(pairs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<h3>` + d.L("显著模块-表型对（|cor| > 0.65，p < 0.05）", "Significant module–trait pairs (|cor| > 0.65, p < 0.05)") + `</h3>`)
	b.WriteString(`<p>` + d.L("下面每个网络展示该模块内按 |GS|×|MM| 排名前 20 的 hub 基因，以及每个 hub 在模块内按 TOM 权重排名前 15 的邻居。节点颜色为模块颜色，hub 节点较大、邻居较小；连线为模块内拓扑重叠（TOM）连接。网络采用力导向布局（ECharts），节点位置仅反映连接关系，不代表生物学坐标。", "Each network below shows the module's top 20 hub genes ranked by |GS|×|MM|, plus each hub's top 15 intra-module neighbors by TOM weight. Node color is the module color; hub nodes are larger and neighbors smaller. Edges are intra-module topological-overlap (TOM) connections. The network uses a force-directed layout (ECharts), so node positions reflect connectivity only, not biological coordinates.") + `</p>`)
	b.WriteString(`<div class="mt-tabs" id="mt-tabs">`)
	for i, p := range pairs {
		active := ""
		if i == 0 {
			active = " active"
		}
		b.WriteString(`<button type="button" class="mt-tab` + active + `" data-idx="` + fmtInt(i) + `">` + html.EscapeString(p.Module) + `-` + html.EscapeString(p.Trait) + `</button>`)
	}
	b.WriteString(`</div>`)
	for i, p := range pairs {
		style := ` style="display:none"`
		if i == 0 {
			style = ""
		}
		b.WriteString(`<div class="mt-panel" data-idx="` + fmtInt(i) + `"` + style + `>`)
		b.WriteString(`<div class="row g-3"><div class="col-md-6"><div id="gsmm-` + fmtInt(i) + `" style="height:420px"></div></div>`)
		key := safeName(p.Module) + "_" + safeName(p.Trait)
		src := "../result/05_module_trait/panels/" + key + "/heatmap.png"
		if du, ok := d.HeatmapDataURLs[key]; ok && du != "" {
			src = du
		}
		b.WriteString(`<div class="col-md-6"><div style="resize:both;overflow:auto;max-width:100%;width:100%"><img src="` + src + `" alt="z-score eigengene" style="width:100%;display:block;border:1px solid var(--line)"></div></div></div>`)
		b.WriteString(`<div id="hubnet-` + fmtInt(i) + `" style="height:460px;border:1px solid var(--line);border-radius:4px;margin-top:14px"></div>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="hub-controls" style="display:flex;flex-wrap:wrap;gap:16px;align-items:center;margin:12px 0">` +
		`<label style="font-size:12px;display:flex;align-items:center;gap:6px">` + d.L("点大小", "Node size") + ` <input type="range" id="hub-node" min="0.5" max="3" step="0.1" value="1" style="width:120px"></label>` +
		`<label style="font-size:12px;display:flex;align-items:center;gap:6px">` + d.L("线粗细", "Edge width") + ` <input type="range" id="hub-edge" min="0.5" max="4" step="0.1" value="1" style="width:120px"></label>` +
		`<label style="font-size:12px;display:flex;align-items:center;gap:6px">` + d.L("线颜色", "Edge color") + ` <input type="color" id="hub-color" value="#9aa5b1" style="width:40px;height:28px"></label>` +
		`<button type="button" id="hub-relayout" style="font-size:12px;padding:4px 10px">` + d.L("重新布局", "Re-layout") + `</button>` +
		`<button type="button" id="hub-zoom-in" style="font-size:12px;padding:4px 10px">` + d.L("放大", "Zoom in") + `</button>` +
		`<button type="button" id="hub-zoom-out" style="font-size:12px;padding:4px 10px">` + d.L("缩小", "Zoom out") + `</button>` +
		`<button type="button" id="hub-fit" style="font-size:12px;padding:4px 10px">` + d.L("适应屏幕", "Fit") + `</button>` +
		`</div>`)
	b.WriteString(`<script type="application/json" id="mt-pairs-data">` + d.PairPanelsJSON + `</script>`)
	b.WriteString(`<script>(function(){var el=document.getElementById('mt-pairs-data');if(!el)return;var pairs=JSON.parse(el.textContent);var charts=[],hubs=[];function hubOpt(p,ns,ew,ec){return reportCharts.hubNetworkOption(p.hub&&p.hub.nodes||[],p.hub&&p.hub.edges||[],ns,ew,ec)}var initialized={};function initPanel(i){if(initialized[i])return;initialized[i]=true;var p=pairs[i];var c=document.getElementById('gsmm-'+i);if(c){var ch=echarts.init(c);ch.setOption(reportCharts.gsMmScatterOption(p.mm,p.gs,p.genes,p.module,p.trait,'` + d.L("GS vs MM", "GS vs MM") + `'));charts[i]=ch}var h=document.getElementById('hubnet-'+i);if(h&&p.hub){var hc=echarts.init(h);hc.setOption(hubOpt(p,1,1,'#9aa5b1'));hubs[i]=hc}}initPanel(0);window.addEventListener('resize',function(){charts.forEach(function(ch){if(ch)ch.resize()});hubs.forEach(function(hc){if(hc)hc.resize()})});var tabs=document.querySelectorAll('#mt-tabs .mt-tab');var activeIdx=0;tabs.forEach(function(t){t.onclick=function(){var idx=t.getAttribute('data-idx');activeIdx=+idx;tabs.forEach(function(x){x.classList.remove('active')});t.classList.add('active');document.querySelectorAll('.mt-panel').forEach(function(pan){pan.style.display=pan.getAttribute('data-idx')===idx?'':'none'});initPanel(+idx);if(charts[+idx])charts[+idx].resize();if(hubs[+idx])hubs[+idx].resize()}});var nodeEl=document.getElementById('hub-node'),edgeEl=document.getElementById('hub-edge'),colorEl=document.getElementById('hub-color'),relayoutEl=document.getElementById('hub-relayout');function refreshHub(){var hc=hubs[activeIdx];if(!hc)return;var p=pairs[activeIdx];hc.setOption(hubOpt(p,+nodeEl.value,+edgeEl.value,colorEl.value),true)}if(nodeEl){nodeEl.oninput=refreshHub;edgeEl.oninput=refreshHub;colorEl.oninput=refreshHub;relayoutEl.onclick=function(){var hc=hubs[activeIdx];if(hc){var p=pairs[activeIdx];hc.setOption(hubOpt(p,+nodeEl.value,+edgeEl.value,colorEl.value))}};var zoomInEl=document.getElementById('hub-zoom-in'),zoomOutEl=document.getElementById('hub-zoom-out'),fitEl=document.getElementById('hub-fit');if(zoomInEl){zoomInEl.onclick=function(){var hc=hubs[activeIdx];if(hc)hc.dispatchAction({type:'graphRoam',zoom:1.25})}}if(zoomOutEl){zoomOutEl.onclick=function(){var hc=hubs[activeIdx];if(hc)hc.dispatchAction({type:'graphRoam',zoom:0.8})}}if(fitEl){fitEl.onclick=function(){var hc=hubs[activeIdx];if(hc)hc.dispatchAction({type:'restore'})}}}})();</script>`)
	return b.String()
}

// moduleEnrichHTML renders the module-function exploration section: per-module
// tabs with GO (BP/MF/CC) + KEGG enrichment (bubble chart + table) and the
// gene annotation table for the selected term.
func moduleEnrichHTML(d reportData) string {
	var payload struct {
		Modules []struct {
			Module string `json:"module"`
		} `json:"modules"`
	}
	if json.Unmarshal([]byte(d.ModuleEnrichJSON), &payload) != nil || len(payload.Modules) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<h3>` + d.L("模块功能探究", "Module function exploration") + `</h3>`)
	b.WriteString(`<div class="me-tabs" id="me-tabs">`)
	for i, m := range payload.Modules {
		active := ""
		if i == 0 {
			active = " active"
		}
		b.WriteString(`<button type="button" class="me-tab` + active + `" data-idx="` + fmtInt(i) + `">` + html.EscapeString(m.Module) + `</button>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`<div class="me-subtabs" id="me-subtabs">`)
	for i, k := range []string{"ALL", "MF", "BP", "CC", "KEGG"} {
		active := ""
		if i == 0 {
			active = " active"
		}
		b.WriteString(`<button type="button" class="me-subtab` + active + `" data-kind="` + k + `">` + k + `</button>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`<div class="row g-3" style="margin-top:12px">` +
		`<div class="col-md-7"><div id="me-bubble" style="height:460px;border:1px solid var(--line);border-radius:4px"></div></div>` +
		`<div class="col-md-5"><div id="me-table" style="max-height:460px;overflow:auto;border:1px solid var(--line);border-radius:4px"></div></div>` +
		`</div><div id="me-anno" style="margin-top:16px"></div>`)
	b.WriteString(`<script type="application/json" id="me-data">` + d.ModuleEnrichJSON + `</script>`)
	b.WriteString(`<script>(function(){var el=document.getElementById('me-data');if(!el)return;var data=JSON.parse(el.textContent);var mods=data.modules||[],anno=data.anno||{};if(!mods.length)return;var activeMod=0,activeKind='ALL',bubble=null;function esc(s){return String(s==null?'':s).replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]})}function rowsFor(m,k){if(k==='KEGG')return m.kegg||[];var go=m.go||[];if(k==='ALL')return go;return go.filter(function(r){return r.ontology===k})}function renderTable(rows){var t=document.getElementById('me-table');if(!rows.length){t.innerHTML='<div class="notice" style="padding:12px">No significant enrichment terms.</div>';return}var h='<table style="white-space:nowrap"><thead><tr><th>ID</th><th>Description</th><th>GeneRatio</th><th>BgRatio</th><th>p.adjust</th><th>Count</th></tr></thead><tbody>';rows.forEach(function(r){h+='<tr class="me-term"><td>'+esc(r.id)+'</td><td>'+esc(r.description)+'</td><td>'+esc(r.gene_ratio)+'</td><td>'+esc(r.bg_ratio)+'</td><td>'+r['p.adjust'].toExponential(2)+'</td><td>'+r.count+'</td></tr>'});h+='</tbody></table>';t.innerHTML=h;var trs=t.querySelectorAll('tr.me-term');rows.forEach(function(r,i){trs[i].onclick=function(){renderAnno(r.id,r.description,r.gene_id)}})}function renderAnno(id,desc,genes){var w=document.getElementById('me-anno');if(!id){w.innerHTML='';return}var h='<h4>'+esc(id)+' · '+esc(desc)+'</h4>';genes=genes||[];if(!genes.length){h+='<div class="notice">No genes associated with this term.</div>'}else{h+='<div style="max-height:420px;overflow:auto;border:1px solid var(--line);border-radius:4px"><table style="white-space:nowrap"><thead><tr><th>Gene ID</th><th>Gene name</th><th>Symbol</th><th>Description</th></tr></thead><tbody>';genes.forEach(function(g){var a=anno[g]||{};h+='<tr><td>'+esc(a.gene_id||g)+'</td><td>'+esc(a.gene_name||'')+'</td><td>'+esc(a.symbol||'')+'</td><td>'+esc(a.description||'')+'</td></tr>'});h+='</tbody></table></div>'}w.innerHTML=h}function render(){var m=mods[activeMod];var rows=rowsFor(m,activeKind);if(bubble)bubble.dispose();bubble=echarts.init(document.getElementById('me-bubble'));bubble.setOption(reportCharts.enrichBubbleOption(rows));bubble.on('click',function(p){renderAnno(p.data.termId,p.data.description,p.data.genes)});renderTable(rows);renderAnno(null,null,null)}var mt=document.querySelectorAll('#me-tabs .me-tab');mt.forEach(function(t){t.onclick=function(){activeMod=+t.getAttribute('data-idx');mt.forEach(function(x){x.classList.remove('active')});t.classList.add('active');render()}});var st=document.querySelectorAll('#me-subtabs .me-subtab');st.forEach(function(t){t.onclick=function(){activeKind=t.getAttribute('data-kind');st.forEach(function(x){x.classList.remove('active')});t.classList.add('active');render()}});render();window.addEventListener('resize',function(){if(bubble)bubble.resize()})})();</script>`)
	return b.String()
}

// moduleOverviewHTML renders the module-overview section: ALL/MF/BP/CC/KEGG
// tabs switch the module overview figure and the clustering overview figure
// (eigengene + z-score heatmap + enrichment), each resizable.
func moduleOverviewHTML(d reportData) string {
	var b strings.Builder
	b.WriteString(overviewTabGroupHTML("module_overview", "module_overview_"))
	b.WriteString(`<h3>` + d.L("聚类对照", "Clustering comparison") + `</h3>`)
	b.WriteString(`<p>` + d.L("独立于 WGCNA 模块的聚类（KMeans / 模糊 c-means）总览，用于对照表达分组。", "Overview of clustering (KMeans / fuzzy c-means) independent of WGCNA modules, for comparison of expression groups.") + `</p>`)
	b.WriteString(overviewTabGroupHTML("cluster_overview", "08_clustering/cluster_overview_"))
	b.WriteString(`<script>(function(){document.querySelectorAll('.ov-tab').forEach(function(t){t.onclick=function(){var g=t.getAttribute('data-group'),k=t.getAttribute('data-kind');document.querySelectorAll('.ov-tab[data-group="'+g+'"]').forEach(function(x){x.classList.remove('active')});t.classList.add('active');document.querySelectorAll('.ov-panel[data-group="'+g+'"]').forEach(function(p){p.style.display=p.getAttribute('data-kind')===k?'':'none'})}})})();</script>`)
	return b.String()
}

// overviewTabGroupHTML renders the ALL/MF/BP/CC/KEGG tab row plus one resizable
// panel per kind (a draggable container around a lazy-loaded overview PNG).
func overviewTabGroupHTML(group, srcPrefix string) string {
	var b strings.Builder
	b.WriteString(`<div class="ov-tabs">`)
	for i, k := range []string{"ALL", "MF", "BP", "CC", "KEGG"} {
		active := ""
		if i == 0 {
			active = " active"
		}
		b.WriteString(`<button type="button" class="ov-tab` + active + `" data-group="` + group + `" data-kind="` + k + `">` + k + `</button>`)
	}
	b.WriteString(`</div>`)
	for i, k := range []string{"ALL", "MF", "BP", "CC", "KEGG"} {
		style := ` style="display:none"`
		if i == 0 {
			style = ""
		}
		b.WriteString(`<div class="ov-panel" data-group="` + group + `" data-kind="` + k + `"` + style + `><div style="resize:both;overflow:auto;max-width:100%;width:100%"><img src="../result/` + srcPrefix + k + `.png" alt="overview" loading="lazy" style="width:100%;display:block;border:1px solid var(--line)"></div></div>`)
	}
	return b.String()
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
