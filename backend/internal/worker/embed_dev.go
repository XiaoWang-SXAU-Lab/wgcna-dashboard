//go:build !release

package worker

// empty in dev; the worker binary is supplied via WGCNA_WORKER instead.
var binary []byte
