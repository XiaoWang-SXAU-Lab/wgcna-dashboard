#!/usr/bin/env Rscript
# Render the scale-free topology check figure:
#   left  = soft connectivity histogram (k vs frequency)
#   right = log-log regression (log10(k) vs log10(p(k))) + fitted line
#
# Optional styling args (with defaults):
#   --size --color --line-color --base-size --axis-size --axis-color --width --height
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

json_path   <- get_arg("--json")
mode        <- get_arg("--mode")
out         <- get_arg("--out")
size        <- as.numeric(get_arg("--size", "2.4"))
color       <- get_arg("--color", "#5470c6")
line_color  <- get_arg("--line-color", "#e11d48")
base_size   <- as.numeric(get_arg("--base-size", "13"))
axis_size   <- as.numeric(get_arg("--axis-size", base_size - 2))
axis_color  <- get_arg("--axis-color", "#333333")
width       <- as.numeric(get_arg("--width", "10"))
height      <- as.numeric(get_arg("--height", "4.6"))
dpi         <- as.numeric(get_arg("--dpi", "150"))

d <- fromJSON(json_path, simplifyVector = TRUE)
check <- d$scale_free_check
if (is.null(check)) stop("no scale_free_check field in sft result")

power     <- as.numeric(check$power)
mids      <- as.numeric(check$hist_mids)
counts    <- as.numeric(check$hist_counts)
log_k     <- as.numeric(check$log_k)
log_p     <- as.numeric(check$log_p)
icpt      <- as.numeric(check$intercept)
slp       <- as.numeric(check$slope)
adj_r2    <- as.numeric(check$adj_r2)

base_theme <- theme_classic(base_size = base_size) +
  theme(
    axis.title = element_text(size = base_size, color = axis_color),
    axis.text  = element_text(size = axis_size, color = axis_color),
    axis.line  = element_line(color = axis_color),
    axis.ticks = element_line(color = axis_color),
    plot.margin = margin(6, 10, 6, 6),
    plot.title = element_text(hjust = 0.5, size = base_size)
  )

# left: soft connectivity histogram
p1 <- ggplot(data.frame(k = mids, freq = counts), aes(k, freq)) +
  geom_col(fill = color, color = "#334155", linewidth = 0.3) +
  labs(x = "k", y = "Frequency",
       title = paste("Soft connectivity (power =", power, ")")) +
  base_theme

# right: log-log regression with fitted line
p2 <- ggplot(data.frame(x = log_k, y = log_p), aes(x, y)) +
  geom_point(size = size, color = color) +
  geom_abline(intercept = icpt, slope = slp, color = line_color, linewidth = 0.9) +
  labs(x = "log10(k)", y = "log10(p(k))",
       title = paste("Check Scale free topology\nscale R^2 =", round(adj_r2, 2), ", slope =", round(slp, 2))) +
  base_theme

combined <- p1 + p2

if (mode == "pdf") {
  ggsave(out, combined, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out, combined, width = width, height = height, device = "svg")
} else {
  ggsave(out, combined, width = width, height = height, dpi = dpi, device = "png")
}

cat("done:", out, "\n")
