#!/usr/bin/env Rscript
# Clustering comparison: KMeans (base) or fuzzy c-means (e1071::cmeans).
# Emits a JSON with cluster assignment, cluster centers (like eigengenes),
# membership/score, and validity metrics (elbow WSS for kmeans; XB index +
# partition coefficient for c-means).
suppressPackageStartupMessages({
  library(jsonlite)
  library(e1071)
})

args <- commandArgs(trailingOnly = TRUE)
get_arg <- function(name, default = NULL) {
  i <- which(args == name)
  if (length(i) == 0) return(default)
  args[i + 1]
}

json_path <- get_arg("--json")
out_path  <- get_arg("--out")
method    <- get_arg("--method", "kmeans")   # kmeans | cmeans
k_val     <- get_arg("--k", NULL)            # NULL = auto (elbow / validity)
seed      <- as.integer(get_arg("--seed", "42"))
kmax      <- as.integer(get_arg("--kmax", "12"))

if (is.null(json_path) || is.null(out_path)) stop("usage: compute_clustering.R --json <f> --out <f>")

d <- fromJSON(json_path, simplifyVector = FALSE)
z <- matrix(as.numeric(unlist(d$z)), nrow = length(d$genes), byrow = TRUE)
rownames(z) <- unlist(d$genes)
colnames(z) <- unlist(d$samples)
genes <- unlist(d$genes)

set.seed(seed)

# ---- elbow (kmeans WSS) ----
elbow_kmeans <- function(mat, kmax) {
  ks <- 2:kmax
  wss <- vapply(ks, function(k) kmeans(mat, centers = k, nstart = 10, iter.max = 100)$tot.withinss, numeric(1))
  # elbow = point farthest from the line connecting endpoints
  line <- seq(wss[1], wss[length(wss)], length.out = length(wss))
  dist <- abs(wss - line)
  list(k = ks[which.max(dist)], wss = wss, ks = ks)
}

# ---- fuzzy c-means validity: Xie-Beni + partition coefficient ----
validity_cmeans <- function(mat, kmax) {
  ks <- 2:kmax
  xb <- numeric(length(ks)); pc <- numeric(length(ks))
  for (i in seq_along(ks)) {
    k <- ks[i]
    res <- cmeans(mat, centers = k, iter.max = 100, m = 2)
    U <- res$membership
    # partition coefficient
    pc[i] <- sum(U^2) / nrow(mat)
    # Xie-Beni: compactness / separation
    centers <- res$centers
    compact <- sum((mat - centers[max.col(U), ])^2)
    d2 <- as.matrix(dist(centers))^2
    diag(d2) <- Inf
    sep <- min(d2)
    xb[i] <- compact / (nrow(mat) * sep)
  }
  # k with minimum XB index
  list(k = ks[which.min(xb)], xb = xb, pc = pc, ks = ks)
}

if (method == "cmeans") {
  # fuzzy c-means
  if (!is.null(k_val)) {
    kk <- as.integer(k_val)
    metrics <- NULL
  } else {
    v <- validity_cmeans(z, kmax)
    kk <- v$k
    metrics <- v
  }
  res <- cmeans(z, centers = kk, iter.max = 100, m = 2)
  hard <- max.col(res$membership)
  cluster_names <- paste0("C", seq_len(kk))
  # order clusters by size (desc)
  sizes <- table(hard)
  order_idx <- as.integer(names(sort(sizes, decreasing = TRUE)))
  # relabel to C1..Ck by size
  new_label <- setNames(seq_along(order_idx), as.character(order_idx))
  hard <- unname(new_label[as.character(hard)])
  cluster_names <- paste0("C", seq_len(kk))
  # centers reordered
  centers <- res$centers[order_idx, , drop = FALSE]
  rownames(centers) <- cluster_names
  membership <- vapply(seq_len(nrow(z)), function(i) res$membership[i, order_idx[hard[i]]], numeric(1))
  score_name <- "membership"
} else {
  # kmeans
  if (!is.null(k_val)) {
    kk <- as.integer(k_val)
    metrics <- NULL
  } else {
    e <- elbow_kmeans(z, kmax)
    kk <- e$k
    metrics <- e
  }
  res <- kmeans(z, centers = kk, nstart = 25, iter.max = 100)
  hard <- res$cluster
  sizes <- table(hard)
  order_idx <- as.integer(names(sort(sizes, decreasing = TRUE)))
  new_label <- setNames(seq_along(order_idx), as.character(order_idx))
  hard <- unname(new_label[as.character(hard)])
  cluster_names <- paste0("C", seq_len(kk))
  centers <- res$centers[order_idx, , drop = FALSE]
  rownames(centers) <- cluster_names
  # score = 1 - (distance to own center / max distance)
  membership <- vapply(seq_len(nrow(z)), function(i) {
    d <- sqrt(sum((z[i, ] - centers[hard[i], ])^2))
    1 - min(d / 3, 1)  # loose "closeness" 0..1
  }, numeric(1))
  score_name <- "score"
}

# centers as named list (module -> vector per sample)
centers_list <- lapply(seq_len(kk), function(i) as.numeric(centers[i, ]))
names(centers_list) <- cluster_names

out <- list(
  genes = genes,
  samples = unlist(d$samples),
  z = d$z,
  clusters = paste0("C", hard),
  cluster_order = cluster_names,
  centers = centers_list,
  score = membership,
  score_name = score_name,
  method = method,
  k = kk,
  metrics = metrics
)
write(toJSON(out, auto_unbox = TRUE, digits = NA), out_path)
cat("done:", out_path, "\n")
