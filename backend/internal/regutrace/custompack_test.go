package regutrace

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func makePackZip(t *testing.T, prefix string, unsafe bool) []byte {
	t.Helper()
	files := map[string][]byte{
		"hmm/models.hmm":        []byte("HMMER3/f\n"),
		"rules/rules.json":      []byte(`{"groups":[]}`),
		"reference-map.tsv":     []byte("tf_id\ttf_symbol\tfamily\tmotif_id\n"),
		"reference-domains.faa": []byte(">REF1\nMK\n"),
		"dbd-models.json":       []byte(`{}`),
		"motifs/motifs.jaspar":  []byte(">MA0001.1 TEST\nA [ 1 0 ]\n"),
	}
	checksums := map[string]string{}
	for name, data := range files {
		sum := sha256.Sum256(data)
		checksums[name] = hex.EncodeToString(sum[:])
	}
	manifest := map[string]any{
		"pack_version": "custom-test",
		"species":      "Test species",
		"assembly":     "test",
		"motif_scope":  "test",
		"files": map[string]string{
			"hmm":               "hmm/models.hmm",
			"rules":             "rules/rules.json",
			"reference_map":     "reference-map.tsv",
			"reference_domains": "reference-domains.faa",
			"dbd_models":        "dbd-models.json",
			"motifs":            "motifs/motifs.jaspar",
		},
		"checksums": checksums,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = manifestBytes
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		f, err := zw.Create(prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if unsafe {
		f, err := zw.Create(prefix + "../outside.txt")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte("escape"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUnpackCustomPackAndTopLevelDirectory(t *testing.T) {
	dir := t.TempDir()
	up := &Upload{Name: "custom.zip", Reader: bytes.NewReader(makePackZip(t, "", false))}
	pack, err := unpackCustomPack(dir, up)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Info.Key != "custom" || pack.Info.Version != "custom-test" {
		t.Fatalf("pack info = %+v", pack.Info)
	}
	if _, err := os.Stat(pack.MotifsPath); err != nil {
		t.Fatalf("motif path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pack.Dir, "manifest.json")); err != nil {
		t.Fatalf("manifest: %v", err)
	}
}

func TestUnpackCustomPackAcceptsSingleTopFolder(t *testing.T) {
	dir := t.TempDir()
	up := &Upload{Name: "custom.zip", Reader: bytes.NewReader(makePackZip(t, "my-pack/", false))}
	pack, err := unpackCustomPack(dir, up)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Dir != filepath.Join(dir, "custom-pack", "my-pack") {
		t.Fatalf("pack dir = %q", pack.Dir)
	}
}

func TestUnpackCustomPackRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	up := &Upload{Name: "custom.zip", Reader: bytes.NewReader(makePackZip(t, "", true))}
	if _, err := unpackCustomPack(dir, up); err == nil {
		t.Fatal("expected path escape to be rejected")
	}
}
