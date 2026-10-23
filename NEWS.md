# News / Changelog

All notable changes to the wgcna-dashboard are documented in this file. The
format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.3.0] — 2026-10-24

### Added

- Data import: optional sample info / gene annotation / Gene2GO / Gene2KEGG
  uploads, a worker-threads option (default cores-2), and a built-in demo
  dataset (load / download).
- Sample tree: multiple classification color bands, red highlight for selected
  samples, and a publication export widget (SVG/PDF/PNG via R).
- Module Explorer panel merging "module of interest" and "hub genes", with tabs
  for scatter+heatmap, network, gene annotation, and enrichment.
- GO/KEGG over-representation analysis (hypergeometric test + Benjamini-Hochberg
  FDR, clusterProfiler-style output); interactive bubble plot with click-to-gene
  table + clustered expression heatmap, and ggplot2 publication export.
- GO ontology support (bundled go-basic.obo): Wang semantic similarity +
  simplify (redundancy removal), plus GO version check/update.
- Network visualization: node color by kME/GS/degree/hub/module, edge weights
  normalized to their actual range, hub emphasis, hover tooltips, legend, and
  hub-and-spoke / hierarchical layouts.

### Changed

- Hub defaults: kME cutoff 0.8, GS cutoff 0, node size by degree.
- pnpm: allow build scripts via `pnpm-workspace.yaml` (pnpm 11).

## [0.2.0] — 2026-10-23

### Added

- **Raw-count `vst (DESeq2)` normalization option** in Data Import (pure-Rust,
  no R dependency).
- `rsWGCNA` brand name + version badge in the navbar.
- AI navigator markdown (rich-text) rendering + "thinking" spinner.
- Per-step status badge + prominent friendly error display with one-click AI
  diagnosis.
- **Gephi-style full-screen network graph** (cytoscape.js + fcose/ForceAtlas2):
  hub-and-spoke extraction, multiple layouts, edge weight gradient, node size
  by degree/kME/GS, zoom/fit, and editable gene→label mapping.
- **ComplexHeatmap** rewrite of the module–trait / eigengene / gene-expression
  heatmaps (clustering + module color band + p-value annotation), with a
  colorblind-friendly RdBu palette.
- GS/MM scatter confidence-interval band.
- Scale-free topology check plots (connectivity histogram + log-log regression).
- Excel (.xlsx/.xls) file input.

### Changed

- Branding: `WGCNA Dashboard` → `rsWGCNA`.
- AI replies render as rich text instead of raw markdown.

### Fixed

- **Dirty cascade**: re-running a step invalidates it and all downstream steps
  (backend) and clears cached downstream results (frontend).
- **Parameter re-run**: re-running a step now sends the current parameters, so
  changing `nKeep` / `sample %` and re-running actually updates the report and
  sample tree (previously used stale `params.json`).
- i18n gaps: file-upload controls, sample-tree hover hint, and all "Download
  PDF/SVG/PNG" buttons now follow EN/CN mode.

## [0.1.0] — 2026-10-21

### Added

- Full 7-step pipeline UI (import → filter → soft-threshold → network →
  module–trait → module detail → hub genes).
- One-step (`run-all`) and step-by-step modes with caching.
- Interactive visualization: ECharts dendrogram (module-colored), module–trait
  heatmap, GS/MM scatter, AG Grid gene tables.
- Publication-quality SFT plot (PDF/SVG download).
- AI navigator: multi-vendor LLM (OpenAI-compatible), local vector RAG,
  encrypted API key, streaming Q&A, error diagnosis.
- Single-binary deployment (frontend + Rust worker embedded).
- EN/CN i18n with collapsible drawer sidebars.

## Build

```bash
make build            # single binary (needs Node + Rust + Go)
```
