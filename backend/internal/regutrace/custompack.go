package regutrace

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxCustomPackFiles = 5000

// unpackCustomPack saves a user-supplied pack ZIP, extracts it below the job
// directory, and verifies the manifest and required file checksums. The ZIP is
// deliberately treated as untrusted input: no absolute paths, parent escapes,
// symlinks, or duplicate required entries are accepted.
func unpackCustomPack(jobDir string, up *Upload) (*referencePack, error) {
	zipPath, _, err := saveUpload(jobDir, "custom-pack.zip", up)
	if err != nil {
		return nil, fmt.Errorf("save custom pack: %w", err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open custom pack ZIP: %w", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		return nil, errors.New("custom pack ZIP is empty")
	}
	if len(zr.File) > maxCustomPackFiles {
		return nil, fmt.Errorf("custom pack ZIP has too many entries (%d)", len(zr.File))
	}
	root := filepath.Join(jobDir, "custom-pack")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var extracted int64
	for _, entry := range zr.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		if strings.HasPrefix(name, "/") || strings.Contains(name, "\x00") {
			return nil, fmt.Errorf("custom pack contains an unsafe path %q", entry.Name)
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
			clean == ".." || filepath.IsAbs(clean) {
			return nil, fmt.Errorf("custom pack contains an unsafe path %q", entry.Name)
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("custom pack contains a symlink %q", entry.Name)
		}
		if seen[clean] {
			return nil, fmt.Errorf("custom pack contains a duplicate entry %q", entry.Name)
		}
		seen[clean] = true
		target := filepath.Join(root, clean)
		if !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return nil, fmt.Errorf("custom pack path escapes its directory: %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		src, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open custom pack entry %q: %w", entry.Name, err)
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = src.Close()
			return nil, err
		}
		written, copyErr := io.Copy(dst, io.LimitReader(src, 512<<20+1))
		closeDstErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("extract custom pack entry %q: %w", entry.Name, copyErr)
		}
		if written > 512<<20 {
			return nil, fmt.Errorf("custom pack entry %q is larger than 512 MB", entry.Name)
		}
		extracted += written
		if extracted > 4<<30 {
			return nil, errors.New("custom pack is larger than 4 GB after extraction")
		}
		if closeDstErr != nil {
			return nil, closeDstErr
		}
	}
	packRoot := root
	if _, err := os.Stat(filepath.Join(packRoot, "manifest.json")); err != nil {
		// Common user mistake: zipping the pack folder itself. Accept exactly
		// one top-level directory that contains manifest.json.
		entries, readErr := os.ReadDir(root)
		if readErr == nil {
			var dirs []string
			for _, entry := range entries {
				if entry.IsDir() {
					dirs = append(dirs, entry.Name())
				}
			}
			if len(dirs) == 1 {
				candidate := filepath.Join(root, dirs[0])
				if _, statErr := os.Stat(filepath.Join(candidate, "manifest.json")); statErr == nil {
					packRoot = candidate
				}
			}
		}
	}
	manifest, err := loadPackManifest(packRoot)
	if err != nil {
		return nil, fmt.Errorf("custom pack: %w", err)
	}
	if err := verifyCustomPackChecksums(packRoot, manifest); err != nil {
		return nil, err
	}
	info := packInfo("custom", manifest)
	info.Key = "custom"
	return &referencePack{
		Info:       info,
		Dir:        packRoot,
		MotifsPath: filepath.Join(packRoot, manifest.Files["motifs"]),
		manifest:   manifest,
	}, nil
}

func verifyCustomPackChecksums(root string, manifest packManifest) error {
	if len(manifest.Files) == 0 {
		return errors.New("custom pack manifest has no files")
	}
	for _, key := range packRequiredKeys {
		rel := strings.TrimSpace(manifest.Files[key])
		if rel == "" {
			return fmt.Errorf("custom pack manifest is missing files.%s", key)
		}
		expected := strings.ToLower(strings.TrimSpace(manifest.Checksums[rel]))
		if expected == "" {
			return fmt.Errorf("custom pack manifest has no checksum for %s", rel)
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		actual, err := fileSHA256(path)
		if err != nil {
			return fmt.Errorf("hash custom pack %s: %w", rel, err)
		}
		if !strings.EqualFold(actual, expected) {
			return fmt.Errorf("custom pack checksum mismatch for %s", rel)
		}
	}
	return nil
}
