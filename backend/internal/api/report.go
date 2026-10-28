package api

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

// Report export: generates a result/ + report/ directory with per-module
// GS/MM + gene heatmap + GO/KEGG enrichment, the overview figure, the overview
// network, and a simple HTML report.

type reportStatus struct {
	State   string   `json:"state"` // running | done | error
	Message string   `json:"message"`
	Dir     string   `json:"dir"`
	Done    []string `json:"done,omitempty"`
}

var (
	reportMu     sync.Mutex
	reportStates = map[string]*reportStatus{}
)

// exportReport starts an async report/result export into <dir>/<name>/.
func (h *Handler) exportReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	target := r.FormValue("dir")
	name := r.FormValue("name")
	if target == "" {
		http.Error(w, "dir required", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = "rsWGCNA-" + time.Now().Format("20060102-150405")
	}
	// sanitize name: no path separators / reserved chars
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, name)

	base := filepath.Join(target, name)
	enrichType := r.FormValue("type")
	if enrichType != "kegg" {
		enrichType = "go"
	}
	lang := r.FormValue("lang")
	if lang != "en" {
		lang = "zh"
	}

	reportMu.Lock()
	st := &reportStatus{State: "running", Dir: base}
	reportStates[id] = st
	reportMu.Unlock()

	go h.doReportExport(id, base, enrichType, lang, st)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "dir": base})
}

// reportStatusHandler returns the current export status for a job.
func (h *Handler) reportStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	reportMu.Lock()
	st := reportStates[id]
	reportMu.Unlock()
	if st == nil {
		http.Error(w, "no export running", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) doReportExport(id, base, enrichType, lang string, st *reportStatus) {
	jobDir := filepath.Join(h.jobs.Dir(), id)
	fail := func(msg string) {
		reportMu.Lock()
		st.State = "error"
		st.Message = msg
		reportMu.Unlock()
	}

	resultDir := filepath.Join(base, "result")
	reportDir := filepath.Join(base, "report")
	_ = os.MkdirAll(reportDir, 0o755)
	_ = os.MkdirAll(resultDir, 0o755)

	// module list (non-grey) from step5_detail_all.json
	modules, err := readDetailAllModules(jobDir)
	if err != nil {
		fail(err.Error())
		return
	}

	// 1. per-module GS/MM + gene heatmap + enrichment
	for _, m := range modules {
		modDir := filepath.Join(resultDir, "06_module_detail", m)
		enrDir := filepath.Join(modDir, "enrichment")
		_ = os.MkdirAll(enrDir, 0o755)

		detailPath := filepath.Join(jobDir, "step5_detail_"+m+".json")
		if _, err := os.Stat(detailPath); err == nil {
			if p, _, err := plot.RenderGsmm(detailPath, "png", plot.PlotParams{}); err == nil {
				copyFile(p, filepath.Join(modDir, "gs_mm.png"))
				os.RemoveAll(filepath.Dir(p))
			}
			if p, _, err := plot.RenderGeneHeatmap(detailPath, "png", plot.PlotParams{}); err == nil {
				copyFile(p, filepath.Join(modDir, "expression_heatmap.png"))
				os.RemoveAll(filepath.Dir(p))
			}
		}

		// enrichment (GO/KEGG; skip if background missing)
		genes, _ := enrich.ModuleGenes(jobDir, m)
		if len(genes) > 0 {
			for _, kind := range []string{"go", "kegg"} {
				if kind != enrichType && enrichType != "both" {
					continue
				}
				var rows []enrich.Result
				var e error
				if kind == "go" {
					rows, e = enrich.GOEnrich(jobDir, genes, 0.05, 20)
				} else {
					rows, e = enrich.KEGGEnrich(jobDir, genes, 0.05, 20)
				}
				if e != nil {
					continue // background not provided -> skip (防呆)
				}
				writeEnrichTSV(filepath.Join(enrDir, kind+".tsv"), rows)
				if len(rows) > 0 {
					if p, _, err := plot.RenderEnrich(rows, "png", plot.PlotParams{}); err == nil {
						copyFile(p, filepath.Join(enrDir, kind+".png"))
						os.RemoveAll(filepath.Dir(p))
					}
				}
			}
		}

		reportMu.Lock()
		st.Done = append(st.Done, m)
		reportMu.Unlock()
	}

	// 2. overview figure + network
	if data, err := overviewData(jobDir, enrichType, "", 500, 0.05, 5); err == nil {
		if p, _, err := plot.RenderModuleOverview(data, "png", plot.PlotParams{Width: 18, Height: 13}); err == nil {
			copyFile(p, filepath.Join(resultDir, "module_overview.png"))
			os.RemoveAll(filepath.Dir(p))
		}
	}
	if nodes, edges, err := readNetworkLimited(jobDir, 2000); err == nil {
		if data, err := json.Marshal(map[string]any{"nodes": nodes, "edges": edges}); err == nil {
			if p, _, err := plot.RenderGeneNetwork(data, "svg", plot.PlotParams{}); err == nil {
				copyFile(p, filepath.Join(resultDir, "overview_network.svg"))
				os.RemoveAll(filepath.Dir(p))
			}
		}
	}

	// 2b. dendrogram + SFT + mtrait figures (referenced by the report)
	_ = os.MkdirAll(filepath.Join(resultDir, "02_filter"), 0o755)
	_ = os.MkdirAll(filepath.Join(resultDir, "03_soft_threshold"), 0o755)
	_ = os.MkdirAll(filepath.Join(resultDir, "04_network"), 0o755)
	_ = os.MkdirAll(filepath.Join(resultDir, "05_module_trait"), 0o755)
	if p, _, err := plot.RenderDendrogram(filepath.Join(jobDir, "step3_modules.json"), "png", plot.PlotParams{}); err == nil {
		copyFile(p, filepath.Join(resultDir, "04_network", "dendrogram.png"))
		os.RemoveAll(filepath.Dir(p))
	}
	if p, _, err := plot.RenderSft(filepath.Join(jobDir, "step2_sft.json"), "png", "", plot.PlotParams{}); err == nil {
		copyFile(p, filepath.Join(resultDir, "03_soft_threshold", "sft.png"))
		os.RemoveAll(filepath.Dir(p))
	}
	if p, _, err := plot.RenderMtrait(filepath.Join(jobDir, "step4_mtrait.json"), "png", plot.PlotParams{}); err == nil {
		copyFile(p, filepath.Join(resultDir, "05_module_trait", "mtrait.png"))
		os.RemoveAll(filepath.Dir(p))
	}
	if stData, err := sampleTreeData(jobDir); err == nil {
		if p, _, err := plot.RenderSampleTree(stData, "png", plot.PlotParams{}); err == nil {
			copyFile(p, filepath.Join(resultDir, "02_filter", "sample_tree.png"))
			os.RemoveAll(filepath.Dir(p))
		}
	}

	// 3. provenance + README + report
	writeProvenance(resultDir, jobDir, id, lang, filepath.Base(base))
	writeREADME(resultDir, modules)
	rd := buildReportData(jobDir, base, modules, enrichType, lang)
	writeReport(reportDir, rd)
	writeChecksumsAndExportManifest(resultDir)

	reportMu.Lock()
	st.State = "done"
	st.Message = ""
	reportMu.Unlock()
}

func readDetailAllModules(jobDir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(jobDir, "step5_detail_all.json"))
	if err != nil {
		return nil, err
	}
	var d struct {
		Modules []struct {
			Module string `json:"module"`
			Trait  string `json:"trait"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range d.Modules {
		if m.Module != "grey" {
			out = append(out, m.Module)
		}
	}
	return out, nil
}

func writeEnrichTSV(path string, rows []enrich.Result) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	_ = w.Write([]string{"id", "description", "ontology", "gene_ratio", "count", "pvalue", "p.adjust"})
	for _, r := range rows {
		_ = w.Write([]string{r.ID, r.Description, r.Ontology, r.GeneRatio, fmt.Sprint(r.Count), fmt.Sprintf("%g", r.Pvalue), fmt.Sprintf("%g", r.PAdjust)})
	}
	w.Flush()
}

func copyFile(src, dst string) {
	if b, err := os.ReadFile(src); err == nil {
		_ = os.WriteFile(dst, b, 0o644)
	}
}

func writeREADME(resultDir string, modules []string) {
	var sb strings.Builder
	sb.WriteString("rsWGCNA 输出结果说明\n\n")
	sb.WriteString("本目录包含本次分析的所有图表与数据文件。\n\n")
	sb.WriteString("目录结构：\n")
	sb.WriteString("  06_module_detail/<模块>/gs_mm.png               GS×MM 散点图（基因显著性 vs 模块隶属度）\n")
	sb.WriteString("  06_module_detail/<模块>/expression_heatmap.png   模块基因表达热图（z-score）\n")
	sb.WriteString("  06_module_detail/<模块>/enrichment/go.png|tsv     GO 富集结果\n")
	sb.WriteString("  06_module_detail/<模块>/enrichment/kegg.png|tsv   KEGG 富集结果\n")
	sb.WriteString("  module_overview.png                             模块总览三合一图\n")
	sb.WriteString("  overview_network.svg                            全基因共表达网络\n\n")
	sb.WriteString("模块列表：\n")
	for _, m := range modules {
		sb.WriteString("  - " + m + "\n")
	}
	_ = os.WriteFile(filepath.Join(resultDir, "README.txt"), []byte(sb.String()), 0o644)
}

// buildReportData assembles the real analysis facts for the report.
func buildReportData(jobDir, base string, modules []string, enrichType, lang string) reportData {
	d := reportData{
		ProjectName: filepath.Base(base),
		Timestamp:   time.Now().Format("2006-01-02 15:04 -07:00"),
		Version:     "v0.5.0",
		Lang:        lang,
		Modules:     modules,
		ModuleSizes: map[string]int{},
	}

	// genes + modules + sizes from step3_modules.json
	if b, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json")); err == nil {
		var mf struct {
			Genes  []string `json:"genes"`
			Colors []string `json:"colors"`
		}
		if json.Unmarshal(b, &mf) == nil {
			d.Genes = len(mf.Genes)
			for _, c := range mf.Colors {
				d.ModuleSizes[c]++
				if c == "grey" {
					d.GreyGenes++
				}
			}
		}
	}

	// samples from step1_filtered.tsv header
	if genes, samples, _, err := readFilteredMatrix(jobDir); err == nil {
		_ = genes
		d.Samples = len(samples)
	}

	// params from params.json
	if b, err := os.ReadFile(filepath.Join(jobDir, "params.json")); err == nil {
		var p map[string]any
		if json.Unmarshal(b, &p) == nil {
			d.Params = p
		}
	}

	// hub count + has significant trait from step4_mtrait.json
	if b, err := os.ReadFile(filepath.Join(jobDir, "step4_mtrait.json")); err == nil {
		var mt struct {
			Modules []string    `json:"modules"`
			Traits  []string    `json:"traits"`
			Cor     [][]float64 `json:"cor"`
			P       [][]float64 `json:"p"`
		}
		if json.Unmarshal(b, &mt) == nil {
			for mi := range mt.Modules {
				for ti := range mt.Traits {
					if mi < len(mt.Cor) && ti < len(mt.Cor[mi]) && mi < len(mt.P) && ti < len(mt.P[mi]) {
						if absFloat(mt.Cor[mi][ti]) > 0.6 && mt.P[mi][ti] < 0.05 {
							d.HasTrait = true
						}
					}
				}
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(jobDir, "step6_hub.json")); err == nil {
		var hub struct {
			Nodes []struct {
				IsHub bool `json:"is_hub"`
			} `json:"nodes"`
		}
		if json.Unmarshal(b, &hub) == nil {
			for _, n := range hub.Nodes {
				if n.IsHub {
					d.HubCount++
				}
			}
		}
	}

	// enrichment availability
	if enrichType == "kegg" {
		_, err := os.Stat(filepath.Join(jobDir, "gene2kegg.tsv"))
		d.EnrichOK = err == nil
	} else {
		_, err := os.Stat(filepath.Join(jobDir, "gene2go.tsv"))
		d.EnrichOK = err == nil
	}

	return d
}

// writeReport copies the CSS/JS and writes the report HTML.
func writeReport(reportDir string, d reportData) {
	_ = os.WriteFile(filepath.Join(reportDir, "design.css"), reportCSS, 0o644)
	_ = os.WriteFile(filepath.Join(reportDir, "design.js"), reportJS, 0o644)

	body := generateReportHTML(d)
	langAttr := "zh-CN"
	if d.Lang == "en" {
		langAttr = "en"
	}
	title := "rsWGCNA 分析报告"
	if d.Lang == "en" {
		title = "rsWGCNA Analysis report"
	}
	html := "<!doctype html><html lang='" + langAttr + "'><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>" + title + "</title><link rel='stylesheet' href='design.css'></head><body>" + body + "<script src='design.js'></script></body></html>"
	_ = os.WriteFile(filepath.Join(reportDir, "index.html"), []byte(html), 0o644)

	// report.qmd wrapper (recompilable with Quarto)
	qmd := fmt.Sprintf("---\ntitle: %q\nlang: %s\nformat:\n  html:\n    theme: none\n    css: design.css\n    toc: false\n    pagetitle: %q\n    include-after-body:\n      text: |\n        <script src=\"design.js\"></script>\n---\n\n```{=html}\n%s\n```\n", title, langAttr, title, body)
	_ = os.WriteFile(filepath.Join(reportDir, "report.qmd"), []byte(qmd), 0o644)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// sampleTreeData builds the sample-tree render payload from step1_filter.json
// and sample_info.tsv, using the trait column as the annotation band.
func sampleTreeData(jobDir string) ([]byte, error) {
	var filter struct {
		Samples []string `json:"samples"`
		Tree    struct {
			Merge  [][]int   `json:"merge"`
			Height []float64 `json:"height"`
			Order  []int     `json:"order"`
		} `json:"sample_tree"`
	}
	b, err := os.ReadFile(filepath.Join(jobDir, "step1_filter.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &filter); err != nil {
		return nil, err
	}

	var bands []map[string]any
	// trait column from params.json
	if pb, err := os.ReadFile(filepath.Join(jobDir, "params.json")); err == nil {
		var p map[string]any
		if json.Unmarshal(pb, &p) == nil {
			if mt, ok := p["module_trait"].(map[string]any); ok {
				if tc, ok := mt["trait_column"].(string); ok && tc != "" {
					bands = append(bands, map[string]any{"name": tc, "values": map[string]string{}})
				}
			}
		}
	}
	if len(bands) > 0 {
		if raw, err := os.ReadFile(filepath.Join(jobDir, "sample_info.tsv")); err == nil {
			if infos, err := enrich.ParseSampleInfo(raw); err == nil {
				attr := bands[0]["name"].(string)
				values := map[string]string{}
				for _, s := range infos {
					values[s.SampleID] = s.Attrs[attr]
				}
				bands[0]["values"] = values
			}
		}
	}

	return json.Marshal(map[string]any{"samples": filter.Samples, "tree": filter.Tree, "bands": bands})
}

// writeProvenance writes a minimal provenance folder (final params + environment).
func writeProvenance(resultDir, jobDir, id, lang, projectName string) {
	prov := filepath.Join(resultDir, "00_provenance")
	_ = os.MkdirAll(prov, 0o755)

	// parameters.final.json + parameters.final.tsv
	if b, err := os.ReadFile(filepath.Join(jobDir, "params.json")); err == nil {
		_ = os.WriteFile(filepath.Join(prov, "parameters.final.json"), b, 0o644)
		writeParamsTSV(filepath.Join(prov, "parameters.final.tsv"), b)
	}

	// analysis_manifest.json
	steps := detectSteps(jobDir)
	manifest := map[string]any{
		"project":      projectName,
		"job_id":       id,
		"language":     lang,
		"generated_at": time.Now().Format(time.RFC3339),
		"version":      "rsWGCNA v0.5.0",
		"steps":        steps,
	}
	if b, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(prov, "analysis_manifest.json"), b, 0o644)
	}

	// input_manifest.tsv (uploaded/input files with size + sha256)
	writeInputManifest(filepath.Join(prov, "input_manifest.tsv"), jobDir)

	// environment.txt
	env := fmt.Sprintf("rsWGCNA v0.5.0\nGenerated: %s\nOS: %s\nArch: %s\n", time.Now().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH)
	_ = os.WriteFile(filepath.Join(prov, "environment.txt"), []byte(env), 0o644)

	_ = os.WriteFile(filepath.Join(prov, "README.txt"), []byte("分析可追溯信息：\n- analysis_manifest.json：项目/任务/语言/时间/步骤\n- parameters.final.json + .tsv：最终参数\n- input_manifest.tsv：输入文件与校验\n- checksums.sha256：导出文件校验\n- export_manifest.tsv：导出文件清单\n- environment.txt：环境信息"), 0o644)
}

// detectSteps returns the steps whose output files are present.
func detectSteps(jobDir string) []string {
	stepFiles := []struct{ step, file string }{
		{"filter", "step1_filtered.tsv"},
		{"soft_threshold", "step2_sft.json"},
		{"network", "step3_modules.json"},
		{"module_trait", "step4_mtrait.json"},
		{"module_detail", "step5_detail.json"},
		{"hub", "step6_hub.json"},
		{"module_detail_all", "step5_detail_all.json"},
		{"clustering", "overview_clusters.json"},
	}
	var out []string
	for _, sf := range stepFiles {
		if _, err := os.Stat(filepath.Join(jobDir, sf.file)); err == nil {
			out = append(out, sf.step)
		}
	}
	return out
}

// writeParamsTSV writes params.json as a step | key | value table.
func writeParamsTSV(path string, raw []byte) {
	var p map[string]any
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	_ = w.Write([]string{"step", "parameter", "value"})
	order := []string{"filter", "soft_threshold", "network", "module_trait", "module_detail", "hub", "overview", "cluster"}
	for _, group := range order {
		v, ok := p[group]
		if !ok {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			j, _ := json.Marshal(m[k])
			_ = w.Write([]string{group, k, string(j)})
		}
	}
	w.Flush()
}

// writeInputManifest records the input files with size + sha256.
func writeInputManifest(path, jobDir string) {
	roles := []struct{ role, file string }{
		{"expression_matrix", "expr.tsv"},
		{"sample_info", "sample_info.tsv"},
		{"annotation", "annotation.tsv"},
		{"gene2go", "gene2go.tsv"},
		{"gene2kegg", "gene2kegg.tsv"},
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	_ = w.Write([]string{"role", "filename", "size_bytes", "sha256"})
	for _, r := range roles {
		fp := filepath.Join(jobDir, r.file)
		st, err := os.Stat(fp)
		if err != nil {
			_ = w.Write([]string{r.role, r.file, "", "not provided"})
			continue
		}
		h, err := fileSha256(fp)
		sum := ""
		if err == nil {
			sum = h
		}
		_ = w.Write([]string{r.role, r.file, fmt.Sprint(st.Size()), sum})
	}
	w.Flush()
}

// writeChecksumsAndExportManifest writes checksums.sha256 and
// export_manifest.tsv for every file under result/.
func writeChecksumsAndExportManifest(resultDir string) {
	prov := filepath.Join(resultDir, "00_provenance")
	cf, err := os.Create(filepath.Join(prov, "checksums.sha256"))
	if err != nil {
		return
	}
	defer cf.Close()
	mf, err := os.Create(filepath.Join(prov, "export_manifest.tsv"))
	if err != nil {
		return
	}
	defer mf.Close()
	mw := csv.NewWriter(mf)
	mw.Comma = '\t'
	_ = mw.Write([]string{"relative_path", "sha256", "status"})

	_ = filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(resultDir, path)
		if rel == "00_provenance/checksums.sha256" || rel == "00_provenance/export_manifest.tsv" {
			return nil
		}
		h, err := fileSha256(path)
		if err != nil {
			_ = mw.Write([]string{rel, "", "error"})
			return nil
		}
		fmt.Fprintf(cf, "%s  %s\n", h, rel)
		_ = mw.Write([]string{rel, h, "generated"})
		return nil
	})
	mw.Flush()
}

func fileSha256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
