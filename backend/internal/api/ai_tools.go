package api

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/ai"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/literature"
)

// --- tool definitions -------------------------------------------------------

func objSchema(props map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func strProp(desc string, enum ...string) map[string]any {
	p := map[string]any{"type": "string", "description": desc}
	if len(enum) > 0 {
		p["enum"] = enum
	}
	return p
}

// toolDefs builds the tool definitions for the AI agent.
func toolDefs() []ai.Tool {
	noArgs := objSchema(map[string]any{})
	moduleArg := objSchema(map[string]any{"module": strProp("模块名，如 blue / turquoise")}, "module")
	enrichArg := objSchema(map[string]any{
		"module": strProp("模块名"),
		"type":   strProp("富集类型", "go", "kegg"),
	}, "module", "type")
	litArg := objSchema(map[string]any{"query": strProp("文献检索关键词，如 HSFA2 heat stress memory Arabidopsis")}, "query")

	return []ai.Tool{
		fn("get_params", "读取当前分析的全部参数（归一化、过滤、软阈值、网络、模块-性状、hub 等）", noArgs),
		fn("get_filter", "读取过滤步骤结果（保留的基因数、样本数、过滤统计）", noArgs),
		fn("get_sft", "读取软阈值分析结果（各 power 的 scale-free R²、mean connectivity、最终推荐 power）", noArgs),
		fn("get_network", "读取网络与模块结果（各模块的大小、颜色）", noArgs),
		fn("get_module_trait", "读取模块-性状相关结果（各模块×各性状的相关系数和 p 值）", noArgs),
		fn("get_module_detail", "读取某模块的基因列表、模块隶属度 MM 和基因显著性 GS", moduleArg),
		fn("query_genes", "查询指定基因属于哪个模块，以及它们的 MM 和 GS 值（用于判断目标基因是否在某模块）", objSchema(map[string]any{
			"genes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "基因 ID 列表，如 [\"AT2G26150\", \"AT5G03720\"]"},
		}, "genes")),
		fn("get_hub", "读取 hub 基因及网络（hub 基因列表 + 网络节点/边）", noArgs),
		fn("get_enrichment", "读取某模块的 GO 或 KEGG 富集分析结果", enrichArg),
		fn("search_literature", "检索领域文献（Europe PMC + PubMed + Semantic Scholar）。当回答涉及生物学机制、通路、基因功能、研究背景或需要文献支撑时必须调用，返回真实文献的 PMID/DOI/标题/摘要", litArg),
	}
}

func fn(name, desc string, params map[string]any) ai.Tool {
	return ai.Tool{Type: "function", Function: ai.FunctionDef{Name: name, Description: desc, Parameters: params}}
}

// --- tool executor ----------------------------------------------------------

// toolRunner implements ai.ToolExecutor with access to the job directory.
type toolRunner struct {
	jobDir string
	lit    []literature.Provider
}

func newToolRunner(jobDir string) *toolRunner {
	return &toolRunner{
		jobDir: jobDir,
		lit: []literature.Provider{
			&literature.EuropePMC{},
			&literature.PubMed{},
			&literature.SemanticScholar{},
		},
	}
}

func (r *toolRunner) Execute(name string, args map[string]any) (string, error) {
	switch name {
	case "get_params":
		return r.readJSON("params.json", nil)
	case "get_filter":
		return r.readJSON("step1_filter.json", nil)
	case "get_sft":
		return r.readJSON("step2_sft.json", []string{"fit_indices", "power_estimate", "power_source", "network_type"})
	case "get_network":
		return r.readJSON("step3_modules.json", []string{"module_sizes", "colors"})
	case "get_module_trait":
		return r.readJSON("step4_mtrait.json", []string{"modules", "traits", "cor", "p"})
	case "get_module_detail":
		return r.moduleDetail()
	case "query_genes":
		return r.queryGenes(args)
	case "get_hub":
		return r.readJSON("step6_hub.json", []string{"module", "trait_name", "hub_genes", "network"})
	case "get_enrichment":
		return r.enrichment(args)
	case "search_literature":
		return r.searchLit(args)
	default:
		return "", fmt.Errorf("未知工具 %q", name)
	}
}

// readJSON reads a job file and returns a compact JSON summary. When `keys` is
// non-empty, only those top-level keys are kept (to avoid huge payloads).
func (r *toolRunner) readJSON(name string, keys []string) (string, error) {
	b, err := os.ReadFile(filepath.Join(r.jobDir, name))
	if err != nil {
		return "", fmt.Errorf("%s 不存在（该步骤可能还没跑）", name)
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}
	out := data
	if keys != nil {
		out = map[string]any{}
		for _, k := range keys {
			if v, ok := data[k]; ok {
				out[k] = v
			}
		}
	}
	return marshalSnippet(out, "来源: "+name+"\n")
}

func (r *toolRunner) moduleDetail() (string, error) {
	b, err := os.ReadFile(filepath.Join(r.jobDir, "step5_detail.json"))
	if err != nil {
		return "", fmt.Errorf("step5_detail.json 不存在（请先运行模块深入步骤）")
	}
	var d struct {
		Module    string    `json:"module"`
		TraitName string    `json:"trait_name"`
		Genes     []string  `json:"genes"`
		GS        []float64 `json:"gs"`
		MM        []float64 `json:"mm"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return "", err
	}
	mmOf := func(i int) float64 {
		if i < len(d.MM) {
			return d.MM[i]
		}
		return 0
	}
	gsOf := func(i int) float64 {
		if i < len(d.GS) {
			return d.GS[i]
		}
		return 0
	}
	// top genes by |MM|
	idx := make([]int, len(d.Genes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return math.Abs(mmOf(idx[a])) > math.Abs(mmOf(idx[b]))
	})
	topN := 100
	if len(idx) < topN {
		topN = len(idx)
	}
	top := make([]map[string]any, 0, topN)
	for _, i := range idx[:topN] {
		top = append(top, map[string]any{
			"gene": d.Genes[i],
			"mm":   round4(mmOf(i)),
			"gs":   round4(gsOf(i)),
		})
	}
	summary := map[string]any{
		"module":           d.Module,
		"trait_name":       d.TraitName,
		"gene_count":       len(d.Genes),
		"all_genes":        d.Genes, // complete gene list
		"top_genes_by_kme": top,
	}
	return marshalSnippet(summary, "来源: step5_detail.json\n", 100000)
}

func (r *toolRunner) queryGenes(args map[string]any) (string, error) {
	var genes []string
	if arr, ok := args["genes"].([]any); ok {
		for _, g := range arr {
			if s, ok := g.(string); ok {
				genes = append(genes, s)
			}
		}
	}
	if len(genes) == 0 {
		return "", fmt.Errorf("需要基因列表")
	}

	// module assignment from step3
	var ma struct {
		Genes  []string `json:"genes"`
		Colors []string `json:"colors"`
	}
	if b, err := os.ReadFile(filepath.Join(r.jobDir, "step3_modules.json")); err == nil {
		_ = json.Unmarshal(b, &ma)
	}
	colorOf := map[string]string{}
	for i, g := range ma.Genes {
		if i < len(ma.Colors) {
			colorOf[g] = ma.Colors[i]
		}
	}

	// mm/gs from step5 (the currently-selected module)
	var d struct {
		Module string    `json:"module"`
		Genes  []string  `json:"genes"`
		GS     []float64 `json:"gs"`
		MM     []float64 `json:"mm"`
	}
	if b, err := os.ReadFile(filepath.Join(r.jobDir, "step5_detail.json")); err == nil {
		_ = json.Unmarshal(b, &d)
	}
	mmMap := map[string]float64{}
	gsMap := map[string]float64{}
	for i, g := range d.Genes {
		if i < len(d.MM) {
			mmMap[g] = d.MM[i]
		}
		if i < len(d.GS) {
			gsMap[g] = d.GS[i]
		}
	}

	out := make([]map[string]any, 0, len(genes))
	for _, g := range genes {
		row := map[string]any{"gene": g, "module": colorOf[g]}
		if colorOf[g] == "" {
			row["module"] = "（不在过滤后的基因集里）"
		}
		if mm, ok := mmMap[g]; ok {
			row["mm"] = round4(mm)
		}
		if gs, ok := gsMap[g]; ok {
			row["gs"] = round4(gs)
		}
		out = append(out, row)
	}
	return marshalSnippet(map[string]any{"genes": out}, "来源: step3_modules.json + step5_detail.json\n")
}

func (r *toolRunner) enrichment(args map[string]any) (string, error) {
	module, _ := args["module"].(string)
	typ, _ := args["type"].(string)
	if module == "" {
		module = r.lastModule()
	}
	if module == "" {
		return "", fmt.Errorf("需要指定模块名")
	}
	if typ != "go" && typ != "kegg" {
		typ = "go"
	}
	genes, err := enrich.ModuleGenes(r.jobDir, module)
	if err != nil {
		return "", err
	}
	var results []enrich.Result
	if typ == "go" {
		results, err = enrich.GOEnrich(r.jobDir, genes, 0.05, 20)
	} else {
		results, err = enrich.KEGGEnrich(r.jobDir, genes, 0.05, 20)
	}
	if err != nil {
		return "", err
	}
	type row struct {
		ID          string  `json:"id"`
		Description string  `json:"description"`
		Ontology    string  `json:"ontology,omitempty"`
		GeneRatio   string  `json:"gene_ratio"`
		PAdjust     float64 `json:"p.adjust"`
		Count       int     `json:"count"`
	}
	rows := make([]row, 0, len(results))
	for _, x := range results {
		rows = append(rows, row{x.ID, x.Description, x.Ontology, x.GeneRatio, x.PAdjust, x.Count})
	}
	summary := map[string]any{"module": module, "type": typ, "top_terms": rows}
	return marshalSnippet(summary, fmt.Sprintf("来源: %s 富集分析（GO/KEGG）\n", module))
}

func (r *toolRunner) searchLit(args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("需要检索关键词")
	}
	papers := literature.MultiSearch(r.lit, query, 10)
	if len(papers) == 0 {
		return "未检索到文献。", nil
	}
	type p struct {
		Title    string   `json:"title"`
		Authors  []string `json:"authors"`
		Year     int      `json:"year"`
		Journal  string   `json:"journal"`
		PMID     string   `json:"pmid,omitempty"`
		DOI      string   `json:"doi,omitempty"`
		URL      string   `json:"url,omitempty"`
		Abstract string   `json:"abstract,omitempty"`
		Source   string   `json:"source"`
	}
	out := make([]p, 0, len(papers))
	for _, x := range papers {
		abs := x.Abstract
		if len(abs) > 500 {
			abs = abs[:500] + "…"
		}
		out = append(out, p{x.Title, x.Authors, x.Year, x.Journal, x.PMID, x.DOI, x.URL, abs, x.Source})
	}
	return marshalSnippet(map[string]any{"count": len(out), "papers": out}, "来源: 文献检索（Europe PMC + PubMed + Semantic Scholar）\n")
}

func (r *toolRunner) lastModule() string {
	b, err := os.ReadFile(filepath.Join(r.jobDir, "step5_detail.json"))
	if err != nil {
		return ""
	}
	var d struct {
		Module string `json:"module"`
	}
	_ = json.Unmarshal(b, &d)
	return d.Module
}

// marshalSnippet marshals v to indented JSON with a prefix, truncating long
// payloads so they fit the LLM context. The optional maxChars overrides the
// default 6000-character cap.
func marshalSnippet(v any, prefix string, maxChars ...int) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	max := 6000
	if len(maxChars) > 0 && maxChars[0] > 0 {
		max = maxChars[0]
	}
	s := string(b)
	if len(s) > max {
		s = s[:max] + "\n…(已截断)"
	}
	return prefix + s, nil
}

func round4(x float64) float64 {
	return float64(int(x*10000+0.5)) / 10000
}
