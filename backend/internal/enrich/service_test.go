package enrich

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleGenesAndAnnotate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "step3_modules.json",
		`{"genes":["g1","g2","g3"],"colors":["red","blue","red"]}`)
	writeFile(t, dir, "annotation.tsv",
		"gene_id\tgene_name\tsymbol\tdescription\n"+
			"g1\tgene one\tG1\tdesc1\n"+
			"g3\tgene three\tG3\tdesc3\n")

	genes, err := ModuleGenes(dir, "red")
	if err != nil {
		t.Fatal(err)
	}
	if len(genes) != 2 || genes[0] != "g1" || genes[1] != "g3" {
		t.Fatalf("red genes = %v", genes)
	}

	rows, err := Annotate(dir, genes)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Symbol != "G1" || rows[1].Symbol != "G3" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestGOEnrichService(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "step3_modules.json",
		`{"genes":["g1","g2","g3","g4"],"colors":["red","red","red","red"]}`)
	writeFile(t, dir, "gene2go.tsv",
		"gene_id\tgo_id\tgo_name\tontology\n"+
			"g1\tGO:1\ttone\tbiological_process\n"+
			"g2\tGO:1\ttone\tbiological_process\n"+
			"g3\tGO:1\ttone\tbiological_process\n"+
			"g4\tGO:2\ttwo\tbiological_process\n")

	res, err := GOEnrich(dir, []string{"g1", "g2", "g3"}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 || res[0].ID != "GO:1" {
		t.Fatalf("unexpected results: %+v", res)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
