// Command server is the WGCNA dashboard: HTTP API + Rust worker orchestration
// + embedded frontend, wrapped in a native WebView window (single binary).
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	webview "github.com/webview/webview_go"

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
	aiHandler := api.NewAI(aiStore, jobsDir)

	mux := http.NewServeMux()
	handler.Register(mux)
	aiHandler.Register(mux)
	mux.Handle("/", web.Handler())

	// pick a free localhost port (desktop app: avoid clashing with other apps)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("bind: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	go func() {
		log.Printf("rsWGCNA listening on %s (worker=%s jobs=%s)", baseURL, workerBin, jobsDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	waitReady(baseURL)

	// headless mode (servers / CI): keep serving without a window
	if os.Getenv("WGCNA_NO_WEBVIEW") != "" {
		select {}
	}

	openWindow(baseURL)

	_ = srv.Shutdown(context.Background())
}

func waitReady(baseURL string) {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 100; i++ {
		resp, err := client.Get(baseURL)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	log.Printf("warning: server not ready at %s", baseURL)
}

// openWindow opens a native WebView window (WKWebView on macOS, WebView2 on
// Windows) pointing at the embedded server.
func openWindow(baseURL string) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("rsWGCNA")
	w.SetSize(1280, 800, webview.HintNone)
	w.Navigate(baseURL)
	w.Run()
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
