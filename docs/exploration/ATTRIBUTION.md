# Attribution and Methodological Differences

## Referenced software

### hdWGCNA

- **Paper (cite this in any publication using the ring / module-network visualization concepts):**
  Morabito S, Reese F, Rahimzadeh N, Miyoshi E, Swarup V.
  *hdWGCNA identifies co-expression networks in high-dimensional transcriptomics data.*
  Cell Reports Methods. 2023. DOI: 10.1016/j.crmeth.2023.100498.
- **License:** GNU GPLv3 (as declared in the package DESCRIPTION and the GitHub repository).
- **Docs:** https://smorabit.github.io/hdWGCNA/articles/network_visualizations.html

### WGCNA

- Langfelder P, Horvath S. *WGCNA: an R package for weighted correlation network analysis.*
  BMC Bioinformatics. 2008;9:559. DOI: 10.1186/1471-2105-9-559.

## What rsWGCNA borrows vs. what is its own

rsWGCNA **independently re-implements** the following visualization *ideas*; it does not
copy or adapt hdWGCNA source code.

| Concept | hdWGCNA reference | rsWGCNA note |
|---|---|---|
| Top-N concentric ring module network | `ModuleNetworkPlot` | Implemented as a **layout strategy** (`ringTopN`) over rsWGCNA's own shared scientific network; top-N / inner-count are user-adjustable. |
| Hub-gene network | `HubGeneNetworkPlot` | rsWGCNA's hub network predates the ring work and uses its own induced-subgraph selection + deterministic sparse display. |
| Network landscape / module UMAP | `RunModuleUMAP` / `ModuleUMAPPlot` | **Not planned for bulk WGCNA.** UMAP reveals structure not already shown elsewhere (single-cell populations); bulk module structure is already faithfully shown by the dendrogram + TOM heatmap + eigengene (PCA), so a non-linear 2D embedding is a lossy, redundant re-projection. Revisit only if a reviewer or user explicitly requests it. |

### rsWGCNA-specific contributions

- Independent Rust network computation and node/edge extraction.
- **Deterministic, structure-aware display sparsification** (hub-first + budgeted non-hub edges).
- **Provenance tracking**: `weight_type`, `network_type`, `power`, `tom_scope`,
  `tom_pool_size`, `used_threshold`, `edge_set_completeness` are recorded with the
  emitted edge set so a display subset is never mistaken for a scientific network.
- Interactive exploration (search, neighbor highlight, editable labels, layout switching)
  applied to the *same* scientific edge set.
- **R-EAN (Reliability- and Evidence-Aware Network)** — rsWGCNA's own, tentatively named
  network-representation method that encodes connection strength, resampling stability,
  and condition differences as *separate* evidence layers. The underlying analysis ideas
  (bootstrap stability, differential co-expression, bridge genes) overlap with existing
  methods and are **not** claimed as rsWGCNA firsts; only the multi-layer evidence-aware
  representation is our own. Currently only the reliability layer (`edge_stability`) is
  implemented.

## TOM scope

The current hub-network TOM is computed on a **400-gene candidate pool**
(`tom_scope = "candidate_pool"`), not on the full filtered gene set. Per the WGCNA TOM
definition, changing the participating gene set changes shared-neighbor counts and
connectivity, so a candidate-pool TOM is **not** a submatrix of a whole-network TOM and
must not be described as such. See the architecture notes for the planned
"canonical TOM scope" work.

## License compliance

- Reading hdWGCNA's paper and re-implementing the *ideas* requires academic citation
  (above); it does not, by itself, impose hdWGCNA's license.
- Copying, translating, or deriving from hdWGCNA **source code** would trigger GPLv3
  obligations. rsWGCNA does not copy hdWGCNA code.
- Citation of the paper does not substitute for software-license obligations where code
  is reused.
