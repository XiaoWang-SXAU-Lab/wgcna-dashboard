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

// PlotParams are optional styling parameters forwarded to the R script.
// Zero values are omitted, so the R script falls back to its defaults.
type PlotParams struct {
	Size      float64 // point size
	Color     string  // point color (hex)
	LineColor string  // line color (hex)
	BaseSize  float64 // font size (pt)
	AxisSize  float64 // axis text size (pt)
	AxisColor string  // axis text/line/tick color (hex)
	Width     float64 // output width (inches)
	Height    float64 // output height (inches)
}

// rArgs renders non-zero params as R script arguments.
func (p PlotParams) rArgs() []string {
	var args []string
	if p.Size != 0 {
		args = append(args, "--size", fmt.Sprintf("%g", p.Size))
	}
	if p.Color != "" {
		args = append(args, "--color", p.Color)
	}
	if p.LineColor != "" {
		args = append(args, "--line-color", p.LineColor)
	}
	if p.BaseSize != 0 {
		args = append(args, "--base-size", fmt.Sprintf("%g", p.BaseSize))
	}
	if p.AxisSize != 0 {
		args = append(args, "--axis-size", fmt.Sprintf("%g", p.AxisSize))
	}
	if p.AxisColor != "" {
		args = append(args, "--axis-color", p.AxisColor)
	}
	if p.Width != 0 {
		args = append(args, "--width", fmt.Sprintf("%g", p.Width))
	}
	if p.Height != 0 {
		args = append(args, "--height", fmt.Sprintf("%g", p.Height))
	}
	return args
}

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
func RenderSft(jsonPath, mode, rsqCut string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode, "--rsq-cut", rsqCut}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(sftScript, args, "sft.pdf", "application/pdf")
	case "svg":
		return renderR(sftScript, args, "sft.svg", "image/svg+xml")
	default:
		return renderR(sftScript, args, "sft.png", "image/png")
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
