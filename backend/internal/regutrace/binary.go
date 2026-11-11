package regutrace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Extract writes the embedded ReguTrace binary to a temporary directory and
// returns its path. It returns an error in dev builds, where the dashboard
// falls back to WGCNA_REGUTRACE or PATH.
func Extract() (string, error) {
	if len(binary) == 0 {
		return "", errors.New("regutrace not embedded: build with -tags release, or set WGCNA_REGUTRACE")
	}
	dir, err := os.MkdirTemp("", "wgcna-regutrace-*")
	if err != nil {
		return "", err
	}
	name := "regutrace"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		return "", err
	}
	return path, nil
}
