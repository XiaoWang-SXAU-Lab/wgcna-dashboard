#!/usr/bin/env Rscript
# GS vs MM scatter (getverboseplot style): |MM| vs |GS|, module-colored points + lm line.
suppressPackageStartupMessages({
  library(jsonlite)
  library(ggplot2)
})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path  <- get_arg("--json")
mode       <- get_arg("--mode", "png")
out_path   <- get_arg("--out")
size       <- as.numeric(get_arg("--size", "2.4"))
line_color <- get_arg("--line-color", "#e11d48")
base_size  <- as.numeric(get_arg("--base-size", "13"))
axis_size  <- as.numeric(get_arg("--axis-size", "11"))
width      <- as.numeric(get_arg("--width", "8"))
height     <- as.numeric(get_arg("--height", "6"))
dpi        <- as.numeric(get_arg("--dpi", "150"))
if (is.null(json_path) || is.null(out_path)) stop("usage: render_gsmm.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = TRUE)
mod <- d$module
trait <- d$trait_name
df <- data.frame(mm = abs(d$mm), gs = abs(d$gs))
point_col <- if (mod %in% colors()) mod else "#2563eb"

fit <- lm(gs ~ mm, data = df)
corv <- cor(df$mm, df$gs)
pv <- cor.test(df$mm, df$gs)$p.value

p <- ggplot(df, aes(mm, gs)) +
  geom_point(color = point_col, size = size, alpha = 0.6) +
  geom_smooth(method = "lm", se = TRUE, color = line_color, fill = "grey60", alpha = 0.3, linewidth = 1) +
  labs(x = "Module Membership (|MM|)", y = "Gene Significance (|GS|)",
       title = paste0("GS vs MM (", mod, " / ", trait, ")"),
       subtitle = paste0("cor = ", sprintf("%.3f", corv), "  p = ", format(pv, scientific = TRUE, digits = 3))) +
  theme_classic(base_size = base_size) +
  theme(axis.text = element_text(size = axis_size),
        plot.title = element_text(hjust = 0.5, size = 14),
        plot.subtitle = element_text(hjust = 0.5, size = 12, color = "grey40"))

if (mode == "pdf") {
  ggsave(out_path, p, width = width, height = height, device = "pdf")
} else if (mode == "svg") {
  ggsave(out_path, p, width = width, height = height, device = "svg")
} else {
  ggsave(out_path, p, width = width, height = height, dpi = dpi, device = "png")
}
cat("done:", out_path, "\n")
