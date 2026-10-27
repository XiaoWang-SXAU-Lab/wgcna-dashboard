#!/usr/bin/env Rscript
# Gene-level module network (overview). Nodes = all network genes colored by
# module, sized by degree; edges colored by weight. Uses igraph's DrL layout
# (designed for large graphs). Renders SVG/PDF/PNG.
suppressPackageStartupMessages({
  library(jsonlite)
  library(igraph)
  library(circlize)
})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
out_path  <- get_arg("--out")
mode      <- get_arg("--mode", "png")
width     <- as.numeric(get_arg("--width", "12"))
height    <- as.numeric(get_arg("--height", "12"))
dpi       <- as.numeric(get_arg("--dpi", "150"))
node_min  <- as.numeric(get_arg("--node-min", "0.4"))
node_max  <- as.numeric(get_arg("--node-max", "4"))
edge_low  <- get_arg("--edge-low", "#e0e7ef")
edge_high <- get_arg("--edge-high", "#286e70")
bg        <- get_arg("--bg", "#ffffff")

if (is.null(json_path) || is.null(out_path)) stop("usage: render_gene_network.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = FALSE)
nodes <- d$nodes
edges <- d$edges
if (length(nodes) == 0) stop("no nodes")

ids  <- vapply(nodes, function(x) x$id, character(1))
colr <- vapply(nodes, function(x) x$color, character(1))
deg  <- as.numeric(vapply(nodes, function(x) if (is.null(x$degree)) 1 else x$degree, numeric(1)))

g <- make_empty_graph(n = length(ids), directed = FALSE)
V(g)$name <- ids
if (length(edges) > 0) {
  src <- match(vapply(edges, function(x) x$source, character(1)), ids)
  tgt <- match(vapply(edges, function(x) x$target, character(1)), ids)
  w   <- as.numeric(vapply(edges, function(x) if (is.null(x$weight)) 0 else x$weight, numeric(1)))
  keep <- !is.na(src) & !is.na(tgt)
  g <- add_edges(g, as.vector(rbind(src[keep], tgt[keep])))
  E(g)$weight <- w[keep]
}

set.seed(42)
lay <- if (vcount(g) >= 20) {
  tryCatch(layout_with_drl(g), error = function(e) layout_with_fr(g))
} else {
  layout_with_fr(g)
}

# node size from degree
if (diff(range(deg)) == 0) {
  vsize <- rep((node_min + node_max) / 2, length(deg))
} else {
  vsize <- node_min + (deg - min(deg)) / diff(range(deg)) * (node_max - node_min)
}

# edge color + width from weight
ecol <- character(0); ewidth <- numeric(0)
if (ecount(g) > 0) {
  w <- E(g)$weight
  wr <- range(w)
  pal <- colorRampPalette(c(edge_low, edge_high))(100)
  edge_col_fun <- colorRamp2(seq(wr[1], wr[2], length.out = 100), pal)
  ecol <- edge_col_fun(w)
  ewidth <- if (diff(wr) == 0) rep(1, length(w)) else 0.2 + (w - wr[1]) / diff(wr) * 2.3
}

if (mode == "pdf") {
  pdf(out_path, width = width, height = height)
} else if (mode == "svg") {
  svg(out_path, width = width, height = height)
} else {
  png(out_path, width = width, height = height, units = "in", res = dpi, bg = bg)
}
par(mar = c(0, 0, 0, 0), bg = bg)
plot(g, layout = lay, vertex.size = vsize, vertex.color = colr,
     vertex.frame.color = adjustcolor(colr, alpha.f = 0.6), vertex.label = NA,
     edge.color = ecol, edge.width = ewidth, edge.arrow.mode = 0)
dev.off()
cat("done:", out_path, "\n")
