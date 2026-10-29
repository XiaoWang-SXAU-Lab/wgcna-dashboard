package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/plot"
)

type reportPlot struct {
	path string
	err  error
}

func rendered(path, _ string, err error) reportPlot { return reportPlot{path, err} }

func checkedCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func reportModules(jobDir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json"))
	if err != nil {
		return nil, err
	}
	var d struct {
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var modules []string
	for _, m := range d.Colors {
		if m == "grey" || seen[m] {
			continue
		}
		if !reportNameOK(m) {
			return nil, fmt.Errorf("invalid module name: %q", m)
		}
		seen[m] = true
		modules = append(modules, m)
	}
	sort.Strings(modules)
	return modules, nil
}

func (h *Handler) doReportExport(id, base, enrichType, lang string, st *reportStatus) {
	jobDir := filepath.Join(h.jobs.Dir(), id)
	label := reportData{Lang: lang}
	update := func(message string) { reportMu.Lock(); st.Message = message; reportMu.Unlock() }
	fail := func(err error) { reportMu.Lock(); st.State = "error"; st.Message = err.Error(); reportMu.Unlock() }
	warn := func(name string, err error) {
		if err == nil {
			return
		}
		message := name + ": " + err.Error()
		if len(message) > 1400 {
			message = message[:1400] + "…"
		}
		reportMu.Lock()
		st.Warnings = append(st.Warnings, message)
		reportMu.Unlock()
	}
	defer func() {
		if value := recover(); value != nil {
			fail(fmt.Errorf("report export: %v", value))
		}
	}()
	update(label.L("正在准备报告数据…", "Preparing report data…"))
	modules, err := reportModules(jobDir)
	if err != nil {
		fail(err)
		return
	}
	reportMu.Lock()
	st.Total = len(significantModules(jobDir))
	reportMu.Unlock()
	snapshot, err := os.MkdirTemp("", "wgcna-report-")
	if err != nil {
		fail(err)
		return
	}
	defer os.RemoveAll(snapshot)
	// A report never changes the interactive job's selections or progress.
	for _, file := range []string{"params.json", "step1_filtered.tsv", "step3_modules.json", "network_nodes.tsv", "network_edges.tsv"} {
		if err := checkedCopy(filepath.Join(jobDir, file), filepath.Join(snapshot, file)); err != nil {
			fail(err)
			return
		}
	}
	for _, file := range []string{"expr.tsv", "step1_filter.json", "step2_sft.json", "step4_mtrait.json", "step5_detail.json", "step6_hub.json", "trait.tsv", "sample_info.tsv", "annotation.tsv", "gene2go.tsv", "gene2kegg.tsv", "overview_clusters.json", "overview_elbow.json"} {
		if _, err := os.Stat(filepath.Join(jobDir, file)); err == nil {
			if err := checkedCopy(filepath.Join(jobDir, file), filepath.Join(snapshot, file)); err != nil {
				fail(err)
				return
			}
		}
	}
	resultDir, reportDir := filepath.Join(base, "result"), filepath.Join(base, "report")
	for _, dir := range []string{resultDir, reportDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(err)
			return
		}
	}
	figureRecords, err := copyReportSnapshots(jobDir, resultDir, significantModules(snapshot))
	if err != nil {
		fail(err)
		return
	}
	figureJSON, _ := json.MarshalIndent(figureRecords, "", "  ")
	warn("figure-snapshots.json", os.MkdirAll(filepath.Join(resultDir, "00_provenance"), 0755))
	warn("figure-snapshots.json", os.WriteFile(filepath.Join(resultDir, "00_provenance", "figure-snapshots.json"), figureJSON, 0644))
	if _, err := os.Stat(filepath.Join(snapshot, "step4_mtrait.json")); err == nil && len(modules) > 0 {
		update(label.L("正在自动计算全部模块详情…", "Computing all module details…"))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		err := h.jobs.PrepareReportDetails(ctx, id, snapshot)
		cancel()
		warn("module_detail_all", err)
	} else {
		warn("module_detail_all", fmt.Errorf("module–trait analysis has not been run; per-module GS/MM is unavailable"))
	}
	// Reuse the selected module's detail only if all-module preparation failed.
	if _, err := os.Stat(filepath.Join(snapshot, "step5_detail_all.json")); err != nil {
		var d struct {
			Module string `json:"module"`
		}
		if b, err := os.ReadFile(filepath.Join(snapshot, "step5_detail.json")); err == nil && json.Unmarshal(b, &d) == nil && reportNameOK(d.Module) {
			warn("selected module detail", checkedCopy(filepath.Join(snapshot, "step5_detail.json"), filepath.Join(snapshot, "step5_detail_"+d.Module+".json")))
		}
	}
	save := func(relative string, p reportPlot) {
		if p.err != nil {
			warn(relative, p.err)
			return
		}
		defer os.RemoveAll(filepath.Dir(p.path))
		warn(relative, checkedCopy(p.path, filepath.Join(resultDir, filepath.FromSlash(relative))))
	}
	for _, keyModule := range significantModules(snapshot) {
		m := keyModule.Module
		update(label.L("正在生成模块图表：", "Rendering module: ") + m)
		prefix := "06_module_detail/" + m + "/"
		detail := filepath.Join(snapshot, "step5_detail_"+m+".json")
		if _, err := os.Stat(detail); err == nil {
			warn(prefix+"detail.json", checkedCopy(detail, filepath.Join(resultDir, filepath.FromSlash(prefix+"detail.json"))))
		}
		genes, err := enrich.ModuleGenes(snapshot, m)
		if err != nil {
			warn(prefix+"enrichment", err)
		} else {
			var rows []enrich.Result
			if enrichType == "kegg" {
				rows, err = enrich.KEGGEnrich(snapshot, genes, 0.05, 20)
			} else {
				rows, err = enrich.GOEnrich(snapshot, genes, 0.05, 20)
			}
			if err != nil {
				warn(prefix+enrichType, err)
			} else {
				dir := filepath.Join(resultDir, filepath.FromSlash(prefix+"enrichment"))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					fail(err)
					return
				}
				warn(prefix+enrichType+".tsv", writeEnrichTSV(filepath.Join(dir, enrichType+".tsv"), rows))
			}
		}
		reportMu.Lock()
		st.Done = append(st.Done, m)
		reportMu.Unlock()
	}
	update(label.L("正在生成总览、网络和聚类图…", "Rendering overview, network and clustering…"))
	if data, err := overviewData(snapshot, enrichType, "", 500, 0.05, 5); err != nil {
		warn("module_overview", err)
	} else {
		save("module_overview.png", rendered(plot.RenderModuleOverview(data, "png", plot.PlotParams{Width: 18, Height: 13})))
	}
	if _, err := os.Stat(filepath.Join(snapshot, "overview_clusters.json")); err == nil {
		if data, err := clusterOverviewData(snapshot, enrichType, "", 500, 0.05, 5); err != nil {
			warn("cluster_overview", err)
		} else {
			save("08_clustering/cluster_overview.png", rendered(plot.RenderModuleOverview(data, "png", plot.PlotParams{Width: 18, Height: 13})))
		}
	}
	// render z-score + eigengene heatmap per significant pair
	heatmapDataURLs := map[string]string{}
	_, colOrder := readSampleMeta(snapshot)
	for _, p := range significantPairs(snapshot) {
		detail := filepath.Join(snapshot, "step5_detail_"+p.Module+".json")
		if _, err := os.Stat(detail); err != nil {
			continue
		}
		key := safeName(p.Module) + "_" + safeName(p.Trait)
		rel := "05_module_trait/panels/" + key + "/heatmap.png"
		// sort samples by id + attach sample_info annotations (same as the App)
		jsonPath := detail
		if prepared, err := prepareGeneHeatmapJSON(snapshot, detail, colOrder); err == nil {
			jsonPath = prepared
		}
		pl := rendered(plot.RenderGeneHeatmap(jsonPath, "png", plot.PlotParams{Width: 9, Height: 8, Dpi: 300, Legend: "right", ClusterCols: "1"}))
		if jsonPath != detail {
			os.Remove(jsonPath)
		}
		if pl.err == nil {
			if b, err := os.ReadFile(pl.path); err == nil {
				heatmapDataURLs[key] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
			}
		}
		save(rel, pl)
	}
	// Preserve the actual numerical results, not just figures.
	for file, dest := range map[string]string{"step1_filter.json": "02_filter/filter.json", "step2_sft.json": "03_soft_threshold/sft.json", "step3_modules.json": "04_network/modules.json", "step4_mtrait.json": "05_module_trait/module_trait.json", "step6_hub.json": "07_hub/hub.json", "overview_clusters.json": "08_clustering/clusters.json", "overview_elbow.json": "08_clustering/elbow.json", "step5_detail_all.json": "06_module_detail/index.json"} {
		if _, err := os.Stat(filepath.Join(snapshot, file)); err == nil {
			warn(dest, checkedCopy(filepath.Join(snapshot, file), filepath.Join(resultDir, filepath.FromSlash(dest))))
		}
	}
	update(label.L("正在写入报告与校验清单…", "Writing report and checksums…"))
	writeProvenance(resultDir, jobDir, id, lang, filepath.Base(base))
	// Parameters must come from the same snapshot used by the figures.
	warn("parameters.final.json", checkedCopy(filepath.Join(snapshot, "params.json"), filepath.Join(resultDir, "00_provenance", "parameters.final.json")))
	if b, err := os.ReadFile(filepath.Join(snapshot, "params.json")); err == nil {
		writeParamsTSV(filepath.Join(resultDir, "00_provenance", "parameters.final.tsv"), b)
	}
	settings := map[string]any{"enrichment": enrichType, "fdr": 0.05, "key_module_criterion": "unadjusted module-trait p < 0.05; strongest significant trait per module", "figures": "analysis snapshots preferred; missing figures rendered by existing App components", "overview": "existing ComplexHeatmap renderer", "global_network": false}
	if b, err := json.MarshalIndent(settings, "", "  "); err == nil {
		warn("report-settings", os.WriteFile(filepath.Join(resultDir, "00_provenance", "report-settings.json"), b, 0o644))
	}
	if err := writeREADME(resultDir, modules); err != nil {
		fail(err)
		return
	}
	rd := buildReportData(snapshot, base, modules, enrichType, lang)
	rd.HeatmapDataURLs = heatmapDataURLs
	if value := pfloat(rd.Params, "filter", "sample_percentage", 0); value > 1 {
		warn("sample_percentage", fmt.Errorf("%g is outside the engine's 0–1 range; low-expression filtering did not remove genes with this threshold", value))
	}
	rd.KeyModules = significantModules(snapshot)
	rd.FigureSources = map[string]string{}
	for _, record := range figureRecords {
		rd.FigureSources[record.Key] = record.Origin
	}
	rd.ModuleTraits = map[string]string{}
	for _, m := range rd.KeyModules {
		rd.ModuleTraits[m.Module] = m.Trait
	}
	reportMu.Lock()
	rd.Warnings = append([]string{}, st.Warnings...)
	reportMu.Unlock()
	rd.Assets = map[string]bool{}
	_ = filepath.Walk(resultDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(resultDir, path)
			rd.Assets[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err := writeReport(reportDir, rd); err != nil {
		fail(err)
		return
	}
	if b, err := json.MarshalIndent(rd.Warnings, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(resultDir, "00_provenance", "export-warnings.json"), b, 0o644); err != nil {
			fail(err)
			return
		}
	}
	if err := writeChecksumsAndExportManifest(resultDir); err != nil {
		fail(err)
		return
	}
	reportMu.Lock()
	st.State = "done"
	st.Message = ""
	st.PreviewURL = "/api/jobs/" + id + "/report/view/report/index.html"
	latest, err := json.MarshalIndent(st, "", "  ")
	reportMu.Unlock()
	if err == nil {
		err = os.WriteFile(filepath.Join(jobDir, "report-latest.json"), latest, 0o644)
	}
	if err != nil {
		warn("save preview state", err)
	}
}

func reportNetworkView(nodes []netNode, edges []netEdge, mode string) ([]netNode, []netEdge) {
	active := map[string]bool{}
	selected := []netEdge{}
	for _, edge := range edges {
		if mode == "combined" || edge.Type == mode {
			selected = append(selected, edge)
			active[edge.Source] = true
			active[edge.Target] = true
		}
	}
	visible := []netNode{}
	for _, node := range nodes {
		if active[node.ID] {
			visible = append(visible, node)
		}
	}
	return visible, selected
}
