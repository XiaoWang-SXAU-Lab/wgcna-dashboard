package enrich

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDemoData exercises the real demo GO annotation (set DEMO_DIR to the
// demo-arabidopsis folder). It is skipped without the env var.
func TestDemoData(t *testing.T) {
	dir := os.Getenv("DEMO_DIR")
	if dir == "" {
		t.Skip("set DEMO_DIR to run the demo-data integration test")
	}

	data, err := os.ReadFile(filepath.Join(dir, "gene2go.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	bg, err := ParseGene2GO(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(bg.Universe) < 20000 {
		t.Fatalf("GO universe too small: %d", len(bg.Universe))
	}
	res := Run(bg.Universe[:300], bg, 1, 10)
	if len(res) == 0 {
		t.Fatal("no enrichment results")
	}

	ann, err := os.ReadFile(filepath.Join(dir, "annotation.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	amap, err := ParseAnnotation(ann)
	if err != nil {
		t.Fatal(err)
	}
	if len(amap) < 20000 {
		t.Fatalf("annotation map too small: %d", len(amap))
	}

	// KEGG path: gene2kegg -> built-in Plants background -> enrichment.
	g2k, err := os.ReadFile(filepath.Join(dir, "gene2kegg.tsv"))
	if err != nil {
		t.Skipf("no gene2kegg.tsv in demo dir: %v", err)
	}
	gene2ko, err := ParseGene2KEGG(g2k)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := BuiltinKEGGBackground()
	if err != nil {
		t.Fatal(err)
	}
	keggBG := kb.BuildBackground(gene2ko)
	if len(keggBG.Universe) == 0 {
		t.Fatal("empty KEGG background universe")
	}
	kres := Run(keggBG.Universe[:300], keggBG, 1, 10)
	if len(kres) == 0 {
		t.Fatal("no KEGG enrichment results")
	}
	t.Logf("go_universe=%d go_terms=%d annotations=%d top_term=%s kegg_universe=%d top_pathway=%s",
		len(bg.Universe), len(bg.TermName), len(amap), res[0].ID, len(keggBG.Universe), kres[0].ID)
}
