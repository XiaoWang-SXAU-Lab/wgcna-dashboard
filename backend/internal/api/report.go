package api

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
)

// Report export: generates a result/ + report/ directory with per-module
// GS/MM + gene heatmap + GO/KEGG enrichment, the overview figure, the overview
// network, and a simple HTML report.

type reportStatus struct {
	State      string   `json:"state"` // running | done | error
	Message    string   `json:"message"`
	Dir        string   `json:"dir"`
	Done       []string `json:"done"`
	Warnings   []string `json:"warnings"`
	Total      int      `json:"total"`
	PreviewURL string   `json:"preview_url,omitempty"`
}

var (
	reportMu     sync.Mutex
	reportStates = map[string]*reportStatus{}
)

// exportReport starts an async report/result export into <dir>/<name>/.
func (h *Handler) exportReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	target := strings.TrimSpace(r.FormValue("dir"))
	name := strings.TrimSpace(r.FormValue("name"))
	if target == "" {
		http.Error(w, "dir required", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = "rsWGCNA-" + time.Now().Format("20060102-150405")
	}
	if !reportNameOK(name) {
		http.Error(w, "Invalid project name", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(filepath.Join(h.jobs.Dir(), id, "step3_modules.json")); err != nil {
		http.Error(w, "Run Network & modules before generating a report.", http.StatusBadRequest)
		return
	}

	target, err := filepath.Abs(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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
	if prior := reportStates[id]; prior != nil && prior.State == "running" {
		reportMu.Unlock()
		http.Error(w, "A report is already running for this job.", http.StatusConflict)
		return
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		reportMu.Unlock()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.Mkdir(base, 0o755); err != nil {
		reportMu.Unlock()
		http.Error(w, "Choose a new project name; output directory cannot be created: "+err.Error(), http.StatusConflict)
		return
	}
	st := &reportStatus{State: "running", Dir: base, Done: []string{}, Warnings: []string{}}
	reportStates[id] = st
	reportMu.Unlock()

	go h.doReportExport(id, base, enrichType, lang, st)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "dir": base})
}

// reportStatusHandler returns the current export status for a job.
func (h *Handler) reportStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st := h.savedReportStatus(id)
	if st == nil {
		http.Error(w, "no export running", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func writeEnrichTSV(path string, rows []enrich.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	_ = w.Write([]string{"id", "description", "ontology", "gene_ratio", "count", "pvalue", "p.adjust"})
	for _, r := range rows {
		_ = w.Write([]string{r.ID, r.Description, r.Ontology, r.GeneRatio, fmt.Sprint(r.Count), fmt.Sprintf("%g", r.Pvalue), fmt.Sprintf("%g", r.PAdjust)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return f.Close()
}

func writeREADME(resultDir string, modules []string) error {
	var sb strings.Builder
	sb.WriteString("rsWGCNA 输出结果说明\n\n")
	sb.WriteString("本目录包含成功生成的图表与数据文件；跳过与失败项目见 00_provenance/export-warnings.json。\n\n")
	sb.WriteString("目录结构：\n")
	sb.WriteString("  06_module_detail/<模块>/gs_mm.png               GS×MM 散点图（基因显著性 vs 模块隶属度）\n")
	sb.WriteString("  06_module_detail/<模块>/expression_heatmap.png   模块基因表达热图（z-score）\n")
	sb.WriteString("  06_module_detail/<模块>/enrichment/go.png|tsv     GO 富集结果\n")
	sb.WriteString("  06_module_detail/<模块>/enrichment/kegg.png|tsv   KEGG 富集结果\n")
	sb.WriteString("  module_overview.png                             模块总览三合一图\n")
	sb.WriteString("  overview_network*.svg|json                       模块内/模块间/组合采样网络\n")
	sb.WriteString("  08_clustering/                                  已运行的聚类对照、建议K指标与图表\n")
	sb.WriteString("  07_hub/hub.json                                 当前选定模块/性状的候选表与网络\n")
	sb.WriteString("  00_provenance/report-settings.json               报告采用的采样与绘图设置（独立于交互画布）\n\n")
	sb.WriteString("模块列表：\n")
	for _, m := range modules {
		sb.WriteString("  - " + m + "\n")
	}
	return os.WriteFile(filepath.Join(resultDir, "README.txt"), []byte(sb.String()), 0o644)
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
		EnrichType:  enrichType,
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
			selectedModule := pstr(d.Params, "hub", "select_module", "")
			if b, err := os.ReadFile(filepath.Join(jobDir, "step6_hub.json")); err == nil {
				var hub struct {
					Module string `json:"module"`
				}
				if json.Unmarshal(b, &hub) == nil {
					selectedModule = hub.Module
				}
			}
			for mi := range mt.Modules {
				if mt.Modules[mi] != selectedModule {
					continue
				}
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
			Module string          `json:"module"`
			Trait  string          `json:"trait_name"`
			Genes  []reportHubGene `json:"hub_genes"`
		}
		if json.Unmarshal(b, &hub) == nil {
			d.HubCount = len(hub.Genes)
			d.HubGenes = hub.Genes
			d.HubModule = hub.Module
			d.HubTrait = hub.Trait
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
	if b, err := os.ReadFile(filepath.Join(jobDir, "overview_clusters.json")); err == nil {
		var cluster struct {
			Method string `json:"method"`
			K      int    `json:"k"`
		}
		if json.Unmarshal(b, &cluster) == nil {
			d.ClusterMethod = cluster.Method
			d.ClusterK = cluster.K
		}
	}
	d.ModuleTraits = map[string]string{}
	if b, err := os.ReadFile(filepath.Join(jobDir, "step5_detail_all.json")); err == nil {
		var summary struct {
			Modules []struct {
				Module string `json:"module"`
				Trait  string `json:"trait"`
			} `json:"modules"`
		}
		if json.Unmarshal(b, &summary) == nil {
			for _, item := range summary.Modules {
				d.ModuleTraits[item.Module] = item.Trait
			}
		}
	}

	// interactive sample-tree data (samples + dendrogram + all sample_info bands)
	if st, err := sampleTreeData(jobDir); err == nil {
		d.SampleTreeJSON = string(st)
	}
	// project-introduction input-file summary
	populateInputs(&d, jobDir)
	// soft-threshold data (fit indices + scale-free check)
	if st, err := sftData(jobDir); err == nil {
		d.SftJSON = string(st)
	}

	return d
}

// writeReport copies the CSS/JS and writes the report HTML.
func writeReport(reportDir string, d reportData) error {
	if err := os.WriteFile(filepath.Join(reportDir, "design.css"), reportCSS, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(reportDir, "design.js"), reportJS, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(reportDir, "report-charts.js"), reportChartsJS, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(reportDir, "echarts.min.js"), echartsJS, 0o644); err != nil {
		return err
	}

	body := generateReportHTML(d)
	langAttr := "zh-CN"
	if d.Lang == "en" {
		langAttr = "en"
	}
	title := "rsWGCNA 分析报告"
	if d.Lang == "en" {
		title = "rsWGCNA Analysis report"
	}
	chartsScript := ""
	if d.SampleTreeJSON != "" || d.SftJSON != "" {
		chartsScript = `<script src="echarts.min.js"></script><script src="report-charts.js"></script>`
	}
	html := "<!doctype html><html lang='" + langAttr + "'><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>" + title + "</title><link rel='stylesheet' href='design.css'>" + chartsScript + "</head><body>" + body + "<script src='design.js'></script></body></html>"
	if err := os.WriteFile(filepath.Join(reportDir, "index.html"), []byte(html), 0o644); err != nil {
		return err
	}

	// report.qmd wrapper (recompilable with Quarto)
	qmd := fmt.Sprintf("---\ntitle: %q\nlang: %s\nformat:\n  html:\n    theme: none\n    css: design.css\n    toc: false\n    pagetitle: %q\n    include-after-body:\n      text: |\n        <script src=\"design.js\"></script>\n---\n\n```{=html}\n%s\n```\n", title, langAttr, title, body)
	return os.WriteFile(filepath.Join(reportDir, "report.qmd"), []byte(qmd), 0o644)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// sampleTreeData builds the sample-tree render payload from step1_filter.json
// and sample_info.tsv, exposing every sample_info column as an annotation band.
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
	return json.Marshal(map[string]any{"samples": filter.Samples, "tree": filter.Tree, "bands": sampleInfoBands(jobDir)})
}

// sampleInfoBands returns every sample_info column (except sample_id /
// sample_name) as {name, values-per-sample}, in the file's column order.
func sampleInfoBands(jobDir string) []map[string]any {
	raw, err := os.ReadFile(filepath.Join(jobDir, "sample_info.tsv"))
	if err != nil {
		return nil
	}
	r := csv.NewReader(strings.NewReader(string(raw)))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil
	}
	infos, err := enrich.ParseSampleInfo(raw)
	if err != nil {
		return nil
	}
	var bands []map[string]any
	for _, col := range header[1:] {
		if col == "sample_id" || col == "sample_name" {
			continue
		}
		values := map[string]string{}
		for _, s := range infos {
			values[s.SampleID] = s.Attrs[col]
		}
		bands = append(bands, map[string]any{"name": col, "values": values})
	}
	return bands
}

// populateInputs fills the project-introduction input-file summary.
func populateInputs(d *reportData, jobDir string) {
	if hdr, rows := readTSVHeaderRows(filepath.Join(jobDir, "expr.tsv")); hdr != nil {
		d.RawSamples = len(hdr) - 1
		d.RawGenes = rows
	}
	if hdr, _ := readTSVHeaderRows(filepath.Join(jobDir, "trait.tsv")); hdr != nil {
		d.TraitCount = len(hdr) - 1
	}
	if hdr, _ := readTSVHeaderRows(filepath.Join(jobDir, "sample_info.tsv")); hdr != nil {
		for _, c := range hdr[1:] {
			if c != "sample_id" && c != "sample_name" {
				d.SampleInfoCols = append(d.SampleInfoCols, c)
			}
		}
	}
	if _, rows := readTSVHeaderRows(filepath.Join(jobDir, "annotation.tsv")); rows > 0 {
		d.AnnoGenes = rows
	}
	d.GoGenes, d.GoEntries = countGeneAnno(filepath.Join(jobDir, "gene2go.tsv"))
	d.KeggGenes, d.KeggEntries = countGeneAnno(filepath.Join(jobDir, "gene2kegg.tsv"))
}

func readTSVHeaderRows(path string) ([]string, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	hdr, err := r.Read()
	if err != nil {
		return nil, 0
	}
	rows := 0
	for {
		if _, err := r.Read(); err != nil {
			break
		}
		rows++
	}
	return hdr, rows
}

func countGeneAnno(path string) (genes, entries int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil { // header
		return 0, 0
	}
	seen := map[string]bool{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 1 {
			continue
		}
		entries++
		if !seen[rec[0]] {
			seen[rec[0]] = true
			genes++
		}
	}
	return genes, entries
}

// sftData reads the soft-threshold result and merges the R² cutoff from params.
func sftData(jobDir string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(jobDir, "step2_sft.json"))
	if err != nil {
		return nil, err
	}
	var sft map[string]any
	if err := json.Unmarshal(b, &sft); err != nil {
		return nil, err
	}
	if pb, err := os.ReadFile(filepath.Join(jobDir, "params.json")); err == nil {
		var p map[string]any
		if json.Unmarshal(pb, &p) == nil {
			if st, ok := p["soft_threshold"].(map[string]any); ok {
				if rc, ok := st["rsquared_cut"].(float64); ok {
					sft["rsquared_cut"] = rc
				}
			}
		}
	}
	return json.Marshal(sft)
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
		{"cluster_elbow", "overview_elbow.json"},
		{"overview_network", "network_edges.tsv"},
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
		{"trait", "trait.tsv"},
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
func writeChecksumsAndExportManifest(resultDir string) error {
	prov := filepath.Join(resultDir, "00_provenance")
	cf, err := os.Create(filepath.Join(prov, "checksums.sha256"))
	if err != nil {
		return err
	}
	defer cf.Close()
	mf, err := os.Create(filepath.Join(prov, "export_manifest.tsv"))
	if err != nil {
		return err
	}
	defer mf.Close()
	mw := csv.NewWriter(mf)
	mw.Comma = '\t'
	_ = mw.Write([]string{"relative_path", "sha256", "status"})

	err = filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(resultDir, path)
		rel = filepath.ToSlash(rel)
		if rel == "00_provenance/checksums.sha256" || rel == "00_provenance/export_manifest.tsv" {
			return nil
		}
		h, err := fileSha256(path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cf, "%s  %s\n", h, rel); err != nil {
			return err
		}
		return mw.Write([]string{rel, h, "generated"})
	})
	if err != nil {
		return err
	}
	mw.Flush()
	if err := mw.Error(); err != nil {
		return err
	}
	if err := cf.Close(); err != nil {
		return err
	}
	return mf.Close()
}

func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
