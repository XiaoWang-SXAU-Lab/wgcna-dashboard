package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
)

type figureSnapshot struct {
	Key         string `json:"key"`
	Trait       string `json:"trait"`
	Fingerprint string `json:"fingerprint"`
	PNG         string `json:"png"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Origin      string `json:"origin"`
	CapturedAt  string `json:"captured_at"`
}

// Only report-owned image names may be saved; never accept filesystem paths.
func figureKeyOK(key string) bool {
	switch key {
	case "02_filter/sample_tree.png", "03_soft_threshold/sft.png", "04_network/dendrogram.png", "05_module_trait/mtrait.png", "07_hub/hub_network.png":
		return true
	}
	parts := strings.Split(key, "/")
	if len(parts) < 3 || parts[0] != "06_module_detail" || !reportNameOK(parts[1]) {
		return false
	}
	return len(parts) == 3 && (parts[2] == "gs_mm.png" || parts[2] == "expression_heatmap.png" || parts[2] == "hub_network.png") || len(parts) == 4 && parts[2] == "enrichment" && (parts[3] == "go.png" || parts[3] == "kegg.png")
}

func figureFingerprint(dir, key string) string {
	files := []string{"step1_filter.json", "sample_info.tsv"}
	switch {
	case strings.HasPrefix(key, "03_"):
		files = []string{"step2_sft.json"}
	case strings.HasPrefix(key, "04_"):
		files = []string{"step3_modules.json"}
	case strings.HasPrefix(key, "05_"):
		files = []string{"step4_mtrait.json"}
	case strings.HasPrefix(key, "06_"):
		files = []string{"step1_filtered.tsv", "step3_modules.json", "step4_mtrait.json", "annotation.tsv"}
		if strings.Contains(key, "/enrichment/") {
			files = append(files, "gene2go.tsv", "gene2kegg.tsv")
		}
	case strings.HasPrefix(key, "07_"):
		files = []string{"step6_hub.json", "annotation.tsv"}
	}
	hash := sha256.New()
	io.WriteString(hash, key)
	for _, name := range files {
		io.WriteString(hash, "\x00"+name+"\x00")
		f, err := os.Open(filepath.Join(dir, name))
		if err == nil {
			_, _ = io.Copy(hash, f)
			_ = f.Close()
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func snapshotFile(dir, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "report-snapshots", hex.EncodeToString(sum[:])+".json")
}

func readFigureSnapshot(dir, key, trait string) (*figureSnapshot, error) {
	b, err := os.ReadFile(snapshotFile(dir, key))
	if err != nil {
		return nil, err
	}
	var s figureSnapshot
	if err = json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Key != key || s.Fingerprint != figureFingerprint(dir, key) || (trait != "" && s.Trait != trait) {
		return nil, fmt.Errorf("snapshot is stale or belongs to another trait")
	}
	return &s, nil
}

func (h *Handler) saveReportSnapshot(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(h.jobs.Dir(), r.PathValue("id"))
	if _, err := os.Stat(filepath.Join(dir, "params.json")); err != nil {
		http.Error(w, "job not found", 404)
		return
	}
	var s figureSnapshot
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&s); err != nil || !figureKeyOK(s.Key) {
		http.Error(w, "invalid figure snapshot", 400)
		return
	}
	if !strings.HasPrefix(s.PNG, "data:image/png;base64,") {
		http.Error(w, "PNG required", 400)
		return
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s.PNG, "data:image/png;base64,"))
	if err != nil {
		http.Error(w, "invalid PNG", 400)
		return
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 100_000_000 {
		http.Error(w, "invalid PNG dimensions", 400)
		return
	}
	fingerprint := figureFingerprint(dir, s.Key)
	if s.Fingerprint != "" && s.Fingerprint != fingerprint {
		http.Error(w, "Analysis changed while rendering; regenerate report.", 409)
		return
	}
	s.Origin = "analysis-snapshot"
	if s.Fingerprint != "" {
		s.Origin = "existing-app-component"
	}
	s.Fingerprint = fingerprint
	s.CapturedAt = time.Now().Format(time.RFC3339)
	dest := snapshotFile(dir, s.Key)
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err == nil {
		var tmp *os.File
		tmp, err = os.CreateTemp(filepath.Dir(dest), ".figure-*")
		if err == nil {
			name := tmp.Name()
			defer os.Remove(name)
			err = json.NewEncoder(tmp).Encode(s)
			closeErr := tmp.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(name, dest)
			}
		}
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved"})
}

type keyModule struct {
	Module string  `json:"module"`
	Trait  string  `json:"trait"`
	Cor    float64 `json:"cor"`
	P      float64 `json:"p"`
}

func significantModules(dir string) []keyModule {
	var mt struct {
		Modules, Traits []string
		Cor, P          [][]float64
	}
	b, err := os.ReadFile(filepath.Join(dir, "step4_mtrait.json"))
	if err != nil || json.Unmarshal(b, &mt) != nil {
		return nil
	}
	result := []keyModule{}
	for mi, m := range mt.Modules {
		if m == "grey" || !reportNameOK(m) {
			continue
		}
		best := -1
		score := -1.0
		for ti := range mt.Traits {
			if mi >= len(mt.P) || ti >= len(mt.P[mi]) || mi >= len(mt.Cor) || ti >= len(mt.Cor[mi]) {
				continue
			}
			p, c := mt.P[mi][ti], mt.Cor[mi][ti]
			if !math.IsNaN(p) && p >= 0 && p < 0.05 && !math.IsNaN(c) && math.Abs(c) > score {
				best = ti
				score = math.Abs(c)
			}
		}
		if best >= 0 {
			result = append(result, keyModule{m, mt.Traits[best], mt.Cor[mi][best], mt.P[mi][best]})
		}
	}
	return result
}

// significantPairs returns every module–trait pair with |cor| > 0.65 and
// p < 0.05 (one entry per pair, for the module–trait tab panels).
func significantPairs(dir string) []keyModule {
	var mt struct {
		Modules, Traits []string
		Cor, P          [][]float64
	}
	b, err := os.ReadFile(filepath.Join(dir, "step4_mtrait.json"))
	if err != nil || json.Unmarshal(b, &mt) != nil {
		return nil
	}
	result := []keyModule{}
	for mi, m := range mt.Modules {
		if m == "grey" || !reportNameOK(m) {
			continue
		}
		for ti := range mt.Traits {
			if mi >= len(mt.P) || ti >= len(mt.P[mi]) || mi >= len(mt.Cor) || ti >= len(mt.Cor[mi]) {
				continue
			}
			p, c := mt.P[mi][ti], mt.Cor[mi][ti]
			if !math.IsNaN(p) && p < 0.05 && !math.IsNaN(c) && math.Abs(c) > 0.65 {
				result = append(result, keyModule{m, mt.Traits[ti], c, p})
			}
		}
	}
	return result
}

type reportFigureTask struct {
	Key         string          `json:"key"`
	Kind        string          `json:"kind"`
	Trait       string          `json:"trait"`
	Fingerprint string          `json:"fingerprint"`
	Data        json.RawMessage `json:"data"`
}

// Prepare data only. All drawing is done by the existing Vue components, in
// an isolated offscreen host, without changing the user's active store/job.
func (h *Handler) prepareReportFigures(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	release, err := h.jobs.ReserveSnapshot(id)
	if err != nil {
		runError(w, err)
		return
	}
	defer release()
	dir := filepath.Join(h.jobs.Dir(), id)
	if _, err := os.Stat(filepath.Join(dir, "step3_modules.json")); err != nil {
		http.Error(w, "Run Network & modules first.", 400)
		return
	}
	tasks := []reportFigureTask{}
	add := func(key, kind, trait string, data []byte) {
		if _, err := readFigureSnapshot(dir, key, trait); err == nil {
			return
		}
		if json.Valid(data) {
			tasks = append(tasks, reportFigureTask{key, kind, trait, figureFingerprint(dir, key), json.RawMessage(data)})
		}
	}
	load := func(file string) []byte { b, _ := os.ReadFile(filepath.Join(dir, file)); return b }
	if data, err := sampleTreeData(dir); err == nil {
		add("02_filter/sample_tree.png", "sample-tree", "", data)
	}
	add("03_soft_threshold/sft.png", "sft", "", load("step2_sft.json"))
	add("04_network/dendrogram.png", "dendrogram", "", load("step3_modules.json"))
	add("05_module_trait/mtrait.png", "trait", "", load("step4_mtrait.json"))
	keys := significantModules(dir)
	if len(keys) > 0 {
		snapshot, err := os.MkdirTemp("", "wgcna-figure-data-")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer os.RemoveAll(snapshot)
		for _, name := range []string{"params.json", "step1_filtered.tsv", "step3_modules.json", "step4_mtrait.json", "trait.tsv", "sample_info.tsv", "gene2go.tsv", "gene2kegg.tsv", "annotation.tsv"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				if err := checkedCopy(filepath.Join(dir, name), filepath.Join(snapshot, name)); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		if err := h.jobs.PrepareReportDetails(ctx, id, snapshot); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, m := range keys {
			prefix := "06_module_detail/" + m.Module + "/"
			detail, err := os.ReadFile(filepath.Join(snapshot, "step5_detail_"+m.Module+".json"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			add(prefix+"gs_mm.png", "scatter", m.Trait, detail)
			add(prefix+"expression_heatmap.png", "expression", m.Trait, detail)
			genes, err := enrich.ModuleGenes(snapshot, m.Module)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			kind := r.URL.Query().Get("type")
			if kind != "kegg" {
				kind = "go"
			}
			var rows []enrich.Result
			if kind == "go" {
				rows, err = enrich.GOEnrich(snapshot, genes, 0.05, 20)
			} else {
				rows, err = enrich.KEGGEnrich(snapshot, genes, 0.05, 20)
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			filtered := []enrich.Result{}
			for _, row := range rows {
				if row.Count <= 500 {
					filtered = append(filtered, row)
				}
			}
			if len(filtered) > 0 {
				b, _ := json.Marshal(filtered)
				add(prefix+"enrichment/"+kind+".png", "enrich-"+kind, "", b)
			}
			var params map[string]any
			b, _ := os.ReadFile(filepath.Join(snapshot, "params.json"))
			_ = json.Unmarshal(b, &params)
			hub, ok := params["hub"].(map[string]any)
			if !ok {
				hub = map[string]any{}
				params["hub"] = hub
			}
			hub["select_module"] = m.Module
			hub["select_trait"] = m.Trait
			b, _ = json.Marshal(params)
			_ = os.WriteFile(filepath.Join(snapshot, "params.json"), b, 0644)
			if err := h.jobs.PrepareReportStep(ctx, id, snapshot, "hub"); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			b, _ = os.ReadFile(filepath.Join(snapshot, "step6_hub.json"))
			add(prefix+"hub_network.png", "hub", m.Trait, b)
		}
	}
	// Preserve the user's selected Hub chart when it exists, independently of
	// the key-module figures generated above.
	var hub struct {
		Trait string `json:"trait_name"`
	}
	hubData := load("step6_hub.json")
	_ = json.Unmarshal(hubData, &hub)
	add("07_hub/hub_network.png", "hub", hub.Trait, hubData)
	writeJSON(w, 200, map[string]any{"figures": tasks, "key_modules": keys, "criterion": "p < 0.05 (unadjusted module–trait test)"})
}

func copyReportSnapshots(jobDir, resultDir string, keys []keyModule) ([]figureSnapshot, error) {
	wanted := map[string]string{"02_filter/sample_tree.png": "", "03_soft_threshold/sft.png": "", "04_network/dendrogram.png": "", "05_module_trait/mtrait.png": "", "07_hub/hub_network.png": ""}
	for _, m := range keys {
		prefix := "06_module_detail/" + m.Module + "/"
		for _, name := range []string{"gs_mm.png", "expression_heatmap.png", "hub_network.png"} {
			wanted[prefix+name] = m.Trait
		}
		wanted[prefix+"enrichment/go.png"] = ""
		wanted[prefix+"enrichment/kegg.png"] = ""
	}
	records := []figureSnapshot{}
	for key, trait := range wanted {
		s, err := readFigureSnapshot(jobDir, key, trait)
		if err != nil {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s.PNG, "data:image/png;base64,"))
		if err != nil {
			return nil, err
		}
		dest := filepath.Join(resultDir, filepath.FromSlash(key))
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, err
		}
		if err = os.WriteFile(dest, b, 0644); err != nil {
			return nil, err
		}
		s.PNG = ""
		records = append(records, *s)
	}
	return records, nil
}
