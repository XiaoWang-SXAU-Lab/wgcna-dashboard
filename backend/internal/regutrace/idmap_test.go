package regutrace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseIDMapAliasesAndConflicts(t *testing.T) {
	data := []byte("source_id,target_id\nG1.1,GENE1\nG1.1,GENE1\nG2.1,GENE2\nG2.1,GENE3\n")
	parsed, err := parseIDMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("rows = %+v", parsed.Rows)
	}
	if parsed.DuplicateRows != 1 || len(parsed.Conflicts) != 1 {
		t.Fatalf("duplicates=%d conflicts=%v", parsed.DuplicateRows, parsed.Conflicts)
	}
	if parsed.Rows[0].Source != "G1.1" || parsed.Rows[0].Target != "GENE1" {
		t.Fatalf("row = %+v", parsed.Rows[0])
	}
}

func TestParseIDMapEggnogStyleHeader(t *testing.T) {
	data := []byte("#original_id\tgene_id\nFta_1G0000020.1\tFta_1G0000020\n")
	parsed, err := parseIDMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0].Target != "Fta_1G0000020" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestPreviewAndApplyIDMappingOnJobState(t *testing.T) {
	dir := t.TempDir()
	gff := filepath.Join(dir, "a.gff3")
	if err := os.WriteFile(gff, []byte("##gff-version 3\nChr1\ttest\tgene\t1\t100\t.\t+\t.\tID=G1\nChr1\ttest\tgene\t200\t300\t.\t+\t.\tID=G2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	annotationPath := filepath.Join(dir, "annotation.tsv")
	if err := os.WriteFile(annotationPath, []byte("gene_id\tsymbol\tdescription\nG1.1\tONE\tfirst\nG2.1\tTWO\tsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{jobs: map[string]*jobState{
		"job": {
			dir:            dir,
			gffPath:        gff,
			annotationPath: annotationPath,
			annotations: map[string]AnnotationEntry{
				"G1.1": {Symbol: "ONE", Description: "first"},
				"G2.1": {Symbol: "TWO", Description: "second"},
			},
			annotationSource: "eggnog",
		},
	}}
	data := []byte("original_id\tgene_id\nG1.1\tG1\nG2.1\tG2\n")
	preview, err := manager.PreviewIDMapping("job", data)
	if err != nil {
		t.Fatal(err)
	}
	if preview.SourceFound != 2 || preview.MappedTargets != 2 || preview.Coverage != 100 {
		t.Fatalf("preview = %+v", preview)
	}
	applied, err := manager.ApplyIDMapping("job", data)
	if err != nil {
		t.Fatal(err)
	}
	if applied.AppliedSources != 2 || applied.RepairedRows != 2 {
		t.Fatalf("applied = %+v", applied)
	}
	repaired, err := os.ReadFile(annotationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repaired), "G1\tONE\tfirst") || !strings.Contains(string(repaired), "G2\tTWO\tsecond") {
		t.Fatalf("repaired annotation:\n%s", repaired)
	}
	original, err := os.ReadFile(filepath.Join(dir, "annotation.original.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(original), "G1.1") {
		t.Fatalf("original annotation was not preserved:\n%s", original)
	}
}

func TestLoadGFFGenesAndPreviewCoverage(t *testing.T) {
	dir := t.TempDir()
	gff := filepath.Join(dir, "a.gff3")
	if err := os.WriteFile(gff, []byte("##gff-version 3\nChr1\ttest\tgene\t1\t100\t.\t+\t.\tID=gene:G1;Name=GENE1\nChr1\ttest\tgene\t200\t300\t.\t+\t.\tID=G2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	genes, err := loadGFFGenes(gff)
	if err != nil {
		t.Fatal(err)
	}
	if !genes["G1"] || !genes["G2"] || genes["gene:G1"] {
		t.Fatalf("genes = %v", genes)
	}
	parsed, err := parseIDMap([]byte("original_id\tgene_id\nG1.1\tG1\nG2.1\tG2\nX1\tG3\n"))
	if err != nil {
		t.Fatal(err)
	}
	annotations := map[string]AnnotationEntry{
		"G1.1": {Symbol: "ONE", Description: "first"},
		"G2.1": {Symbol: "TWO", Description: "second"},
	}
	preview := previewIDMapping(parsed, annotations, genes)
	if preview.Rows != 3 || preview.SourceFound != 2 || preview.TargetFound != 2 || preview.MappedTargets != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Coverage != 100 || len(preview.UnmatchedSources) != 1 || len(preview.MissingTargets) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
}
