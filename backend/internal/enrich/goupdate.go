package enrich

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/netutil"
)

const oboURL = "https://current.geneontology.org/ontology/go-basic.obo"

// CurrentVersion returns the data-version of the ontology in use (disk copy if
// present, otherwise the embedded one).
func CurrentVersion() (string, error) {
	g, err := LoadGO()
	if err != nil {
		return "", err
	}
	return g.DataVersion, nil
}

// LatestVersion fetches the go-basic.obo header and returns its data-version.
func LatestVersion() (string, error) {
	client := netutil.Client(30 * time.Second)
	req, _ := http.NewRequest(http.MethodGet, oboURL, nil)
	req.Header.Set("Range", "bytes=0-8191")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}
	ver := extractDataVersion(b)
	if ver == "" {
		return "", fmt.Errorf("data-version not found in response")
	}
	return ver, nil
}

// Update downloads the latest go-basic.obo, validates it, gzips it into the
// user data dir and invalidates the cached ontology.
func Update() (string, error) {
	client := netutil.Client(5 * time.Minute)
	resp, err := client.Get(oboURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	gz := buf.Bytes()

	// Validate before committing: parse and require a sane term count.
	g, err := parseOBO(gz)
	if err != nil {
		return "", err
	}
	if len(g.terms) < 35000 {
		return "", fmt.Errorf("validated GO terms too few: %d", len(g.terms))
	}

	path := goDataPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, gz, 0o644); err != nil {
		return "", err
	}
	ReloadGO()
	return g.DataVersion, nil
}

func extractDataVersion(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "data-version: ") {
			return strings.TrimSpace(line[len("data-version: "):])
		}
	}
	return ""
}
