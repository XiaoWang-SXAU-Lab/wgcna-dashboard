package regutrace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTFViewBuildsGeneModelDomainsAndMotifMappings(t *testing.T) {
	dir := t.TempDir()
	gffPath := filepath.Join(dir, "genome.gff3")
	gff := "##gff-version 3\n" +
		"chr1\ttest\tgene\t100\t500\t.\t+\t.\tID=G1;Name=MYB1;gene=MYB1;biotype=protein_coding\n" +
		"chr1\ttest\tmRNA\t100\t500\t.\t+\t.\tID=T1;Parent=G1;protein_id=P1\n" +
		"chr1\ttest\texon\t100\t200\t.\t+\t.\tID=E1;Parent=T1\n" +
		"chr1\ttest\texon\t300\t500\t.\t+\t.\tID=E2;Parent=T1\n" +
		"chr1\ttest\tCDS\t120\t200\t.\t+\t0\tID=C1;Parent=T1\n" +
		"chr1\ttest\tCDS\t300\t450\t.\t+\t0\tID=C2;Parent=T1\n"
	if err := os.WriteFile(gffPath, []byte(gff), 0o644); err != nil {
		t.Fatal(err)
	}
	tfGenes := "gene_id\ttf_symbol\tfamily\tidentification_status\tsource\tevidence_json\n" +
		"G1\tG1\tMYB\tidentified\ttest\t" +
		`[{"protein_id":"P1","families":[{"family":"MYB","subfamily":"R2R3"}],"domains":[{"conditional_evalue":1e-9,"domain_score":42.5,"full_evalue":1e-8,"model_accession":"PF1","model_end":40,"model_length":50,"model_name":"Myb_DNA-binding","model_start":2,"protein_end":20,"protein_start":5}]}]` + "\n"
	tfGenesPath := filepath.Join(dir, "tf_genes.tsv")
	if err := os.WriteFile(tfGenesPath, []byte(tfGenes), 0o644); err != nil {
		t.Fatal(err)
	}
	tfMap := "tf_id\ttf_symbol\tfamily\tmotif_id\treference_tf_id\tmapping_method\tdomain_mode\textraction_family\n" +
		"G1\tG1\tMYB\tMA0001.1\tREF1\treciprocal_dbd_similarity\tfull_architecture\tR2R3\n"
	tfMapPath := filepath.Join(dir, "tf_map.tsv")
	if err := os.WriteFile(tfMapPath, []byte(tfMap), 0o644); err != nil {
		t.Fatal(err)
	}
	js := &jobState{
		gff:         newGFFCatalog(gffPath),
		tfGenesPath: tfGenesPath,
		tfMapPath:   tfMapPath,
		motifs:      map[string]MotifInfo{"MA0001.1": {ID: "MA0001.1", Name: "MYB motif"}},
	}
	view, err := js.tfView("G1")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Available || view.TFName != "MYB1" {
		t.Fatalf("unexpected view header: %+v", view)
	}
	if view.Gene == nil || len(view.Gene.Transcripts) != 1 {
		t.Fatalf("gene model = %+v", view.Gene)
	}
	tr := view.Gene.Transcripts[0]
	if view.DefaultTranscript != "T1" || tr.ProteinID != "P1" {
		t.Fatalf("default transcript = %q, transcript = %+v", view.DefaultTranscript, tr)
	}
	if len(tr.FiveUTR) != 1 || tr.FiveUTR[0] != (GFFSegment{Start: 100, End: 119}) {
		t.Fatalf("5' UTR = %+v", tr.FiveUTR)
	}
	if len(tr.ThreeUTR) != 1 || tr.ThreeUTR[0] != (GFFSegment{Start: 451, End: 500}) {
		t.Fatalf("3' UTR = %+v", tr.ThreeUTR)
	}
	if len(view.Proteins) != 1 || view.Proteins[0].ProteinID != "P1" || len(view.Proteins[0].Domains) != 1 {
		t.Fatalf("proteins = %+v", view.Proteins)
	}
	if view.Proteins[0].Length != 77 {
		t.Fatalf("protein length = %d, want 77 from the CDS", view.Proteins[0].Length)
	}
	if len(tr.Projected) != 1 {
		t.Fatalf("projected domains = %+v", tr.Projected)
	}
	if got := tr.Projected[0].Genomic; len(got) != 1 || got[0] != (GFFSegment{Start: 132, End: 179}) {
		t.Fatalf("projected genomic = %+v", got)
	}
	if len(view.MotifMappings) != 1 || view.MotifMappings[0].MotifID != "MA0001.1" ||
		view.MotifMappings[0].MotifName != "MYB motif" {
		t.Fatalf("motif mappings = %+v", view.MotifMappings)
	}
	view2, err := js.tfView("G1")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(view2.Gene.Transcripts[0].Projected); got != 1 {
		t.Fatalf("repeated tfView duplicated projected domains: %d", got)
	}
	targetView, err := js.tfViewWithTarget("G1", "G1")
	if err != nil {
		t.Fatal(err)
	}
	if targetView.TargetGene == nil || targetView.TargetGene.Gene == nil ||
		targetView.TargetGene.TargetName != "MYB1" ||
		targetView.TargetGene.DefaultTranscript != "T1" {
		t.Fatalf("target gene view = %+v", targetView.TargetGene)
	}
}

func TestTFViewProjectsDomainsOnMinusStrand(t *testing.T) {
	dir := t.TempDir()
	gffPath := filepath.Join(dir, "genome.gff3")
	gff := "##gff-version 3\n" +
		"chr1\ttest\tgene\t1000\t1500\t.\t-\t.\tID=G2;Name=MYB2\n" +
		"chr1\ttest\tmRNA\t1000\t1500\t.\t-\t.\tID=T2;Parent=G2;protein_id=P2\n" +
		"chr1\ttest\texon\t1000\t1200\t.\t-\t.\tParent=T2\n" +
		"chr1\ttest\texon\t1300\t1500\t.\t-\t.\tParent=T2\n" +
		"chr1\ttest\tCDS\t1100\t1200\t.\t-\t0\tParent=T2\n" +
		"chr1\ttest\tCDS\t1300\t1400\t.\t-\t0\tParent=T2\n"
	if err := os.WriteFile(gffPath, []byte(gff), 0o644); err != nil {
		t.Fatal(err)
	}
	tfGenes := "gene_id\ttf_symbol\tfamily\tidentification_status\tsource\tevidence_json\n" +
		"G2\tG2\tMYB\tidentified\ttest\t" +
		`[{"protein_id":"P2","families":[{"family":"MYB","subfamily":"R2R3"}],"domains":[{"protein_end":20,"protein_start":5}]}]` + "\n"
	tfGenesPath := filepath.Join(dir, "tf_genes.tsv")
	if err := os.WriteFile(tfGenesPath, []byte(tfGenes), 0o644); err != nil {
		t.Fatal(err)
	}
	js := &jobState{gff: newGFFCatalog(gffPath), tfGenesPath: tfGenesPath}
	view, err := js.tfView("G2")
	if err != nil {
		t.Fatal(err)
	}
	tr := view.Gene.Transcripts[0]
	if len(tr.FiveUTR) != 1 || tr.FiveUTR[0] != (GFFSegment{Start: 1401, End: 1500}) {
		t.Fatalf("minus 5' UTR = %+v", tr.FiveUTR)
	}
	if len(tr.ThreeUTR) != 1 || tr.ThreeUTR[0] != (GFFSegment{Start: 1000, End: 1099}) {
		t.Fatalf("minus 3' UTR = %+v", tr.ThreeUTR)
	}
	if len(tr.Projected) != 1 {
		t.Fatalf("projected = %+v", tr.Projected)
	}
	if got := tr.Projected[0].Genomic; len(got) != 1 || got[0] != (GFFSegment{Start: 1341, End: 1388}) {
		t.Fatalf("minus projected genomic = %+v", got)
	}
}

func TestTFViewExplainsMotifOnlyTF(t *testing.T) {
	dir := t.TempDir()
	gffPath := filepath.Join(dir, "genome.gff3")
	if err := os.WriteFile(gffPath, []byte("##gff-version 3\nchr1\ttest\tgene\t1\t100\t.\t+\t.\tID=G1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	js := &jobState{gff: newGFFCatalog(gffPath)}
	view, err := js.tfView("MA0001.1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Available || view.Reason == "" {
		t.Fatalf("expected unavailable motif-only view, got %+v", view)
	}
}
