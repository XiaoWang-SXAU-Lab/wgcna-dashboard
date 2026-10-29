// Package api exposes the HTTP endpoints for job submission, step execution,
// and status.
package api

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/demo"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

type Handler struct {
	jobs *job.Manager
}

func NewHandler(jobs *job.Manager) *Handler {
	return &Handler{jobs: jobs}
}

// Register adds all API routes to mux (Go 1.22 method+path patterns).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/jobs", h.createJob)
	mux.HandleFunc("GET /api/jobs/{id}", h.getJob)
	mux.HandleFunc("POST /api/jobs/{id}/trait", h.uploadTrait)
	mux.HandleFunc("POST /api/jobs/{id}/meta", h.uploadMeta)
	mux.HandleFunc("GET /api/jobs/{id}/annotation", h.getAnnotation)
	mux.HandleFunc("GET /api/jobs/{id}/enrich", h.enrich)
	mux.HandleFunc("GET /api/jobs/{id}/expression", h.expression)
	mux.HandleFunc("GET /api/jobs/{id}/sample-info", h.getSampleInfo)
	mux.HandleFunc("POST /api/demo/load", h.loadDemo)
	mux.HandleFunc("GET /api/demo/download", h.downloadDemo)
	mux.HandleFunc("GET /api/meta/go-version", h.goVersion)
	mux.HandleFunc("POST /api/meta/go-update", h.goUpdate)
	mux.HandleFunc("POST /api/jobs/{id}/steps/{step}/run", h.runStep)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/jobs/{id}/results/{step}", h.getResult)
	mux.HandleFunc("GET /api/jobs/{id}/plots/dendrogram", h.plotDendrogram)
	mux.HandleFunc("GET /api/jobs/{id}/plots/sft", h.plotSft)
	mux.HandleFunc("GET /api/jobs/{id}/plots/sft-check", h.plotSftCheck)
	mux.HandleFunc("GET /api/jobs/{id}/plots/mtrait", h.plotMtrait)
	mux.HandleFunc("GET /api/jobs/{id}/plots/eigengene", h.plotEigengene)
	mux.HandleFunc("GET /api/jobs/{id}/plots/gsmm", h.plotGsmm)
	mux.HandleFunc("GET /api/jobs/{id}/plots/geneheatmap", h.plotGeneHeatmap)
	mux.HandleFunc("GET /api/jobs/{id}/plots/enrich", h.plotEnrich)
	mux.HandleFunc("GET /api/jobs/{id}/plots/sample-tree", h.plotSampleTree)
	mux.HandleFunc("GET /api/jobs/{id}/plots/enrich-heatmap", h.plotEnrichHeatmap)
	mux.HandleFunc("GET /api/jobs/{id}/overview", h.overview)
	mux.HandleFunc("GET /api/jobs/{id}/plots/overview", h.plotOverview)
	mux.HandleFunc("GET /api/jobs/{id}/plots/overview-cluster", h.plotOverviewCluster)
	mux.HandleFunc("POST /api/jobs/{id}/overview/cluster", h.runCluster)
	mux.HandleFunc("GET /api/jobs/{id}/overview/cluster", h.getCluster)
	mux.HandleFunc("POST /api/jobs/{id}/overview/elbow", h.runClusterElbow)
	mux.HandleFunc("GET /api/jobs/{id}/overview/elbow", h.getClusterElbow)
	mux.HandleFunc("POST /api/jobs/{id}/overview/network", h.runOverviewNetwork)
	mux.HandleFunc("GET /api/jobs/{id}/overview/network", h.getOverviewNetwork)
	mux.HandleFunc("GET /api/jobs/{id}/plots/overview-network", h.plotOverviewNetwork)
	mux.HandleFunc("GET /api/jobs/{id}/overview/export.xlsx", h.exportOverviewXlsx)
	mux.HandleFunc("POST /api/jobs/{id}/report/export", h.exportReport)
	mux.HandleFunc("POST /api/jobs/{id}/report/snapshot", h.saveReportSnapshot)
	mux.HandleFunc("POST /api/jobs/{id}/report/prepare", h.prepareReportFigures)
	mux.HandleFunc("GET /api/jobs/{id}/report/status", h.reportStatusHandler)
	mux.HandleFunc("GET /api/jobs/{id}/report/view/{file...}", h.reportPreview)
	mux.HandleFunc("GET /api/folder-picker", h.pickFolder)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "wgcna-dashboard"})
}

// createJob accepts a multipart form with an "expr" file and an optional
// "params" JSON string, and creates (but does not run) the job.
func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	expr, err := readFile(r, "expr")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	params := json.RawMessage(r.FormValue("params"))
	threads, _ := strconv.Atoi(r.FormValue("threads"))
	if threads <= 0 {
		threads = runtime.NumCPU()
	}
	id, err := h.jobs.Create(expr, params, threads)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := h.jobs.Get(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	// Expose live worker progress while a step is running.
	if j.Status == "running" {
		if p := h.jobs.LiveProgress(id); p != nil {
			j.Progress = p
		}
	}
	writeJSON(w, http.StatusOK, j)
}

// uploadTrait accepts a multipart form with a "trait" file.
func (h *Handler) uploadTrait(w http.ResponseWriter, r *http.Request) {
	trait, err := readFile(r, "trait")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.jobs.UploadTrait(r.PathValue("id"), trait); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// uploadMeta accepts optional multipart files: sample_info, annotation,
// gene2go, gene2kegg.
func (h *Handler) uploadMeta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	files := map[string][]byte{}
	for _, name := range []string{"sample_info", "annotation", "gene2go", "gene2kegg"} {
		file, _, err := r.FormFile(name)
		if err != nil {
			continue // optional
		}
		b, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(b) > 0 {
			files[name] = b
		}
	}
	if err := h.jobs.StoreMeta(id, files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getAnnotation returns the annotation rows for one module's genes.
func (h *Handler) getAnnotation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	module := r.URL.Query().Get("module")
	if module == "" {
		http.Error(w, "module required", http.StatusBadRequest)
		return
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	genes, err := enrich.ModuleGenes(jobDir, module)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := enrich.Annotate(jobDir, genes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// enrich runs GO or KEGG over-representation for one module.
func (h *Handler) enrich(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	module := r.URL.Query().Get("module")
	kind := r.URL.Query().Get("type")
	if module == "" {
		http.Error(w, "module required", http.StatusBadRequest)
		return
	}
	if kind != "go" && kind != "kegg" {
		kind = "go"
	}
	fdr := qfloat(r, "fdr")
	if fdr <= 0 || fdr > 1 {
		fdr = 0.05
	}
	top := int(qfloat(r, "top"))
	if top <= 0 {
		top = 20
	}

	jobDir := filepath.Join(h.jobs.Dir(), id)
	genes, err := enrich.ModuleGenes(jobDir, module)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var results []enrich.Result
	if kind == "go" {
		results, err = enrich.GOEnrich(jobDir, genes, fdr, top)
	} else {
		results, err = enrich.KEGGEnrich(jobDir, genes, fdr, top)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// drop overly-broad terms (gene count > max_count, default 500, 0 = off)
	maxCount := int(qfloat(r, "max_count"))
	if !r.URL.Query().Has("max_count") {
		maxCount = 500
	}
	if maxCount > 0 {
		kept := results[:0]
		for _, rr := range results {
			if rr.Count <= maxCount {
				kept = append(kept, rr)
			}
		}
		results = kept
	}
	writeJSON(w, http.StatusOK, results)
}

// expression returns expression values for the requested genes (comma-separated).
func (h *Handler) expression(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	genesParam := r.URL.Query().Get("genes")
	if genesParam == "" {
		http.Error(w, "genes required", http.StatusBadRequest)
		return
	}
	genes := strings.Split(genesParam, ",")
	res, err := enrich.Expression(filepath.Join(h.jobs.Dir(), id), genes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// getSampleInfo returns the parsed sample metadata (from sample_info.tsv).
func (h *Handler) getSampleInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := os.ReadFile(filepath.Join(h.jobs.Dir(), id, "sample_info.tsv"))
	if err != nil {
		http.Error(w, "sample info not available", http.StatusNotFound)
		return
	}
	infos, err := enrich.ParseSampleInfo(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, infos)
}

// plotEnrich renders a GO/KEGG enrichment dot plot (ggplot2).
func (h *Handler) plotEnrich(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	module := r.URL.Query().Get("module")
	kind := r.URL.Query().Get("type")
	mode := r.URL.Query().Get("mode")
	if module == "" {
		http.Error(w, "module required", http.StatusBadRequest)
		return
	}
	if kind != "go" && kind != "kegg" {
		kind = "go"
	}
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	fdr := qfloat(r, "fdr")
	if fdr <= 0 || fdr > 1 {
		fdr = 0.05
	}
	top := int(qfloat(r, "top"))
	if top <= 0 {
		top = 20
	}

	jobDir := filepath.Join(h.jobs.Dir(), id)
	genes, err := enrich.ModuleGenes(jobDir, module)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var results []enrich.Result
	if kind == "go" {
		results, err = enrich.GOEnrich(jobDir, genes, fdr, top)
		// Apply the same ontology filter (MF/CC/BP) used on-screen so the
		// exported plot matches the ECharts display.
		if ont := r.URL.Query().Get("ontology"); ont != "" && ont != "ALL" {
			kept := results[:0]
			for _, res := range results {
				if res.Ontology == ont {
					kept = append(kept, res)
				}
			}
			results = kept
		}
	} else {
		results, err = enrich.KEGGEnrich(jobDir, genes, fdr, top)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		High:         r.URL.Query().Get("high"),
		BaseSize:     qfloat(r, "base_size"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		X:            r.URL.Query().Get("x"),
		ColorVar:     r.URL.Query().Get("color_var"),
		ShowCategory: int(qfloat(r, "show_category")),
		SizeMin:      qfloat(r, "size_min"),
		SizeMax:      qfloat(r, "size_max"),
		Title:        r.URL.Query().Get("title"),
		Wrap:         int(qfloat(r, "wrap")),
	}
	out, ctype, err := plot.RenderEnrich(results, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="enrich_`+kind+`.`+mode+`"`)
	http.ServeFile(w, r, out)
}

// plotSampleTree renders the sample dendrogram with optional color bands.
func (h *Handler) plotSampleTree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)

	var filter struct {
		Samples []string `json:"samples"`
		Tree    struct {
			Merge  [][]int   `json:"merge"`
			Height []float64 `json:"height"`
			Order  []int     `json:"order"`
		} `json:"sample_tree"`
	}
	if b, err := os.ReadFile(filepath.Join(jobDir, "step1_filter.json")); err == nil {
		if err := json.Unmarshal(b, &filter); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "sample tree not available", http.StatusNotFound)
		return
	}

	var bands []map[string]any
	if raw, err := os.ReadFile(filepath.Join(jobDir, "sample_info.tsv")); err == nil {
		if infos, err := enrich.ParseSampleInfo(raw); err == nil {
			for _, attr := range strings.Split(r.URL.Query().Get("attrs"), ",") {
				attr = strings.TrimSpace(attr)
				if attr == "" {
					continue
				}
				values := map[string]string{}
				for _, s := range infos {
					values[s.SampleID] = s.Attrs[attr]
				}
				bands = append(bands, map[string]any{"name": attr, "values": values})
			}
		}
	}

	combined := map[string]any{"samples": filter.Samples, "tree": filter.Tree, "bands": bands}
	data, err := json.Marshal(combined)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{Width: qfloat(r, "width"), Height: qfloat(r, "height"), Dpi: qfloat(r, "dpi")}
	out, ctype, err := plot.RenderSampleTree(data, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="sample_tree.`+mode+`"`)
	http.ServeFile(w, r, out)
}

// plotEnrichHeatmap renders the enrichment gene-expression heatmap (ComplexHeatmap).
func (h *Handler) plotEnrichHeatmap(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	genesParam := r.URL.Query().Get("genes")
	if genesParam == "" {
		http.Error(w, "genes required", http.StatusBadRequest)
		return
	}
	genes := strings.Split(genesParam, ",")
	jobDir := filepath.Join(h.jobs.Dir(), id)
	expr, err := enrich.Expression(jobDir, genes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	n := len(expr.Samples)
	z := make([][]float64, len(expr.Rows))
	labels := make([]string, len(expr.Rows))
	for i, row := range expr.Rows {
		mean := row.Mean
		var ss float64
		for _, v := range row.Values {
			ss += (v - mean) * (v - mean)
		}
		sd := math.Sqrt(ss / float64(maxInt(n-1, 1)))
		zi := make([]float64, n)
		for j, v := range row.Values {
			if sd == 0 {
				zi[j] = 0
			} else {
				zi[j] = (v - mean) / sd
			}
		}
		z[i] = zi
		if row.Symbol != "" {
			labels[i] = row.Symbol
		} else {
			labels[i] = row.Gene
		}
	}

	group := make([]string, n)
	if raw, err := os.ReadFile(filepath.Join(jobDir, "sample_info.tsv")); err == nil {
		if infos, perr := enrich.ParseSampleInfo(raw); perr == nil {
			m := map[string]string{}
			for _, s := range infos {
				if g := s.Attrs["group"]; g != "" {
					m[s.SampleID] = g
				} else if g := s.Attrs["genotype"]; g != "" {
					m[s.SampleID] = g
				} else {
					m[s.SampleID] = s.Attrs["condition"]
				}
			}
			for j, s := range expr.Samples {
				group[j] = m[s]
			}
		}
	}

	// Sort samples by group (stable) so biological replicates of the same
	// group are adjacent; only rows (genes) are clustered.
	type sg struct {
		i int
		g string
	}
	order := make([]sg, n)
	for i := range expr.Samples {
		order[i] = sg{i, group[i]}
	}
	sort.SliceStable(order, func(a, b int) bool { return order[a].g < order[b].g })
	samples2 := make([]string, n)
	group2 := make([]string, n)
	for j, o := range order {
		samples2[j] = expr.Samples[o.i]
		group2[j] = o.g
	}
	z2 := make([][]float64, len(z))
	for i, row := range z {
		zi := make([]float64, n)
		for j, o := range order {
			zi[j] = row[o.i]
		}
		z2[i] = zi
	}

	data, err := json.Marshal(map[string]any{"samples": samples2, "genes": labels, "z": z2, "group": group2})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		BaseSize:     qfloat(r, "base_size"),
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Legend:       r.URL.Query().Get("legend"),
		ClusterRows:  r.URL.Query().Get("cluster_rows"),
		ClusterCols:  r.URL.Query().Get("cluster_cols"),
		ShowRowNames: r.URL.Query().Get("show_rownames"),
		ShowColNames: r.URL.Query().Get("show_colnames"),
		Border:       r.URL.Query().Get("border"),
		CellFontSize: qfloat(r, "cell_fontsize"),
	}
	out, ctype, err := plot.RenderEnrichHeatmap(data, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="enrich_heatmap.`+mode+`"`)
	http.ServeFile(w, r, out)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// loadDemo creates a job from the bundled demo dataset (expr + trait + meta).
func (h *Handler) loadDemo(w http.ResponseWriter, r *http.Request) {
	expr, err := demo.Expr()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	threads, _ := strconv.Atoi(r.URL.Query().Get("threads"))
	if threads <= 0 {
		threads = runtime.NumCPU()
	}
	id, err := h.jobs.Create(expr, json.RawMessage(`{"filter":{"normalize_method":"vst"}}`), threads)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if trait, err := demo.Trait(); err == nil {
		_ = h.jobs.UploadTrait(id, trait)
	}
	if meta, err := demo.Meta(); err == nil {
		_ = h.jobs.StoreMeta(id, meta)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// downloadDemo returns a zip of the demo dataset.
func (h *Handler) downloadDemo(w http.ResponseWriter, _ *http.Request) {
	z, err := demo.Zip()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="rs-wgcna-demo.zip"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(z)))
	_, _ = w.Write(z)
}

// goVersion reports the bundled/in-use GO data version and the latest upstream.
func (h *Handler) goVersion(w http.ResponseWriter, _ *http.Request) {
	current, err := enrich.CurrentVersion()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	latest, lerr := enrich.LatestVersion()
	if lerr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"current": current, "latest": "", "update_available": false, "error": lerr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current": current, "latest": latest, "update_available": current != latest,
	})
}

// goUpdate downloads and installs the latest go-basic.obo.
func (h *Handler) goUpdate(w http.ResponseWriter, _ *http.Request) {
	ver, err := enrich.Update()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": ver})
}

// runStep runs a single step asynchronously.
func (h *Handler) runStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	step := r.PathValue("step")
	if !validStep(step) {
		http.Error(w, "unknown step: "+step, http.StatusBadRequest)
		return
	}
	if p := r.FormValue("params"); p != "" {
		if err := h.jobs.UpdateParams(id, json.RawMessage(p)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if t := r.FormValue("threads"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			h.jobs.SetThreads(id, n)
		}
	}
	go h.jobs.RunStep(id, step)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": step})
}

// cancel cancels the currently-running step, if any.
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.jobs.Cancel(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling"})
}

// getResult returns a step's JSON output (soft_threshold, network, ...).
func (h *Handler) getResult(w http.ResponseWriter, r *http.Request) {
	result, ok := h.jobs.Result(r.PathValue("id"), r.PathValue("step"))
	if !ok {
		http.Error(w, "result not available", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(result)
}

// plotDendrogram renders the module dendrogram via R (ggplot2/plotly).
func (h *Handler) plotDendrogram(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "png"
	}
	params := plot.PlotParams{
		Size:      qfloat(r, "size"),
		Color:     r.URL.Query().Get("color"),
		LineColor: r.URL.Query().Get("line_color"),
		BaseSize:  qfloat(r, "base_size"),
		AxisSize:  qfloat(r, "axis_size"),
		AxisColor: r.URL.Query().Get("axis_color"),
		Width:     qfloat(r, "width"),
		Height:    qfloat(r, "height"),
		Dpi:       qfloat(r, "dpi"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step3_modules.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "dendrogram not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderDendrogram(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	http.ServeFile(w, r, out)
}

// plotSft renders the soft-threshold plot via R (PDF/SVG/PNG, publication
// quality) and forces a download.
func (h *Handler) plotSft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	rsqCut := r.URL.Query().Get("rsq_cut")
	if rsqCut == "" {
		rsqCut = "0.85"
	}
	params := plot.PlotParams{
		Size:      qfloat(r, "size"),
		Color:     r.URL.Query().Get("color"),
		LineColor: r.URL.Query().Get("line_color"),
		BaseSize:  qfloat(r, "base_size"),
		AxisSize:  qfloat(r, "axis_size"),
		AxisColor: r.URL.Query().Get("axis_color"),
		Width:     qfloat(r, "width"),
		Height:    qfloat(r, "height"),
		Dpi:       qfloat(r, "dpi"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step2_sft.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "sft result not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderSft(jsonPath, mode, rsqCut, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"sft."+mode+"\"")
	http.ServeFile(w, r, out)
}

// plotSftCheck renders the scale-free topology check via R (PDF/SVG/PNG).
func (h *Handler) plotSftCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	params := plot.PlotParams{
		Size:      qfloat(r, "size"),
		Color:     r.URL.Query().Get("color"),
		LineColor: r.URL.Query().Get("line_color"),
		BaseSize:  qfloat(r, "base_size"),
		AxisSize:  qfloat(r, "axis_size"),
		AxisColor: r.URL.Query().Get("axis_color"),
		Width:     qfloat(r, "width"),
		Height:    qfloat(r, "height"),
		Dpi:       qfloat(r, "dpi"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step2_sft.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "sft result not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderSftCheck(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"sft-check."+mode+"\"")
	http.ServeFile(w, r, out)
}

// plotMtrait renders the module-trait heatmap via R (PDF/SVG/PNG).
func (h *Handler) plotMtrait(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Legend:       r.URL.Query().Get("legend"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		ClusterRows:  r.URL.Query().Get("cluster_rows"),
		ClusterCols:  r.URL.Query().Get("cluster_cols"),
		ShowRowNames: r.URL.Query().Get("show_rownames"),
		ShowColNames: r.URL.Query().Get("show_colnames"),
		Border:       r.URL.Query().Get("border"),
		CellFontSize: qfloat(r, "cell_fontsize"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step4_mtrait.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "module-trait result not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderMtrait(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"mtrait."+mode+"\"")
	http.ServeFile(w, r, out)
}

// plotGsmm renders the GS vs MM scatter via R (PDF/SVG/PNG).
func (h *Handler) plotGsmm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	params := plot.PlotParams{
		Size:      qfloat(r, "size"),
		LineColor: r.URL.Query().Get("line_color"),
		BaseSize:  qfloat(r, "base_size"),
		AxisSize:  qfloat(r, "axis_size"),
		Width:     qfloat(r, "width"),
		Height:    qfloat(r, "height"),
		Dpi:       qfloat(r, "dpi"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step5_detail.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "module detail not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderGsmm(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"gsmm."+mode+"\"")
	http.ServeFile(w, r, out)
}

// prepareGeneHeatmapJSON sorts samples by id, reorders the expression columns
// and eigengene values, and attaches all sample_info columns as annotations, so
// the geneheatmap R script can draw the color blocks + eigengene barplot.
func prepareGeneHeatmapJSON(jobDir, detailPath string) (string, error) {
	b, err := os.ReadFile(detailPath)
	if err != nil {
		return "", err
	}
	var d struct {
		Samples    []string    `json:"samples"`
		Expression [][]float64 `json:"expression"`
		Eigengene  []float64   `json:"eigengene"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return "", err
	}
	idx := make([]int, len(d.Samples))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return d.Samples[idx[a]] < d.Samples[idx[b]] })
	samples := permute(d.Samples, idx)
	expr := make([][]float64, len(d.Expression))
	for i, row := range d.Expression {
		expr[i] = permute(row, idx)
	}
	eg := permute(d.Eigengene, idx)

	meta, colOrder := readSampleMeta(jobDir)
	ann := buildSampleAnnotations(samples, meta, colOrder)

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	out["samples"] = samples
	out["expression"] = expr
	out["eigengene"] = eg
	out["sample_annotations"] = ann

	tmp, err := os.CreateTemp("", "geneheatmap-*.json")
	if err != nil {
		return "", err
	}
	if err := json.NewEncoder(tmp).Encode(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}

// plotGeneHeatmap renders the module gene heatmap via R (PDF/SVG/PNG).
func (h *Handler) plotGeneHeatmap(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Legend:       r.URL.Query().Get("legend"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		ClusterRows:  r.URL.Query().Get("cluster_rows"),
		ClusterCols:  r.URL.Query().Get("cluster_cols"),
		ShowRowNames: r.URL.Query().Get("show_rownames"),
		ShowColNames: r.URL.Query().Get("show_colnames"),
		Border:       r.URL.Query().Get("border"),
		CellFontSize: qfloat(r, "cell_fontsize"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step5_detail.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "module detail not available", http.StatusNotFound)
		return
	}
	if prepared, err := prepareGeneHeatmapJSON(h.jobs.Dir(), jsonPath); err == nil {
		jsonPath = prepared
		defer os.Remove(prepared)
	}
	out, ctype, err := plot.RenderGeneHeatmap(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"geneheatmap."+mode+"\"")
	http.ServeFile(w, r, out)
}

// plotEigengene renders the eigengene adjacency heatmap via R (PDF/SVG/PNG).
func (h *Handler) plotEigengene(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Legend:       r.URL.Query().Get("legend"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		ClusterRows:  r.URL.Query().Get("cluster_rows"),
		ClusterCols:  r.URL.Query().Get("cluster_cols"),
		ShowRowNames: r.URL.Query().Get("show_rownames"),
		ShowColNames: r.URL.Query().Get("show_colnames"),
		Border:       r.URL.Query().Get("border"),
		CellFontSize: qfloat(r, "cell_fontsize"),
	}
	jsonPath := filepath.Join(h.jobs.Dir(), id, "step4_mtrait.json")
	if _, err := os.Stat(jsonPath); err != nil {
		http.Error(w, "module-trait result not available", http.StatusNotFound)
		return
	}
	out, ctype, err := plot.RenderEigengene(jsonPath, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", "attachment; filename=\"eigengene."+mode+"\"")
	http.ServeFile(w, r, out)
}

func qfloat(r *http.Request, key string) float64 {
	v, _ := strconv.ParseFloat(r.URL.Query().Get(key), 64)
	return v
}

func readFile(r *http.Request, field string) ([]byte, error) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		return nil, err
	}
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, io.EOF
	}
	return b, nil
}

func validStep(s string) bool {
	for _, x := range job.StepOrder {
		if x == s {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
