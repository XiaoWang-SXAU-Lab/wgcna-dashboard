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

//go:embed report_assets/sample_tree.js
var sampleTreeJS []byte

//go:embed report_assets/echarts.min.js
var echartsJS []byte

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
		d.Version = "v0.5.0"
	}
	if d.Lang == "" {
		d.Lang = "zh"
	}
	var b strings.Builder

	// ---- rail (toc) ----
	nav := []string{
		d.L("项目介绍", "Project introduction"),
		d.L("第一步 · 数据清洗", "Step 1 · Data cleaning"),
		d.L("后续章节", "Further sections"),
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

	// ---- hero ----
	b.WriteString(`<div class="topline"><div class="eyebrow">` + d.L("加权基因共表达网络分析", "Weighted gene co-expression network analysis") + `</div><div class="actions"><button data-print>` + d.L("打印 / 保存 PDF", "Print / Save PDF") + `</button></div></div>`)
	b.WriteString(`<header class="hero"><div class="eyebrow">` + html.EscapeString(d.ProjectName) + `</div><h1>` + d.L("从表达矩阵，<br>到共表达网络。", "From expression,<br>to connection.") + `</h1><p>` + d.L("一份可追溯的分析记录。从输入文件、数据清洗到模块解析，逐步说明本次分析做了什么，以及每张图应该怎么看。", "A traceable analysis record. Follow the inputs, filtering decisions and module analysis, with a practical guide to reading every figure.") + `</p><div class="project-line">` + html.EscapeString(d.ProjectName) + ` · ` + d.Timestamp + ` · rsWGCNA ` + d.Version + `</div><div class="metrics">`)
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.RawSamples, d.L("样本", "samples")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, d.RawGenes, d.L("原始基因", "raw genes")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div>`, len(d.Modules), d.L("共表达模块", "modules")))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>%s</small></div></div></header>`, d.HubCount, d.L("Hub 候选", "hub candidates")))

	// ---- step 1: 项目介绍 ----
	b.WriteString(stepHead(d, 1, nav[0], d.L("罗列本次分析使用的输入文件；有则显示，无则不显示。", "Input files used in this analysis.")))
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

	// ---- placeholder ----
	b.WriteString(stepHead(d, 3, nav[2], d.L("后续章节（软阈值、网络构建、模块解析等）将陆续补充。", "Further sections (soft threshold, network construction, module analysis, …) will be added.")))
	b.WriteString(`<p class="step-intro">` + d.L("此处留白，等待后续章节。", "Placeholder for upcoming sections.") + `</p>`)
	b.WriteString(`</section>`)

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
