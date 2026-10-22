#!/usr/bin/env Rscript
# Module eigengene adjacency heatmap (plotEigengeneNetworks style): top dendrogram
# (x-axis clustering only), left color band, no axis lines.
suppressPackageStartupMessages({
  library(jsonlite)
  library(ggplot2)
  library(patchwork)
  library(ggdendro)
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
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "8"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
low       <- get_arg("--low", "#313695")
mid       <- get_arg("--mid", "#ffffff")
high      <- get_arg("--high", "#a50026")
if (is.null(json_path) || is.null(out_path)) stop("usage: render_eigengene.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
modules <- d$modules
eg <- as.matrix(d$eigengenes)

corm <- cor(eg)
adj <- (1 + corm) / 2
hc <- hclust(as.dist(1 - adj), method = "average")
ord <- hc$order
mod_ord <- modules[ord]
adj_ord <- adj[ord, ord]
n <- length(modules)

heat <- expand.grid(j = seq_len(n), i = seq_len(n))
heat$adj <- mapply(function(ii, jj) adj_ord[ii, jj], heat$i, heat$j)
heat$x <- factor(mod_ord[heat$j], levels = mod_ord)
heat$y <- factor(mod_ord[heat$i], levels = rev(mod_ord))

# --- top dendrogram (x-axis clustering) ---
ddata <- ggdendro::dendro_data(as.dendrogram(hc))
p_dendro <- ggplot(ddata$segments) +
  geom_segment(aes(x = x, y = y, xend = xend, yend = yend), linewidth = 0.4) +
  scale_x_continuous(expand = c(0, 0), limits = c(0.5, n + 0.5)) +
  scale_y_continuous(expand = c(0, 0.05)) +
  theme_void() +
  theme(plot.margin = margin(6, 10, 0, 2))

blank <- ggplot() + theme_void()

# --- color band (left) ---
band <- data.frame(module = factor(mod_ord, levels = rev(mod_ord)), color = mod_ord)
p_band <- ggplot(band, aes(1, module)) +
  geom_tile(aes(fill = color), colour = "#333333", linewidth = 0.5) +
  scale_fill_identity() +
  scale_x_continuous(expand = c(0, 0)) +
  scale_y_discrete(expand = c(0, 0)) +
  theme_classic(base_size = 12) +
  theme(
    axis.title = element_blank(),
    axis.text.x = element_blank(),
    axis.ticks = element_blank(),
    axis.line = element_blank(),
    legend.position = "none",
    plot.margin = margin(0, 0, 6, 2)
  )

# --- heatmap (no y labels, x labels 90deg) ---
p_heat <- ggplot(heat, aes(x, y)) +
  geom_tile(aes(fill = adj), colour = NA) +
  scale_fill_gradient2(low = low, mid = mid, high = high, midpoint = 0.5, limits = c(0, 1)) +
  scale_x_discrete(expand = c(0, 0)) +
  scale_y_discrete(expand = c(0, 0)) +
  theme_classic(base_size = 12) +
  theme(
    axis.title = element_blank(),
    axis.text.x = element_text(angle = 90, hjust = 1, vjust = 0.5),
    axis.text.y = element_blank(),
    axis.ticks = element_blank(),
    axis.line = element_blank(),
    legend.position = "none",
    plot.margin = margin(0, 10, 6, 0)
  )

# --- assemble: [blank|dendro] on top, [band|heat] on bottom ---
top <- blank + p_dendro + plot_layout(widths = c(1, 8))
bottom <- p_band + p_heat + plot_layout(widths = c(1, 8))
combined <- top / bottom + plot_layout(heights = c(1, 5))

if (mode == "pdf") {
  ggsave(out_path, combined, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, combined, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, combined, width = width, height = height, dpi = dpi, device = "png")
}
cat("done:", out_path, "\n")
