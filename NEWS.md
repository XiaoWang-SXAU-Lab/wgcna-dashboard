# News / Changelog

All notable changes to the wgcna-dashboard are documented in this file. The
format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — 2026-11-05

### Added

- **Persistent project workspace**: create/open/recent projects, revisioned
  drafts and results, project-level question card, per-step execution
  provenance, safe reopening, and stale-result detection.
- **Evidence-aware candidate priority** (`hub_priority`): fixed full-module
  bootstrap ranking of kWithin stability with valid/invalid round accounting,
  midrank/P05/P95, top-K frequency, deterministic seeds, and an explicit
  engineering budget (`g²×B ≤ 1e9`). The UI includes a plain-language summary,
  a technical table with `Symbol/Description` annotation columns, annotated CSV
  export, and a clear warning that this is connectivity stability—not a
  probability, trait association, or causal hub.
- **Offline evidence interpretation**: server-side reconstruction from the
  current atomic `CandidateReportEnvelope` into an evidence bundle, deterministic
  `OfflineSuggestion`, and `Evaluate` rendering; `GET
  /api/projects/{id}/evidence/hub-priority` returns safe `not_current` errors and
  never exposes underlying paths. The report card is offline and does not call an
  LLM.
- **Biologist-first report page**: one-sentence conclusion, what the analysis
  did, how to read it, what it cannot conclude, and next steps, with technical
  details kept in a collapsible section.

### Changed

- Module Explorer shows the TOM-weighted network by default. Edge stability is
  a collapsed, optional check below the network, with CSV export.
- Data Import `sample_percentage` is now a **0–1 slider**. `0` disables
  low-expression filtering; legacy projects storing values > 1 are migrated
  (e.g. `50 → 0.5`). The Go backend rejects out-of-range filter values before
  starting the worker, so a saved percentage can no longer silently disable
  filtering.
- Candidate-priority tables load annotation for the selected priority module,
  show `Symbol/name` and `Description`, and include them in the table CSV. The
  raw worker TSV link is explicitly labelled as having no annotation.
- Candidate-priority runs above the engineering budget are blocked in the UI
  with the maximum safe B and a one-click “Set B=100” action; workers still
  enforce the same cap.
- Module overview/report figures show a clear “Figure not generated” notice
  instead of a broken image when a module asset is missing.

### Fixed

- High idle CPU on high-refresh displays: the navbar toolkits pulse and the
  mascot's idle tail/blink animations kept the whole WebView repainting at the
  display refresh rate. They are now static; hover, thinking and happy cues
  remain. On a 165 Hz display, idle Module Explorer CPU dropped from ~90% to
  <10%, and the home page from ~34% to <1%.
- Large result tables now render only the visible window of rows. The 8,000-gene
  kME table (8,000 × 7) no longer creates ~72,000 DOM nodes; search, sort and CSV
  export still use the full data. Candidate-priority and module-overview tables
  use the same component.
- Stale enrichment assertion in `report_test.go`; the report baseline now
  validates the interactive module-enrichment contract.
- Project/worker errors are surfaced with fixed safe messages instead of
  leaking internal error strings or filesystem paths on stale/not-run evidence.

### Documentation

- Added `docs/exploration/R_EAN_DIFFERENTIAL_PROPOSAL.md` (exploratory
  differential co-expression design with n=3 constraints).
- Handoff and DSH continuation notes updated for project workspace, evidence,
  candidate priority, and the sample-fraction guard.

## [0.6.0] — 2026-10-30

### Changed

- Redesigned the bilingual homepage around rsWGCNA analysis capabilities; removed the obsolete one-step entry.
- Unified application colors, controls and tables while preserving analysis-page layouts and parameter sidebars.
- Added project GitHub and contact-email links to the navbar and updated homepage affiliations.
- Moved report directory, project name and export controls into the report sidebar; removed the enrichment selector.
- Restyled offline reports with scientific result cards, responsive chart/table layouts and clearer module/enrichment tabs.
- Revised report descriptions and scale-free topology interpretations; corrected the personal laboratory link and embedded the project icon.
- Improved enrichment bubble-chart label containment without changing analysis data.

### Fixed

- Use pnpm's hoisted linker for fresh Windows installs without symlink privileges.
- Resolve the default worker source path after PowerShell parameter binding.
- Document the persistent local release folder and required bundled R runtime.

### Documentation

- Retire the R-free packaging roadmap item: some plots continue to use ggplot2, ComplexHeatmap and circlize.
- Bump application, report and desktop packaging metadata to 0.6.0.

## [0.5.0] — 2026-10-26

### Added

- **Agentic AI navigator**: tool-calling agent loop that reads the params and
  results of every pipeline step (9 tools: params / filter / SFT / network /
  module-trait / module detail / hub / enrichment / literature).
- **Desktop packaging**: Windows Inno Setup installer (`rsWGCNA-setup-0.5.0.exe`,
  bundled R, app icon, GUI subsystem so no console window appears) and a macOS
  `rsWGCNA-0.5.0.dmg`.
- **Multi-source literature search** (Europe PMC + PubMed + Semantic Scholar,
  with optional Google Scholar), merged + de-duplicated by PMID/DOI.
- Source attribution + anti-hallucination: answers must cite sources, and
  literature must come from the search tool (no fabricated PMID/DOI).
- Citation formatting: in-text "Author et al. (year)" + a References section.
- Chat skills (`@`/`/`), Enter=newline / Cmd+Enter=send, animated cat mascot,
  and citation links open in a new tab.

### Changed

- Enrichment split into GO/KEGG panels with an ALL/MF/BP/CC ontology selector
  and chart↔table linkage.
- Module explorer has five sub-panels; the annotation panel now loads on entry.
- Network graph gains Gephi-style expand/contract (radial scale).
- GS/MM scatter: proper 95% CI band (closed polygon) + t-distribution.

## [0.4.0] — 2026-10-26

### Added

- Enrichment split into separate **GO enrichment** and **KEGG enrichment**
  panels; GO gains an ALL/MF/BP/CC ontology selector (ontology read from the
  gene2go table with an obo fallback + GO-id normalization).
- Enrichment chart↔table linkage: clicking a dot highlights the matching table
  row, and clicking a table row opens the gene detail + heatmap.
- AI navigator skills: type `@` or `/` to summon a skill menu (literature /
  interpret / background / troubleshoot); animated cat mascot with a random
  reminder bubble.

### Changed

- Module explorer is now five sub-panels: Scatter & heatmap / Network /
  Annotation / GO enrichment / KEGG enrichment.
- Dotplot ordering follows clusterProfiler (GeneRatio), with a drawn x-axis,
  inward x title, outward y-axis, and >30-char term truncation (full term in
  hover).
- Module heatmap x-axis sorts samples by name (natural order).
- Annotation table gains kME/GS columns; the network/hub table gains gene
  annotation columns (when annotation is uploaded).
- Chat input: Enter = newline, ⌘/Ctrl+Enter = send.
- Network type exposes `signed hybrid`.

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
