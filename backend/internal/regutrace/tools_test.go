package regutrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTestToolBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"hmmsearch.exe": "hmmsearch-test",
		"blastp.exe":    "blastp-test",
		"cygwin1.dll":   "cygwin-test",
		"nghttp2.dll":   "nghttp2-test",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hash := func(name string) string {
		sum, err := fileSHA256(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	manifest := map[string]any{
		"schema": 1,
		"tools": map[string]any{
			"hmmsearch": map[string]string{"version": "3.3.2", "file": "hmmsearch.exe", "sha256": hash("hmmsearch.exe"), "license": "HMMER BSD-3-Clause"},
			"blastp":    map[string]string{"version": "2.17.0+", "file": "blastp.exe", "sha256": hash("blastp.exe"), "license": "BLAST public domain"},
		},
		"files": map[string]string{"cygwin1.dll": hash("cygwin1.dll"), "nghttp2.dll": hash("nghttp2.dll")},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestVerifyBundledTools(t *testing.T) {
	dir := writeTestToolBundle(t)
	statuses, paths, err := VerifyBundledTools(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || paths["hmmsearch"] == "" || paths["blastp"] == "" {
		t.Fatalf("statuses=%+v paths=%+v", statuses, paths)
	}
	for _, status := range statuses {
		if status.Status != "ok" {
			t.Fatalf("status = %+v", status)
		}
	}
	mgr := NewManager(t.TempDir(), "", t.TempDir())
	mgr.SetToolRoot(dir)
	if len(mgr.toolStatus) != 2 || mgr.toolErr != nil || len(mgr.toolFiles) != 2 {
		t.Fatalf("manager tools: status=%+v err=%v files=%+v", mgr.toolStatus, mgr.toolErr, mgr.toolFiles)
	}
	if got := mgr.toolPath("hmmsearch"); got != filepath.Join(dir, "hmmsearch.exe") {
		t.Fatalf("hmmsearch path = %q", got)
	}
}

func TestVerifyBundledToolsRejectsChecksumMismatch(t *testing.T) {
	dir := writeTestToolBundle(t)
	if err := os.WriteFile(filepath.Join(dir, "blastp.exe"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyBundledTools(dir); err == nil {
		t.Fatal("checksum mismatch must fail verification")
	}
	mgr := NewManager(t.TempDir(), "", t.TempDir())
	mgr.SetToolRoot(dir)
	if mgr.toolErr == nil {
		t.Fatal("manager must retain bundled-tool verification failure")
	}
}
