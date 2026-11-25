package regutrace

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func closeFloat(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestHypergeomSF(t *testing.T) {
	cases := []struct {
		observed, draws, successes, population int
		want                                   float64
	}{
		{5, 20, 10, 100, 0.025464546427043124},
		{3, 10, 5, 50, 0.0482603031962091},
		{0, 10, 0, 50, 1},
		{6, 20, 10, 100, 0.0039330764667913545},
	}
	for _, tc := range cases {
		got := hypergeomSF(tc.observed, tc.draws, tc.successes, tc.population)
		if math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("hypergeomSF(%d,%d,%d,%d)=%g want %g", tc.observed, tc.draws, tc.successes, tc.population, got, tc.want)
		}
	}
}

func TestBHQValues(t *testing.T) {
	got := bhQValues([]float64{0.01, 0.04, 0.03, 0.002})
	want := []float64{0.02, 0.04, 0.04, 0.008}
	for i := range want {
		if !closeFloat(got[i], want[i], 1e-12) {
			t.Fatalf("bhQValues[%d]=%g want %g (all=%v)", i, got[i], want[i], got)
		}
	}
	if q := bhQValues(nil); len(q) != 0 {
		t.Fatalf("empty BH result = %v", q)
	}
}

func TestMappingStatusHelpers(t *testing.T) {
	if got := normalizeMappingStatus("strict", "rbh"); got != "strict_rbh" {
		t.Fatalf("normalize strict = %q", got)
	}
	if got := normalizeMappingStatus("", "dbd_transfer_calibrated"); got != "dbd_calibrated" {
		t.Fatalf("normalize calibrated = %q", got)
	}
	if got := bestStatus([]string{"dbd_ambiguous", "dbd_exploratory", "strict_rbh"}); got != "strict_rbh" {
		t.Fatalf("bestStatus = %q", got)
	}
	if !statusAllowed("balanced", "dbd_calibrated") || statusAllowed("balanced", "dbd_exploratory") {
		t.Fatalf("statusAllowed balanced policy is wrong")
	}
}

func siteJob(t *testing.T, mode string) (*jobState, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.tsv")
	rows := "motif_ids\tgene_id\thit_start\thit_end\tscore\tp_value\n" +
		"M1\tG1\t10\t20\t9.0\t1e-5\n" +
		"M1\tG1\t30\t40\t8.0\t1e-6\n" +
		"M1\tG2\t10\t20\t7.0\t2e-5\n" +
		"M2\tG3\t10\t20\t9.5\t1e-7\n"
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	mappings := map[string][]TFMappingRow{
		"TF1": {{TFID: "TF1", MotifID: "M1", Status: "strict_rbh"}},
		"TF2": {{TFID: "TF2", MotifID: "M2", Status: "dbd_exploratory"}},
	}
	ds, err := buildSiteDataset(path, map[string]MotifInfo{
		"M1": {ID: "M1", Name: "motif one"}, "M2": {ID: "M2", Name: "motif two"},
	}, map[string]AnnotationEntry{
		"G1": {Symbol: "gene one"}, "G2": {Symbol: "gene two"}, "G3": {Symbol: "gene three"},
	}, 3, "test", mappings, false)
	if err != nil {
		t.Fatal(err)
	}
	js := &jobState{
		data: ds, motifs: map[string]MotifInfo{"M1": {ID: "M1", Name: "motif one"}, "M2": {ID: "M2", Name: "motif two"}},
		annotations: map[string]AnnotationEntry{"G1": {Symbol: "gene one"}, "G2": {Symbol: "gene two"}, "G3": {Symbol: "gene three"}},
		promoters: map[string]*Promoter{
			"G1": {GeneID: "G1"}, "G2": {GeneID: "G2"}, "G3": {GeneID: "G3"},
		},
		phase4: &phase4Metadata{
			Mappings: mappings,
			MotifTFs: map[string][]string{"M1": {"TF1"}, "M2": {"TF2"}},
		},
		Job: Job{Summary: &Summary{TFCatalogue: []TFCatalogueItem{{ID: "TF1"}, {ID: "TF2"}}}},
	}
	js.Params.MappingMode = mode
	return js, path
}

func TestSiteTFTargetsAndCache(t *testing.T) {
	js, path := siteJob(t, "exploratory")
	res, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || len(res.Rows) != 1 || res.Rows[0].TargetID != "G1" {
		t.Fatalf("unexpected first page: %+v", res)
	}
	if res.Rows[0].BestPValue == nil || *res.Rows[0].BestPValue != 1e-6 || res.Rows[0].SiteCount != 2 {
		t.Fatalf("unexpected aggregate: %+v", res.Rows[0])
	}
	page, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 1, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 1 || page.Rows[0].TargetID != "G2" {
		t.Fatalf("unexpected second page: %+v", page)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 5}); err != nil {
		t.Fatalf("cached query should not reread the site table: %v", err)
	}
}

func TestSiteTFTargetsModeFilter(t *testing.T) {
	js, _ := siteJob(t, "strict")
	res, err := js.siteTFTargets(TFTargetOptions{TFID: "TF2", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatalf("strict mode should hide exploratory-only TF: %+v", res)
	}
	js.Params.MappingMode = "exploratory"
	res, err = js.siteTFTargets(TFTargetOptions{TFID: "TF2", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Rows[0].TargetID != "G3" {
		t.Fatalf("exploratory mode should expose TF2 target: %+v", res)
	}
}

func TestSiteTargetRegulators(t *testing.T) {
	js, _ := siteJob(t, "exploratory")
	res, err := js.siteTargetRegulators("G1", 10, 0, "", "", MotifPolicyAll, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Rows[0].TFID != "TF1" || res.Rows[0].SiteCount != 2 {
		t.Fatalf("unexpected regulator aggregation: %+v", res)
	}
	filtered, err := js.siteTargetRegulators("G1", 10, 0, "", "dbd_exploratory", MotifPolicyAll, 0)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 0 {
		t.Fatalf("status filter should exclude TF1: %+v", filtered)
	}
}
