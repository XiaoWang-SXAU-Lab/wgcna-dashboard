//go:build release

package web

import "embed"

//go:embed all:dist
var distFS embed.FS

const embedded = true
