package regutrace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPackModeEndToEnd is opt-in because it runs the real ReguTrace pipeline.
// Required env: WGCNA_REGUTRACE, WGCNA_PACK_TEST_ROOT, WGCNA_PACK_TEST_KEY,
// WGCNA_PACK_TEST_GENOME, WGCNA_PACK_TEST_GFF, WGCNA_PACK_TEST_PROTEINS.
// WGCNA_HMMERSEARCH/WGCNA_BLASTP are passed through when set.
func TestPackModeEndToEnd(t *testing.T) {
	packRoot := os.Getenv("WGCNA_PACK_TEST_ROOT")
	genome := os.Getenv("WGCNA_PACK_TEST_GENOME")
	gff := os.Getenv("WGCNA_PACK_TEST_GFF")
	proteins := os.Getenv("WGCNA_PACK_TEST_PROTEINS")
	key := os.Getenv("WGCNA_PACK_TEST_KEY")
	if packRoot == "" || genome == "" || gff == "" || proteins == "" || key == "" || os.Getenv("WGCNA_REGUTRACE") == "" {
		t.Skip("set WGCNA_PACK_TEST_* and WGCNA_REGUTRACE to run the pack pipeline test")
	}
	mgr := NewManager(t.TempDir(), os.Getenv("WGCNA_REGUTRACE"), packRoot)
	if len(mgr.Info().Packs) == 0 {
		t.Fatal("no packs listed")
	}
	open := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	gf, ff, pf := open(genome), open(gff), open(proteins)
	defer gf.Close()
	defer ff.Close()
	defer pf.Close()
	job, err := mgr.Create(Uploads{
		Genome:        &Upload{Name: filepath.Base(genome), Reader: gf},
		GFF:           &Upload{Name: filepath.Base(gff), Reader: ff},
		Proteins:      &Upload{Name: filepath.Base(proteins), Reader: pf},
		ReferencePack: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Start(job.ID, Params{
		PromoterLen: 1000, PValue: 1e-4,
		Background: [4]float64{0.25, 0.25, 0.25, 0.25}, Threads: 4,
	}, "run"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		j, ok := mgr.Get(job.ID)
		if !ok {
			t.Fatal("job disappeared")
		}
		if j.Status == "succeeded" {
			if j.Summary == nil || j.Summary.TFTargetEdges == 0 {
				t.Fatalf("summary = %+v", j.Summary)
			}
			mgr.mu.RLock()
			js := mgr.jobs[job.ID]
			mgr.mu.RUnlock()
			if len(js.promoters) == 0 || len(js.donors) == 0 {
				t.Fatalf("promoters=%d donors=%d", len(js.promoters), len(js.donors))
			}
			for _, name := range []string{"promoters.fa", "tf_motif_trace.tsv", "scan-report.json"} {
				if _, err := os.Stat(filepath.Join(js.pipelineDir, name)); err != nil {
					t.Fatalf("pipeline output %s: %v", name, err)
				}
			}
			for _, kind := range []string{"report", "promoters", "evidence"} {
				if _, _, err := mgr.ExportPath(job.ID, kind); err != nil {
					t.Fatalf("export %s: %v", kind, err)
				}
			}
			return
		}
		if j.Status == "failed" || j.Status == "cancelled" {
			t.Fatalf("job = %+v", j)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("pack pipeline timed out")
}
