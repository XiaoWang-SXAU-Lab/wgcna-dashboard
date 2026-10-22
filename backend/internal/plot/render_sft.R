#!/usr/bin/env Rscript
# Render the soft-threshold (SFT) plot as a publication-quality figure:
#   left  = scale-free fit index (signed R²) vs soft threshold power
#   right = mean connectivity vs soft threshold power
#
# Optional styling args (with defaults):
#   --size       point size
#   --color      point/line color (hex)
#   --base-size  global font size (pt)
#   --width      output width (inches)
#   --height     output height (inches)
# Output: PDF (vector), SVG (vector), or PNG (preview, 150 dpi).

suppressPackageStartupMessages({
  library(jsonlite)
  library(ggplot2)
  library(patchwork)
})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) {
    if (is.null(default)) stop(paste("missing argument:", name))
    return(default)
  }
  args[i + 1]
}

json_path <- get_arg("--json")
mode      <- get_arg("--mode")
out       <- get_arg("--out")
rsq_cut   <- as.numeric(get_arg("--rsq-cut", "0.85"))
size      <- as.numeric(get_arg("--size", "2.4"))
color     <- get_arg("--color", "#5470c6")
base_size <- as.numeric(get_arg("--base-size", "13"))
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "4.6"))

d <- fromJSON(json_path, simplifyVector = TRUE)
fit <- d$fit_indices

power  <- as.numeric(fit$power)
rsq    <- as.numeric(fit$rsq)
mean_k <- as.numeric(fit$mean_k)

base_theme <- theme_classic(base_size = base_size) +
  theme(
    axis.title = element_text(size = base_size),
    axis.text  = element_text(size = base_size - 2, color = "black"),
    plot.margin = margin(6, 10, 6, 6)
  )

p1 <- ggplot(data.frame(power = power, rsq = rsq), aes(power, rsq)) +
  geom_point(size = size, color = color) +
  geom_line(linewidth = 0.8, color = color) +
  geom_hline(yintercept = rsq_cut, linetype = "dashed", color = "#e11d48", linewidth = 0.7) +
  scale_x_continuous(breaks = power) +
  labs(
    x = "Soft Threshold (power)",
    y = expression(paste("Scale Free Topology Model Fit, signed ", R^2))
  ) +
  base_theme

p2 <- ggplot(data.frame(power = power, mean_k = mean_k), aes(power, mean_k)) +
  geom_point(size = size, color = color) +
  geom_line(linewidth = 0.8, color = color) +
  scale_x_continuous(breaks = power) +
  labs(x = "Soft Threshold (power)", y = "Mean Connectivity") +
  base_theme

combined <- p1 + p2

if (mode == "pdf") {
  ggsave(out, combined, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out, combined, width = width, height = height, device = "svg")
} else {
  ggsave(out, combined, width = width, height = height, dpi = 150, device = "png")
}

cat("done:", out, "\n")
