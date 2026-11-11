package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/regutrace"
)

// RegutraceHandler exposes the local TF-target toolkit API.
type RegutraceHandler struct {
	mgr *regutrace.Manager
}

func NewRegutraceHandler(mgr *regutrace.Manager) *RegutraceHandler {
	return &RegutraceHandler{mgr: mgr}
}

func (h *RegutraceHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/toolkits/regutrace/info", h.info)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs", h.createJob)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}", h.getJob)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/validate", h.validate)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/run", h.run)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/query", h.query)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/promoter", h.promoter)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/export", h.export)
}

func (h *RegutraceHandler) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.Info())
}

func formUpload(r *http.Request, field string) (*regutrace.Upload, func(), error) {
	f, hdr, err := r.FormFile(field)
	if err != nil {
		return nil, nil, err
	}
	up := &regutrace.Upload{Name: hdr.Filename, Size: hdr.Size, Reader: f}
	return up, func() { _ = f.Close() }, nil
}

func (h *RegutraceHandler) createJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<30)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "multipart upload required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	var ups regutrace.Uploads
	var closers []func()
	defer func() {
		for _, c := range closers {
			c()
		}
	}()
	open := func(field string, required bool) (*regutrace.Upload, error) {
		up, closer, err := formUpload(r, field)
		if err != nil {
			if required {
				return nil, fmt.Errorf("%s is required", field)
			}
			return nil, nil
		}
		closers = append(closers, closer)
		return up, nil
	}
	var err error
	if ups.Genome, err = open("genome", true); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.GFF, err = open("gff", true); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.Motifs, err = open("motifs", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.TFMap, err = open("tf_map", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.Annotation, err = open("annotation", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ups.MotifLibrary = strings.TrimSpace(r.FormValue("motif_library"))
	job, err := h.mgr.Create(ups)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func decodeParams(r *http.Request) (regutrace.Params, error) {
	var p regutrace.Params
	if r.Body == nil {
		return p, nil
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return p, err
	}
	return p, nil
}

func (h *RegutraceHandler) start(w http.ResponseWriter, r *http.Request, mode string) {
	p, err := decodeParams(r)
	if err != nil {
		http.Error(w, "invalid params JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	job, err := h.mgr.Start(r.PathValue("id"), p, mode)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "already running") {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *RegutraceHandler) validate(w http.ResponseWriter, r *http.Request) {
	h.start(w, r, "validate")
}

func (h *RegutraceHandler) run(w http.ResponseWriter, r *http.Request) {
	h.start(w, r, "run")
}

func (h *RegutraceHandler) getJob(w http.ResponseWriter, r *http.Request) {
	job, ok := h.mgr.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *RegutraceHandler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Cancel(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cancelling"})
}

func (h *RegutraceHandler) query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := regutrace.QueryOptions{
		Direction: q.Get("direction"),
		Query:     q.Get("q"),
		Motif:     q.Get("motif"),
	}
	if raw := q.Get("max_p"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0 || v > 1 {
			http.Error(w, "max_p must be in [0, 1]", http.StatusBadRequest)
			return
		}
		opts.MaxPValue = v
	}
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		opts.Limit = v
	}
	if raw := q.Get("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			http.Error(w, "offset must be a non-negative integer", http.StatusBadRequest)
			return
		}
		opts.Offset = v
	}
	res, err := h.mgr.Query(r.PathValue("id"), opts)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "not completed") {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) promoter(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		http.Error(w, "target is required", http.StatusBadRequest)
		return
	}
	view, err := h.mgr.PromoterView(r.PathValue("id"), target)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "has not completed") {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *RegutraceHandler) export(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "evidence"
	}
	path, name, err := h.mgr.ExportPath(r.PathValue("id"), kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, path)
}
