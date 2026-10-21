// Package api exposes the HTTP endpoints for job submission and status.
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

type Handler struct {
	jobs *job.Manager
}

func NewHandler(jobs *job.Manager) *Handler {
	return &Handler{jobs: jobs}
}

// Routes returns the mux with all API routes (Go 1.22 method+path patterns).
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/jobs", h.createJob)
	mux.HandleFunc("GET /api/jobs/{id}", h.getJob)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "wgcna-dashboard"})
}

// createJob accepts a multipart form with an "expr" file and an optional
// "params" JSON string, then starts the pipeline.
func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("expr")
	if err != nil {
		http.Error(w, "missing 'expr' file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	expr, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read expr: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(expr) == 0 {
		http.Error(w, "empty 'expr' file", http.StatusBadRequest)
		return
	}

	params := json.RawMessage(r.FormValue("params"))
	id, err := h.jobs.Create(expr, params)
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
	writeJSON(w, http.StatusOK, j)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
