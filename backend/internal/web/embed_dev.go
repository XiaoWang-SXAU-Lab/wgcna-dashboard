//go:build !release

package web

import "embed"

var distFS embed.FS

const embedded = false
