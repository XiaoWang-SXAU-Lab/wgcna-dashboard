//go:build release

package worker

import _ "embed"

//go:embed worker-bin
var binary []byte
