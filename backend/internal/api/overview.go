package api

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/xuri/excelize/v2"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

func overviewData(jobDir, enrichType, ontology string, maxCount int, fdr float64, topN int) (json.RawMessage, error) {
	if fdr <= 0 || fdr > 1 {
		fdr = 0.05
	}
	if topN <= 0 {
		topN = 5
	}

	genes, samples, mat, err := readFilteredMatrix(jobDir)
	if err != nil {
		return nil, err
	}
	g, n := len(genes), len(samples)

	// sample metadata + ordering (group by "group", else sort sample id) + annotations
	meta, colOrder := readSampleMeta(jobDir)
	groupCol := ""
	for _, c := range colOrder {
		if c == "group" {
			groupCol = "group"
			break
		}
	}
	order := computeSampleOrder(samples, meta, groupCol)

	// reorder samples + matrix columns
	samples = permute(samples, order)
	for i := range mat {
		mat[i] = permute(mat[i], order)
	}
	sampleAnn := buildSampleAnnotations(samples, meta, colOrder)

	// z-score each gene (row)
	z := make([][]float64, g)
	for i := 0; i < g; i++ {
		mean := 0.0
		for j := 0; j < n; j++ {
			mean += mat[i][j]
		}
		mean /= float64(n)
		sd := 0.0
		for j := 0; j < n; j++ {
			d := mat[i][j] - mean
			sd += d * d
		}
		sd = math.Sqrt(sd / float64(n))
		if sd < 1e-12 {
			sd = 1
		}
		row := make([]float64, n)
		for j := 0; j < n; j++ {
			row[j] = (mat[i][j] - mean) / sd
		}
		z[i] = row
	}

	// module colors (per gene)
	colors, err := moduleColors(jobDir, g)
	if err != nil {
		return nil, err
	}

	// exclude grey genes
	keepIdx := make([]int, 0, g)
	for i, c := range colors {
		if c != "grey" {
			keepIdx = append(keepIdx, i)
		}
	}
	genes = permute(genes, keepIdx)
	colors = permute(colors, keepIdx)
	zz := make([][]float64, len(keepIdx))
	for i, ki := range keepIdx {
		zz[i] = z[ki]
	}
	z = zz
	g = len(genes)

	// module sizes + display order (grey already excluded)
	sizeMap := map[string]int{}
	for _, c := range colors {
		sizeMap[c]++
	}
	type modInfo struct {
		color string
		size  int
	}
	mods := make([]modInfo, 0, len(sizeMap))
	for c, s := range sizeMap {
		mods = append(mods, modInfo{c, s})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].size > mods[j].size })
	moduleOrder := make([]string, len(mods))
	for i, m := range mods {
		moduleOrder[i] = m.color
	}

	// eigengenes (transpose step4_mtrait.json samples×modules -> module->[per sample])
	// + reorder samples per `order` + exclude grey modules.
	eigengenes := map[string][]float64{}
	kmeByGene := map[string]float64{}
	modIndex := map[string]int{}
	mt := struct {
		Modules    []string    `json:"modules"`
		Eigengenes [][]float64 `json:"eigengenes"`
		Kme        [][]float64 `json:"kme"`
	}{}
	if b, err := os.ReadFile(filepath.Join(jobDir, "step4_mtrait.json")); err == nil {
		if json.Unmarshal(b, &mt) == nil && len(mt.Eigengenes) == n {
			for mi, m := range mt.Modules {
				modIndex[m] = mi
				vec := make([]float64, n)
				for j := 0; j < n && j < len(mt.Eigengenes); j++ {
					if mi < len(mt.Eigengenes[j]) {
						vec[j] = mt.Eigengenes[j][mi]
					}
				}
				eigengenes[m] = permute(vec, order)
			}
			// kME per kept gene (colors[i] is post-filter)
			for i := range genes {
				origIdx := keepIdx[i]
				if origIdx < len(mt.Kme) {
					if mi, ok := modIndex[colors[i]]; ok && mi < len(mt.Kme[origIdx]) {
						kmeByGene[genes[i]] = mt.Kme[origIdx][mi]
					}
				}
			}
		}
	}

	// enrichment per module (GO or KEGG); skip gracefully when no background.
	enrichMap := map[string][]map[string]any{}
	enrichAvailable := true
	if enrichType == "kegg" {
		if _, err := os.Stat(filepath.Join(jobDir, "gene2kegg.tsv")); err != nil {
			enrichAvailable = false
		}
	} else {
		if _, err := os.Stat(filepath.Join(jobDir, "gene2go.tsv")); err != nil {
			enrichAvailable = false
		}
	}
	if enrichAvailable {
		for _, m := range moduleOrder {
			mgenes, err := enrich.ModuleGenes(jobDir, m)
			if err != nil || len(mgenes) == 0 {
				continue
			}
			var rows []enrich.Result
			if enrichType == "kegg" {
				rows, err = enrich.KEGGEnrich(jobDir, mgenes, fdr, 20)
			} else {
				rows, err = enrich.GOEnrich(jobDir, mgenes, fdr, 20)
			}
			if err != nil {
				continue
			}
			enrichMap[m] = selectEnrichTerms(rows, enrichType, ontology, maxCount, topN)
		}
	}

	payload := map[string]any{
		"genes":              genes,
		"samples":            samples,
		"z":                  z,
		"modules":            colors,
		"module_order":       moduleOrder,
		"module_colors":      moduleColorMap(moduleOrder),
		"eigengenes":         eigengenes,
		"kme":                kmeByGene,
		"enrich":             enrichMap,
		"module_sizes":       sizeMap,
		"enrich_type":        enrichType,
		"enrich_ok":          enrichAvailable,
		"sample_annotations": sampleAnn,
	}
	return json.Marshal(payload)
}

// permute reorders a string or float slice by the given indices.
func permute[T any](s []T, idx []int) []T {
	out := make([]T, len(idx))
	for i, j := range idx {
		out[i] = s[j]
	}
	return out
}

// moduleColorMap maps each module to its color. For WGCNA the module name is
// itself a valid color name (turquoise, blue, ...); kmeans/cmeans pass "C1..Ck"
// which need generated colors, handled by the caller.
func moduleColorMap(moduleOrder []string) map[string]string {
	m := make(map[string]string, len(moduleOrder))
	for _, mod := range moduleOrder {
		m[mod] = mod
	}
	return m
}

// normOntology maps a GO ontology query value to the canonical short code.
// maxCountQ returns the max-term-gene-count filter (default 500 when absent;
// 0 disables the filter). Terms with more genes are dropped as too broad.
func maxCountQ(r *http.Request) int {
	if !r.URL.Query().Has("max_count") {
		return 500
	}
	return int(qfloat(r, "max_count"))
}

func normOntology(q string) string {
	switch strings.ToLower(q) {
	case "bp":
		return "BP"
	case "cc":
		return "CC"
	case "mf":
		return "MF"
	default:
		return ""
	}
}

// geneRatioFrac parses a "k/n" GeneRatio string to its fraction.
func geneRatioFrac(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	k, _ := strconv.ParseFloat(parts[0], 64)
	n, _ := strconv.ParseFloat(parts[1], 64)
	if n == 0 {
		return 0
	}
	return k / n
}

// selectEnrichTerms filters rows by GO ontology and max gene count, sorts by
// GeneRatio descending (matching the module-explorer ECharts order), and
// returns the top-N terms.
func selectEnrichTerms(rows []enrich.Result, enrichType, ontology string, maxCount, topN int) []map[string]any {
	filtered := make([]enrich.Result, 0, len(rows))
	for _, r := range rows {
		if enrichType == "go" && ontology != "" && r.Ontology != ontology {
			continue
		}
		if maxCount > 0 && r.Count > maxCount {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return geneRatioFrac(filtered[i].GeneRatio) > geneRatioFrac(filtered[j].GeneRatio)
	})
	if len(filtered) > topN {
		filtered = filtered[:topN]
	}
	terms := make([]map[string]any, 0, len(filtered))
	for _, r := range filtered {
		terms = append(terms, map[string]any{"term": r.Description, "padj": r.PAdjust, "ontology": r.Ontology})
	}
	return terms
}

// clusterPalette provides distinct colors for kmeans/cmeans clusters "C1..Ck".
var clusterPalette = []string{
	"#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
	"#8c564b", "#e377c2", "#7f7f7f", "#bcbd22", "#17becf",
	"#aec7e8", "#ffbb78", "#98df8a", "#ff9896", "#c5b0d5",
}

func clusterColorMap(clusterOrder []string) map[string]string {
	m := make(map[string]string, len(clusterOrder))
	for i, c := range clusterOrder {
		m[c] = clusterPalette[i%len(clusterPalette)]
	}
	return m
}

// zscoreRows z-scores each row of mat.
func zscoreRows(mat [][]float64) [][]float64 {
	g, n := len(mat), len(mat[0])
	z := make([][]float64, g)
	for i := 0; i < g; i++ {
		mean := 0.0
		for j := 0; j < n; j++ {
			mean += mat[i][j]
		}
		mean /= float64(n)
		sd := 0.0
		for j := 0; j < n; j++ {
			d := mat[i][j] - mean
			sd += d * d
		}
		sd = math.Sqrt(sd / float64(n))
		if sd < 1e-12 {
			sd = 1
		}
		row := make([]float64, n)
		for j := 0; j < n; j++ {
			row[j] = (mat[i][j] - mean) / sd
		}
		z[i] = row
	}
	return z
}

// clusterOverviewData builds the overview figure payload for a kmeans/cmeans
// result (overview_clusters.json), reusing the same plotting script as WGCNA.
func clusterOverviewData(jobDir, enrichType, ontology string, maxCount int, fdr float64, topN int) (json.RawMessage, error) {
	if fdr <= 0 || fdr > 1 {
		fdr = 0.05
	}
	if topN <= 0 {
		topN = 5
	}

	var cr struct {
		Genes        []string `json:"genes"`
		Samples      []string `json:"samples"`
		Clusters     []string `json:"clusters"`
		ClusterOrder []string `json:"cluster_order"`
		Method       string   `json:"method"`
	}
	b, err := os.ReadFile(filepath.Join(jobDir, "overview_clusters.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &cr); err != nil {
		return nil, err
	}
	if len(cr.Clusters) != len(cr.Genes) {
		return nil, os.ErrInvalid
	}

	// filtered matrix (genes × samples)
	genes, samples, mat, err := readFilteredMatrix(jobDir)
	if err != nil {
		return nil, err
	}
	// align: cluster genes -> filtered matrix row
	rowIdx := make(map[string]int, len(genes))
	for i, g := range genes {
		rowIdx[g] = i
	}
	clusterMat := make([][]float64, 0, len(cr.Genes))
	for _, cg := range cr.Genes {
		if ri, ok := rowIdx[cg]; ok {
			clusterMat = append(clusterMat, mat[ri])
		}
	}
	if len(clusterMat) != len(cr.Genes) {
		return nil, os.ErrInvalid
	}

	// sample reorder (group by "group", else sort id) + annotations
	meta, colOrder := readSampleMeta(jobDir)
	groupCol := ""
	for _, c := range colOrder {
		if c == "group" {
			groupCol = "group"
			break
		}
	}
	order := computeSampleOrder(samples, meta, groupCol)
	samples = permute(samples, order)
	for i := range clusterMat {
		clusterMat[i] = permute(clusterMat[i], order)
	}
	sampleAnn := buildSampleAnnotations(samples, meta, colOrder)

	z := zscoreRows(clusterMat)

	// enrichment per cluster (reuse Go enrich, genes are the cluster members)
	enrichMap := map[string][]map[string]any{}
	enrichAvailable := true
	if enrichType == "kegg" {
		if _, err := os.Stat(filepath.Join(jobDir, "gene2kegg.tsv")); err != nil {
			enrichAvailable = false
		}
	} else {
		if _, err := os.Stat(filepath.Join(jobDir, "gene2go.tsv")); err != nil {
			enrichAvailable = false
		}
	}
	if enrichAvailable {
		for _, c := range cr.ClusterOrder {
			var cgenes []string
			for i, cl := range cr.Clusters {
				if cl == c {
					cgenes = append(cgenes, cr.Genes[i])
				}
			}
			if len(cgenes) == 0 {
				continue
			}
			var rows []enrich.Result
			if enrichType == "kegg" {
				rows, err = enrich.KEGGEnrich(jobDir, cgenes, fdr, 20)
			} else {
				rows, err = enrich.GOEnrich(jobDir, cgenes, fdr, 20)
			}
			if err != nil {
				continue
			}
			enrichMap[c] = selectEnrichTerms(rows, enrichType, ontology, maxCount, topN)
		}
	}

	payload := map[string]any{
		"genes":              cr.Genes,
		"samples":            samples,
		"z":                  z,
		"modules":            cr.Clusters,
		"module_order":       cr.ClusterOrder,
		"module_colors":      clusterColorMap(cr.ClusterOrder),
		"enrich":             enrichMap,
		"enrich_type":        enrichType,
		"enrich_ok":          enrichAvailable,
		"sample_annotations": sampleAnn,
		"cluster_method":     cr.Method,
	}
	return json.Marshal(payload)
}

// readSampleMeta reads sample_info.tsv into map[sampleID]map[column]value + column order.
func readSampleMeta(jobDir string) (map[string]map[string]string, []string) {
	b, err := os.ReadFile(filepath.Join(jobDir, "sample_info.tsv"))
	if err != nil {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(string(b)))
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

// computeSampleOrder returns indices that group samples by groupCol (stable),
// or sort by sample id when groupCol is empty.
func computeSampleOrder(samples []string, meta map[string]map[string]string, groupCol string) []int {
	idx := make([]int, len(samples))
	for i := range idx {
		idx[i] = i
	}
	if groupCol != "" {
		sort.SliceStable(idx, func(a, b int) bool {
			ga := ""
			gb := ""
			if meta != nil {
				ga = meta[samples[idx[a]]][groupCol]
				gb = meta[samples[idx[b]]][groupCol]
			}
			return ga < gb
		})
	} else {
		sort.SliceStable(idx, func(a, b int) bool { return samples[idx[a]] < samples[idx[b]] })
	}
	return idx
}

// buildSampleAnnotations returns the first up-to-3 metadata columns (excluding
// sample id/name) as {name, values-per-sample} annotations for the x-axis.
func buildSampleAnnotations(samples []string, meta map[string]map[string]string, colOrder []string) []map[string]any {
	var ann []map[string]any
	for _, col := range colOrder {
		values := make([]string, len(samples))
		has := false
		for i, s := range samples {
			if meta != nil {
				if v, ok := meta[s][col]; ok {
					values[i] = v
					has = true
				}
			}
		}
		if !has {
			continue
		}
		ann = append(ann, map[string]any{"name": col, "values": values})
		if len(ann) >= 3 {
			break
		}
	}
	return ann
}

// readFilteredMatrix reads step1_filtered.tsv as genes (rows) × samples (cols).
func readFilteredMatrix(jobDir string) ([]string, []string, [][]float64, error) {
	path := filepath.Join(jobDir, "step1_filtered.tsv")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil, nil, err
	}
	samples := header[1:]

	var genes []string
	var mat [][]float64
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 2 {
			continue
		}
		genes = append(genes, rec[0])
		row := make([]float64, len(samples))
		for j := 0; j < len(samples) && j+1 < len(rec); j++ {
			v, _ := strconv.ParseFloat(rec[j+1], 64)
			row[j] = v
		}
		mat = append(mat, row)
	}
	return genes, samples, mat, nil
}

// moduleColors returns the module color per gene from step3_modules.json.
func moduleColors(jobDir string, g int) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json"))
	if err != nil {
		return nil, err
	}
	var mf struct {
		Genes  []string `json:"genes"`
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(b, &mf); err != nil {
		return nil, err
	}
	if len(mf.Colors) != g {
		return nil, os.ErrInvalid
	}
	return mf.Colors, nil
}

// overview returns the module overview data payload.
func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	jobDir := filepath.Join(h.jobs.Dir(), id)
	enrichType := r.URL.Query().Get("type")
	if enrichType != "kegg" {
		enrichType = "go"
	}
	data, err := overviewData(jobDir, enrichType, normOntology(r.URL.Query().Get("onto")), maxCountQ(r), qfloat(r, "fdr"), int(qfloat(r, "top")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(data))
}

// plotOverview renders the three-part module overview figure.
func (h *Handler) plotOverview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	enrichType := r.URL.Query().Get("type")
	if enrichType != "kegg" {
		enrichType = "go"
	}
	data, err := overviewData(jobDir, enrichType, normOntology(r.URL.Query().Get("onto")), maxCountQ(r), qfloat(r, "fdr"), int(qfloat(r, "top")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		BaseSize:     qfloat(r, "base_size"),
		ShowCategory: int(qfloat(r, "top")),
	}
	out, ctype, err := plot.RenderModuleOverview(data, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="module_overview.`+mode+`"`)
	http.ServeFile(w, r, out)
}

// plotOverviewCluster renders the three-part figure for a kmeans/cmeans result.
func (h *Handler) plotOverviewCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	enrichType := r.URL.Query().Get("type")
	if enrichType != "kegg" {
		enrichType = "go"
	}
	data, err := clusterOverviewData(jobDir, enrichType, normOntology(r.URL.Query().Get("onto")), maxCountQ(r), qfloat(r, "fdr"), int(qfloat(r, "top")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{
		Low:          r.URL.Query().Get("low"),
		Mid:          r.URL.Query().Get("mid"),
		High:         r.URL.Query().Get("high"),
		Width:        qfloat(r, "width"),
		Height:       qfloat(r, "height"),
		Dpi:          qfloat(r, "dpi"),
		BaseSize:     qfloat(r, "base_size"),
		ShowCategory: int(qfloat(r, "top")),
	}
	out, ctype, err := plot.RenderModuleOverview(data, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="cluster_overview.`+mode+`"`)
	http.ServeFile(w, r, out)
}
func updateParamsField(jobDir, key string, value any) error {
	path := filepath.Join(jobDir, "params.json")
	var p map[string]any
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	if p == nil {
		p = map[string]any{}
	}
	p[key] = value
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// runCluster triggers the Rust KMeans / fuzzy c-means comparison step.
func (h *Handler) runCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	method := r.URL.Query().Get("method")
	if method != "cmeans" {
		method = "kmeans"
	}
	k := int(qfloat(r, "k"))
	seed := int(qfloat(r, "seed"))
	if seed <= 0 {
		seed = 42
	}
	kmax := int(qfloat(r, "kmax"))
	if kmax <= 0 {
		kmax = 12
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	if err := updateParamsField(jobDir, "cluster", map[string]any{
		"method": method, "k": k, "seed": seed, "kmax": kmax,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	go h.jobs.RunAuxStep(id, "cluster")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": "cluster"})
}

// getCluster returns the clustering result (overview_clusters.json).
func (h *Handler) getCluster(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := os.ReadFile(filepath.Join(h.jobs.Dir(), id, "overview_clusters.json"))
	if err != nil {
		http.Error(w, "clustering not run yet", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(b))
}

// runClusterElbow triggers the elbow/validity metrics computation (k=2..kmax)
// without running the final clustering; getClusterElbow reads the result.
func (h *Handler) runClusterElbow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	method := r.URL.Query().Get("method")
	if method != "cmeans" {
		method = "kmeans"
	}
	seed := int(qfloat(r, "seed"))
	if seed <= 0 {
		seed = 42
	}
	kmax := int(qfloat(r, "kmax"))
	if kmax <= 0 {
		kmax = 12
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	if err := updateParamsField(jobDir, "cluster", map[string]any{"method": method, "seed": seed, "kmax": kmax}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	go h.jobs.RunAuxStep(id, "cluster_elbow")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": "cluster_elbow"})
}

func (h *Handler) getClusterElbow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := os.ReadFile(filepath.Join(h.jobs.Dir(), id, "overview_elbow.json"))
	if err != nil {
		http.Error(w, "elbow not computed yet", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(b))
}

// runOverviewNetwork triggers the overview network edge/degree computation.
func (h *Handler) runOverviewNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	jobDir := filepath.Join(h.jobs.Dir(), id)
	pct := qfloat(r, "percentile")
	if pct <= 0 || pct > 100 {
		pct = 10
	}
	if err := updateParamsField(jobDir, "overview", map[string]any{"edge_percentile": pct}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	go h.jobs.RunAuxStep(id, "overview_network")
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": "overview_network"})
}

// getOverviewNetwork returns the overview network (nodes + limited edges).
// per_module caps the number of edges kept per module (and cross-module), so the
// WebGL preview stays responsive; the exported SVG/PDF uses the full edge set.
func (h *Handler) getOverviewNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	jobDir := filepath.Join(h.jobs.Dir(), id)
	maxNodes := int(qfloat(r, "max_nodes"))
	if maxNodes <= 0 {
		maxNodes = 2000
	}
	minPerModule := int(qfloat(r, "min_per_module"))
	if minPerModule <= 0 {
		minPerModule = 20
	}
	edgesPerGene := int(qfloat(r, "edges_per_gene"))
	if edgesPerGene <= 0 {
		edgesPerGene = 8
	}
	hideGrey := true
	if q := r.URL.Query().Get("hide_grey"); q == "0" || q == "false" {
		hideGrey = false
	}
	nodes, edges, err := readNetworkSampled(jobDir, maxNodes, minPerModule, edgesPerGene, hideGrey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "edges": edges})
}

type netNode struct {
	ID     string  `json:"id"`
	Color  string  `json:"color"`
	Degree int     `json:"degree"`
	Kme    float64 `json:"kme"` // |kME| to own module (hub metric)
}
type netEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float64 `json:"weight"`
}

// readKmeByGene returns |kME| to each gene's own module from step4_mtrait.json.
func readKmeByGene(jobDir string, geneMod map[string]string) map[string]float64 {
	b, err := os.ReadFile(filepath.Join(jobDir, "step4_mtrait.json"))
	if err != nil {
		return nil
	}
	var mt struct {
		Modules []string    `json:"modules"`
		Genes   []string    `json:"genes"`
		Kme     [][]float64 `json:"kme"`
	}
	if json.Unmarshal(b, &mt) != nil {
		return nil
	}
	modIdx := map[string]int{}
	for i, m := range mt.Modules {
		modIdx[m] = i
	}
	out := map[string]float64{}
	for i, g := range mt.Genes {
		if i >= len(mt.Kme) {
			break
		}
		mi, ok := modIdx[geneMod[g]]
		if !ok || mi >= len(mt.Kme[i]) {
			continue
		}
		out[g] = math.Abs(mt.Kme[i][mi])
	}
	return out
}

// readNetworkSampled returns a module-aware, hub-prioritized sample of the
// network: all modules are preserved, each keeps a minimum number of genes plus
// a sublinear (sqrt) share of the remaining budget, ordered by |kME| so hub
// genes are kept. Edges are intra-module only, top edgesPerGene per gene,
// deduplicated undirected.
func readNetworkSampled(jobDir string, maxNodes, minPerModule, edgesPerGene int, hideGrey bool) ([]netNode, []netEdge, error) {
	nodes, geneMod, err := readNetworkNodes(filepath.Join(jobDir, "network_nodes.tsv"))
	if err != nil {
		return nil, nil, err
	}
	kme := readKmeByGene(jobDir, geneMod)

	// group + order each module by |kME| descending (hub genes first)
	byMod := map[string][]netNode{}
	for _, n := range nodes {
		if hideGrey && n.Color == "grey" {
			continue
		}
		n.Kme = kme[n.ID]
		byMod[n.Color] = append(byMod[n.Color], n)
	}
	for _, list := range byMod {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Kme != list[j].Kme {
				return list[i].Kme > list[j].Kme
			}
			return list[i].Degree > list[j].Degree
		})
	}
	mods := make([]string, 0, len(byMod))
	for m := range byMod {
		mods = append(mods, m)
	}
	sort.Strings(mods)

	// sublinear allocation: sqrt(size) share, with a per-module floor
	sqrtSum := 0.0
	for _, m := range mods {
		sqrtSum += math.Sqrt(float64(len(byMod[m])))
	}
	visible := map[string]bool{}
	var sampled []netNode
	for _, m := range mods {
		list := byMod[m]
		alloc := minPerModule
		if sqrtSum > 0 {
			alloc = int(float64(maxNodes) * math.Sqrt(float64(len(list))) / sqrtSum)
		}
		if alloc < minPerModule {
			alloc = minPerModule
		}
		if alloc > len(list) {
			alloc = len(list)
		}
		for i := 0; i < alloc; i++ {
			sampled = append(sampled, list[i])
			visible[list[i].ID] = true
		}
	}

	// edges: intra-module + visible endpoints, top-N per gene, dedup
	type ew struct {
		b string
		w float64
	}
	adj := map[string][]ew{}
	f, err := os.Open(filepath.Join(jobDir, "network_edges.tsv"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil { // header
		return nil, nil, err
	}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 3 {
			continue
		}
		a, b := rec[0], rec[1]
		if !visible[a] || !visible[b] || geneMod[a] != geneMod[b] {
			continue
		}
		w, _ := strconv.ParseFloat(rec[2], 64)
		adj[a] = append(adj[a], ew{b, w})
		adj[b] = append(adj[b], ew{a, w})
	}

	seen := map[string]bool{}
	var out []netEdge
	key := func(a, b string) string {
		if a > b {
			a, b = b, a
		}
		return a + "\x00" + b
	}
	for g, list := range adj {
		sort.Slice(list, func(i, j int) bool { return list[i].w > list[j].w })
		for i, e := range list {
			if i >= edgesPerGene {
				break
			}
			k := key(g, e.b)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, netEdge{g, e.b, e.w})
		}
	}
	return sampled, out, nil
}

// readNetworkLimited reads network_nodes.tsv + network_edges.tsv and keeps the
// top per_module edges per module (and cross-module) by weight.
func readNetworkLimited(jobDir string, perModule int) ([]netNode, []netEdge, error) {
	nodes, geneMod, err := readNetworkNodes(filepath.Join(jobDir, "network_nodes.tsv"))
	if err != nil {
		return nil, nil, err
	}

	type edge struct {
		a, b    string
		w       float64
		cross   bool
	}
	byModule := map[string][]edge{}
	var cross []edge

	f, err := os.Open(filepath.Join(jobDir, "network_edges.tsv"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil { // header
		return nil, nil, err
	}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 3 {
			continue
		}
		w, _ := strconv.ParseFloat(rec[2], 64)
		ma := geneMod[rec[0]]
		mb := geneMod[rec[1]]
		e := edge{rec[0], rec[1], w, ma != mb}
		if ma != "" && ma == mb {
			byModule[ma] = append(byModule[ma], e)
		} else {
			cross = append(cross, e)
		}
	}

	keep := func(list []edge, n int) []edge {
		if len(list) <= n {
			return list
		}
		sort.Slice(list, func(i, j int) bool { return list[i].w > list[j].w })
		return list[:n]
	}
	var out []netEdge
	for _, list := range byModule {
		for _, e := range keep(list, perModule) {
			out = append(out, netEdge{e.a, e.b, e.w})
		}
	}
	for _, e := range keep(cross, perModule) {
		out = append(out, netEdge{e.a, e.b, e.w})
	}
	return nodes, out, nil
}

func readNetworkNodes(path string) ([]netNode, map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil { // header
		return nil, nil, err
	}
	var nodes []netNode
	geneMod := map[string]string{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 3 {
			continue
		}
		d, _ := strconv.Atoi(rec[2])
		nodes = append(nodes, netNode{ID: rec[0], Color: rec[1], Degree: d})
		geneMod[rec[0]] = rec[1]
	}
	return nodes, geneMod, nil
}

// plotOverviewNetwork renders the gene-level overview network (igraph DrL).
func (h *Handler) plotOverviewNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mode := r.URL.Query().Get("mode")
	if mode != "pdf" && mode != "svg" {
		mode = "png"
	}
	jobDir := filepath.Join(h.jobs.Dir(), id)
	// Export uses a larger per-module edge budget than the WebGL preview.
	perModule := int(qfloat(r, "per_module"))
	if perModule <= 0 {
		perModule = 5000
	}
	nodes, edges, err := readNetworkLimited(jobDir, perModule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	data, err := json.Marshal(map[string]any{"nodes": nodes, "edges": edges})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{
		Width:  qfloat(r, "width"),
		Height: qfloat(r, "height"),
		Dpi:    qfloat(r, "dpi"),
	}
	out, ctype, err := plot.RenderGeneNetwork(data, mode, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(filepath.Dir(out))
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="overview_network.`+mode+`"`)
	http.ServeFile(w, r, out)
}

// exportOverviewXlsx writes a multi-sheet xlsx: one sheet per module (mode=modules)
// or per cluster (mode=clusters). Columns: gene, module/cluster, kME/score.
func (h *Handler) exportOverviewXlsx(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	jobDir := filepath.Join(h.jobs.Dir(), id)
	mode := r.URL.Query().Get("mode")

	f := excelize.NewFile()
	defer f.Close()

	if mode == "clusters" {
		if err := writeClusterSheets(f, filepath.Join(jobDir, "overview_clusters.json")); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	} else {
		if err := writeModuleSheets(f, jobDir); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}
	// Delete the default "Sheet1" after the real sheets exist (DeleteSheet is a
	// no-op when there is only one sheet).
	_ = f.DeleteSheet("Sheet1")

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="overview.xlsx"`)
	if err := f.Write(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeModuleSheets writes one sheet per WGCNA module with gene + kME.
func writeModuleSheets(f *excelize.File, jobDir string) error {
	modB, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json"))
	if err != nil {
		return err
	}
	var mf struct {
		Genes  []string `json:"genes"`
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(modB, &mf); err != nil {
		return err
	}

	// kME matrix (genes × modules) from step4_mtrait.json
	type mtraitT struct {
		Modules []string    `json:"modules"`
		Kme     [][]float64 `json:"kme"`
	}
	var mt mtraitT
	if b, err := os.ReadFile(filepath.Join(jobDir, "step4_mtrait.json")); err == nil {
		_ = json.Unmarshal(b, &mt)
	}
	modIndex := map[string]int{}
	for i, m := range mt.Modules {
		modIndex[m] = i
	}

	// group genes by module
	byMod := map[string][][2]any{} // module -> rows {gene, kme}
	for i, gene := range mf.Genes {
		m := mf.Colors[i]
		var kme any
		if mi, ok := modIndex[m]; ok && i < len(mt.Kme) && mi < len(mt.Kme[i]) {
			kme = mt.Kme[i][mi]
		} else {
			kme = ""
		}
		byMod[m] = append(byMod[m], [2]any{gene, kme})
	}

	// sort modules by size desc, grey last
	mods := make([]string, 0, len(byMod))
	for m := range byMod {
		mods = append(mods, m)
	}
	sort.Slice(mods, func(i, j int) bool {
		if mods[i] == "grey" {
			return false
		}
		if mods[j] == "grey" {
			return true
		}
		return len(byMod[mods[i]]) > len(byMod[mods[j]])
	})

	for _, m := range mods {
		name := m
		if len(name) > 31 {
			name = name[:31]
		}
		idx, err := f.NewSheet(name)
		if err != nil {
			continue
		}
		_ = f.SetCellValue(name, "A1", "gene")
		_ = f.SetCellValue(name, "B1", "module")
		_ = f.SetCellValue(name, "C1", "kME")
		for r, row := range byMod[m] {
			rowIdx := r + 2
			_ = f.SetCellValue(name, "A"+strconv.Itoa(rowIdx), row[0])
			_ = f.SetCellValue(name, "B"+strconv.Itoa(rowIdx), m)
			_ = f.SetCellValue(name, "C"+strconv.Itoa(rowIdx), row[1])
		}
		_ = idx
	}
	return nil
}

// writeClusterSheets writes one sheet per cluster with gene + score.
func writeClusterSheets(f *excelize.File, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var d struct {
		Genes    []string  `json:"genes"`
		Clusters []string  `json:"clusters"`
		Score    []float64 `json:"score"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	byCl := map[string][][2]any{}
	for i, gene := range d.Genes {
		c := d.Clusters[i]
		var s any
		if i < len(d.Score) {
			s = d.Score[i]
		} else {
			s = ""
		}
		byCl[c] = append(byCl[c], [2]any{gene, s})
	}
	clusters := make([]string, 0, len(byCl))
	for c := range byCl {
		clusters = append(clusters, c)
	}
	sort.Strings(clusters)
	for _, c := range clusters {
		idx, err := f.NewSheet(c)
		if err != nil {
			continue
		}
		_ = f.SetCellValue(c, "A1", "gene")
		_ = f.SetCellValue(c, "B1", "cluster")
		_ = f.SetCellValue(c, "C1", "score")
		for r, row := range byCl[c] {
			rowIdx := r + 2
			_ = f.SetCellValue(c, "A"+strconv.Itoa(rowIdx), row[0])
			_ = f.SetCellValue(c, "B"+strconv.Itoa(rowIdx), c)
			_ = f.SetCellValue(c, "C"+strconv.Itoa(rowIdx), row[1])
		}
		_ = idx
	}
	return nil
}
