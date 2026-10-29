#!/usr/bin/env Rscript
# Module overview figure — plotting logic reverse-engineered from
# ClusterGVis::visCluster (junjunlab/ClusterGVis), re-implemented with only
# ComplexHeatmap + circlize (no dplyr/ggplot2/clusterProfiler). Clustering and
# enrichment are performed upstream in Rust; this script only plots.
# Reference: junjunlab/ClusterGVis <https://github.com/junjunlab/ClusterGVis>
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
out_path  <- get_arg("--out")
mode      <- get_arg("--mode", "png")
width     <- as.numeric(get_arg("--width", "18"))
height    <- as.numeric(get_arg("--height", "12"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
top_n     <- as.integer(get_arg("--top-n", "5"))
font_size <- as.numeric(get_arg("--base-size", "8"))
line_col  <- get_arg("--line-col", "#CC3333")
low       <- get_arg("--low", "#08519C")
mid       <- get_arg("--mid", "white")
high      <- get_arg("--high", "#A50F15")

if (is.null(json_path) || is.null(out_path)) stop("usage: render_module_overview.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = FALSE)

genes        <- unlist(d$genes)
samples      <- unlist(d$samples)
z            <- matrix(as.numeric(unlist(d$z)), nrow = length(genes), byrow = TRUE)
rownames(z)  <- genes
colnames(z)  <- samples
modules      <- unlist(d$modules)
module_order <- unlist(d$module_order)
enrich       <- d$enrich
sample_ann   <- d$sample_annotations   # list of {name, values}

# explicit color per module/cluster (WGCNA uses its own color names; kmeans/cmeans
# pass generated colors). Fall back to the module name (valid R color for WGCNA).
module_colors <- d$module_colors
if (is.null(module_colors)) {
  module_colors <- setNames(module_order, module_order)
} else {
  module_colors <- unlist(module_colors)
}

module_factor <- factor(modules, levels = module_order)
col_fun <- colorRamp2(c(-2, 0, 2), c(low, mid, high))
n_samples <- length(samples)

# ---- sample annotations (all sample-info metadata columns as color blocks) ----
column_split <- NULL
top_anno <- NULL
if (!is.null(sample_ann) && length(sample_ann) > 0) {
  df <- as.data.frame(lapply(sample_ann, function(a) factor(unlist(a$values))),
                      stringsAsFactors = FALSE)
  names(df) <- vapply(sample_ann, function(a) a$name, character(1))
  top_anno <- HeatmapAnnotation(
    df = df, which = "column", show_annotation_name = FALSE,
    annotation_name_gp = gpar(fontsize = font_size), gp = gpar(col = "white")
  )
}

# ---- left annotation: median-expression line per module (visCluster panel_fun) ----
panel_fun <- function(index, nm) {
  tmpmat <- z[index, , drop = FALSE]
  rg <- range(tmpmat)
  mdia <- apply(tmpmat, 2, median)
  pushViewport(viewport(xscale = c(-0.1, 1.1), yscale = c(0, 1)))
  grid.rect()
  # rescale sample axis to c(0.1, 0.9), value to c(0,1) from rg±0.5 (visCluster)
  pos <- (seq_len(ncol(tmpmat)) - 1) / max(ncol(tmpmat) - 1, 1) * 0.8 + 0.1
  y <- (mdia - (rg[1] - 0.5)) / ((rg[2] + 0.5) - (rg[1] - 0.5))
  linec <- module_colors[[nm]]
  if (is.null(linec)) linec <- nm
  grid.lines(x = pos, y = y, gp = gpar(lwd = 3, col = linec))
  grid.text(paste("Gene size:", length(index)), x = 0.5, y = 0.8,
            gp = gpar(fontsize = font_size, fontface = "italic"))
  popViewport()
}

left_anno <- rowAnnotation(
  line = anno_link(
    align_to = module_factor, which = "row", panel_fun = panel_fun,
    size = unit(2, "cm"), gap = unit(0.25, "cm"), width = unit(4, "cm"),
    side = "left", link_gp = gpar(fill = "grey90", col = NA)
  ),
  show_annotation_name = FALSE
)

# module color block (WGCNA uses its own color names; kmeans/cmeans use generated colors)
left_mod <- rowAnnotation(
  Module = anno_block(
    gp = gpar(fill = unname(module_colors[module_order])),
    labels = rep("", length(module_order)),
    width = unit(0.4, "cm"),
    show_name = FALSE
  ),
  show_annotation_name = FALSE
)

# ---- right annotation: enrichment terms (anno_textbox, one term per line) ----
text_list <- lapply(module_order, function(m) {
  terms <- enrich[[m]]
  if (is.null(terms) || length(terms) == 0) {
    return(data.frame(text = "\u2014", stringsAsFactors = FALSE))
  }
  terms <- head(terms, top_n)
  data.frame(
    text = vapply(terms, function(t) t$term, character(1)),
    col = rep("#233b40", length(terms)),
    fontsize = rep(font_size, length(terms)),
    stringsAsFactors = FALSE
  )
})
names(text_list) <- module_order
right_anno <- rowAnnotation(
  textbox = anno_textbox(
    module_factor, text_list, which = "row", side = "right",
    word_wrap = TRUE, add_new_line = TRUE,
    background_gp = gpar(fill = "grey95", col = "grey50")
  ),
  show_annotation_name = FALSE
)

# ---- heatmap ----
ht <- Heatmap(
  z,
  name = "Z-score",
  col = col_fun,
  row_split = module_factor,
  row_title = NULL,
  cluster_rows = TRUE,
  cluster_row_slices = FALSE,
  show_row_dend = FALSE,
  cluster_columns = FALSE,
  show_row_names = FALSE,
  show_column_names = n_samples <= 50,
  column_names_rot = 45,
  column_names_side = "top",
  column_names_gp = gpar(fontsize = font_size),
  column_split = column_split,
  column_title = NULL,
  top_annotation = top_anno,
  left_annotation = c(left_anno, left_mod),
  right_annotation = right_anno,
  border = TRUE
)

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = "transparent")
}
draw(ht, background = "transparent")
dev.off()
cat("done:", out_path, "\n")
