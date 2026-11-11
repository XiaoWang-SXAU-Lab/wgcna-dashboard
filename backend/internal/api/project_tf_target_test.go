package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTFTargetPriorUploadAndModuleQuery(t *testing.T) {
	bin := os.Getenv("WGCNA_REGUTRACE")
	if bin == "" {
		t.Skip("set WGCNA_REGUTRACE to run the prior integration test")
	}
	t.Setenv("WGCNA_REGUTRACE", bin)
	worker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, mux := projectHarness(t, worker, t.TempDir())
	created := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": t.TempDir(), "name": "prior test"}, 201)
	id := created.State.ID
	tsv := "tf_id\ttf_symbol\ttarget_id\ttarget_symbol\tevidence_type\tevidence_level\tmethod\tsource\tpmid\tcondition\tpromoter_start\tpromoter_end\tmotif_id\thit_start\thit_end\tstrand\tscore\tp_value\tnotes\n" +
		"TF1\tTF1\tG1\tGENE1\tprediction_motif\tD\tJASPAR\t\t\theat\t101\t1100\tMA1\t10\t16\t+\t8.5\t1e-6\ta\n" +
		"TF2\tTF2\tG2\tGENE2\tprediction_motif\tD\tJASPAR\t\t\theat\t201\t1200\tMA1\t15\t21\t+\t6.0\t3e-4\tb\n"
	state := projectUpload(t, mux, id, "tf_target", tsv)
	if !strings.HasSuffix(state.State.Inputs["tf_target"].Path, "tf_target.db") {
		t.Fatalf("tf_target input path = %q", state.State.Inputs["tf_target"].Path)
	}

	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(map[string]any{"genes": []string{"G1", "G2"}})
	r := httptest.NewRequest("POST", "/api/projects/"+id+"/tf-target/upstream", &body)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("upstream: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		GeneCount int                   `json:"gene_count"`
		Rows      []tfTargetUpstreamRow `json:"rows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.GeneCount != 2 || len(out.Rows) != 2 {
		t.Fatalf("upstream = %+v", out)
	}
	if out.Rows[0].ModuleTargets != 1 {
		t.Fatalf("module targets = %+v", out.Rows[0])
	}

	var embedded bytes.Buffer
	_ = json.NewEncoder(&embedded).Encode(map[string]any{"genes": []string{"G1\nG2"}})
	r = httptest.NewRequest("POST", "/api/projects/"+id+"/tf-target/upstream", &embedded)
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("embedded newline: %d %s", w.Code, w.Body.String())
	}

	r = httptest.NewRequest("POST", "/api/projects/"+id+"/tf-target/upstream", strings.NewReader(`{"genes":["G1"]}garbage`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON: %d %s", w.Code, w.Body.String())
	}
}

func TestParsePriorNumbers(t *testing.T) {
	if _, err := parsePriorCount("not-a-count", "n_sites", 2); err == nil {
		t.Fatal("malformed count accepted")
	}
	if _, err := parsePriorCount("-1", "n_sites", 2); err == nil {
		t.Fatal("negative count accepted")
	}
	p, err := parsePriorProbability("0", 2)
	if err != nil || p == nil || *p != 0 {
		t.Fatalf("zero p-value = %v, %v", p, err)
	}
	if _, err := parsePriorProbability("NaN", 2); err == nil {
		t.Fatal("NaN p-value accepted")
	}
	if p, err := parsePriorProbability("", 2); err != nil || p != nil {
		t.Fatalf("empty p-value = %v, %v", p, err)
	}
}

func TestValidateSQLitePrior(t *testing.T) {
	bin := os.Getenv("WGCNA_REGUTRACE")
	if bin == "" {
		t.Skip("set WGCNA_REGUTRACE to run the prior validation test")
	}
	dir := t.TempDir()
	tsv := filepath.Join(dir, "evidence.tsv")
	db := filepath.Join(dir, "evidence.db")
	data := "tf_id\ttf_symbol\ttarget_id\ttarget_symbol\tevidence_type\tevidence_level\tmethod\tsource\tpmid\tcondition\tpromoter_start\tpromoter_end\tmotif_id\thit_start\thit_end\tstrand\tscore\tp_value\tnotes\n" +
		"TF1\tTF1\tG1\tGENE1\tprediction_motif\tD\tJASPAR\t\t\theat\t101\t1100\tMA1\t10\t16\t+\t8.5\t1e-6\ta\n"
	if err := os.WriteFile(tsv, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "convert", "--input", tsv, "--out", db, "--format", "sqlite").CombinedOutput(); err != nil {
		t.Fatalf("convert: %v\n%s", err, out)
	}
	if err := validateSQLitePrior(context.Background(), bin, db); err != nil {
		t.Fatalf("valid prior rejected: %v", err)
	}
	bad := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(bad, []byte("SQLite format 3\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSQLitePrior(context.Background(), bin, bad); err == nil {
		t.Fatal("truncated SQLite accepted as prior")
	}
}
