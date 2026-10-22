#!/usr/bin/env Rscript
# Module gene expression heatmap (z-scored) + eigengene barplot (moduleheatmap style).
suppressPackageStartupMessages({
  library(jsonlite)
  library(ggplot2)
  library(patchwork)
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
low       <- get_arg("--low", "green")
mid       <- get_arg("--mid", "black")
high      <- get_arg("--high", "red")
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "8"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
if (is.null(json_path) || is.null(out_path)) stop("usage: render_geneheatmap.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
mod <- d$module
genes <- d$genes
samples <- d$samples
expr <- as.matrix(d$expression)   # genes x samples
eg <- d$eigengene                 # samples
if (is.null(samples) || length(samples) == 0) {
  samples <- paste0("S", seq_len(ncol(expr)))
}

# z-score each gene (row) across samples
z <- t(scale(t(expr)))

heat <- expand.grid(sample = seq_along(samples), gene = seq_along(genes))
heat$z <- mapply(function(gi, si) z[gi, si], heat$gene, heat$sample)
heat$sample_lab <- factor(samples[heat$sample], levels = samples)
heat$gene_lab <- factor(genes[heat$gene], levels = rev(genes))

p1 <- ggplot(heat, aes(sample_lab, gene_lab)) +
  geom_tile(aes(fill = z), colour = NA) +
  scale_fill_gradient2(low = low, mid = mid, high = high) +
  theme_classic(base_size = 12) +
  theme(legend.position = "none", axis.title = element_blank(),
        axis.text.y = element_blank(),
        axis.text.x = element_text(angle = 90, vjust = 0.5, hjust = 1, size = 8),
        axis.line.y = element_blank(), axis.ticks.y = element_blank(),
        axis.line.x = element_blank(), axis.ticks.x = element_blank(),
        plot.margin = margin(6, 10, 0, 2))

bar <- data.frame(Sample = factor(samples, levels = samples), Eigengene = eg)
bar_fill <- if (mod %in% colors()) mod else "#2563eb"
p2 <- ggplot(bar, aes(x = Sample, y = Eigengene)) +
  geom_bar(stat = "identity", fill = bar_fill) +
  theme_classic(base_size = 12) +
  theme(axis.title = element_blank(), axis.line = element_blank(),
        axis.text = element_blank(), axis.ticks = element_blank(),
        plot.margin = margin(0, 10, 6, 2))

combined <- p1 / p2 + plot_layout(heights = c(6, 1))

if (mode == "pdf") {
  ggsave(out_path, combined, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, combined, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, combined, width = width, height = height, dpi = dpi, device = "png")
}
cat("done:", out_path, "\n")
