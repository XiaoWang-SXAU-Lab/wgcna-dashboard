package regutrace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func u64ptr(v uint64) *uint64   { return &v }
func f64ptr(v float64) *float64 { return &v }

func TestThreadsParamValidation(t *testing.T) {
	for _, n := range []int{0, 1, 7, 256} {
		if _, err := normalizeParams(Params{Threads: n}); err != nil {
			t.Fatalf("threads %d: %v", n, err)
		}
	}
	for _, n := range []int{-1, 257} {
		if _, err := normalizeParams(Params{Threads: n}); err == nil {
			t.Fatalf("threads %d: expected error", n)
		}
	}
}

func TestParseAnnotationEggnog(t *testing.T) {
	data := []byte("#query\tpreferred_name\tdescription\nG1\tMYB5\tmyb domain protein\nG2\t\tuncharacterized\n")
	ann, normalized, source, err := parseAnnotation(data)
	if err != nil {
		t.Fatal(err)
	}
	if source != "eggnog" {
		t.Fatalf("source = %q", source)
	}
	if ann["G1"].Symbol != "MYB5" || ann["G1"].Description != "myb domain protein" {
		t.Fatalf("G1 = %+v", ann["G1"])
	}
	if !strings.Contains(string(normalized), "G1\tMYB5") || !strings.Contains(string(normalized), "G2\tG2") {
		t.Fatalf("normalized annotation:\n%s", normalized)
	}
}

func TestParseAnnotationVersionedIDsAndComments(t *testing.T) {
	data := []byte("#query\tDescription\tPreferred_name\n" +
		"Fta_1G0000020.1\tPhagocyte signaling-impaired protein\t-\n" +
		"Fta_1G0000030.1\tBTB POZ domain-containing protein\tPHOT\n" +
		"## 2 queries scanned\n")
	ann, normalized, source, err := parseAnnotation(data)
	if err != nil {
		t.Fatal(err)
	}
	if source != "eggnog" {
		t.Fatalf("source = %q", source)
	}
	if ann["Fta_1G0000020.1"].Symbol != "" || ann["Fta_1G0000020.1"].Description != "Phagocyte signaling-impaired protein" {
		t.Fatalf("exact entry = %+v", ann["Fta_1G0000020.1"])
	}
	if strings.Contains(string(normalized), "##") {
		t.Fatalf("comment row leaked into normalized annotation:\n%s", normalized)
	}
	if !strings.Contains(string(normalized), "Fta_1G0000020.1\tFta_1G0000020\tPhagocyte signaling-impaired protein") {
		t.Fatalf("description missing from normalized annotation:\n%s", normalized)
	}

	aliased := withVersionAliases(ann)
	if got := aliased["Fta_1G0000020"]; got.Description != "Phagocyte signaling-impaired protein" || got.Symbol != "" {
		t.Fatalf("gene alias = %+v", got)
	}
	if got := aliased["Fta_1G0000030"]; got.Symbol != "PHOT" {
		t.Fatalf("gene alias with symbol = %+v", got)
	}

	js := testJobState()
	js.annotations = aliased
	js.promoters["Fta_1G0000020"] = &Promoter{GeneID: "Fta_1G0000020", Chrom: "Chr1", Strand: "+", Start: 1, End: 2, Sequence: "AC"}
	symbol, desc, _ := js.annotationFor("Fta_1G0000020", "Fta_1G0000020.1")
	if desc != "Phagocyte signaling-impaired protein" || symbol != "Fta_1G0000020" {
		t.Fatalf("annotationFor symbol=%q description=%q", symbol, desc)
	}
}

func TestVersionAliasesKeepExactKeys(t *testing.T) {
	exact := map[string]AnnotationEntry{
		"G1":   {Symbol: "EXACT", Description: "exact gene"},
		"G1.1": {Symbol: "ISOFORM", Description: "isoform"},
		"G2.1": {Symbol: "ONE", Description: "iso1"},
		"G2.2": {Symbol: "TWO", Description: "iso2"},
	}
	aliased := withVersionAliases(exact)
	if aliased["G1"].Symbol != "EXACT" {
		t.Fatalf("exact key overridden: %+v", aliased["G1"])
	}
	if aliased["G2"].Symbol != "ONE" {
		t.Fatalf("deterministic isoform alias = %+v", aliased["G2"])
	}
}

func TestParseAnnotationGeneric(t *testing.T) {
	data := []byte("gene_id\tgene_name\tdescription\nG1\tMYB5\tmyb domain protein\n")
	ann, normalized, source, err := parseAnnotation(data)
	if err != nil {
		t.Fatal(err)
	}
	if source != "annotation_table" {
		t.Fatalf("source = %q", source)
	}
	if ann["G1"].Symbol != "MYB5" || !strings.Contains(string(normalized), "G1\tMYB5") {
		t.Fatalf("annotation = %+v, normalized = %s", ann, normalized)
	}
}

func TestParseAnnotationRejectsNoIdentifier(t *testing.T) {
	if _, _, _, err := parseAnnotation([]byte("symbol\tdescription\nMYB5\tx\n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseMotifMeta(t *testing.T) {
	data := []byte(">MA0001.1 TEST\nA [ 10 0 1 ]\nC [ 0 10 1 ]\nG [ 0 0 10 ]\nT [ 0 0 0 ]\n")
	meta, err := parseMotifMeta(data)
	if err != nil {
		t.Fatal(err)
	}
	m := meta["MA0001.1"]
	if m.Name != "TEST" || m.Length != 3 || m.Consensus != "ACG" {
		t.Fatalf("meta = %+v", m)
	}
}

func testJobState() *jobState {
	seq := "ACGTACGGGG"
	return &jobState{
		Job: Job{Params: Params{PromoterLen: 6, Downstream: 2}},
		promoters: map[string]*Promoter{
			"G1": {GeneID: "G1", Chrom: "Chr1", Strand: "+", Start: 100, End: 109, Sequence: seq},
			"G2": {GeneID: "G2", Chrom: "Chr1", Strand: "-", Start: 200, End: 209, Sequence: seq},
		},
		motifs: map[string]MotifInfo{
			"M1": {ID: "M1", Name: "MYB motif", Consensus: "ACGTAC", Length: 6},
		},
		annotations: map[string]AnnotationEntry{
			"G1": {Symbol: "GENE1", Description: "myb target"},
			"G2": {Symbol: "GENE2", Description: "minus target"},
		},
		annotationSource: "eggnog",
		edges: []Edge{
			{
				TFID: "TF1", TFSymbol: "MYB5", TFFamily: "MYB", TargetID: "G1", TargetSymbol: "GENE1",
				EvidenceType: "prediction_motif", EvidenceLevel: "D", MotifID: "M1",
				HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+", Score: f64ptr(10), PValue: f64ptr(1e-5),
				Notes: "family=MYB",
			},
			{
				TFID: "TF2", TFSymbol: "MYB6", TFFamily: "MYB", TargetID: "G1", TargetSymbol: "GENE1",
				EvidenceType: "prediction_motif", EvidenceLevel: "D", MotifID: "M1",
				HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+", Score: f64ptr(10), PValue: f64ptr(1e-5),
				Notes: "family=MYB",
			},
			{
				TFID: "TF1", TFSymbol: "MYB5", TFFamily: "MYB", TargetID: "G2", TargetSymbol: "GENE2",
				EvidenceType: "prediction_motif", EvidenceLevel: "D", MotifID: "M1",
				HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "-", Score: f64ptr(9), PValue: f64ptr(1e-4),
				Notes: "family=MYB",
			},
		},
	}
}

func TestQueryAndSummary(t *testing.T) {
	js := testJobState()
	summary := buildSummary(js.promoters, js.motifs, js.edges, js.annotationSource)
	if summary.MotifHits != 2 {
		t.Fatalf("motif hits = %d, want 2", summary.MotifHits)
	}
	if summary.TFTargetPairs != 3 {
		t.Fatalf("tf-target pairs = %d, want 3", summary.TFTargetPairs)
	}
	if summary.UniqueTFs != 2 || summary.UniqueTargets != 2 || summary.UniqueMotifs != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	res := js.query(QueryOptions{Direction: "tf", Query: "MYB5", Limit: 10})
	if res.Total != 2 || len(res.Rows) != 2 {
		t.Fatalf("query result = %+v", res)
	}
	if res.Rows[0].TargetID != "G1" || res.Rows[0].TargetDescription != "myb target" {
		t.Fatalf("row = %+v", res.Rows[0])
	}
	if res.Rows[0].AnnotationSource != "eggnog" {
		t.Fatalf("annotation source = %q", res.Rows[0].AnnotationSource)
	}

	byTarget := js.query(QueryOptions{Direction: "target", Query: "GENE2", Limit: 10})
	if byTarget.Total != 1 || byTarget.Rows[0].TFSymbol != "MYB5" {
		t.Fatalf("target query = %+v", byTarget)
	}
}

func TestPromoterViewPlusAndMinus(t *testing.T) {
	js := testJobState()
	plus, err := js.promoterView("G1")
	if err != nil {
		t.Fatal(err)
	}
	if plus.TargetSymbol != "GENE1" || plus.TSSPosition != 7 || plus.Length != 10 {
		t.Fatalf("plus view = %+v", plus)
	}
	if len(plus.Hits) != 1 {
		t.Fatalf("plus hits = %d, want 1 grouped site", len(plus.Hits))
	}
	h := plus.Hits[0]
	if h.Site != "ACGTAC" || h.MatchedSite != "ACGTAC" || h.GenomicStart != 100 || h.GenomicEnd != 105 {
		t.Fatalf("plus hit = %+v", h)
	}
	if len(h.TFs) != 2 || h.TFs[0].Symbol != "MYB5" {
		t.Fatalf("plus TFs = %+v", h.TFs)
	}

	minus, err := js.promoterView("G2")
	if err != nil {
		t.Fatal(err)
	}
	if minus.TSSPosition != 7 || minus.Length != 10 {
		t.Fatalf("minus TSS = %d, length = %d", minus.TSSPosition, minus.Length)
	}
	if len(minus.Hits) != 1 {
		t.Fatalf("minus hits = %d", len(minus.Hits))
	}
	m := minus.Hits[0]
	if m.Site != "ACGTAC" || m.MatchedSite != "GTACGT" || m.GenomicStart != 204 || m.GenomicEnd != 209 {
		t.Fatalf("minus hit = %+v", m)
	}
}

func TestReadEvidenceAndPromoters(t *testing.T) {
	dir := t.TempDir()
	ev := filepath.Join(dir, "evidence.tsv")
	text := "tf_id\ttf_symbol\ttarget_id\ttarget_symbol\tevidence_type\tevidence_level\tmotif_id\thit_start\thit_end\tstrand\tscore\tp_value\tnotes\n" +
		"TF1\tMYB5\tG1\tGENE1\tprediction_motif\tD\tM1\t1\t6\t+\t10\t1e-5\tfamily=MYB\n"
	if err := os.WriteFile(ev, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	edges, err := readEvidence(ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].TFFamily != "MYB" || *edges[0].PValue != 1e-5 {
		t.Fatalf("edges = %+v", edges)
	}

	pf := filepath.Join(dir, "promoters.fa")
	if err := os.WriteFile(pf, []byte(">G1|Chr1:100-109(+)\nACGTACGGGG\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	promoters, err := readPromoters(pf)
	if err != nil {
		t.Fatal(err)
	}
	p := promoters["G1"]
	if p == nil || p.Chrom != "Chr1" || p.Start != 100 || p.End != 109 || p.Strand != "+" || p.Sequence != "ACGTACGGGG" {
		t.Fatalf("promoter = %+v", p)
	}
}

func TestPromoterWindowIsPartOfHitKey(t *testing.T) {
	js := testJobState()
	js.edges = append(js.edges, Edge{
		TFID: "TF3", TFSymbol: "MYB7", TFFamily: "MYB", TargetID: "G1", TargetSymbol: "GENE1",
		EvidenceType: "prediction_motif", EvidenceLevel: "D", MotifID: "M1",
		PromoterStart: u64ptr(300), PromoterEnd: u64ptr(309),
		HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+", Score: f64ptr(9), PValue: f64ptr(1e-4),
	})
	summary := buildSummary(js.promoters, js.motifs, js.edges, js.annotationSource)
	if summary.MotifHits != 3 {
		t.Fatalf("motif hits = %d, want 3 distinct target/motif/window/hit keys", summary.MotifHits)
	}
	view, err := js.promoterView("G1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Hits) != 1 {
		t.Fatalf("G1 hits = %+v", view.Hits)
	}
	for _, tf := range view.Hits[0].TFs {
		if tf.ID == "TF3" {
			t.Fatalf("promoter view merged a hit from another promoter window: %+v", view.Hits[0])
		}
	}
	if view.Hits[0].PromoterStart == nil || *view.Hits[0].PromoterStart != 100 ||
		view.Hits[0].PromoterEnd == nil || *view.Hits[0].PromoterEnd != 109 {
		t.Fatalf("promoter window = %+v", view.Hits[0])
	}
}

func TestSiteEvidenceUsesFullKey(t *testing.T) {
	js := testJobState()
	for i := range js.edges {
		if js.edges[i].TargetID == "G1" {
			js.edges[i].PromoterStart = u64ptr(100)
			js.edges[i].PromoterEnd = u64ptr(109)
		}
	}
	res, err := js.siteEvidence(SiteEvidenceOptions{
		TargetID: "G1", MotifID: "M1",
		PromoterStart: u64ptr(100), PromoterEnd: u64ptr(109),
		HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("site rows = %+v", res)
	}
	for _, row := range res.Rows {
		if row.PromoterStart == nil || *row.PromoterStart != 100 {
			t.Fatalf("row = %+v", row)
		}
	}
	wrongWindow, err := js.siteEvidence(SiteEvidenceOptions{
		TargetID: "G1", MotifID: "M1",
		PromoterStart: u64ptr(300), PromoterEnd: u64ptr(309),
		HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+",
	})
	if err != nil {
		t.Fatal(err)
	}
	if wrongWindow.Total != 0 {
		t.Fatalf("wrong-window rows = %+v", wrongWindow)
	}
	oneTF, err := js.siteEvidence(SiteEvidenceOptions{
		TargetID: "G1", MotifID: "M1",
		PromoterStart: u64ptr(100), PromoterEnd: u64ptr(109),
		HitStart: u64ptr(1), HitEnd: u64ptr(6), Strand: "+", TFID: "TF1",
	})
	if err != nil || oneTF.Total != 1 || oneTF.Rows[0].TFID != "TF1" {
		t.Fatalf("TF-filtered site rows = %+v, err=%v", oneTF, err)
	}
}
