package regutrace

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func policyJob(rows []TFMappingRow, motifs map[string]MotifInfo) *jobState {
	js := &jobState{
		motifs: motifs,
		phase4: &phase4Metadata{
			Mappings: map[string][]TFMappingRow{"TF1": rows},
			MotifTFs: map[string][]string{},
		},
	}
	return js
}

func policyRow(motif string, status string, start, end float64, coverage, identity, bitscore float64, spec string) TFMappingRow {
	return TFMappingRow{
		TFID: "TF1", MotifID: motif, Status: status, DomainMode: "full_architecture",
		ExtractionFamily: "AP2", QueryDBDStart: start, QueryDBDEnd: end,
		QueryDBDCoverage: "0", DonorDBDCoverage: "0", DBDIdentity: "0",
		SpecificityGroups: spec, DBDBitscore: bitscore,
	}
}

func policyTestRow(motif string, status string, start, end float64, coverage, identity, bitscore float64, pwmHash, spec string) TFMappingRow {
	row := policyRow(motif, status, start, end, coverage, identity, bitscore, spec)
	row.QueryDBDCoverage = formatFloat(coverage)
	row.DonorDBDCoverage = formatFloat(coverage)
	row.DBDIdentity = formatFloat(identity)
	_ = pwmHash // exact-PWM tests supply hashes through the motif metadata map
	return row
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func TestMotifPolicyBestPerCoordinateGroup(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.95, 0.70, 100, "h1", ""),
		policyTestRow("M2", "dbd_exploratory", 20, 100, 0.90, 0.99, 200, "h2", ""),
		policyTestRow("M3", "dbd_exploratory", 300, 400, 0.80, 0.90, 90, "h3", ""),
		policyTestRow("M4", "dbd_ambiguous", 300, 400, 0.99, 0.99, 999, "h4", ""),
	}
	js := policyJob(rows, nil)
	got := js.policyMotifs("TF1", MotifPolicyBest)
	want := map[string]bool{"M1": true, "M3": true}
	if len(got) != len(want) {
		t.Fatalf("best motifs = %v, want %v", got, want)
	}
	for _, motif := range got {
		if !want[motif] {
			t.Fatalf("best motifs = %v, want %v", got, want)
		}
	}
	info := js.policySelection("TF1", MotifPolicyBest)
	if info.SelectedMotifs != 2 || info.CoordinateAvailable != true || info.Fallback {
		t.Fatalf("unexpected best policy info: %+v", info)
	}
	if all := js.policyMotifs("TF1", MotifPolicyAll); len(all) != 4 {
		t.Fatalf("all policy must retain ambiguous evidence: %v", all)
	}
}

func TestMotifPolicyPreservesStrictAndCalibrated(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("S1", "strict_rbh", 10, 100, 0.10, 0.10, 1, "hs", ""),
		policyTestRow("C1", "dbd_calibrated", 10, 100, 0.10, 0.10, 1, "hc", ""),
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.99, 0.99, 999, "h1", "g1"),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.50, 0.50, 5, "h2", "g1"),
	}
	js := policyJob(rows, nil)
	// Representative grouping runs on the 0.80 group table only; the row-level
	// specificity_groups column (0.90 calibration) is deliberately ignored.
	js.phase4.SpecificityGroups = map[string][]string{"M1": {"g1"}, "M2": {"g1"}}
	for _, policy := range []MotifPolicy{MotifPolicyBest, MotifPolicyReps} {
		got := js.policyMotifs("TF1", policy)
		want := map[string]bool{"S1": true, "C1": true, "M1": true}
		if len(got) != len(want) {
			t.Fatalf("%s motifs = %v, want fixed rows plus best exploratory", policy, got)
		}
		for _, motif := range got {
			if !want[motif] {
				t.Fatalf("%s motifs = %v, want %v", policy, got, want)
			}
		}
	}
}

func TestMotifPolicyRepresentativeSpecificityGroups(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.95, 0.80, 100, "h1", "g1|g2"),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.90, 0.70, 90, "h2", "g1"),
		policyTestRow("M3", "dbd_exploratory", 10, 100, 0.80, 0.60, 80, "h3", "g3"),
		policyTestRow("M4", "dbd_ambiguous", 10, 100, 0.99, 0.99, 999, "h4", "g4"),
	}
	js := policyJob(rows, nil)
	js.phase4.SpecificityGroups = map[string][]string{"M1": {"g1", "g2"}, "M2": {"g1"}, "M3": {"g3"}}
	got := js.policyMotifs("TF1", MotifPolicyReps)
	want := map[string]bool{"M1": true, "M3": true}
	if len(got) != len(want) {
		t.Fatalf("reps motifs = %v, want %v", got, want)
	}
	for _, motif := range got {
		if !want[motif] {
			t.Fatalf("reps motifs = %v, want %v", got, want)
		}
	}
	best := js.policyMotifs("TF1", MotifPolicyBest)
	if len(best) != 1 || best[0] != "M1" {
		t.Fatalf("best motifs = %v, want M1 only", best)
	}
}

// The tf_map specificity_groups column is built from the 0.90 calibration
// grouping. Grouping reps on it would apply two thresholds inside one policy,
// so it must never steer representative selection.
func TestMotifPolicyIgnoresRowSpecificityColumn(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.95, 0.80, 100, "h1", "g1"),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.90, 0.70, 90, "h2", "g1"),
	}
	js := policyJob(rows, nil)
	got := js.policyMotifs("TF1", MotifPolicyReps)
	if len(got) != 2 || got[0] != "M1" || got[1] != "M2" {
		t.Fatalf("reps motifs = %v, want one per motif when no 0.80 group exists", got)
	}
	if js.specificityAvailable(rows) {
		t.Fatal("row-level specificity_groups must not count as 0.80 group data")
	}
}

func TestMotifPolicyUsesRegistryGroup080(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.95, 0.80, 100, "h1", "g1|g2"),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.90, 0.70, 90, "h2", "g1"),
	}
	js := policyJob(rows, nil)
	js.phase4.SpecificityGroups = map[string][]string{"M1": {"g1"}, "M2": {"g2"}}
	got := js.policyMotifs("TF1", MotifPolicyReps)
	if len(got) != 2 || got[0] != "M1" || got[1] != "M2" {
		t.Fatalf("reps motifs = %v, want one representative for each registry group", got)
	}
}

func TestMotifPolicyCoordinateFallback(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 0, 0, 0.95, 0.80, 100, "h1", "g1"),
		policyTestRow("M2", "dbd_exploratory", 0, 0, 0.90, 0.70, 90, "h2", "g2"),
	}
	js := policyJob(rows, nil)
	best := js.policyMotifs("TF1", MotifPolicyBest)
	if len(best) != 1 || best[0] != "M1" {
		t.Fatalf("fallback best = %v, want one opaque group", best)
	}
	info := js.policySelection("TF1", MotifPolicyBest)
	if !info.Fallback || info.CoordinateAvailable {
		t.Fatalf("expected coordinate fallback: %+v", info)
	}
	reps := js.policyMotifs("TF1", MotifPolicyReps)
	if len(reps) != 1 || reps[0] != "M1" {
		t.Fatalf("no-coordinate representative fallback should select one best row: %v", reps)
	}
}

func TestMotifPolicyExcludesUncoordinatedExploratory(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h1", ""),
		policyTestRow("M2", "dbd_exploratory", 0, 0, 0.99, 0.99, 999, "h2", ""),
	}
	js := policyJob(rows, nil)
	best := js.policyMotifs("TF1", MotifPolicyBest)
	if len(best) != 1 || best[0] != "M1" {
		t.Fatalf("uncoordinated exploratory motif must not enter best: %v", best)
	}
	info := js.policySelection("TF1", MotifPolicyBest)
	if !info.CoordinateAvailable || info.UncoordinatedMotifs != 1 {
		t.Fatalf("unexpected coordinate coverage info: %+v", info)
	}
	if all := js.policyMotifs("TF1", MotifPolicyAll); len(all) != 2 {
		t.Fatalf("all policy must retain uncoordinated evidence: %v", all)
	}
}

func TestMotifPolicyExactPWMDedup(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.95, 0.80, 100, "same-pwm", ""),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.90, 0.70, 90, "same-pwm", ""),
		policyTestRow("M3", "dbd_exploratory", 300, 400, 0.80, 0.60, 80, "other-pwm", ""),
	}
	js := policyJob(rows, map[string]MotifInfo{
		"M1": {ID: "M1", PWMHash: "same-pwm"},
		"M2": {ID: "M2", PWMHash: "same-pwm"},
		"M3": {ID: "M3", PWMHash: "other-pwm"},
	})
	got := js.policyMotifs("TF1", MotifPolicyBest)
	if len(got) != 2 || got[0] != "M1" || got[1] != "M3" {
		t.Fatalf("exact-PWM best = %v, want M1 and M3", got)
	}
}

func TestMotifPolicyTieBreaksByMotifID(t *testing.T) {
	rows := []TFMappingRow{
		policyTestRow("M9", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h9", ""),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h2", ""),
	}
	js := policyJob(rows, nil)
	got := js.policyMotifs("TF1", MotifPolicyBest)
	if len(got) != 1 || got[0] != "M2" {
		t.Fatalf("tie-break = %v, want lower motif ID", got)
	}
}

func TestMotifPolicyInvalidValue(t *testing.T) {
	if _, err := normalizeMotifPolicy("all-motifs"); err == nil {
		t.Fatal("invalid policy should be rejected")
	}
}

func policySiteJob(t *testing.T) *jobState {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.tsv")
	rows := "motif_ids\tgene_id\thit_start\thit_end\tscore\tp_value\n" +
		"M1\tG1\t10\t20\t9.0\t1e-5\n" +
		"M2\tG2\t10\t20\t8.0\t1e-6\n" +
		"M5\tG5\t10\t20\t8.5\t2e-6\n" +
		"M3\tG3\t10\t20\t7.0\t2e-5\n" +
		"M4\tG4\t10\t20\t6.0\t3e-5\n"
	if err := os.WriteFile(path, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	mappings := map[string][]TFMappingRow{
		"TF1": {
			policyTestRow("M1", "strict_rbh", 10, 100, 0.10, 0.10, 1, "h1", ""),
			policyTestRow("M2", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h2", ""),
			policyTestRow("M5", "dbd_exploratory", 10, 100, 0.90, 0.80, 85, "h5", ""),
			policyTestRow("M3", "dbd_exploratory", 300, 400, 0.80, 0.80, 80, "h3", ""),
			policyTestRow("M4", "dbd_ambiguous", 300, 400, 0.99, 0.99, 999, "h4", ""),
		},
	}
	motifs := map[string]MotifInfo{
		"M1": {ID: "M1", Name: "motif one"}, "M2": {ID: "M2", Name: "motif two"},
		"M3": {ID: "M3", Name: "motif three"}, "M4": {ID: "M4", Name: "motif four"},
		"M5": {ID: "M5", Name: "motif five"},
	}
	ds, err := buildSiteDataset(path, motifs, map[string]AnnotationEntry{
		"G1": {Symbol: "gene one"}, "G2": {Symbol: "gene two"}, "G3": {Symbol: "gene three"},
		"G4": {Symbol: "gene four"}, "G5": {Symbol: "gene five"},
	}, 5, "test", mappings, false)
	if err != nil {
		t.Fatal(err)
	}
	js := &jobState{
		data: ds, motifs: motifs,
		annotations: map[string]AnnotationEntry{
			"G1": {Symbol: "gene one"}, "G2": {Symbol: "gene two"}, "G3": {Symbol: "gene three"},
			"G4": {Symbol: "gene four"}, "G5": {Symbol: "gene five"},
		},
		promoters: map[string]*Promoter{
			"G1": {GeneID: "G1"}, "G2": {GeneID: "G2"}, "G3": {GeneID: "G3"},
			"G4": {GeneID: "G4"}, "G5": {GeneID: "G5"},
		},
		phase4: &phase4Metadata{
			Mappings: mappings,
			MotifTFs: map[string][]string{"M1": {"TF1"}, "M2": {"TF1"}, "M3": {"TF1"}, "M4": {"TF1"}, "M5": {"TF1"}},
		},
		Job: Job{Summary: &Summary{TFCatalogue: []TFCatalogueItem{{ID: "TF1"}}}},
	}
	js.Params.MappingMode = "exploratory"
	return js
}

func TestSiteTFTargetsMotifPolicyFiltering(t *testing.T) {
	js := policySiteJob(t)
	all, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 5 {
		t.Fatalf("all policy total = %d, want 5 (%+v)", all.Total, all.Rows)
	}
	best, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "best"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"G1": true, "G3": true, "G5": true}
	if best.Total != len(want) {
		t.Fatalf("best policy total = %d, want %d (%+v)", best.Total, len(want), best.Rows)
	}
	for _, row := range best.Rows {
		if !want[row.TargetID] {
			t.Fatalf("unexpected best-policy target %s", row.TargetID)
		}
	}
	if best.Policy == nil || best.Policy.SelectedMotifs != 3 || best.Policy.FixedMappings != 1 {
		t.Fatalf("unexpected best policy metadata: %+v", best.Policy)
	}
	reps, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "reps"})
	if err != nil {
		t.Fatal(err)
	}
	// Without specificity metadata, exact PWMs are their own fallback groups.
	wantReps := map[string]bool{"G1": true, "G2": true, "G3": true, "G5": true}
	if reps.Total != len(wantReps) {
		t.Fatalf("reps policy total = %d, want %d", reps.Total, len(wantReps))
	}
	if reps.Policy == nil || !reps.Policy.Fallback {
		t.Fatalf("reps should report the specificity fallback: %+v", reps.Policy)
	}
	if _, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "bogus"}); err == nil {
		t.Fatal("invalid motif policy should be rejected")
	}
	motifFiltered, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Motif: "M2", Policy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if motifFiltered.Total != 1 || motifFiltered.Rows[0].TargetID != "G2" {
		t.Fatalf("motif filter should restrict site targets: %+v", motifFiltered.Rows)
	}
}

func TestSiteTargetRegulatorsMotifPolicy(t *testing.T) {
	js := policySiteJob(t)
	all, err := js.siteTargetRegulators("G4", 10, 0, "", "", MotifPolicyAll, 0)
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 1 || all.Rows[0].TFID != "TF1" {
		t.Fatalf("all policy should expose ambiguous regulator: %+v", all)
	}
	best, err := js.siteTargetRegulators("G4", 10, 0, "", "", MotifPolicyBest, 0)
	if err != nil {
		t.Fatal(err)
	}
	if best.Total != 0 {
		t.Fatalf("best policy should exclude ambiguous-only regulator: %+v", best)
	}
	strict, err := js.siteTargetRegulators("G1", 10, 0, "", "", MotifPolicyBest, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strict.Total != 1 || strict.Rows[0].TFID != "TF1" {
		t.Fatalf("best policy should preserve strict mapping: %+v", strict)
	}
}

// A single marginal site in a 1 kb promoter is not evidence: at p=1e-4 one
// motif already reaches chance-level promoter coverage. min_sites requires
// several sites of one and the same motif, and deliberately does not let sites
// from different motifs add up.
func TestSiteTFTargetsMinSitesAdmission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.tsv")
	sites := "motif_ids\tgene_id\thit_start\thit_end\tscore\tp_value\n" +
		"M1\tGA\t10\t20\t9.0\t1e-5\n" +
		"M1\tGA\t100\t110\t8.5\t2e-5\n" +
		"M2\tGA\t300\t310\t8.0\t3e-5\n" +
		"M2\tGA\t400\t410\t7.5\t4e-5\n" +
		"M1\tGB\t50\t60\t9.0\t1e-5\n" +
		"M2\tGB\t55\t65\t8.0\t2e-5\n"
	if err := os.WriteFile(path, []byte(sites), 0o644); err != nil {
		t.Fatal(err)
	}
	mappings := map[string][]TFMappingRow{"TF1": {
		policyTestRow("M1", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h1", ""),
		policyTestRow("M2", "dbd_exploratory", 10, 100, 0.70, 0.70, 70, "h2", ""),
	}}
	motifs := map[string]MotifInfo{"M1": {ID: "M1", Name: "one"}, "M2": {ID: "M2", Name: "two"}}
	annotations := map[string]AnnotationEntry{"GA": {Symbol: "GA"}, "GB": {Symbol: "GB"}}
	ds, err := buildSiteDataset(path, motifs, annotations, 2, "test", mappings, false)
	if err != nil {
		t.Fatal(err)
	}
	js := &jobState{
		data: ds, motifs: motifs, annotations: annotations,
		promoters: map[string]*Promoter{"GA": {GeneID: "GA"}, "GB": {GeneID: "GB"}},
		phase4: &phase4Metadata{
			Mappings: mappings,
			MotifTFs: map[string][]string{"M1": {"TF1"}, "M2": {"TF1"}},
		},
		Job: Job{Summary: &Summary{TFCatalogue: []TFCatalogueItem{{ID: "TF1"}}}},
	}
	js.Params.MappingMode = "exploratory"

	unfiltered, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if unfiltered.Total != 2 {
		t.Fatalf("unfiltered total = %d, want 2", unfiltered.Total)
	}

	filtered, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "all", MinSites: 2})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Rows[0].TargetID != "GA" {
		t.Fatalf("min_sites=2 should keep only GA (two sites of one motif): %+v", filtered.Rows)
	}
	if filtered.Rows[0].MaxMotifSites != 2 {
		t.Fatalf("GA max_motif_sites = %d, want 2", filtered.Rows[0].MaxMotifSites)
	}

	// The filter must not poison the cache for the unfiltered query.
	again, err := js.siteTFTargets(TFTargetOptions{TFID: "TF1", Limit: 10, Policy: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Total != 2 {
		t.Fatalf("cached unfiltered total = %d, want 2", again.Total)
	}
}

// Site-only jobs must expose a top-TF list: the overview uses it to open a TF,
// and parse.go only fills TopTFs for the legacy edge dataset.
func TestSiteSummaryTopTFs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.tsv")
	sites := "motif_ids\tgene_id\thit_start\thit_end\tscore\tp_value\n" +
		"M1\tG1\t10\t20\t9.0\t1e-5\n" +
		"M1\tG2\t10\t20\t9.0\t1e-5\n" +
		"M2\tG3\t10\t20\t8.0\t2e-5\n" +
		"M2\tG4\t10\t20\t8.0\t2e-5\n" +
		"M2\tG5\t10\t20\t8.0\t2e-5\n"
	if err := os.WriteFile(path, []byte(sites), 0o644); err != nil {
		t.Fatal(err)
	}
	mappings := map[string][]TFMappingRow{
		// TF1 covers two motifs (2 + 3 targets); TF2 covers one motif (3 targets).
		"TF1": {
			policyTestRow("M1", "strict_rbh", 10, 100, 0.10, 0.10, 1, "h1", ""),
			policyTestRow("M2", "dbd_exploratory", 10, 100, 0.80, 0.80, 80, "h2", ""),
		},
		"TF2": {policyTestRow("M1", "strict_rbh", 10, 100, 0.10, 0.10, 1, "h1", "")},
	}
	ds, err := buildSiteDataset(path, map[string]MotifInfo{"M1": {ID: "M1"}, "M2": {ID: "M2"}},
		map[string]AnnotationEntry{}, 5, "test", mappings, false)
	if err != nil {
		t.Fatal(err)
	}
	if ds.summary == nil || len(ds.summary.TopTFs) != 2 {
		t.Fatalf("site summary must expose a top-TF list: %+v", ds.summary)
	}
	if ds.summary.TopTFs[0].ID != "TF1" || ds.summary.TopTFs[0].Targets != 5 {
		t.Fatalf("top TF = %+v, want TF1 with 5 targets", ds.summary.TopTFs[0])
	}
	if ds.summary.TopTFs[1].ID != "TF2" || ds.summary.TopTFs[1].Targets != 2 {
		t.Fatalf("second TF = %+v, want TF2 with 2 targets", ds.summary.TopTFs[1])
	}
}
