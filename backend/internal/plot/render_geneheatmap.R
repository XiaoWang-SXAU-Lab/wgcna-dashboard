#!/usr/bin/env Rscript
# Module gene expression heatmap (z-scored) + eigengene barplot (ComplexHeatmap).
# NOTE: body size uses ComplexHeatmap's default unit(1,"npc") (fills the canvas);
# do NOT set an explicit width/height in Heatmap().
suppressPackageStartupMessages({
  library(jsonlite)
  library(ComplexHeatmap)
  library(circlize)
})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
mode      <- get_arg("--mode", "png")
out_path  <- get_arg("--out")
low       <- get_arg("--low", "#2166ac")
mid       <- get_arg("--mid", "#ffffff")
high      <- get_arg("--high", "#b2182b")
legend    <- get_arg("--legend", "right")   # right | bottom | none
width     <- as.numeric(get_arg("--width", "7"))
height    <- as.numeric(get_arg("--height", "7"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
cluster_rows  <- as.integer(get_arg("--cluster-rows", "1"))
cluster_cols  <- as.integer(get_arg("--cluster-cols", "0"))
show_rownames <- as.integer(get_arg("--show-rownames", "0"))
show_colnames <- as.integer(get_arg("--show-colnames", "1"))
border        <- as.integer(get_arg("--border", "0"))
title         <- get_arg("--title", NULL)
if (is.null(json_path) || is.null(out_path)) stop("usage: render_geneheatmap.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
d_raw <- fromJSON(json_path, simplifyVector = FALSE)
mod <- d$module
if (is.null(title)) title <- paste0("Module ", mod, " gene expression")
genes <- d$genes
samples <- d$samples
expr <- as.matrix(d$expression)   # genes x samples
eg <- d$eigengene                 # samples
if (is.null(samples) || length(samples) == 0) {
  samples <- paste0("S", seq_len(ncol(expr)))
}

# z-score each gene (row) across samples
z <- t(scale(t(expr)))
rownames(z) <- genes
colnames(z) <- samples

col_fun <- colorRamp2(c(-2, 0, 2), c(low, mid, high))

# sample-info annotation color blocks (top)
sample_ann <- d_raw$sample_annotations
top_ha <- NULL
if (!is.null(sample_ann) && length(sample_ann) > 0) {
  df <- as.data.frame(lapply(sample_ann, function(a) factor(unlist(a$values))),
                      stringsAsFactors = FALSE)
  names(df) <- vapply(sample_ann, function(a) a$name, character(1))
  top_ha <- HeatmapAnnotation(
    df = df, which = "column", show_annotation_name = FALSE,
    annotation_name_gp = gpar(fontsize = 8), gp = gpar(col = "white")
  )
}

# eigengene barplot (bottom annotation)
bar_fill <- if (mod %in% colors()) mod else "#2563eb"
bottom_ha <- columnAnnotation(
  Eigengene = anno_barplot(eg, gp = gpar(fill = bar_fill), border = FALSE),
  show_annotation_name = FALSE
)

show_legend <- legend != "none"
leg_direction <- if (legend == "bottom") "horizontal" else "vertical"

ht <- Heatmap(
  z,
  name = "z-score",
  col = col_fun,
  cluster_rows = cluster_rows == 1,
  cluster_columns = cluster_cols == 1,
  show_row_names = show_rownames == 1,
  show_column_names = show_colnames == 1,
  column_names_rot = 45,
  top_annotation = top_ha,
  bottom_annotation = bottom_ha,
  heatmap_legend_param = list(
    title = "Z-score",
    direction = leg_direction,
    title_position = if (legend == "bottom") "topcenter" else "lefttop-rot"
  ),
  show_heatmap_legend = show_legend,
  border_gp = gpar(col = if (border == 1) "grey40" else NA),
  column_title = title,
  column_title_gp = gpar(fontsize = 15, fontface = "bold")
)

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi)
}
if (legend == "bottom") {
  draw(ht, heatmap_legend_side = "bottom")
} else {
  draw(ht, heatmap_legend_side = "right")
}
dev.off()
cat("done:", out_path, "\n")
