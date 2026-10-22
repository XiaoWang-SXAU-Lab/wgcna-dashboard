# News / Changelog

All notable changes to the wgcna-dashboard are documented in this file. The
format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Raw-count `vst (DESeq2)` normalization option** in Data Import (pure-Rust,
  no R dependency).
- `rsWGCNA` brand name + version badge in the navbar.
- AI navigator markdown (rich-text) rendering + "thinking" spinner.
- Per-step status badge + prominent friendly error display with one-click AI
  diagnosis.

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
