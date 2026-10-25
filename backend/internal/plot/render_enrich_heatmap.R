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
col_fun <- colorRamp2(c(-mx, 0, mx), c("#2166ac", "#ffffff", "#b2182b"))

ht <- Heatmap(
  m,
  name = "z-score",
  top_annotation = ha,
  cluster_rows = TRUE,
  cluster_columns = FALSE,
  show_column_names = TRUE,
  column_names_rot = 45,
  row_names_gp = gpar(fontsize = font_size),
  column_names_gp = gpar(fontsize = font_size),
  col = col_fun
)

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = "white")
}
draw(ht)
dev.off()
cat("done:", out_path, "\n")
