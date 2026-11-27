package regutrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPack(t *testing.T, root, key string) string {
	t.Helper()
	dir := filepath.Join(root, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"hmm":               "hmm content\n",
		"rules":             "{}\n",
		"reference_map":     "tf_id\ttf_symbol\ttf_family\tmotif_id\nD1\tDONOR\tF\tM1\n",
		"reference_domains": ">D1\nACGT\n",
		"dbd_models":        "{}\n",
		"motifs":            ">M1 TEST\nA [ 10 0 ]\nC [ 0 10 ]\nG [ 0 0 ]\nT [ 0 0 ]\n",
	}
	rel := map[string]string{
		"hmm":               "tf.hmm",
		"rules":             "rules.json",
		"reference_map":     "reference-map.tsv",
		"reference_domains": "reference-domains.faa",
		"dbd_models":        "dbd-models.json",
		"motifs":            "motifs.txt",
	}
	for key, name := range rel {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(files[key]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := map[string]any{
		"pack_version": "test-pack-1",
		"species":      "Test species",
		"assembly":     "test-assembly",
		"motif_scope":  "test motifs",
		"files":        rel,
		"tools":        map[string]string{"hmmsearch": "hmmsearch", "blastp": "blastp"},
		"defaults":     map[string]any{"promoter_len": 800, "pvalue": 0.0001},
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

func TestListAndResolveReferencePacks(t *testing.T) {
	root := t.TempDir()
	dir := writeTestPack(t, root, "test-pack-1")
	packs := ListReferencePacks(root)
	if len(packs) != 1 || packs[0].Key != "test-pack-1" || packs[0].Version != "test-pack-1" || packs[0].PromoterLen != 800 {
		t.Fatalf("packs = %+v", packs)
	}
	pack, err := ResolveReferencePack(root, "test-pack-1")
	if err != nil {
		t.Fatal(err)
	}
	if pack.Dir != dir || pack.MotifsPath != filepath.Join(dir, "motifs.txt") {
		t.Fatalf("pack = %+v", pack)
	}
	if _, err := ResolveReferencePack(root, "../test-pack-1"); err == nil {
		t.Fatal("traversal key must be rejected")
	}
}

func TestCreatePackModeSkipsManualHMMAndTFMap(t *testing.T) {
	packRoot := t.TempDir()
	writeTestPack(t, packRoot, "test-pack-1")
	mgr := NewManager(t.TempDir(), "", packRoot)
	job, err := mgr.Create(Uploads{
		Genome:        &Upload{Name: "genome.fa", Reader: strings.NewReader(">Chr1\nACGTACGT\n")},
		GFF:           &Upload{Name: "genome.gff3", Reader: strings.NewReader("##gff-version 3\n")},
		Proteins:      &Upload{Name: "proteins.faa", Reader: strings.NewReader(">gene1\nACGT\n")},
		ReferencePack: "test-pack-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Inputs.ReferencePack != "test-pack-1" {
		t.Fatalf("inputs = %+v", job.Inputs)
	}
	mgr.mu.RLock()
	js := mgr.jobs[job.ID]
	mgr.mu.RUnlock()
	if js == nil || js.referencePackDir == "" || js.tfMapPath != "" || js.tfGenesPath != "" {
		t.Fatalf("job state = %+v", js)
	}
	if len(js.motifs) != 1 {
		t.Fatalf("pack motifs = %+v", js.motifs)
	}
	// The standard three-file workflow must not require an annotation upload.
	if job.Inputs.Annotation != "" || job.Inputs.AnnotationSource != "GFF" || js.annotationPath != "" {
		t.Fatalf("optional annotation did not fall back to GFF: %+v", job.Inputs)
	}
	if strings.Contains(strings.Join(mgr.buildArgs(js, "run"), " "), "--annotation") {
		t.Fatal("three-file pipeline unexpectedly adds --annotation")
	}
	js.Params.IncludeSingleDomainFallback = true
	args := mgr.buildArgs(js, "run")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--include-single-domain-fallback") {
		t.Fatalf("args missing fallback flag: %s", joined)
	}
	for _, want := range []string{"pipeline", "--reference-pack", "--out-dir", "--proteins", "--background"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--tf-map") || strings.Contains(joined, "--motifs") {
		t.Fatalf("pack args should not use manual tf-map/motifs: %s", joined)
	}
}

func TestPackSpecificityMetadataUsesManifestFilename(t *testing.T) {
	root := t.TempDir()
	dir := writeTestPack(t, root, "test")
	path := filepath.Join(dir, "manifest.json")
	raw, _ := os.ReadFile(path)
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	files := manifest["files"].(map[string]any)
	files["specificity_groups"] = "frozen-groups.tsv"
	if err := os.WriteFile(filepath.Join(dir, "frozen-groups.tsv"), []byte("motif_id\tgroup_080\nM1\tF_001\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	js := &jobState{referencePackDir: dir, motifs: map[string]MotifInfo{}}
	js.loadPhase4Metadata(nil)
	if groups := js.phase4.SpecificityGroups["M1"]; len(groups) != 1 || groups[0] != "F_001" {
		t.Fatalf("manifest specificity metadata lost: %v", groups)
	}
	files["specificity_groups"] = "../outside.tsv"
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if got := packMetadataPath(dir, "specificity_groups"); got != "" {
		t.Fatalf("outside metadata accepted: %s", got)
	}
}
