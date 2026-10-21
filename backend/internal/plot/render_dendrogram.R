#!/usr/bin/env Rscript
# Render the WGCNA module dendrogram (plotDendroAndColors style).
#   static      -> ggplot2 PNG
#   interactive -> plotly (ggplotly) self-contained HTML
suppressPackageStartupMessages({
  library(jsonlite)
  library(ggplot2)
  library(ggdendro)
})

args <- commandArgs(trailingOnly = TRUE)
getarg <- function(name) {
  i <- match(name, args)
  if (is.na(i)) return(NULL)
  args[i + 1]
}
json_path <- getarg("--json")
mode <- getarg("--mode")
out_path <- getarg("--out")
if (is.null(mode)) mode <- "static"
if (is.null(json_path) || is.null(out_path)) stop("usage: render_dendrogram.R --json <f> --mode static|interactive --out <f>")

dat <- fromJSON(json_path)
merge <- dat$dendrogram$merge
height <- dat$dendrogram$height
order <- dat$dendrogram$order   # 1-based leaf indices
colors <- dat$colors            # module color name per gene
n <- length(order)

# Reconstruct hclust; labels are 1-based gene indices (match order).
hc <- structure(
  list(merge = merge, height = height, order = order,
       labels = as.character(seq_len(n)), method = "average",
       call = NULL, dist.method = "euclidean"),
  class = "hclust"
)

ddata <- dendro_data(as.dendrogram(hc))
seg <- ddata$segments
lbl <- ddata$labels

maxH <- max(height)
barH <- 0.25 * maxH   # plotDendroAndColors: color band = 20% of total (4:1)

band <- data.frame(
  x = lbl$x,
  y = -barH / 2,
  col = colors[as.integer(lbl$label)]
)

p <- ggplot() +
  geom_segment(data = seg, aes(x = x, y = y, xend = xend, yend = yend),
               colour = "black", linewidth = 0.3) +
  geom_tile(data = band, aes(x = x, y = y, fill = col), width = 1, height = barH) +
  scale_fill_identity() +
  coord_cartesian(xlim = c(0.5, n + 0.5), ylim = c(-barH, maxH * 1.02), expand = FALSE) +
  theme_void() +
  theme(plot.margin = margin(12, 12, 12, 12))

if (identical(mode, "interactive")) {
  suppressPackageStartupMessages(library(plotly))
  pp <- ggplotly(p, tooltip = "all")
  htmlwidgets::saveWidget(pp, out_path, selfcontained = TRUE)
} else {
  ggsave(out_path, p, width = 10, height = 6, dpi = 140, bg = "white")
}
cat("done:", out_path, "\n")
