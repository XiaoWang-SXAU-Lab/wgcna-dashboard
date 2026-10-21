// Package api exposes the HTTP endpoints for job submission, step execution,
// and status.
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
	mux.HandleFunc("POST /api/jobs/{id}/trait", h.uploadTrait)
	mux.HandleFunc("POST /api/jobs/{id}/steps/{step}/run", h.runStep)
	mux.HandleFunc("POST /api/jobs/{id}/run-all", h.runAll)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", h.cancel)
	return mux
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
	id, err := h.jobs.Create(expr, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	j, ok := h.jobs.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
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

// runStep runs a single step asynchronously.
func (h *Handler) runStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	step := r.PathValue("step")
	if !validStep(step) {
		http.Error(w, "unknown step: "+step, http.StatusBadRequest)
		return
	}
	go h.jobs.RunStep(id, step)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started", "step": step})
}

// runAll runs the whole pipeline (one-step mode) asynchronously.
func (h *Handler) runAll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	go h.jobs.RunAll(id)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.jobs.Cancel(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling"})
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
