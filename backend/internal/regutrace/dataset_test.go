package regutrace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatasetSummaryQueryAndPromoter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.tsv")
	rows := "tf_id\ttf_symbol\ttarget_id\ttarget_symbol\tevidence_type\tevidence_level\tmotif_id\thit_start\thit_end\tstrand\tscore\tp_value\tnotes\n" +
		"TF1\tMYB5\tG1\tGENE1\tprediction_motif\tD\tM1\t1\t6\t+\t10\t1e-5\tfamily=MYB\n" +
		"TF2\tMYB6\tG1\tGENE1\tprediction_motif\tD\tM1\t1\t6\t+\t10\t1e-5\tfamily=MYB\n" +
		"TF1\tMYB5\tG2\tGENE2\tprediction_motif\tD\tM1\t1\t6\t-\t9\t1e-4\tfamily=MYB\n"
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	js := testJobState()
	data, err := buildDataset(path, js.motifs, js.annotations, 2, js.annotationSource, 2)
	if err != nil {
		t.Fatal(err)
	}
	if data.summary.MotifHits != 2 || data.summary.TFTargetPairs != 3 || data.summary.TFTargetEdges != 3 {
		t.Fatalf("summary = %+v", data.summary)
	}
	if data.summary.UniqueTargets != 2 || data.summary.UniqueTFs != 2 || data.summary.UniqueMotifs != 1 {
		t.Fatalf("summary counts = %+v", data.summary)
	}
	if len(data.summary.TopTFs) != 2 || data.summary.TopTFs[0].ID != "TF1" || data.summary.TopTFs[0].Targets != 2 {
		t.Fatalf("top TFs = %+v", data.summary.TopTFs)
	}
	js.data = data
	js.edges = nil

	res := js.query(QueryOptions{Direction: "tf", Query: "MYB5", Limit: 10})
	if res.Total != 2 || len(res.Rows) != 2 {
		t.Fatalf("query = %+v", res)
	}
	byTarget := js.query(QueryOptions{Direction: "target", Query: "GENE1", Limit: 10})
	if byTarget.Total != 2 {
		t.Fatalf("target query = %+v", byTarget)
	}
	all := js.query(QueryOptions{Direction: "tf", Limit: 10})
	if all.Total != 3 || len(all.Rows) != 3 {
		t.Fatalf("unfiltered query = %+v", all)
	}
	motifOnly := js.query(QueryOptions{Motif: "M1", Limit: 10})
	if motifOnly.Total != 3 || len(motifOnly.Rows) != 3 {
		t.Fatalf("motif query = %+v", motifOnly)
	}
	if all.Rows[0].PValue == nil || *all.Rows[0].PValue != 1e-5 {
		t.Fatalf("query sort = %+v", all.Rows)
	}

	view, err := js.promoterView("G1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Hits) != 1 || len(view.Hits[0].TFs) != 2 || view.Hits[0].Site != "ACGTAC" {
		t.Fatalf("promoter view = %+v", view)
	}
}
