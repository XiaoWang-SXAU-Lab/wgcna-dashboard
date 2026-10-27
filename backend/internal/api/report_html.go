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

// generateReportHTML builds the report body (rail + hero + steps) using the
// Codex v2 visual language, populated with real analysis data.
func generateReportHTML(d reportData) string {
	if d.Version == "" {
		d.Version = "v0.5.0"
	}
	var b strings.Builder

	// ---- rail ----
	nav := []string{"输入与背景", "最终参数", "过滤与样本质控", "软阈值选择", "网络与模块", "模块-性状", "模块总览", "各模块深入", "Hub 基因", "文件与复现"}
	b.WriteString(`<aside class="rail"><a class="brand">rs<em>WGCNA</em><span style="font-size:10px;letter-spacing:0;margin-left:8px">REPORT</span></a><div class="eyebrow">分析记录</div><nav>`)
	for i, n := range nav {
		cls := ""
		if i == 0 {
			cls = ` class="active"`
		}
		b.WriteString(fmt.Sprintf(`<a href="#step-%d"%s><span>%02d</span>%s</a>`, i+1, cls, i+1, n))
	}
	b.WriteString(`</nav><div class="rail-bottom">rsWGCNA 分析报告<a href="https://github.com/XiaoWang-SXAU-Lab/xiaowang-sxau-lab.github.io" target="_blank" rel="noopener">Xiao Wang Lab ↗</a></div></aside><main class="report">`)

	// ---- topline + hero ----
	b.WriteString(`<div class="topline"><div class="eyebrow">加权基因共表达网络分析</div><div class="actions"><button data-print>打印 / 保存 PDF</button></div></div>`)
	b.WriteString(`<header class="hero"><div class="eyebrow">` + html.EscapeString(d.ProjectName) + `</div><h1>从表达矩阵，<br>到共表达网络。</h1><p>一份可追溯的分析记录。从输入文件、数据筛选到模块解析，逐步说明本次分析做了什么，以及每张图应该怎么看。</p><div class="project-line">` + html.EscapeString(d.ProjectName) + ` · ` + d.Timestamp + ` · rsWGCNA ` + d.Version + `</div><div class="metrics">`)
	nMod := len(d.Modules)
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>样本</small></div>`, d.Samples))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>进入网络的基因</small></div>`, d.Genes))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>共表达模块（不含 grey）</small></div>`, nMod))
	b.WriteString(fmt.Sprintf(`<div class="metric"><strong>%d</strong><small>Hub 候选</small></div></div></header>`, d.HubCount))

	// ---- step 1: 输入与背景 ----
	b.WriteString(stepHead(1, "输入与背景", "先记录分析从哪里开始。"))
	b.WriteString(tableHTML([]string{"内容", "数值"}, [][]string{
		{"样本数", fmt.Sprint(d.Samples)},
		{"进入网络分析的基因数", fmt.Sprint(d.Genes)},
		{"共表达模块数（不含 grey）", fmt.Sprint(nMod)},
		{"grey 未分配基因数", fmt.Sprint(d.GreyGenes)},
	}))
	if d.EnrichOK {
		b.WriteString(`<div class="reading">已提供 GO/KEGG 背景文件，各模块均已执行功能富集分析。</div>`)
	} else {
		b.WriteString(`<div class="notice">未提供 GO/KEGG 背景文件，本次未执行功能富集分析。</div>`)
	}
	b.WriteString(`</section>`)

	// ---- step 2: 最终参数 ----
	b.WriteString(stepHead(2, "最终参数", "这里记录实际运行使用的最终参数。"))
	b.WriteString(paramsHTML(d.Params))
	b.WriteString(`</section>`)

	// ---- step 3: 过滤 ----
	b.WriteString(stepHead(3, "过滤与样本质控", "明确区分样本删除、低表达过滤和高变基因选择。"))
	b.WriteString(`<div class="story"><p>表达矩阵经归一化、低表达过滤和 MAD/VAR 高变基因选择后，保留前 ` + fmt.Sprint(d.Genes) + ` 个基因进入网络分析。</p></div>`)
	b.WriteString(`<figure><img src="../result/02_filter/sample_tree.png" alt="样本聚类树"><figcaption><span><b>图 1.</b> 样本聚类树；色带为用户所选标签。</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 4: 软阈值 ----
	b.WriteString(stepHead(4, "软阈值选择", "先观察候选值，再记录最终采用值。"))
	b.WriteString(`<figure><img src="../result/03_soft_threshold/sft.png" alt="软阈值曲线"><figcaption><span><b>图 2.</b> 无标度拟合 R² 与平均连通性曲线。</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 5: 网络与模块 ----
	b.WriteString(stepHead(5, "网络与模块", "将表达模式相似的基因分组。"))
	b.WriteString(`<figure><img src="../result/04_network/dendrogram.png" alt="模块树状图"><figcaption><span><b>图 3.</b> 基因聚类树与模块色带。</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 6: 模块-性状 ----
	b.WriteString(stepHead(6, "模块-性状", "用模块特征向量概括模块表达。"))
	b.WriteString(`<figure><img src="../result/05_module_trait/mtrait.png" alt="模块性状热图"><figcaption><span><b>图 4.</b> 模块-性状相关热图。</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 7: 模块总览（NEW）----
	b.WriteString(stepHead(7, "模块总览", "一张图看懂所有模块的表达趋势与功能。"))
	b.WriteString(`<figure><img src="../result/module_overview.png" alt="模块总览三合一图"><figcaption><span><b>图 5.</b> 模块总览：左侧 eigengene 折线 + 中间 z-score 热图（按模块切分）+ 右侧富集术语。</span></figcaption></figure>`)
	b.WriteString(`<figure><img src="../result/overview_network.svg" alt="全基因网络"><figcaption><span><b>图 6.</b> 全基因共表达网络（igraph DrL 布局，节点色=模块色、大小=degree、边色=weight）。</span></figcaption></figure>`)
	b.WriteString(`</section>`)

	// ---- step 8: 各模块深入（NEW）----
	b.WriteString(stepHead(8, "各模块深入", "每个模块的基因表现与功能线索。"))
	for _, m := range d.Modules {
		b.WriteString(`<h3>模块 ` + html.EscapeString(m) + `</h3>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/gs_mm.png" alt="` + m + ` GS×MM"><figcaption><span><b>` + m + `.</b> GS×MM 散点。</span></figcaption></figure>`)
		b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/expression_heatmap.png" alt="` + m + ` 基因热图"><figcaption><span><b>` + m + `.</b> 模块基因表达热图。</span></figcaption></figure>`)
		if d.EnrichOK {
			b.WriteString(`<figure><img src="../result/06_module_detail/` + m + `/enrichment/go.png" alt="` + m + ` GO 富集"><figcaption><span><b>` + m + `.</b> GO 富集。</span></figcaption></figure>`)
		}
	}
	b.WriteString(`</section>`)

	// ---- step 9: Hub 基因 ----
	b.WriteString(stepHead(9, "Hub 基因", "候选筛选与网络展示。"))
	if d.HasTrait {
		b.WriteString(`<div class="story"><p>存在与模块显著强相关（p &lt; 0.05 且 |cor| &gt; 0.6）的性状，Hub 基因同时参考 |GS| 与 |kME| 阈值。</p></div>`)
	} else {
		b.WriteString(`<div class="story"><p>未检测到显著强相关的性状，Hub 基因仅按 |kME| 阈值筛选。</p></div>`)
	}
	b.WriteString(`</section>`)

	// ---- step 10: 文件与复现 ----
	b.WriteString(stepHead(10, "文件与复现", "回到原始图表、数据和分析设定。"))
	b.WriteString(`<div class="download-row"><a class="button" href="../result/README.txt" download>下载读图说明</a></div>`)
	b.WriteString(`</section>`)

	b.WriteString(`<footer class="footer"><span>© 2026 Xiao Wang Lab · rsWGCNA</span></footer></main>`)
	return b.String()
}

func stepHead(i int, title, intro string) string {
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

func paramsHTML(params map[string]any) string {
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
		b.WriteString(`<details class="parameter" open><summary>` + group + `</summary>`)
		var rows [][]string
		for k, val := range m {
			j, _ := json.Marshal(val)
			rows = append(rows, []string{k, string(j)})
		}
		b.WriteString(tableHTML([]string{"参数", "最终值"}, rows))
		b.WriteString(`</details>`)
	}
	return b.String()
}
