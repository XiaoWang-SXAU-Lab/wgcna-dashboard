// Package worker manages the embedded rs-wgcna-worker binary (release builds)
// and falls back to the WGCNA_WORKER env var in dev.
package worker

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Extract writes the embedded worker binary to a temp dir and returns its path.
// It returns an error in dev builds (built without -tags release), where the
// caller should rely on WGCNA_WORKER instead.
func Extract() (string, error) {
	if len(binary) == 0 {
		return "", errors.New("worker not embedded: build with -tags release, or set WGCNA_WORKER")
	}
	dir, err := os.MkdirTemp("", "wgcna-worker-*")
	if err != nil {
		return "", err
	}
	name := "rs-wgcna-worker"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		return "", err
	}
	return path, nil
}
