package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
)

// materializeEnrichDir writes the files the enrich package needs (step3_modules.json,
// annotation.tsv, gene2go.tsv, gene2kegg.tsv) into a temp dir so the existing
// enrich.GOEnrich / KEGGEnrich / Annotate functions can run unchanged.
func materializeEnrichDir(s *projectSession) (string, error) {
	dir, err := os.MkdirTemp("", "project-enrich-*")
	if err != nil {
		return "", err
	}
	write := func(name string, data []byte) {
		if data == nil {
			return
		}
		_ = os.WriteFile(filepath.Join(dir, name), data, 0o600)
	}
	if data, err := s.p.Result("network"); err == nil {
		write("step3_modules.json", data)
	}
	for role, name := range map[string]string{"annotation": "annotation.tsv", "gene2go": "gene2go.tsv", "gene2kegg": "gene2kegg.tsv"} {
		if data, err := s.p.Input(role); err == nil {
			write(name, data)
		}
	}
	return dir, nil
}

func (h *ProjectHandler) projectAnnotation(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	module := r.URL.Query().Get("module")
	if module == "" {
		http.Error(w, "module required", http.StatusBadRequest)
		return
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
	rows, err := enrich.Annotate(dir, genes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *ProjectHandler) projectEnrich(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
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
	} else {
		results, err = enrich.KEGGEnrich(dir, genes, fdr, top)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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

// projectExpression returns expression values for the requested genes across all
// samples, reading the filtered matrix and annotation from the project.
func (h *ProjectHandler) projectExpression(w http.ResponseWriter, r *http.Request) {
	s := h.session(w, r)
	if s == nil {
		return
	}
	genesParam := r.URL.Query().Get("genes")
	if genesParam == "" {
		http.Error(w, "genes required", http.StatusBadRequest)
		return
	}
	genes := strings.Split(genesParam, ",")
	dir, err := os.MkdirTemp("", "project-expr-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)
	if data, err := s.p.Output("filter", "step1_filtered.tsv"); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "step1_filtered.tsv"), data, 0o600)
	} else if data, err := s.p.Input("expr"); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "expr.tsv"), data, 0o600)
	}
	if data, err := s.p.Input("annotation"); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "annotation.tsv"), data, 0o600)
	}
	res, err := enrich.Expression(dir, genes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
