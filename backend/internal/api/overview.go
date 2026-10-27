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

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/xuri/excelize/v2"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

// overviewData assembles the "module overview" payload for the three-part figure:
// z-scored expression, per-gene module colors, per-module eigengenes, and
// per-module enrichment top-N terms (skipped when no background file is present).
func overviewData(jobDir, enrichType string, fdr float64, topN int) (json.RawMessage, error) {
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

	// module colors (per gene) + sizes + display order
	colors, err := moduleColors(jobDir, g)
	if err != nil {
		return nil, err
	}
	type modInfo struct {
		color string
		size  int
	}
	sizeMap := map[string]int{}
	for _, c := range colors {
		sizeMap[c]++
	}
	mods := make([]modInfo, 0, len(sizeMap))
	for c, s := range sizeMap {
		mods = append(mods, modInfo{c, s})
	}
	sort.Slice(mods, func(i, j int) bool {
		if mods[i].color == "grey" {
			return false
		}
		if mods[j].color == "grey" {
			return true
		}
		return mods[i].size > mods[j].size
	})
	moduleOrder := make([]string, len(mods))
	for i, m := range mods {
		moduleOrder[i] = m.color
	}

	// eigengenes (transpose step4_mtrait.json samples×modules -> module->[per sample])
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
				eigengenes[m] = vec
			}
			for i := range genes {
				if i < len(mt.Kme) {
					if mi, ok := modIndex[colors[i]]; ok && mi < len(mt.Kme[i]) {
						kmeByGene[genes[i]] = mt.Kme[i][mi]
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
			if m == "grey" {
				continue
			}
			mgenes, err := enrich.ModuleGenes(jobDir, m)
			if err != nil || len(mgenes) == 0 {
				continue
			}
			var rows []enrich.Result
			if enrichType == "kegg" {
				rows, err = enrich.KEGGEnrich(jobDir, mgenes, fdr, topN)
			} else {
				rows, err = enrich.GOEnrich(jobDir, mgenes, fdr, topN)
			}
			if err != nil {
				continue
			}
			terms := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				terms = append(terms, map[string]any{"term": r.Description, "padj": r.PAdjust})
			}
			if len(terms) > topN {
				terms = terms[:topN]
			}
			enrichMap[m] = terms
		}
	}

	payload := map[string]any{
		"genes":         genes,
		"samples":       samples,
		"z":             z,
		"modules":       colors,
		"module_order":  moduleOrder,
		"eigengenes":    eigengenes,
		"kme":           kmeByGene,
		"enrich":        enrichMap,
		"module_sizes":  sizeMap,
		"enrich_type":   enrichType,
		"enrich_ok":     enrichAvailable,
	}
	return json.Marshal(payload)
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
	data, err := overviewData(jobDir, enrichType, qfloat(r, "fdr"), int(qfloat(r, "top")))
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
	data, err := overviewData(jobDir, enrichType, qfloat(r, "fdr"), int(qfloat(r, "top")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := plot.PlotParams{
		Low:       r.URL.Query().Get("low"),
		Mid:       r.URL.Query().Get("mid"),
		High:      r.URL.Query().Get("high"),
		Width:     qfloat(r, "width"),
		Height:    qfloat(r, "height"),
		Dpi:       qfloat(r, "dpi"),
		BaseSize:  qfloat(r, "base_size"),
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

// updateParamsField merges one top-level field into the job's params.json.
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
	perModule := int(qfloat(r, "per_module"))
	if perModule <= 0 {
		perModule = 300
	}
	nodes, edges, err := readNetworkLimited(jobDir, perModule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "edges": edges})
}

type netNode struct {
	ID     string `json:"id"`
	Color  string `json:"color"`
	Degree int    `json:"degree"`
}
type netEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float64 `json:"weight"`
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
		nodes = append(nodes, netNode{rec[0], rec[1], d})
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
