package regutrace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ToolStatus is the path-free status of one bundled external tool.
type ToolStatus struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	License string `json:"license,omitempty"`
	Status  string `json:"status"` // ok | missing | checksum_mismatch | error
	Error   string `json:"error,omitempty"`
}

type toolsManifestEntry struct {
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	License string `json:"license"`
}

type toolsManifest struct {
	Schema int                           `json:"schema"`
	Tools  map[string]toolsManifestEntry `json:"tools"`
	Files  map[string]string             `json:"files,omitempty"`
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func safeBundleFile(root, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(filepath.Clean(rel), "..") {
		return "", fmt.Errorf("invalid bundled tool path %q", rel)
	}
	return filepath.Join(root, rel), nil
}

// VerifyBundledTools validates the build-generated tools/manifest.json and the
// SHA-256 of every bundled executable/DLL. An empty root means the developer
// build has no bundled tools; the pipeline may still use its manifest/PATH.
func VerifyBundledTools(root string) ([]ToolStatus, map[string]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("read bundled tools manifest: %w", err)
	}
	var manifest toolsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, fmt.Errorf("parse bundled tools manifest: %w", err)
	}
	if manifest.Schema != 1 {
		return nil, nil, fmt.Errorf("unsupported bundled tools manifest schema %d", manifest.Schema)
	}
	paths := map[string]string{}
	statuses := make([]ToolStatus, 0, len(manifest.Tools))
	for name, entry := range manifest.Tools {
		status := ToolStatus{Name: name, Version: entry.Version, License: entry.License, Status: "ok"}
		path, err := safeBundleFile(root, entry.File)
		if err != nil {
			status.Status = "error"
			status.Error = err.Error()
			statuses = append(statuses, status)
			continue
		}
		actual, err := fileSHA256(path)
		switch {
		case err != nil:
			status.Status = "missing"
			status.Error = err.Error()
		case !strings.EqualFold(actual, strings.TrimSpace(entry.SHA256)):
			status.Status = "checksum_mismatch"
			status.Error = "SHA-256 does not match tools/manifest.json"
		default:
			paths[name] = path
		}
		statuses = append(statuses, status)
	}
	for rel, want := range manifest.Files {
		path, err := safeBundleFile(root, rel)
		if err != nil {
			return statuses, paths, err
		}
		actual, err := fileSHA256(path)
		if err != nil {
			return statuses, paths, fmt.Errorf("%s: %w", rel, err)
		}
		if !strings.EqualFold(actual, strings.TrimSpace(want)) {
			return statuses, paths, fmt.Errorf("%s: SHA-256 does not match tools/manifest.json", rel)
		}
	}
	for _, required := range []string{"hmmsearch", "blastp"} {
		if _, ok := paths[required]; !ok {
			return statuses, paths, fmt.Errorf("required bundled tool %q is missing or unverified", required)
		}
	}
	return statuses, paths, nil
}
