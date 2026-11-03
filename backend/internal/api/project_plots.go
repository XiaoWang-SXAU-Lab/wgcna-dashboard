package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

// plotProject renders a committed step result via R (publication-quality
// PDF/SVG/PNG) for the project workspace. The frontend requests a preview with
// mode=png and downloads with mode=pdf/svg/png.
func (h *ProjectHandler) plotProject(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	kind := r.PathValue("kind")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "png"
	}
	// writeStep copies a step's committed result JSON into a temp file for R.
	writeStep := func(step string) (string, error) {
		data, err := s.p.Result(step)
		if err != nil {
			return "", err
		}
		tmp, err := os.CreateTemp("", "project-plot-*.json")
		if err != nil {
			return "", err
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		return tmp.Name(), nil
	}
	serve := func(out, ctype string, err error, name string) {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(filepath.Dir(out))
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		http.ServeFile(w, r, out)
	}

	switch kind {
	case "dendrogram":
		p, err := writeStep("network")
		if err != nil {
			http.Error(w, "dendrogram not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderDendrogram(p, mode, plotSizeParams(r))
		serve(out, ctype, err, "dendro."+mode)
	case "sft":
		p, err := writeStep("soft_threshold")
		if err != nil {
			http.Error(w, "sft not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		rsq := r.URL.Query().Get("rsq_cut")
		if rsq == "" {
			rsq = "0.85"
		}
		out, ctype, err := plot.RenderSft(p, mode, rsq, plotSizeParams(r))
		serve(out, ctype, err, "sft."+mode)
	case "sft-check":
		p, err := writeStep("soft_threshold")
		if err != nil {
			http.Error(w, "sft not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderSftCheck(p, mode, plotSizeParams(r))
		serve(out, ctype, err, "sft-check."+mode)
	case "mtrait":
		p, err := writeStep("module_trait")
		if err != nil {
			http.Error(w, "module-trait not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderMtrait(p, mode, plotHeatParams(r))
		serve(out, ctype, err, "mtrait."+mode)
	case "eigengene":
		p, err := writeStep("module_trait")
		if err != nil {
			http.Error(w, "module-trait not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderEigengene(p, mode, plotHeatParams(r))
		serve(out, ctype, err, "eigengene."+mode)
	case "gsmm":
		p, err := writeStep("module_detail")
		if err != nil {
			http.Error(w, "module detail not available", http.StatusNotFound)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderGsmm(p, mode, plotSizeParams(r))
		serve(out, ctype, err, "gsmm."+mode)
	case "sample-tree":
		data, err := s.p.Result("filter")
		if err != nil {
			http.Error(w, "sample tree not available", http.StatusNotFound)
			return
		}
		var filter map[string]any
		if err := json.Unmarshal(data, &filter); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var bands []map[string]any
		if raw, err := s.p.Input("sample_info"); err == nil {
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
		combined := map[string]any{"samples": filter["samples"], "tree": filter["sample_tree"], "bands": bands}
		data2, err := json.Marshal(combined)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out, ctype, err := plot.RenderSampleTree(data2, mode, plot.PlotParams{Width: qfloat(r, "width"), Height: qfloat(r, "height"), Dpi: qfloat(r, "dpi")})
		serve(out, ctype, err, "sample_tree."+mode)
	case "geneheatmap":
		detail, err := s.p.Result("module_detail")
		if err != nil {
			http.Error(w, "module detail not available", http.StatusNotFound)
			return
		}
		var sampleInfo []byte
		if raw, e := s.p.Input("sample_info"); e == nil {
			sampleInfo = raw
		}
		annCols := strings.Split(r.URL.Query().Get("ann"), ",")
		if len(annCols) == 1 && annCols[0] == "" {
			annCols = nil
		}
		p, err := prepareProjectGeneHeatmapJSON(detail, sampleInfo, annCols)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(p)
		out, ctype, err := plot.RenderGeneHeatmap(p, mode, plotHeatParams(r))
		serve(out, ctype, err, "geneheatmap."+mode)
	case "enrich":
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
		dir, err := materializeEnrichDir(s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(dir)
		genes, err := enrich.ModuleGenes(dir, module)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var results []enrich.Result
		if kind == "go" {
			results, err = enrich.GOEnrich(dir, genes, fdr, top)
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
			results, err = enrich.KEGGEnrich(dir, genes, fdr, top)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out, ctype, err := plot.RenderEnrich(results, mode, enrichPlotParams(r))
		serve(out, ctype, err, "enrich_"+kind+"."+mode)
	default:
		http.Error(w, "unknown plot kind", http.StatusNotFound)
	}
}

func plotSizeParams(r *http.Request) plot.PlotParams {
	return plot.PlotParams{
		Size: qfloat(r, "size"), Color: r.URL.Query().Get("color"),
		LineColor: r.URL.Query().Get("line_color"), BaseSize: qfloat(r, "base_size"),
		AxisSize: qfloat(r, "axis_size"), AxisColor: r.URL.Query().Get("axis_color"),
		Width: qfloat(r, "width"), Height: qfloat(r, "height"), Dpi: qfloat(r, "dpi"),
	}
}

func plotHeatParams(r *http.Request) plot.PlotParams {
	return plot.PlotParams{
		Low: r.URL.Query().Get("low"), Mid: r.URL.Query().Get("mid"), High: r.URL.Query().Get("high"),
		Legend: r.URL.Query().Get("legend"), Width: qfloat(r, "width"), Height: qfloat(r, "height"), Dpi: qfloat(r, "dpi"),
		ClusterRows: r.URL.Query().Get("cluster_rows"), ClusterCols: r.URL.Query().Get("cluster_cols"),
		ShowRowNames: r.URL.Query().Get("show_rownames"), ShowColNames: r.URL.Query().Get("show_colnames"),
		Border: r.URL.Query().Get("border"), CellFontSize: qfloat(r, "cell_fontsize"),
	}
}

func enrichPlotParams(r *http.Request) plot.PlotParams {
	return plot.PlotParams{
		Low: r.URL.Query().Get("low"), High: r.URL.Query().Get("high"), BaseSize: qfloat(r, "base_size"),
		Width: qfloat(r, "width"), Height: qfloat(r, "height"), Dpi: qfloat(r, "dpi"),
		X: r.URL.Query().Get("x"), ColorVar: r.URL.Query().Get("color_var"),
		ShowCategory: int(qfloat(r, "show_category")), SizeMin: qfloat(r, "size_min"), SizeMax: qfloat(r, "size_max"),
		Title: r.URL.Query().Get("title"), Wrap: int(qfloat(r, "wrap")),
	}
}

// sampleMeta reads sample_info.tsv into map[sampleID]map[column]value + column order.
func sampleMeta(data []byte) (map[string]map[string]string, []string) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil
	}
	meta := map[string]map[string]string{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 2 {
			continue
		}
		row := map[string]string{}
		for i, col := range header {
			if i < len(rec) {
				row[col] = rec[i]
			}
		}
		meta[rec[0]] = row
	}
	var colOrder []string
	for _, c := range header {
		if c != "sample_id" && c != "sample_name" {
			colOrder = append(colOrder, c)
		}
	}
	return meta, colOrder
}

// prepareProjectGeneHeatmapJSON sorts samples by id, reorders the expression
// columns and eigengene values, and attaches sample_info annotation bands.
func prepareProjectGeneHeatmapJSON(detail, sampleInfo []byte, annCols []string) (string, error) {
	var d struct {
		Samples    []string    `json:"samples"`
		Expression [][]float64 `json:"expression"`
		Eigengene  []float64   `json:"eigengene"`
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		return "", err
	}
	idx := make([]int, len(d.Samples))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return d.Samples[idx[a]] < d.Samples[idx[b]] })
	samples := make([]string, len(d.Samples))
	for i, j := range idx {
		samples[i] = d.Samples[j]
	}
	expr := make([][]float64, len(d.Expression))
	for i, row := range d.Expression {
		expr[i] = make([]float64, len(row))
		for j, k := range idx {
			expr[i][j] = row[k]
		}
	}
	eg := make([]float64, len(d.Eigengene))
	for i, j := range idx {
		eg[i] = d.Eigengene[j]
	}
	meta, colOrder := sampleMeta(sampleInfo)
	ann := buildSampleAnnotations(samples, meta, colOrder)
	keep := map[string]bool{}
	for _, c := range annCols {
		keep[c] = true
	}
	var filtered []map[string]any
	for _, a := range ann {
		if name, ok := a["name"].(string); ok && keep[name] {
			filtered = append(filtered, a)
		}
	}
	var out map[string]any
	if err := json.Unmarshal(detail, &out); err != nil {
		return "", err
	}
	out["samples"] = samples
	out["expression"] = expr
	out["eigengene"] = eg
	out["sample_annotations"] = filtered
	tmp, err := os.CreateTemp("", "geneheatmap-*.json")
	if err != nil {
		return "", err
	}
	if err := json.NewEncoder(tmp).Encode(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}
