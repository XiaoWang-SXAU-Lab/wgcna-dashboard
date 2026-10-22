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

//go:embed render_sft_check.R
var sftCheckScript []byte

//go:embed render_mtrait.R
var mtraitScript []byte

//go:embed render_eigengene.R
var eigengeneScript []byte

//go:embed render_gsmm.R
var gsmmScript []byte

//go:embed render_geneheatmap.R
var geneHeatmapScript []byte

// PlotParams are optional styling parameters forwarded to the R script.
// Zero values are omitted, so the R script falls back to its defaults.
type PlotParams struct {
	Size      float64 // point size
	Color     string  // point color (hex)
	LineColor string  // line color (hex)
	BaseSize  float64 // font size (pt)
	AxisSize  float64 // axis text size (pt)
	AxisColor string  // axis text/line/tick color (hex)
	Low       string  // heatmap low color (hex)
	Mid       string  // heatmap mid color (hex)
	High      string  // heatmap high color (hex)
	Width     float64 // output width (inches)
	Height    float64 // output height (inches)
	Dpi       float64 // raster resolution (PNG only)
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
	if p.Dpi != 0 {
		args = append(args, "--dpi", fmt.Sprintf("%g", p.Dpi))
	}
	if p.Low != "" {
		args = append(args, "--low", p.Low)
	}
	if p.Mid != "" {
		args = append(args, "--mid", p.Mid)
	}
	if p.High != "" {
		args = append(args, "--high", p.High)
	}
	return args
}

// RenderDendrogram renders the module dendrogram (from step3_modules.json).
// mode is "pdf", "svg", "png"/"static" or "interactive" (self-contained HTML).
func RenderDendrogram(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(dendrogramScript, args, "dendro.pdf", "application/pdf")
	case "svg":
		return renderR(dendrogramScript, args, "dendro.svg", "image/svg+xml")
	case "interactive":
		return renderR(dendrogramScript, args, "dendro.html", "text/html; charset=utf-8")
	default:
		return renderR(dendrogramScript, args, "dendro.png", "image/png")
	}
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

// RenderSftCheck renders the scale-free topology check (connectivity histogram
// + log-log regression) from step2_sft.json. mode is "pdf", "svg" or "png".
func RenderSftCheck(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(sftCheckScript, args, "sft_check.pdf", "application/pdf")
	case "svg":
		return renderR(sftCheckScript, args, "sft_check.svg", "image/svg+xml")
	default:
		return renderR(sftCheckScript, args, "sft_check.png", "image/png")
	}
}

// RenderMtrait renders the module-trait correlation heatmap from step4_mtrait.json.
func RenderMtrait(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(mtraitScript, args, "mtrait.pdf", "application/pdf")
	case "svg":
		return renderR(mtraitScript, args, "mtrait.svg", "image/svg+xml")
	default:
		return renderR(mtraitScript, args, "mtrait.png", "image/png")
	}
}

// RenderGsmm renders the GS vs MM scatter (module-colored points + lm line).
func RenderGsmm(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(gsmmScript, args, "gsmm.pdf", "application/pdf")
	case "svg":
		return renderR(gsmmScript, args, "gsmm.svg", "image/svg+xml")
	default:
		return renderR(gsmmScript, args, "gsmm.png", "image/png")
	}
}

// RenderGeneHeatmap renders the module gene heatmap + eigengene barplot.
func RenderGeneHeatmap(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(geneHeatmapScript, args, "geneheatmap.pdf", "application/pdf")
	case "svg":
		return renderR(geneHeatmapScript, args, "geneheatmap.svg", "image/svg+xml")
	default:
		return renderR(geneHeatmapScript, args, "geneheatmap.png", "image/png")
	}
}

// RenderEigengene renders the module eigengene adjacency heatmap.
func RenderEigengene(jsonPath, mode string, params PlotParams) (string, string, error) {
	args := []string{"--json", jsonPath, "--mode", mode}
	args = append(args, params.rArgs()...)
	switch mode {
	case "pdf":
		return renderR(eigengeneScript, args, "eigengene.pdf", "application/pdf")
	case "svg":
		return renderR(eigengeneScript, args, "eigengene.svg", "image/svg+xml")
	default:
		return renderR(eigengeneScript, args, "eigengene.png", "image/png")
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
