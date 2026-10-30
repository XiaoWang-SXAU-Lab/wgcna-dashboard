package api

import (
	"io"
	"net/http"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/inputcheck"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

// validateInputs is read-only: no job or analysis is created for a preflight.
func (h *Handler) validateInputs(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	report := inputcheck.NewReport()
	tables := map[string]*inputcheck.Table{}
	dataType := r.FormValue("data_type")
	if dataType == "" {
		dataType = "count"
	}
	for _, kind := range []string{"expr", "trait", "sample_info", "annotation", "gene2go", "gene2kegg"} {
		f, _, err := r.FormFile(kind)
		if err == http.ErrMissingFile {
			continue
		}
		if err != nil {
			report.Add("error", kind, err.Error(), 0, 0)
			continue
		}
		raw, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			report.Add("error", kind, err.Error(), 0, 0)
			continue
		}
		table, checked := h.jobs.CheckInput(raw, kind, dataType)
		report.Merge(checked)
		if table != nil {
			tables[kind] = table
		}
	}
	if expression := tables["expr"]; expression != nil {
		for _, kind := range []string{"trait", "sample_info"} {
			if table := tables[kind]; table != nil {
				_, r := inputcheck.Align(table, expression.Samples(), nil, kind)
				report.Merge(r)
			}
		}
	}
	if tables["expr"] == nil && report.Valid && (tables["trait"] != nil || tables["sample_info"] != nil) {
		report.Add("warning", "samples", "Select the expression matrix to complete cross-file sample matching.", 0, 0)
	}
	writeJSON(w, 200, report)
}

func inputError(w http.ResponseWriter, err error) {
	if _, ok := err.(*job.GuardError); ok {
		runError(w, err)
		return
	}
	if report, ok := err.(inputcheck.Report); ok {
		http.Error(w, report.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
