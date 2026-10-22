// Package plot renders WGCNA plots with R (ggplot2 for static/vector output,
// plotly for interactive). R scripts are embedded and extracted at runtime.
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

//go:embed render_sft.R
var sftScript []byte

// RenderDendrogram renders the module dendrogram (from step3_modules.json).
// mode is "static" (PNG) or "interactive" (self-contained HTML).
func RenderDendrogram(jsonPath, mode string) (string, string, error) {
	out := "dendro.png"
	ctype := "image/png"
	if mode == "interactive" {
		out = "dendro.html"
		ctype = "text/html; charset=utf-8"
	}
	return renderR(dendrogramScript, []string{"--json", jsonPath, "--mode", mode}, out, ctype)
}

// RenderSft renders the soft-threshold plot (scale-free fit R² and mean
// connectivity vs power) from step2_sft.json. mode is "pdf", "svg" or "png".
func RenderSft(jsonPath, mode, rsqCut string) (string, string, error) {
	switch mode {
	case "pdf":
		return renderR(sftScript, []string{"--json", jsonPath, "--mode", "pdf", "--rsq-cut", rsqCut}, "sft.pdf", "application/pdf")
	case "svg":
		return renderR(sftScript, []string{"--json", jsonPath, "--mode", "svg", "--rsq-cut", rsqCut}, "sft.svg", "image/svg+xml")
	default:
		return renderR(sftScript, []string{"--json", jsonPath, "--mode", "png", "--rsq-cut", rsqCut}, "sft.png", "image/png")
	}
}

// renderR writes an embedded R script to a temp dir, runs it with Rscript, and
// returns the generated output file path + HTTP content type.
func renderR(script []byte, args []string, outName, ctype string) (string, string, error) {
	dir, err := os.MkdirTemp("", "wgcna-plot-*")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	scriptPath := filepath.Join(dir, "render.R")
	if err := os.WriteFile(scriptPath, script, 0o644); err != nil {
		cleanup()
		return "", "", err
	}

	out := filepath.Join(dir, outName)
	rs := os.Getenv("WGCNA_RSCRIPT")
	if rs == "" {
		rs = "Rscript"
	}

	full := append([]string{scriptPath}, args...)
	full = append(full, "--out", out)
	cmd := exec.Command(rs, full...)
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
