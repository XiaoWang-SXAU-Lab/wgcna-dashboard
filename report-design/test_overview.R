#!/usr/bin/env Rscript
# Module overview — three-part figure (ClusterGVis style):
#   left  : per-module eigengene sparkline (aligned to each module block)
#   middle: z-scored gene heatmap, row_split by module, genes clustered within module
#   right : per-module GO/KEGG enrichment top-N terms
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
width     <- as.numeric(get_arg("--width", "16"))
height    <- as.numeric(get_arg("--height", "12"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
low       <- get_arg("--low", "#2166ac")
mid       <- get_arg("--mid", "#ffffff")
high      <- get_arg("--high", "#b2182b")
top_n     <- as.integer(get_arg("--top-n", "5"))
font_size <- as.numeric(get_arg("--base-size", "8"))

if (is.null(json_path) || is.null(out_path)) stop("usage: render_module_overview.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = FALSE)

z <- matrix(as.numeric(unlist(d$z)), nrow = length(d$genes), byrow = TRUE)
rownames(z) <- unlist(d$genes)
colnames(z) <- unlist(d$samples)
modules    <- unlist(d$modules)
mod_order  <- unlist(d$module_order)
eigengenes <- d$eigengenes          # named list: module -> numeric vector (per sample)
enrich     <- d$enrich              # named list: module -> list of {term, padj} (display order)

module_factor <- factor(modules, levels = mod_order)
col_fun <- colorRamp2(c(-2, 0, 2), c(low, mid, high))

# module color block (adjacent to heatmap body)
left_mod <- rowAnnotation(
  Module = anno_block(
    gp = gpar(fill = mod_order),
    labels = mod_order,
    labels_gp = gpar(col = "#111", fontsize = font_size),
    width = unit(0.9, "cm"),
    show_name = FALSE
  ),
  show_annotation_name = FALSE
)

# eigengene sparkline (leftmost); panel_fun's `nm` is the 1-based block index,
# so map it back to the module name via a closure over mod_order.
left_eig <- local({
  eg <- eigengenes
  ord <- mod_order
  rowAnnotation(
    Eigengene = anno_block(
      width = unit(3.4, "cm"),
      show_name = FALSE,
      panel_fun = function(index, nm) {
        mod_name <- ord[as.integer(nm)]
        v <- as.numeric(eg[[mod_name]])
        if (length(v) < 2) return(invisible(NULL))
        rng <- range(v)
        y <- if (diff(rng) == 0) rep(0.5, length(v)) else (v - rng[1]) / diff(rng)
        x <- (seq_along(v) - 1) / max(length(v) - 1, 1)
        grid.lines(x, y, gp = gpar(col = mod_name, lwd = 2))
        grid.points(x, y, pch = 16, size = unit(0.6, "mm"), gp = gpar(col = mod_name))
      }
    ),
    show_annotation_name = FALSE
  )
})

# enrichment terms (right); map block index -> module name the same way.
right_enrich <- local({
  en <- enrich
  ord <- mod_order
  rowAnnotation(
    Enrichment = anno_block(
      width = unit(5.5, "cm"),
      show_name = FALSE,
      panel_fun = function(index, nm) {
        mod_name <- ord[as.integer(nm)]
        terms <- en[[mod_name]]
        if (is.null(terms) || length(terms) == 0) {
          grid.text("—", x = 0.02, y = 0.5, just = "left",
                    gp = gpar(col = "#9aa0a6", fontsize = font_size))
          return(invisible(NULL))
        }
        terms <- head(terms, top_n)
        n <- length(terms)
        for (i in seq_len(n)) {
          yy <- 1 - (i - 0.5) / n
          grid.text(terms[[i]]$term, x = 0.02, y = yy, just = "left",
                    gp = gpar(col = "#233b40", fontsize = font_size, lineheight = 0.85))
        }
      }
    ),
    show_annotation_name = FALSE
  )
})

ht <- Heatmap(
  z,
  name = "z-score",
  col = col_fun,
  row_split = module_factor,
  cluster_rows = TRUE,
  cluster_row_slices = FALSE,
  cluster_columns = FALSE,
  show_row_names = FALSE,
  show_column_names = TRUE,
  column_names_rot = 45,
  column_names_gp = gpar(fontsize = font_size),
  left_annotation = c(left_eig, left_mod),
  right_annotation = right_enrich,
  show_heatmap_legend = TRUE,
  border_gp = gpar(col = "grey40")
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
