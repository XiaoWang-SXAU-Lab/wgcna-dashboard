package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/tf", h.tf)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/targets", h.targets)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/regulators", h.regulators)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/motifs", h.motifs)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/motifs/{motif}", h.motif)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/enrich", h.enrich)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/provenance", h.provenance)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/site", h.site)
	mux.HandleFunc("GET /api/toolkits/regutrace/jobs/{id}/export", h.export)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/id-map/preview", h.previewIDMap)
	mux.HandleFunc("POST /api/toolkits/regutrace/jobs/{id}/id-map/apply", h.applyIDMap)
}

func readIDMapUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return nil, fmt.Errorf("multipart ID mapping upload required: %w", err)
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, errors.New("ID mapping file is required")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 64<<20 {
		return nil, errors.New("ID mapping file is larger than 64 MB")
	}
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("ID mapping gzip: %w", err)
		}
		defer gz.Close()
		data, err = io.ReadAll(io.LimitReader(gz, (256<<20)+1))
		if err != nil {
			return nil, err
		}
		if len(data) > 256<<20 {
			return nil, errors.New("decompressed ID mapping is larger than 256 MB")
		}
	}
	return data, nil
}

func (h *RegutraceHandler) previewIDMap(w http.ResponseWriter, r *http.Request) {
	data, err := readIDMapUpload(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	preview, err := h.mgr.PreviewIDMapping(r.PathValue("id"), data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *RegutraceHandler) applyIDMap(w http.ResponseWriter, r *http.Request) {
	data, err := readIDMapUpload(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	preview, err := h.mgr.ApplyIDMapping(r.PathValue("id"), data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, preview)
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
	if ups.TFGenes, err = open("tf_genes", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.Proteins, err = open("proteins", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.CDS, err = open("cds", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.HMM, err = open("hmm", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.FamilyMap, err = open("family_map", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.Trace, err = open("trace", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.CustomPack, err = open("custom_pack", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ups.Annotation, err = open("annotation", false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ups.MotifLibrary = strings.TrimSpace(r.FormValue("motif_library"))
	ups.ReferencePack = strings.TrimSpace(r.FormValue("reference_pack"))
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
		switch {
		case errors.Is(err, regutrace.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, regutrace.ErrNotCompleted):
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *RegutraceHandler) tf(w http.ResponseWriter, r *http.Request) {
	tf := strings.TrimSpace(r.URL.Query().Get("tf"))
	if tf == "" {
		http.Error(w, "tf is required", http.StatusBadRequest)
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	view, err := h.mgr.TFView(r.PathValue("id"), tf, target)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, regutrace.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, regutrace.ErrNotCompleted):
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func optionalUint(q url.Values, key string) (*uint64, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return &v, nil
}

func (h *RegutraceHandler) site(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := regutrace.SiteEvidenceOptions{
		TargetID: strings.TrimSpace(q.Get("target")),
		MotifID:  strings.TrimSpace(q.Get("motif")),
		Strand:   strings.TrimSpace(q.Get("strand")),
		TFID:     strings.TrimSpace(q.Get("tf")),
	}
	if opts.TargetID == "" || opts.MotifID == "" {
		http.Error(w, "target and motif are required", http.StatusBadRequest)
		return
	}
	var err error
	if opts.PromoterStart, err = optionalUint(q, "promoter_start"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if opts.PromoterEnd, err = optionalUint(q, "promoter_end"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if opts.HitStart, err = optionalUint(q, "hit_start"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if opts.HitEnd, err = optionalUint(q, "hit_end"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if opts.PromoterStart == nil || opts.PromoterEnd == nil {
		http.Error(w, "promoter_start and promoter_end are required", http.StatusBadRequest)
		return
	}
	if opts.HitStart == nil || opts.HitEnd == nil || opts.Strand == "" {
		http.Error(w, "hit_start, hit_end and strand are required", http.StatusBadRequest)
		return
	}
	res, err := h.mgr.SiteEvidence(r.PathValue("id"), opts)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, regutrace.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, regutrace.ErrNotCompleted):
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func queryInt(q url.Values, key string, fallback int, min, max int) (int, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || (max > 0 && value > max) {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return value, nil
}

func (h *RegutraceHandler) targets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tf := strings.TrimSpace(q.Get("tf"))
	if tf == "" {
		http.Error(w, "tf is required", http.StatusBadRequest)
		return
	}
	limit, err := queryInt(q, "limit", 50, 1, 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offset, err := queryInt(q, "offset", 0, 0, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	maxP := 0.0
	if raw := q.Get("max_p"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 || value > 1 {
			http.Error(w, "max_p must be in [0, 1]", http.StatusBadRequest)
			return
		}
		maxP = value
	}
	// min_sites requires that many sites of one and the same motif before a
	// target is admitted; 0/1 disables the filter.
	minSites := 0
	if raw := q.Get("min_sites"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 10 {
			http.Error(w, "min_sites must be an integer in [0, 10]", http.StatusBadRequest)
			return
		}
		minSites = value
	}
	res, err := h.mgr.TFTargets(r.PathValue("id"), regutrace.TFTargetOptions{
		TFID: tf, Limit: limit, Offset: offset, Motif: q.Get("motif"),
		Status: q.Get("mapping_status"), Policy: q.Get("motif_policy"), MaxP: maxP,
		MinSites: minSites,
	})
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) regulators(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := strings.TrimSpace(q.Get("target"))
	if target == "" {
		http.Error(w, "target is required", http.StatusBadRequest)
		return
	}
	limit, err := queryInt(q, "limit", 50, 1, 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offset, err := queryInt(q, "offset", 0, 0, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	maxP := 0.0
	if raw := q.Get("max_p"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 || value > 1 {
			http.Error(w, "max_p must be in [0, 1]", http.StatusBadRequest)
			return
		}
		maxP = value
	}
	res, err := h.mgr.TargetRegulators(r.PathValue("id"), target, limit, offset, q.Get("motif"), q.Get("mapping_status"), q.Get("motif_policy"), maxP)
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) motifs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := queryInt(q, "limit", 50, 1, 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offset, err := queryInt(q, "offset", 0, 0, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := h.mgr.MotifList(r.PathValue("id"), regutrace.MotifListOptions{
		Query: q.Get("q"), Family: q.Get("family"), Species: q.Get("species"),
		Status: q.Get("status"), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) motif(w http.ResponseWriter, r *http.Request) {
	res, err := h.mgr.MotifDetail(r.PathValue("id"), r.PathValue("motif"))
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) enrich(w http.ResponseWriter, r *http.Request) {
	var req regutrace.EnrichmentRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid enrichment request", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid enrichment request", http.StatusBadRequest)
		return
	}
	res, err := h.mgr.Enrich(r.PathValue("id"), req)
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *RegutraceHandler) provenance(w http.ResponseWriter, r *http.Request) {
	res, err := h.mgr.Provenance(r.PathValue("id"))
	if err != nil {
		writeRegutraceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeRegutraceError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, regutrace.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, regutrace.ErrNotCompleted):
		status = http.StatusConflict
	case errors.Is(err, regutrace.ErrUnsupportedFormat):
		status = http.StatusBadRequest
	}
	http.Error(w, err.Error(), status)
}

func (h *RegutraceHandler) export(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "evidence"
	}
	format := r.URL.Query().Get("format")
	var path, name string
	var err error
	switch kind {
	case "background":
		path, name, err = h.mgr.ExportBackground(r.Context(), r.PathValue("id"))
	case "prior":
		includeExploratory := strings.EqualFold(r.URL.Query().Get("include_exploratory"), "true") || r.URL.Query().Get("include_exploratory") == "1"
		path, name, err = h.mgr.ExportPrior(r.Context(), r.PathValue("id"), r.URL.Query().Get("policy"), r.URL.Query().Get("tfs"), includeExploratory)
	case "tf_catalogue", "tf_motif", "manifest", "sites":
		path, name, err = h.mgr.ExportMetadata(r.PathValue("id"), kind, r.URL.Query().Get("mode"))
	default:
		path, name, err = h.mgr.ExportFormat(r.Context(), r.PathValue("id"), kind, format)
	}
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, regutrace.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, regutrace.ErrUnsupportedFormat):
			status = http.StatusBadRequest
		case errors.Is(err, regutrace.ErrNotCompleted):
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, path)
}
