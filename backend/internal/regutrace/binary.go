package regutrace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "regutrace.exe"
	}
	return "regutrace"
}

// sidecarPath returns the expected path of the ReguTrace binary next to exe.
func sidecarPath(exe string) string {
	return filepath.Join(filepath.Dir(exe), binaryName())
}

// isExecutable reports whether path names a usable sidecar. Unix uses execute
// bits; Windows checks the PE "MZ" header because it has no execute bits.
func isExecutable(path string, info os.FileInfo) bool {
	if info == nil || info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" {
		return info.Mode()&0o111 != 0
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [2]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic[0] == 'M' && magic[1] == 'Z'
}

// Resolve returns the ReguTrace path in this order:
//  1. WGCNA_REGUTRACE (development/override)
//  2. regutrace next to the running application
//  3. PATH
//
// It never writes an embedded binary to a temporary directory; release builds
// ship regutrace as a signed sidecar next to the main executable.
func Resolve() (string, error) {
	if p := os.Getenv("WGCNA_REGUTRACE"); p != "" {
		if info, err := os.Stat(p); err != nil || !isExecutable(p, info) {
			return "", fmt.Errorf("WGCNA_REGUTRACE=%q is not an executable file", p)
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		p := sidecarPath(exe)
		if info, statErr := os.Stat(p); statErr == nil && isExecutable(p, info) {
			return p, nil
		}
	}
	if p, err := exec.LookPath(binaryName()); err == nil {
		return p, nil
	}
	return "", fmt.Errorf(
		"%s not found or not executable; place it next to the application or set WGCNA_REGUTRACE",
		binaryName(),
	)
}
