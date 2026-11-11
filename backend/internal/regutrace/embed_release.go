//go:build release

package regutrace

import _ "embed"

//go:embed regutrace-bin
var binary []byte
