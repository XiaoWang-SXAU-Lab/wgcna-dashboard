// Package netutil provides proxy-aware HTTP clients shared by the enrich,
// literature and AI subsystems, so outbound calls honour HTTP(S)_PROXY and an
// optional WGCNA_PROXY override (e.g. "http://127.0.0.1:7890").
package netutil

import (
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client returns an http.Client that uses the standard proxy environment plus
// an optional WGCNA_PROXY override.
func Client(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if p := os.Getenv("WGCNA_PROXY"); p != "" {
		if u, err := url.Parse(p); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
