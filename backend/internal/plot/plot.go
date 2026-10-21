// Package plot renders WGCNA plots with R (ggplot2 for static, plotly for
// interactive). The R script is embedded and extracted at runtime.
package plot

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed render_dendrogram.R
var dendrogramScript []byte

// RenderDendrogram renders the module dendrogram (from step3_modules.json).
// mode is "static" (PNG) or "interactive" (self-contained HTML).
// It returns the output file path and its HTTP content type.
func RenderDendrogram(jsonPath, mode string) (string, string, error) {
	dir, err := os.MkdirTemp("", "wgcna-plot-*")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	script := filepath.Join(dir, "render_dendrogram.R")
	if err := os.WriteFile(script, dendrogramScript, 0o644); err != nil {
		cleanup()
		return "", "", err
	}

	out := filepath.Join(dir, "dendro.png")
	ctype := "image/png"
	if mode == "interactive" {
		out = filepath.Join(dir, "dendro.html")
		ctype = "text/html; charset=utf-8"
	}

	rs := os.Getenv("WGCNA_RSCRIPT")
	if rs == "" {
		rs = "Rscript"
	}

	cmd := exec.Command(rs, script, "--json", jsonPath, "--mode", mode, "--out", out)
	cmd.Env = plotEnv(rs)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", fmt.Errorf("R plot failed: %v: %s", err, string(b))
	}
	return out, ctype, nil
}

// plotEnv returns the subprocess environment with the Rscript directory
// prepended to PATH (so pandoc and friends next to Rscript are found).
func plotEnv(rsPath string) []string {
	base := os.Environ()
	dir := filepath.Dir(rsPath)
	if dir == "" || dir == "." {
		return base
	}
	out := make([]string, 0, len(base)+1)
	prefix := "PATH=" + dir + string(os.PathListSeparator)
	for _, kv := range base {
		if len(kv) >= 5 && kv[:5] == "PATH=" {
			out = append(out, prefix+kv[5:])
		} else {
			out = append(out, kv)
		}
	}
	return out
}
