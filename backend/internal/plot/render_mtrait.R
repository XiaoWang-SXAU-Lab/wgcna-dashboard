#!/usr/bin/env Rscript
# Module-trait correlation heatmap with left module color band (ggplot2).
# Color band sits right next to the heatmap; module labels on its left; no axes.
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
low       <- get_arg("--low", "#4575b4")
mid       <- get_arg("--mid", "#ffffff")
high      <- get_arg("--high", "#d73027")
width     <- as.numeric(get_arg("--width", "10"))
height    <- as.numeric(get_arg("--height", "8"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
if (is.null(json_path) || is.null(out_path)) stop("usage: render_mtrait.R --json <f> --out <f>")

pstars <- function(p) {
  if (is.na(p)) return("")
  if (p < 0.001) return("***")
  if (p < 0.01) return("**")
  if (p < 0.05) return("*")
  ""
}

d <- fromJSON(json_path, simplifyVector = TRUE)
modules <- d$modules
traits  <- d$traits
cor     <- as.matrix(d$cor)
pval    <- as.matrix(d$p)

heat <- expand.grid(t = seq_along(traits), m = seq_along(modules))
heat$cor <- mapply(function(mm, tt) cor[mm, tt], heat$m, heat$t)
heat$pv  <- mapply(function(mm, tt) pval[mm, tt], heat$m, heat$t)
heat$trait  <- factor(traits[heat$t], levels = traits)
heat$module <- factor(modules[heat$m], levels = rev(modules))
heat$label  <- paste0(sprintf("%.2f", heat$cor), "\n", mapply(pstars, heat$pv))

band <- data.frame(module = factor(modules, levels = rev(modules)), color = modules)

# color band WITH module labels on its left
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
    plot.margin = margin(6, 0, 6, 2)
  )

# heatmap WITHOUT y labels, no axis lines
p_heat <- ggplot(heat, aes(trait, module)) +
  geom_tile(aes(fill = cor), colour = "#333333", linewidth = 0.6) +
  geom_text(aes(label = label), size = 2.8, lineheight = 0.8) +
  scale_fill_gradient2(low = low, mid = mid, high = high, limits = c(-1, 1), midpoint = 0) +
  scale_x_discrete(expand = c(0, 0)) +
  scale_y_discrete(expand = c(0, 0)) +
  theme_classic(base_size = 12) +
  theme(
    axis.title = element_blank(),
    axis.text.x = element_text(angle = 45, hjust = 1, vjust = 1),
    axis.text.y = element_blank(),
    axis.ticks = element_blank(),
    axis.line = element_blank(),
    legend.position = "none",
    plot.margin = margin(6, 10, 6, 0)
  )

combined <- p_band + p_heat + plot_layout(widths = c(1, 8))

if (mode == "pdf") {
  ggsave(out_path, combined, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, combined, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, combined, width = width, height = height, dpi = dpi, device = "png")
}
cat("done:", out_path, "\n")
