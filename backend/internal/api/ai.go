package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/ai"
)

// AI exposes the AI-navigator endpoints: config management + agentic chat.
type AI struct {
	store   *ai.Store
	jobsDir string
}

func NewAI(store *ai.Store, jobsDir string) *AI {
	return &AI{store: store, jobsDir: jobsDir}
}

func (a *AI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ai/config", a.getConfig)
	mux.HandleFunc("POST /api/ai/config", a.saveConfig)
	mux.HandleFunc("POST /api/ai/chat", a.chat)
}

func (a *AI) getConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, _ := a.store.Load()
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":   cfg.Provider,
		"base_url":   cfg.BaseURL,
		"model":      cfg.Model,
		"configured": cfg.BaseURL != "" && cfg.APIKeyEnc != "",
	})
}

func (a *AI) saveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if p, ok := ai.Providers[req.Provider]; ok && req.BaseURL == "" {
		req.BaseURL = p.BaseURL
		req.Model = p.Model
	}
	cfg := ai.Config{Provider: req.Provider, BaseURL: req.BaseURL, Model: req.Model}
	if err := a.store.Save(cfg, req.APIKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type chatRequest struct {
	Question string `json:"question"`
	Error    string `json:"error"`
	JobID    string `json:"job_id"`
	Module   string `json:"module"`
}

func (a *AI) chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	nav, err := ai.NewNavigator(a.store)
	if err != nil {
		writeSSE(w, flusher, map[string]any{"error": err.Error()})
		return
	}
	writeSSE(w, flusher, map[string]any{"start": true})

	// attach job-context tools when a job is specified
	if req.JobID != "" {
		jobDir := filepath.Join(a.jobsDir, req.JobID)
		nav.SetTools(toolDefs(), newToolRunner(jobDir))
	}

	var answer string
	if req.JobID != "" {
		answer, err = nav.AnswerAgent(req.Question, req.Error)
	} else {
		// no job context → streaming path (no tools)
		err = nav.AnswerStream(req.Question, req.Error, func(delta string) {
			writeSSE(w, flusher, map[string]any{"delta": delta})
		})
	}
	if err != nil {
		writeSSE(w, flusher, map[string]any{"error": err.Error()})
		return
	}
	if req.JobID != "" {
		writeSSE(w, flusher, map[string]any{"delta": answer})
	}
	writeSSE(w, flusher, map[string]any{"done": true})
}

func writeSSE(w http.ResponseWriter, f http.Flusher, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", b)
	f.Flush()
}
