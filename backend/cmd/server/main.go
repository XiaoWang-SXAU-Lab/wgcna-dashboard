// Command server is the WGCNA dashboard backend: HTTP API + Rust worker
// orchestration + embedded frontend (single-binary in release builds).
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/ai"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/api"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/web"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/worker"
)

func main() {
	workerBin := resolveWorker()
	jobsDir := os.Getenv("WGCNA_JOBS_DIR")
	if jobsDir == "" {
		jobsDir = filepath.Join(os.TempDir(), "wgcna-jobs")
	}

	manager := job.NewManager(jobsDir, workerBin, runtime.NumCPU())
	handler := api.NewHandler(manager)

	aiStore, err := ai.NewStore()
	if err != nil {
		log.Fatalf("ai store: %v", err)
	}
	aiHandler := api.NewAI(aiStore)

	mux := http.NewServeMux()
	handler.Register(mux)
	aiHandler.Register(mux)
	mux.Handle("/", web.Handler())

	addr := ":8080"
	log.Printf("wgcna-dashboard listening on %s (worker=%s jobs=%s)", addr, workerBin, jobsDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// resolveWorker prefers the WGCNA_WORKER env var (dev), then falls back to the
// embedded worker binary (release builds).
func resolveWorker() string {
	if p := os.Getenv("WGCNA_WORKER"); p != "" {
		return p
	}
	p, err := worker.Extract()
	if err != nil {
		log.Fatalf("no worker available: %v (set WGCNA_WORKER, or build with -tags release)", err)
	}
	return p
}
