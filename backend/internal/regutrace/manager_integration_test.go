package regutrace

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestManagerEndToEnd runs the real ReguTrace binary when WGCNA_REGUTRACE is
// set. It is skipped in normal unit-test runs, where the Rust toolchain may not
// be available.
func TestManagerEndToEnd(t *testing.T) {
	bin := os.Getenv("WGCNA_REGUTRACE")
	if bin == "" {
		t.Skip("set WGCNA_REGUTRACE to run the ReguTrace integration test")
	}
	motif := "ACGTAC"
	genome := strings.Repeat("A", 3000)
	genome = genome[:150] + motif + genome[156:]
	genome = genome[:800] + "GTACGT" + genome[806:]
	gff := "##gff-version 3\n" +
		"Chr1\ttest\tgene\t201\t400\t.\t+\t.\tID=gene:G1;Name=GENE1\n" +
		"Chr1\ttest\tmRNA\t201\t400\t.\t+\t.\tID=transcript:G1.t1;Parent=gene:G1\n" +
		"Chr1\ttest\tgene\t601\t800\t.\t-\t.\tID=gene:G2;Name=GENE2\n" +
		"Chr1\ttest\tmRNA\t601\t800\t.\t-\t.\tID=transcript:G2.t1;Parent=gene:G2\n"
	pwm := ">MA0001.1 TEST\nA [ 10 0 0 0 10 0 ]\nC [ 0 10 0 0 0 10 ]\nG [ 0 0 10 0 0 0 ]\nT [ 0 0 0 10 0 0 ]\n"
	tfMap := "tf_id\ttf_symbol\tfamily\tmotif_id\nAT1G00010\tTF1\tHSF\tMA0001.1\n"

	mgr := NewManager(t.TempDir(), bin)
	job, err := mgr.Create(Uploads{
		Genome: &Upload{Name: "genome.fa", Reader: strings.NewReader(">Chr1\n" + genome + "\n")},
		GFF:    &Upload{Name: "annotation.gff3", Reader: strings.NewReader(gff)},
		Motifs: &Upload{Name: "motifs.jaspar", Reader: strings.NewReader(pwm)},
		TFMap:  &Upload{Name: "tf_map.tsv", Reader: strings.NewReader(tfMap)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Inputs.Motifs != "motifs.jaspar" {
		t.Fatalf("inputs = %+v", job.Inputs)
	}
	if _, err := mgr.Start(job.ID, Params{PromoterLen: 100, Downstream: 100, PValue: 1e-3}, "run"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	var got Job
	for time.Now().Before(deadline) {
		got, _ = mgr.Get(job.ID)
		if got.Status == "succeeded" || got.Status == "failed" || got.Status == "cancelled" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got.Status != "succeeded" {
		t.Fatalf("job = %+v\nlog: %s", got, readLogForTest(t, mgr, job.ID))
	}
	if got.Summary == nil || got.Summary.TFTargetPairs != 2 || got.Summary.MotifHits != 2 {
		t.Fatalf("summary = %+v", got.Summary)
	}
	if got.Summary.UniqueTargets != 2 || got.Summary.UniqueTFs != 1 || got.Summary.UniqueMotifs != 1 {
		t.Fatalf("summary counts = %+v", got.Summary)
	}
	res, err := mgr.Query(job.ID, QueryOptions{Direction: "tf", Query: "TF1", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total < 2 {
		t.Fatalf("query = %+v", res)
	}
	all, err := mgr.Query(job.ID, QueryOptions{Direction: "tf", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 || len(all.Rows) != 2 {
		t.Fatalf("unfiltered query = %+v", all)
	}
	byTarget, err := mgr.Query(job.ID, QueryOptions{Direction: "target", Query: "G2", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if byTarget.Total != 1 || byTarget.Rows[0].TargetID != "G2" {
		t.Fatalf("target query = %+v", byTarget)
	}
	view, err := mgr.PromoterView(job.ID, "G1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Hits) == 0 || view.Sequence == "" {
		t.Fatalf("promoter view = %+v", view)
	}
	mgr.mu.RLock()
	evidencePath := mgr.jobs[job.ID].evidencePath
	mgr.mu.RUnlock()
	if !strings.HasSuffix(evidencePath, "evidence.1.tsv") {
		t.Fatalf("evidence path = %q", evidencePath)
	}
	if _, _, err := mgr.ExportPath(job.ID, "evidence"); err != nil {
		t.Fatal(err)
	}
	extensions := map[string]string{"sqlite": ".db", "csv": ".csv", "tsv.gz": ".tsv.gz"}
	for _, format := range []string{"sqlite", "csv", "tsv.gz"} {
		path, name, err := mgr.ExportFormat(context.Background(), job.ID, "evidence", format)
		if err != nil {
			t.Fatalf("export %s: %v", format, err)
		}
		if !strings.Contains(name, "evidence") || !strings.HasSuffix(name, extensions[format]) {
			t.Fatalf("export %s name = %q", format, name)
		}
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("export %s path %q: %v", format, path, err)
		}
	}
}

func TestBuiltinLibraries(t *testing.T) {
	mgr := NewManager(t.TempDir(), "")
	info := mgr.Info()
	if len(info.Libraries) != 7 || info.MotifLibrary == "" || info.MotifCount != 805 {
		t.Fatalf("info = %+v", info)
	}
	counts := map[string]int{}
	for _, lib := range info.Libraries {
		counts[lib.Key] = lib.Count
	}
	if counts["plants"] != 805 || counts["vertebrates"] != 879 || counts["insects"] != 286 ||
		counts["nematodes"] != 103 || counts["fungi"] != 178 || counts["urochordates"] != 94 || counts["diatoms"] != 1 {
		t.Fatalf("library counts = %+v", counts)
	}
	job, err := mgr.Create(Uploads{
		Genome:       &Upload{Name: "genome.fa", Reader: strings.NewReader(">Chr1\nACGT\n")},
		GFF:          &Upload{Name: "annotation.gff3", Reader: strings.NewReader("##gff-version 3\n")},
		MotifLibrary: "vertebrates",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Inputs.MotifLibraryKey != "vertebrates" || !strings.Contains(job.Inputs.Motifs, "vertebrates") {
		t.Fatalf("inputs = %+v", job.Inputs)
	}
}

func readLogForTest(t *testing.T, mgr *Manager, id string) string {
	t.Helper()
	path, _, err := mgr.ExportPath(id, "log")
	if err != nil {
		return ""
	}
	b, _ := os.ReadFile(path)
	return string(b)
}
