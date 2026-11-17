package regutrace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var packRequiredKeys = []string{"hmm", "rules", "reference_map", "reference_domains", "dbd_models", "motifs"}

// ReferencePackInfo is the safe, path-free description of a server-side
// reference pack. The key is the directory name under the pack root.
type ReferencePackInfo struct {
	Key         string            `json:"key"`
	Version     string            `json:"version"`
	Species     string            `json:"species,omitempty"`
	Assembly    string            `json:"assembly,omitempty"`
	MotifScope  string            `json:"motif_scope,omitempty"`
	Tools       map[string]string `json:"tools,omitempty"`
	PromoterLen int               `json:"promoter_len,omitempty"`
	PValue      float64           `json:"pvalue,omitempty"`
	GeneBiotype string            `json:"gene_biotype,omitempty"`
}

type packManifestDefaults struct {
	PromoterLen *int     `json:"promoter_len"`
	PValue      *float64 `json:"pvalue"`
	GeneBiotype *string  `json:"gene_biotype"`
}

type packManifest struct {
	PackVersion string               `json:"pack_version"`
	Species     string               `json:"species"`
	Assembly    string               `json:"assembly"`
	MotifScope  string               `json:"motif_scope"`
	Files       map[string]string    `json:"files"`
	Checksums   map[string]string    `json:"checksums"`
	Tools       map[string]string    `json:"tools"`
	Defaults    packManifestDefaults `json:"defaults"`
}

type referencePack struct {
	Info       ReferencePackInfo
	Dir        string
	MotifsPath string
	manifest   packManifest
}

func loadPackManifest(dir string) (packManifest, error) {
	var manifest packManifest
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse manifest: %w", err)
	}
	if strings.TrimSpace(manifest.PackVersion) == "" {
		return manifest, errors.New("pack manifest has no pack_version")
	}
	for _, key := range packRequiredKeys {
		rel := strings.TrimSpace(manifest.Files[key])
		if rel == "" {
			return manifest, fmt.Errorf("pack manifest is missing files.%s", key)
		}
		if filepath.IsAbs(rel) || strings.HasPrefix(filepath.Clean(rel), "..") {
			return manifest, fmt.Errorf("pack files.%s must be relative", key)
		}
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			return manifest, fmt.Errorf("pack file %s: %w", rel, err)
		}
	}
	return manifest, nil
}

func packInfo(key string, manifest packManifest) ReferencePackInfo {
	info := ReferencePackInfo{
		Key:        key,
		Version:    manifest.PackVersion,
		Species:    manifest.Species,
		Assembly:   manifest.Assembly,
		MotifScope: manifest.MotifScope,
		Tools:      manifest.Tools,
	}
	if manifest.Defaults.PromoterLen != nil {
		info.PromoterLen = *manifest.Defaults.PromoterLen
	}
	if manifest.Defaults.PValue != nil {
		info.PValue = *manifest.Defaults.PValue
	}
	if manifest.Defaults.GeneBiotype != nil {
		info.GeneBiotype = *manifest.Defaults.GeneBiotype
	}
	return info
}

// ListReferencePacks scans one directory of versioned reference packs. Invalid
// or incomplete packs are skipped; pipeline will perform the full SHA-256 check.
func ListReferencePacks(root string) []ReferencePackInfo {
	if root == "" {
		return []ReferencePackInfo{}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return []ReferencePackInfo{}
	}
	out := make([]ReferencePackInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		key := entry.Name()
		dir := filepath.Join(root, key)
		manifest, err := loadPackManifest(dir)
		if err != nil {
			continue
		}
		out = append(out, packInfo(key, manifest))
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Key < out[k].Key })
	return out
}

// ResolveReferencePack resolves a pack key under root and validates the layout.
func ResolveReferencePack(root, key string) (*referencePack, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("reference pack key is required")
	}
	if key != filepath.Base(key) || strings.ContainsAny(key, `/\`) {
		return nil, errors.New("invalid reference pack key")
	}
	if root == "" {
		return nil, errors.New("no reference pack directory configured")
	}
	dir := filepath.Join(root, key)
	manifest, err := loadPackManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("reference pack %q: %w", key, err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if rel, err := filepath.Rel(rootAbs, dirAbs); err != nil || strings.HasPrefix(rel, "..") {
		return nil, errors.New("reference pack resolves outside the pack directory")
	}
	return &referencePack{
		Info:       packInfo(key, manifest),
		Dir:        dir,
		MotifsPath: filepath.Join(dir, manifest.Files["motifs"]),
		manifest:   manifest,
	}, nil
}
