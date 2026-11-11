package regutrace

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestDatasetLargePerf is an opt-in check for indexing a large streamed scan.
// Set WGCNA_DATASET_DIR to a directory containing evidence.tsv, promoters.fa
// and report.json, and WGCNA_DATASET_MOTIFS to the motif library used.
func TestDatasetLargePerf(t *testing.T) {
	dir := os.Getenv("WGCNA_DATASET_DIR")
	motifPath := os.Getenv("WGCNA_DATASET_MOTIFS")
	if dir == "" || motifPath == "" {
		t.Skip("set WGCNA_DATASET_DIR and WGCNA_DATASET_MOTIFS")
	}
	raw, err := os.ReadFile(motifPath)
	if err != nil {
		t.Fatal(err)
	}
	motifs, err := parseMotifMeta(raw)
	if err != nil {
		t.Fatal(err)
	}
	promoters, err := readPromoters(filepath.Join(dir, "promoters.fa"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := readReport(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	motifHits := reportContextInt(report, "SCAN_SUMMARY", "motif_hits")

	start := time.Now()
	data, err := buildDataset(filepath.Join(dir, "evidence.tsv"), motifs, nil, len(promoters), "GFF", motifHits)
	if err != nil {
		t.Fatal(err)
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("indexed %d edges, %d targets, %d TFs in %s; heap alloc %.0f MB",
		data.total, data.summary.UniqueTargets, data.summary.UniqueTFs,
		time.Since(start).Round(time.Millisecond), float64(ms.Alloc)/(1<<20))
	if data.summary.MotifHits != motifHits {
		t.Fatalf("motif hits = %d, report = %d", data.summary.MotifHits, motifHits)
	}
	if len(data.summary.TopTFs) == 0 {
		t.Fatal("no top TFs")
	}
	qStart := time.Now()
	res := (&jobState{data: data, motifs: motifs}).query(QueryOptions{
		Direction: "tf", Query: data.summary.TopTFs[0].Name, Limit: 200,
	})
	t.Logf("query %q -> %d rows in %s", data.summary.TopTFs[0].Name, res.Total, time.Since(qStart).Round(time.Millisecond))
	if res.Total == 0 || len(res.Rows) == 0 {
		t.Fatalf("query = %+v", res)
	}
	pStart := time.Now()
	js := &jobState{data: data, motifs: motifs, promoters: promoters, Job: Job{Params: defaultParams()}}
	view, err := js.promoterView(res.Rows[0].TargetID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("promoter view for %s -> %d sites in %s", view.TargetID, len(view.Hits), time.Since(pStart).Round(time.Millisecond))
}
