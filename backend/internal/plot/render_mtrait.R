#!/usr/bin/env Rscript
# Module-trait correlation heatmap + module color band (ComplexHeatmap).
# Faithful port of the old Shiny app's ComplexHeatmap code (oneStepWGCNA.R).
suppressPackageStartupMessages({
  library(jsonlite)
  library(ComplexHeatmap)
  library(circlize)
  library(grid)
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
height    <- as.numeric(get_arg("--height", "6"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
cluster_rows  <- as.integer(get_arg("--cluster-rows", "0"))
cluster_cols  <- as.integer(get_arg("--cluster-cols", "0"))
show_rownames <- as.integer(get_arg("--show-rownames", "1"))
show_colnames <- as.integer(get_arg("--show-colnames", "1"))
border        <- as.integer(get_arg("--border", "1"))
cell_fontsize <- as.numeric(get_arg("--cell-fontsize", "12"))
title         <- get_arg("--title", "Module-trait relationships")
if (is.null(json_path) || is.null(out_path)) stop("usage: render_mtrait.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
modules <- d$modules
traits  <- d$traits
cor     <- as.matrix(d$cor)
pval    <- as.matrix(d$p)
rownames(cor) <- modules
colnames(cor) <- traits

# textMatrix: correlation (2 sig figs) + newline + (p-value in parentheses)
cor_rounded <- signif(cor, 2)
p_rounded   <- signif(pval, 2)
text_mat <- matrix(paste0(cor_rounded, "\n(", p_rounded, ")"), nrow = nrow(cor))

col_fun <- colorRamp2(c(-1, 0, 1), c(low, mid, high))

# module color band (WGCNA color names are valid R colors)
mod_col <- setNames(modules, modules)
left_anno <- rowAnnotation(
  Module = modules,
  col = list(Module = mod_col),
  show_legend = FALSE,
  show_annotation_name = FALSE
)

# NOTE: do NOT set an explicit Heatmap width/height. ComplexHeatmap's default
# is unit(1, "npc") (fills the whole canvas); an explicit cm/in size leaves
# whitespace around the body when the canvas is larger.
show_legend <- legend != "none"
leg_direction <- if (legend == "bottom") "horizontal" else "vertical"

ht <- Heatmap(
  cor,
  name = "correlation",
  col = col_fun,
  cluster_rows = cluster_rows == 1,
  cluster_columns = cluster_cols == 1,
  show_row_names = show_rownames == 1,
  show_column_names = show_colnames == 1,
  left_annotation = left_anno,
  cell_fun = function(j, i, x, y, width, height, fill) {
    grid.text(text_mat[i, j], x, y, gp = gpar(fontsize = cell_fontsize))
  },
  row_names_side = "left",
  column_names_rot = 90,
  heatmap_legend_param = list(
    at = c(-1, -0.5, 0, 0.5, 1),
    labels = c("-1", "-0.5", "0", "0.5", "1"),
    title = "Correlation",
    direction = leg_direction,
    title_position = if (legend == "bottom") "topcenter" else "lefttop-rot"
  ),
  show_heatmap_legend = show_legend,
  rect_gp = gpar(col = if (border == 1) "black" else NA, lwd = 1.2),
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
