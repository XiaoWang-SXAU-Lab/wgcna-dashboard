package regutrace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTraceDonorsKeepsIDPairs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tf_motif_trace.tsv")
	text := "tf_id\tmotif_id\treference_donors\n" +
		"TF1\tM1\t[{\"tf_id\":\"D2\",\"tf_symbol\":\"ALPHA\",\"tf_family\":\"F1\"},{\"tf_id\":\"D1\",\"tf_symbol\":\"BETA\",\"tf_family\":\"F2\"}]\n" +
		"TF2\tM1\t[{\"tf_id\":\"D3\"}]\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	donors, err := readTraceDonors(path)
	if err != nil {
		t.Fatal(err)
	}
	got := donors[donorKey{TFID: "TF1", MotifID: "M1"}]
	if len(got) != 2 {
		t.Fatalf("TF1 donors = %+v", got)
	}
	byID := map[string]DonorInfo{}
	for _, d := range got {
		byID[d.TFID] = d
	}
	if byID["D1"].TFSymbol != "BETA" || byID["D1"].TFFamily != "F2" {
		t.Fatalf("D1 paired incorrectly: %+v", byID["D1"])
	}
	if byID["D2"].TFSymbol != "ALPHA" || byID["D2"].TFFamily != "F1" {
		t.Fatalf("D2 paired incorrectly: %+v", byID["D2"])
	}
	if got := donors[donorKey{TFID: "TF2", MotifID: "M1"}]; len(got) != 1 || got[0].TFID != "D3" || got[0].TFSymbol != "" {
		t.Fatalf("TF2 donors = %+v", got)
	}
}

func TestLoadDonorsFallsBackToTFMapIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tf_map.tsv")
	text := "tf_id\ttf_symbol\tfamily\tmotif_id\treference_tf_id\n" +
		"TF1\tTF1\tF\tM1\tD1;D2\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	donors, err := loadDonors("", path)
	if err != nil {
		t.Fatal(err)
	}
	got := donors[donorKey{TFID: "TF1", MotifID: "M1"}]
	if len(got) != 2 || got[0].TFID != "D1" || got[1].TFID != "D2" {
		t.Fatalf("fallback donors = %+v", got)
	}
}

func TestLoadDonorsKeepsTFMapAuthority(t *testing.T) {
	dir := t.TempDir()
	tfMapPath := filepath.Join(dir, "tf_map.tsv")
	tfMap := "tf_id\ttf_symbol\tfamily\tmotif_id\treference_tf_id\n" +
		"TF1\tTF1\tF\tM1\tD1\n" +
		"TF2\tTF2\tF\tM1\tD2\n"
	if err := os.WriteFile(tfMapPath, []byte(tfMap), 0o644); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(dir, "tf_motif_trace.tsv")
	trace := "tf_id\tmotif_id\treference_donors\n" +
		"TF1\tM1\t[{\"tf_id\":\"D9\",\"tf_symbol\":\"STALE\"},{\"tf_id\":\"D1\",\"tf_symbol\":\"GOOD\",\"tf_family\":\"FAM\"}]\n" +
		"TF3\tM1\t[{\"tf_id\":\"D3\",\"tf_symbol\":\"EXTRA\"}]\n"
	if err := os.WriteFile(tracePath, []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}
	donors, err := loadDonors(tracePath, tfMapPath)
	if err != nil {
		t.Fatal(err)
	}
	tf1 := donors[donorKey{TFID: "TF1", MotifID: "M1"}]
	if len(tf1) != 1 || tf1[0].TFID != "D1" || tf1[0].TFSymbol != "GOOD" || tf1[0].TFFamily != "FAM" {
		t.Fatalf("TF1 donors = %+v (stale/extra trace donors must be ignored)", tf1)
	}
	tf2 := donors[donorKey{TFID: "TF2", MotifID: "M1"}]
	if len(tf2) != 1 || tf2[0].TFID != "D2" || tf2[0].TFSymbol != "" {
		t.Fatalf("TF2 donors = %+v (partial trace coverage must keep tf-map ID)", tf2)
	}
	if _, ok := donors[donorKey{TFID: "TF3", MotifID: "M1"}]; ok {
		t.Fatalf("trace-only TF3 key must not be added: %+v", donors)
	}
}

func TestLoadDonorsWithoutTFMapIgnoresTrace(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "tf_motif_trace.tsv")
	trace := "tf_id\tmotif_id\treference_donors\nTF1\tM1\t[{\"tf_id\":\"D1\",\"tf_symbol\":\"DONOR\"}]\n"
	if err := os.WriteFile(tracePath, []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}
	donors, err := loadDonors(tracePath, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(donors) != 0 {
		t.Fatalf("trace must not create donors without tf-map authority: %+v", donors)
	}
}
