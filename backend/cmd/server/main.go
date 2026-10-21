// Command server is the WGCNA dashboard backend: HTTP API + Rust worker
// orchestration. Phase 1: job submission (multipart expr + params) and status
// polling over a three-step pipeline (filter -> soft_threshold -> network).
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/api"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

func main() {
	workerBin := os.Getenv("WGCNA_WORKER")
	if workerBin == "" {
		workerBin = "rs-wgcna-worker"
	}
	jobsDir := os.Getenv("WGCNA_JOBS_DIR")
	if jobsDir == "" {
		jobsDir = filepath.Join(os.TempDir(), "wgcna-jobs")
	}

	manager := job.NewManager(jobsDir, workerBin, runtime.NumCPU())
	handler := api.NewHandler(manager)

	addr := ":8080"
	log.Printf("wgcna-dashboard listening on %s (worker=%s jobs=%s)", addr, workerBin, jobsDir)
	if err := http.ListenAndServe(addr, handler.Routes()); err != nil {
		log.Fatal(err)
	}
}
