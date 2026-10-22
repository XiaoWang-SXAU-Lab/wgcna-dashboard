#!/usr/bin/env Rscript
# Render WGCNA module dendrogram + module color band using BASE R,
# mirroring plotDendroAndColors: plot.hclust(hang=0.03) + rect color band.
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
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "6"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
if (is.null(json_path) || is.null(out_path)) stop("usage: render_dendrogram.R --json <f> --out <f>")

dat <- fromJSON(json_path)
merge <- dat$dendrogram$merge
hgts  <- dat$dendrogram$height
order <- dat$dendrogram$order   # 1-based leaf indices
colors <- dat$colors            # module color name per gene
n <- length(order)

hc <- structure(
  list(merge = merge, height = hgts, order = order,
       labels = as.character(seq_len(n)), method = "average",
       call = NULL, dist.method = "euclidean"),
  class = "hclust"
)

band_colors <- colors[order]  # module colors in leaf order

# Open the output device.
if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = "white")
}

# Layout: dendrogram (80%) + color band (20%).
colorHeight <- 0.2
layout(matrix(1:2, 2, 1), heights = c(1 - colorHeight, colorHeight))

# --- Dendrogram (base R plot.hclust with hanging leaves) ---
par(mar = c(0.5, 4.5, 2.5, 1))
plot(hc, hang = 0.03, labels = FALSE, axes = FALSE, frame.plot = FALSE,
     xlab = "", ylab = "Height", main = "", sub = "",
     xlim = c(0.5, n + 0.5), lwd = 0.4)
axis(2)

# --- Module color band ---
par(mar = c(2.5, 4.5, 0, 1))
plot(0, 0, type = "n", xlim = c(0.5, n + 0.5), ylim = c(0, 1),
     axes = FALSE, xlab = "", ylab = "", frame.plot = FALSE)
for (i in 1:n) {
  rect(i - 0.5, 0, i + 0.5, 1, col = band_colors[i], border = band_colors[i])
}
# top + bottom + outer border of the color band
rect(0.5, 0, n + 0.5, 1, col = NA, border = "black", lwd = 1)
mtext("Module color", side = 2, line = 3, las = 0, cex = 0.9)

dev.off()
cat("done:", out_path, "\n")
