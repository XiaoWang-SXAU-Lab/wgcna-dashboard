rsWGCNA design preview — all data and figures are synthetic; no biological conclusions.

Open report/index.html. Steps below follow the report.
01_input/sample_info.tsv: all 18 input samples. genotype, condition and replicate describe each sample.
00_provenance/parameters.final.json: all example parameters. Production export records actual run parameters.
00_provenance/view-state.json: example sample-tree annotations, colors and module/trait selection.
02_filter/expression_filtered.preview.tsv: an 8-gene × 17-sample synthetic download example, not the complete filtered matrix. Rows are genes, columns are samples.
02_filter/removed_samples.tsv: excluded samples and recorded actions. Unspecified reasons are not inferred.
Figure 1 / 02_filter/sample-tree.svg: sample-tree illustration; branches indicate similarity; condition/genotype annotation order is fixed; red marks manual exclusion.
Figure 2 / 03_soft_threshold/soft-threshold.svg: candidate power versus fit R²; target .85; chosen power 8 (illustrative).
Figure 3 / 04_network/modules.svg: gene tree and module rows. Colors are identifiers, not significance; grey is unassigned.
Figure 4 / 05_module_trait/module-trait.svg: correlation heatmap illustration; red positive, blue negative. No actual p-values or significance claims.
Figure 5 / 06_module_detail/turquoise/gs-mm.svg: synthetic |kME| versus |GS|; dashed lines show thresholds.
Figure 6 / 06_module_detail/turquoise/enrichment.svg: illustrative GO plot, not enrichment results.
Figure 7 / 07_hub/turquoise/Drought/network.svg: network layout illustration, not actual candidate-gene connections.

The complete production directory specification is in ../../DIRECTORY-STRUCTURE.md. This preview includes review files only.
