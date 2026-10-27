package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

	reportMu.Lock()
	st := &reportStatus{State: "running", Dir: base}
	reportStates[id] = st
	reportMu.Unlock()

	go h.doReportExport(id, base, enrichType, st)

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

func (h *Handler) doReportExport(id, base, enrichType string, st *reportStatus) {
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
	if data, err := overviewData(jobDir, enrichType, 0.05, 5); err == nil {
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

	// 3. README + report
	writeREADME(resultDir, modules)
	rd := buildReportData(jobDir, base, modules, enrichType)
	writeReport(reportDir, rd)

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
func buildReportData(jobDir, base string, modules []string, enrichType string) reportData {
	d := reportData{
		ProjectName: filepath.Base(base),
		Timestamp:   time.Now().Format("2006-01-02 15:04 -07:00"),
		Version:     "v0.5.0",
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
	html := "<!doctype html><html lang='zh-CN'><head><meta charset='utf-8'><meta name='viewport' content='width=device-width, initial-scale=1'><title>rsWGCNA 分析报告</title><link rel='stylesheet' href='design.css'></head><body>" + body + "<script src='design.js'></script></body></html>"
	_ = os.WriteFile(filepath.Join(reportDir, "index.html"), []byte(html), 0o644)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
