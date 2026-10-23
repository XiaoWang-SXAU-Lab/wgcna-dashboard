#!/usr/bin/env Rscript
# Render a clusterProfiler-style dot plot for GO/KEGG enrichment results.
suppressPackageStartupMessages({library(jsonlite); library(ggplot2)})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
mode      <- get_arg("--mode", "png")
out_path  <- get_arg("--out")
width     <- as.numeric(get_arg("--width", "9"))
height    <- as.numeric(get_arg("--height", "6"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
low       <- get_arg("--low", "#2166ac")
high      <- get_arg("--high", "#b2182b")
base_size <- as.numeric(get_arg("--base-size", "12"))
title     <- get_arg("--title", "Enrichment")

if (is.null(json_path) || is.null(out_path)) stop("usage: render_enrich.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyDataFrame = TRUE)
if (length(d) == 0 || nrow(d) == 0) stop("no enrichment rows")

d$gene_ratio_num <- vapply(strsplit(as.character(d$gene_ratio), "/"),
                           function(x) as.numeric(x[1]) / as.numeric(x[2]), numeric(1))
d$Description <- factor(d$description, levels = rev(d$description))
d$padj <- d[["p.adjust"]]

p <- ggplot(d, aes(x = gene_ratio_num, y = Description, size = count, color = padj)) +
  geom_point() +
  scale_color_gradient(low = low, high = high, name = "p.adjust") +
  labs(x = "GeneRatio", y = "", title = title) +
  theme_bw(base_size = base_size) +
  theme(axis.text.y = element_text(size = base_size - 1))

if (mode == "pdf") {
  ggsave(out_path, p, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, p, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, p, width = width, height = height, dpi = dpi)
}
cat("done:", out_path, "\n")
