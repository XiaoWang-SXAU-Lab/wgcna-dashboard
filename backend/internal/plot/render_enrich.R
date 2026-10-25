#!/usr/bin/env Rscript
# Faithful replication of clusterProfiler / DOSE `dotplot` for enrichment results.
suppressPackageStartupMessages({library(jsonlite); library(ggplot2)})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path  <- get_arg("--json")
mode       <- get_arg("--mode", "png")
out_path   <- get_arg("--out")
width      <- as.numeric(get_arg("--width", "9"))
height     <- as.numeric(get_arg("--height", "6"))
dpi        <- as.numeric(get_arg("--dpi", "150"))
x_var      <- get_arg("--x", "GeneRatio")
col_var    <- get_arg("--color-var", "p.adjust")
show_cat   <- as.integer(get_arg("--show-category", "10"))
size_min   <- as.numeric(get_arg("--size-min", "3"))
size_max   <- as.numeric(get_arg("--size-max", "8"))
low        <- get_arg("--low", "red")
high       <- get_arg("--high", "blue")
font_size  <- as.numeric(get_arg("--base-size", "12"))
title      <- get_arg("--title", "")
wrap       <- as.integer(get_arg("--wrap", "0"))

if (is.null(json_path) || is.null(out_path)) stop("usage: render_enrich.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyDataFrame = TRUE)
if (length(d) == 0 || nrow(d) == 0) stop("no enrichment rows")

# Truncate long term labels (clusterProfiler label_format default 30).
truncate_label <- function(x, n = 30) {
  x <- as.character(x)
  ifelse(nchar(x) > n, paste0(substr(x, 1, n), "..."), x)
}
wrap_label <- function(x, n) {
  x <- as.character(x)
  vapply(x, function(s) {
    if (nchar(s) <= n) return(s)
    starts <- seq(1, nchar(s), by = n)
    paste(substr(s, starts, starts + n - 1), collapse = "\n")
  }, character(1), USE.NAMES = FALSE)
}
if (!is.na(wrap) && wrap > 0) {
  d$Description <- wrap_label(d$description, wrap)
} else {
  d$Description <- truncate_label(d$description, 30)
}

if (x_var == "Count") {
  d$x_val <- as.numeric(d$count)
  x_lab <- "Count"
} else {
  d$x_val <- vapply(strsplit(as.character(d$gene_ratio), "/"),
                    function(s) as.numeric(s[1]) / as.numeric(s[2]), numeric(1))
  x_lab <- "GeneRatio"
}

d$col_val <- d[[col_var]]

if (!is.na(show_cat) && show_cat > 0 && nrow(d) > show_cat) {
  d <- d[seq_len(show_cat), , drop = FALSE]
}

# Most significant at top (reversed factor levels).
d$Description <- factor(d$Description, levels = rev(unique(d$Description)))

# clusterProfiler's theme_dose.
theme_dose <- function(font.size = 12) {
  theme(
    axis.text.x = element_text(colour = "black", size = font.size, vjust = 0.5),
    axis.text.y = element_text(colour = "black", size = font.size),
    axis.title.x = element_text(colour = "black", size = font.size),
    axis.title.y = element_text(colour = "black", size = font.size, angle = 90),
    plot.title = element_text(colour = "black", size = font.size, hjust = 0.5),
    legend.text = element_text(size = font.size, colour = "black"),
    legend.title = element_text(size = font.size, colour = "black"),
    panel.grid.major = element_line(colour = "grey90"),
    panel.grid.minor = element_blank(),
    panel.background = element_rect(fill = "white", colour = NA),
    panel.border = element_rect(colour = "grey80", fill = NA, linewidth = 0.5),
    axis.line = element_blank()
  )
}

p <- ggplot(d, aes(x = x_val, y = Description, size = count, color = col_val)) +
  geom_point() +
  scale_color_continuous(low = low, high = high, name = col_var,
                         guide = guide_colorbar(reverse = TRUE)) +
  scale_size(range = c(size_min, size_max)) +
  xlab(x_lab) + ylab(NULL) + ggtitle(title) +
  theme_dose(font_size)

if (mode == "pdf") {
  ggsave(out_path, p, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, p, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, p, width = width, height = height, dpi = dpi)
}
cat("done:", out_path, "\n")
