// Package plot renders WGCNA plots with R (ggplot2 for static/vector output,
// plotly for interactive). R scripts are embedded and extracted at runtime.
package plot

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// findRscript resolves the Rscript executable: an explicit WGCNA_RSCRIPT env
// var wins, then a bundled R next to the executable, then PATH.
func findRscript() string {
	if rs := os.Getenv("WGCNA_RSCRIPT"); rs != "" {
		return rs
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, "R", "R-4.5.1", "bin", "Rscript.exe"),
			filepath.Join(dir, "R", "bin", "Rscript.exe"),
			filepath.Join(dir, "RPortable", "App", "R-Portable", "bin", "Rscript.exe"),
			filepath.Join(dir, "RPortable", "App", "R-Portable", "bin", "x64", "Rscript.exe"),
			// Bundled R.framework inside a macOS .app bundle.
			filepath.Join(dir, "..", "Frameworks", "R.framework", "Versions", "Current", "Resources", "bin", "Rscript"),
			filepath.Join(dir, "..", "Frameworks", "R.framework", "Resources", "bin", "Rscript"),
			filepath.Join(dir, "..", "Frameworks", "R.framework", "Versions", "4.5", "Resources", "bin", "Rscript"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return "Rscript"
}

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

//go:embed render_enrich.R
var enrichScript []byte

//go:embed render_sample_tree.R
var sampleTreeScript []byte

//go:embed render_enrich_heatmap.R
var enrichHeatmapScript []byte

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
	Legend    string  // heatmap legend position: right | bottom | none
	Width     float64 // output width (inches)
	Height    float64 // output height (inches)
	Dpi       float64 // raster resolution (PNG only)
	X            string  // enrichment x-axis: GeneRatio | Count
	ColorVar     string  // enrichment color variable: p.adjust | pvalue | qvalue
	ShowCategory int     // enrichment top-N terms
	SizeMin      float64 // enrichment dot size range
	SizeMax      float64
	Title        string  // enrichment plot title
	Wrap         int     // enrichment label wrap width (chars; 0 = off)
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
	if p.Legend != "" {
		args = append(args, "--legend", p.Legend)
	}
	if p.X != "" {
		args = append(args, "--x", p.X)
	}
	if p.ColorVar != "" {
		args = append(args, "--color-var", p.ColorVar)
	}
	if p.ShowCategory > 0 {
		args = append(args, "--show-category", strconv.Itoa(p.ShowCategory))
	}
	if p.SizeMin > 0 {
		args = append(args, "--size-min", fmt.Sprintf("%g", p.SizeMin))
	}
	if p.SizeMax > 0 {
		args = append(args, "--size-max", fmt.Sprintf("%g", p.SizeMax))
	}
	if p.Title != "" {
		args = append(args, "--title", p.Title)
	}
	if p.Wrap > 0 {
		args = append(args, "--wrap", strconv.Itoa(p.Wrap))
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

// RenderEnrich renders a GO/KEGG enrichment dot plot (ggplot2). mode is
// "pdf", "svg" or "png".
func RenderEnrich(results []enrich.Result, mode string, params PlotParams) (string, string, error) {
	dir, err := os.MkdirTemp("", "wgcna-plot-*")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := os.WriteFile(filepath.Join(dir, "render.R"), enrichScript, 0o644); err != nil {
		cleanup()
		return "", "", err
	}
	data, err := json.Marshal(results)
	if err != nil {
		cleanup()
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "enrich.json"), data, 0o644); err != nil {
		cleanup()
		return "", "", err
	}

	rs := findRscript()
	out := filepath.Join(dir, "enrich."+mode)
	args := []string{filepath.Join(dir, "render.R"), "--json", filepath.Join(dir, "enrich.json"), "--mode", mode, "--out", out}
	args = append(args, params.rArgs()...)
	cmd := exec.Command(rs, args...)
	subproc.HideWindow(cmd)
	cmd.Env = plotEnv(rs)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", fmt.Errorf("R plot failed: %v: %s", err, string(b))
	}

	ctype := "image/png"
	if mode == "pdf" {
		ctype = "application/pdf"
	} else if mode == "svg" {
		ctype = "image/svg+xml"
	}
	return out, ctype, nil
}

// RenderSampleTree renders the sample dendrogram with optional color bands.
// data is the pre-assembled JSON: {samples, tree:{merge,height,order}, bands:[{name,values}]}.
func RenderSampleTree(data []byte, mode string, params PlotParams) (string, string, error) {
	dir, err := os.MkdirTemp("", "wgcna-plot-*")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := os.WriteFile(filepath.Join(dir, "render.R"), sampleTreeScript, 0o644); err != nil {
		cleanup()
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), data, 0o644); err != nil {
		cleanup()
		return "", "", err
	}

	rs := findRscript()
	out := filepath.Join(dir, "sample_tree."+mode)
	args := []string{filepath.Join(dir, "render.R"), "--json", filepath.Join(dir, "data.json"), "--mode", mode, "--out", out}
	args = append(args, params.rArgs()...)
	cmd := exec.Command(rs, args...)
	subproc.HideWindow(cmd)
	cmd.Env = plotEnv(rs)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", fmt.Errorf("R plot failed: %v: %s", err, string(b))
	}

	ctype := "image/png"
	if mode == "pdf" {
		ctype = "application/pdf"
	} else if mode == "svg" {
		ctype = "image/svg+xml"
	}
	return out, ctype, nil
}

// RenderEnrichHeatmap renders the enrichment expression heatmap (ComplexHeatmap).
func RenderEnrichHeatmap(data []byte, mode string, params PlotParams) (string, string, error) {
	dir, err := os.MkdirTemp("", "wgcna-plot-*")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := os.WriteFile(filepath.Join(dir, "render.R"), enrichHeatmapScript, 0o644); err != nil {
		cleanup()
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), data, 0o644); err != nil {
		cleanup()
		return "", "", err
	}

	rs := findRscript()
	out := filepath.Join(dir, "enrich_heatmap."+mode)
	args := []string{filepath.Join(dir, "render.R"), "--json", filepath.Join(dir, "data.json"), "--mode", mode, "--out", out}
	args = append(args, params.rArgs()...)
	cmd := exec.Command(rs, args...)
	subproc.HideWindow(cmd)
	cmd.Env = plotEnv(rs)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", fmt.Errorf("R plot failed: %v: %s", err, string(b))
	}

	ctype := "image/png"
	if mode == "pdf" {
		ctype = "application/pdf"
	} else if mode == "svg" {
		ctype = "image/svg+xml"
	}
	return out, ctype, nil
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
	rs := findRscript()

	full := append([]string{scriptPath}, args...)
	full = append(full, "--out", out)
	cmd := exec.Command(rs, full...)
	subproc.HideWindow(cmd)
	cmd.Env = plotEnv(rs)
	if b, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", "", fmt.Errorf("R plot failed: %v: %s", err, string(b))
	}
	return out, ctype, nil
}

// plotEnv returns the subprocess environment with the Rscript directory
// prepended to PATH (so pandoc and friends next to Rscript are found). For a
// bundled macOS R.framework it also pins R_HOME and the dynamic-library search
// paths so R keeps working after the framework is relocated inside the .app.
func plotEnv(rsPath string) []string {
	base := os.Environ()
	dir := filepath.Dir(rsPath)
	if dir == "" || dir == "." {
		return base
	}

	rHome, libDir := frameworkDirs(rsPath)
	out := make([]string, 0, len(base)+3)
	prefix := "PATH=" + dir + string(os.PathListSeparator)
	for _, kv := range base {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		switch key {
		case "PATH":
			out = append(out, prefix+kv[5:])
		case "R_HOME":
			if rHome == "" {
				out = append(out, kv)
			}
			// else: dropped and replaced below with the bundled framework's R_HOME.
		case "DYLD_FALLBACK_LIBRARY_PATH":
			if libDir != "" {
				out = append(out, kv+string(os.PathListSeparator)+libDir)
			} else {
				out = append(out, kv)
			}
		default:
			out = append(out, kv)
		}
	}
	if rHome != "" {
		out = append(out, "R_HOME="+rHome)
	}
	if libDir != "" {
		out = append(out, "DYLD_LIBRARY_PATH="+libDir)
	}
	return out
}

// frameworkDirs extracts R_HOME (the framework's Resources dir) and the lib dir
// from an Rscript path that lives inside an R.framework bundle.
func frameworkDirs(rsPath string) (string, string) {
	idx := strings.Index(rsPath, "R.framework")
	if idx < 0 {
		return "", ""
	}
	seg := rsPath[idx:]
	ri := strings.LastIndex(seg, "Resources")
	if ri < 0 {
		return "", ""
	}
	rHome := rsPath[:idx] + seg[:ri+len("Resources")]
	return rHome, filepath.Join(rHome, "lib")
}
