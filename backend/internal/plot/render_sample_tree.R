#!/usr/bin/env Rscript
# Render the sample clustering dendrogram with optional color bands (one per
# selected sample attribute), mirroring WGCNA's plotDendroAndColors.
suppressPackageStartupMessages(library(jsonlite))

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
mode      <- get_arg("--mode", "png")
out_path  <- get_arg("--out")
width     <- as.numeric(get_arg("--width", "12"))
height    <- as.numeric(get_arg("--height", "8"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
base_size <- as.numeric(get_arg("--base-size", "10"))
line_col  <- get_arg("--line-color", "#111827")

if (is.null(json_path) || is.null(out_path)) stop("usage: render_sample_tree.R --json <f> --out <f>")

dat <- fromJSON(json_path, simplifyVector = FALSE)
merge  <- do.call(rbind, dat$tree$merge)
tree_height <- unlist(dat$tree$height)
order  <- unlist(dat$tree$order)
samples <- unlist(dat$samples)
n <- length(order)

hc <- structure(
  list(merge = merge, height = tree_height, order = order,
       labels = as.character(seq_len(n)), method = "average",
       call = NULL, dist.method = "euclidean"),
  class = "hclust"
)

bands <- dat$bands
nb <- length(bands)
PAL <- c("#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b",
         "#e377c2", "#7f7f7f", "#bcbd22", "#17becf", "#aec7e8", "#ffbb78")

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = "white")
}

n_panels <- 1 + nb + ifelse(nb > 0, 1, 0)
if (nb > 0) {
  heights <- c(0.56, rep(0.30 / nb, nb), 0.14)
} else {
  heights <- 1
}
layout(matrix(seq_len(n_panels), n_panels, 1), heights = heights)

par(mar = c(0.5, 7, 2, 1))
par(col = line_col)
plot(hc, hang = 0.03, labels = FALSE, axes = FALSE, xlab = "", ylab = "Height",
     main = "", sub = "", xlim = c(0.5, n + 0.5), lwd = 0.5)
axis(2)

leaf_samples <- samples[order]

for (bi in seq_len(nb)) {
  b <- bands[[bi]]
  vals <- unlist(b$values)
  vv <- vals[leaf_samples]
  u <- sort(unique(vv))
  colvec <- PAL[match(vv, u)]

  par(mar = c(0.1, 7, 0.1, 1))
  par(col = "#111827")
  plot(0, 0, type = "n", xlim = c(0.5, n + 0.5), ylim = c(0, 1),
       axes = FALSE, xlab = "", ylab = "", frame.plot = FALSE)
  mtext(b$name, side = 2, line = 1.5, las = 0, cex = base_size / 12)
  for (i in seq_len(n)) {
    rect(i - 0.5, 0, i + 0.5, 1, col = colvec[i], border = colvec[i])
  }
  rect(0.5, 0, n + 0.5, 1, col = NA, border = "black", lwd = 0.8)
}

if (nb > 0) {
  par(mar = c(0.5, 2, 0.5, 1))
  plot(0, 0, type = "n", xlim = c(0, 1), ylim = c(0.2, nb + 0.8),
       axes = FALSE, xlab = "", ylab = "", frame.plot = FALSE)
  for (bi in seq_len(nb)) {
    b <- bands[[bi]]
    vals <- unlist(b$values)
    vv <- vals[leaf_samples]
    u <- sort(unique(vv))
    legend(0, nb - bi + 0.5, legend = u, fill = PAL[seq_along(u)],
           bty = "n", horiz = TRUE, cex = base_size / 14, title = b$name, title.adj = 0)
  }
}

dev.off()
cat("done:", out_path, "\n")
