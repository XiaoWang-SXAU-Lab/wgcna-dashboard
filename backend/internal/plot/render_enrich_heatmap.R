#!/usr/bin/env Rscript
# Render the enrichment gene-expression heatmap with ComplexHeatmap: row/column
# clustering + group annotation for the samples.
suppressPackageStartupMessages({library(jsonlite); library(ComplexHeatmap); library(circlize)})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
mode      <- get_arg("--mode", "png")
out_path  <- get_arg("--out")
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "8"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
font_size <- as.numeric(get_arg("--base-size", "9"))
low         <- get_arg("--low", "#2166ac")
mid         <- get_arg("--mid", "#ffffff")
high        <- get_arg("--high", "#b2182b")
legend      <- get_arg("--legend", "right")   # right | bottom | none
cluster_rows  <- as.integer(get_arg("--cluster-rows", "1"))
cluster_cols  <- as.integer(get_arg("--cluster-cols", "0"))
show_rownames <- as.integer(get_arg("--show-rownames", "1"))
show_colnames <- as.integer(get_arg("--show-colnames", "1"))
border        <- as.integer(get_arg("--border", "0"))
title         <- get_arg("--title", "")

if (is.null(json_path) || is.null(out_path)) stop("usage: render_enrich_heatmap.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = FALSE)
m <- matrix(as.numeric(unlist(d$z)), nrow = length(d$genes), byrow = TRUE)
rownames(m) <- unlist(d$genes)
colnames(m) <- unlist(d$samples)
group <- as.character(unlist(d$group))

pal <- c("#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b",
         "#e377c2", "#7f7f7f", "#bcbd22", "#17becf", "#aec7e8", "#ffbb78")
u <- sort(unique(group))
gcols <- setNames(pal[seq_along(u)], u)

ha <- HeatmapAnnotation(
  Group = group,
  col = list(Group = gcols),
  annotation_legend_param = list(title = "Group")
)

mx <- max(abs(m), na.rm = TRUE)
col_fun <- colorRamp2(c(-mx, 0, mx), c(low, mid, high))

show_legend <- legend != "none"

ht <- Heatmap(
  m,
  name = "z-score",
  top_annotation = ha,
  cluster_rows = cluster_rows == 1,
  cluster_columns = cluster_cols == 1,
  show_row_names = show_rownames == 1,
  show_column_names = show_colnames == 1,
  column_names_rot = 45,
  row_names_gp = gpar(fontsize = font_size),
  column_names_gp = gpar(fontsize = font_size),
  col = col_fun,
  show_heatmap_legend = show_legend,
  border_gp = gpar(col = if (border == 1) "grey40" else NA),
  column_title = title
)

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = "white")
}
if (legend == "bottom") {
  draw(ht, heatmap_legend_side = "bottom")
} else {
  draw(ht, heatmap_legend_side = "right")
}
dev.off()
cat("done:", out_path, "\n")
