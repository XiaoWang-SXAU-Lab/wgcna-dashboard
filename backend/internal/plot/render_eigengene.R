#!/usr/bin/env Rscript
# Module eigengene adjacency heatmap (ComplexHeatmap, plotEigengeneNetworks style).
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
cluster_cols  <- as.integer(get_arg("--cluster-cols", "1"))
show_rownames <- as.integer(get_arg("--show-rownames", "1"))
show_colnames <- as.integer(get_arg("--show-colnames", "1"))
border        <- as.integer(get_arg("--border", "0"))
title         <- get_arg("--title", "Eigengene adjacency")
if (is.null(json_path) || is.null(out_path)) stop("usage: render_eigengene.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
modules <- d$modules
eg <- as.matrix(d$eigengenes)

corm <- cor(eg)
adj <- (1 + corm) / 2
rownames(adj) <- modules
colnames(adj) <- modules
hc <- hclust(as.dist(1 - adj), method = "average")

col_fun <- colorRamp2(c(0, 0.5, 1), c(low, mid, high))

mod_col <- setNames(modules, modules)
row_ha <- rowAnnotation(
  Module = modules,
  col = list(Module = mod_col),
  show_legend = FALSE,
  show_annotation_name = FALSE
)
col_ha <- columnAnnotation(
  Module = modules,
  col = list(Module = mod_col),
  show_legend = FALSE,
  show_annotation_name = FALSE
)

show_legend <- legend != "none"
leg_direction <- if (legend == "bottom") "horizontal" else "vertical"

ht <- Heatmap(
  adj,
  name = "adjacency",
  col = col_fun,
  cluster_rows = if (cluster_rows == 1) hc else FALSE,
  cluster_columns = if (cluster_cols == 1) hc else FALSE,
  show_row_names = show_rownames == 1,
  show_column_names = show_colnames == 1,
  left_annotation = row_ha,
  top_annotation = col_ha,
  row_names_gp = gpar(fontsize = 10),
  column_names_gp = gpar(fontsize = 10),
  column_names_rot = 45,
  heatmap_legend_param = list(
    title = "Adjacency",
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
