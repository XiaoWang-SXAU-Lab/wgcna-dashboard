// Package main is the WGCNA dashboard backend entry point.
//
// Phase 0 skeleton: a health endpoint only. The full job API / worker
// orchestration / SSE progress streaming land in Phase 1.
package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "wgcna-dashboard",
		})
	})

	addr := ":8080"
	log.Printf("wgcna-dashboard listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
