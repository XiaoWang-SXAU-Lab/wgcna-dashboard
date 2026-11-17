package regutrace

import (
	"os"
	"testing"
)

// This test is opt-in because it reads a real GFF/catalogue pair. It is used
// during release acceptance against a completed reference-pack job.
func TestTFViewRealGFFAndCatalogue(t *testing.T) {
	gff := os.Getenv("WGCNA_TFVIEW_GFF")
	catalogue := os.Getenv("WGCNA_TFVIEW_TF_GENES")
	tf := os.Getenv("WGCNA_TFVIEW_TF")
	target := os.Getenv("WGCNA_TFVIEW_TARGET")
	if gff == "" || catalogue == "" || tf == "" {
		t.Skip("set WGCNA_TFVIEW_GFF, WGCNA_TFVIEW_TF_GENES and WGCNA_TFVIEW_TF")
	}
	js := &jobState{gff: newGFFCatalog(gff), tfGenesPath: catalogue}
	view, err := js.tfView(tf)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Available || view.Gene == nil {
		t.Fatalf("no gene model for %s: %+v", tf, view)
	}
	if view.TFName == "" {
		t.Fatalf("expected a display name for %s", tf)
	}
	if len(view.Gene.Transcripts) == 0 {
		t.Fatalf("gene has no transcripts: %+v", view.Gene)
	}
	tr := view.Gene.Transcripts[0]
	if len(tr.CDS) == 0 || len(tr.Exons) == 0 {
		t.Fatalf("transcript has no exons/CDS: %+v", tr)
	}
	if len(view.Proteins) == 0 || len(view.Proteins[0].Domains) == 0 {
		t.Fatalf("catalogue has no HMMER domains: %+v", view.Proteins)
	}
	if len(tr.Projected) == 0 {
		t.Fatalf("no projected domains: %+v", tr)
	}
	t.Logf("%s = %s; transcripts=%d; protein=%s domains=%d; exons=%d cds=%d 5utr=%d 3utr=%d projected=%d",
		tf, view.TFName, len(view.Gene.Transcripts), view.Proteins[0].ProteinID,
		len(view.Proteins[0].Domains), len(tr.Exons), len(tr.CDS), len(tr.FiveUTR), len(tr.ThreeUTR), len(tr.Projected))
	if target != "" {
		withTarget, err := js.tfViewWithTarget(tf, target)
		if err != nil {
			t.Fatal(err)
		}
		if withTarget.TargetGene == nil || withTarget.TargetGene.Gene == nil {
			t.Fatalf("no target gene model for %s: %+v", target, withTarget.TargetGene)
		}
		targetTR := withTarget.TargetGene.Gene.Transcripts[0]
		if len(targetTR.Exons) == 0 {
			t.Fatalf("target transcript has no exons: %+v", targetTR)
		}
		t.Logf("target %s = %s; transcripts=%d; first transcript exons=%d cds=%d",
			target, withTarget.TargetGene.TargetName, len(withTarget.TargetGene.Gene.Transcripts),
			len(targetTR.Exons), len(targetTR.CDS))
	}
}
