package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

func TestReportWithoutAllModuleIndexAndCurrentSchemas(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"step3_modules.json":     `{"genes":["a","b","c"],"colors":["pink","blue","grey"]}`,
		"step1_filtered.tsv":     "gene\ts1\ts2\na\t1\t2\n",
		"params.json":            `{"hub":{"select_module":"blue"},"filter":{"sample_percentage":0.5}}`,
		"step4_mtrait.json":      `{"modules":["pink","blue"],"traits":["t"],"cor":[[0.9],[0.2]],"p":[[0.001],[0.5]]}`,
		"step6_hub.json":         `{"module":"blue","trait_name":"t","hub_genes":[{"gene":"a","kme":0.9,"gs":0.1},{"gene":"b","kme":0.8,"gs":0.2}],"network":{"nodes":[]}}`,
		"overview_clusters.json": `{"method":"cmeans","k":3}`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	modules, err := reportModules(dir)
	if err != nil || !reflect.DeepEqual(modules, []string{"blue", "pink"}) {
		t.Fatalf("modules without detail_all: %v, %v", modules, err)
	}
	d := buildReportData(dir, filepath.Join(dir, "project"), modules, "kegg", "en")
	if d.HubCount != 2 || d.HasTrait || d.ClusterMethod != "cmeans" || d.ClusterK != 3 {
		t.Fatalf("wrong report facts: %+v", d)
	}
	d.KeyModules = significantModules(dir)
	d.Assets = map[string]bool{}
	// Enrichment is an interactive table/bubble panel; static kegg.png was retired.
	d.ModuleEnrichJSON = `{"modules":[{"module":"pink","go":[],"kegg":[{"id":"KEGG_TEST_TERM","description":"test pathway","gene_ratio":"1/2","bg_ratio":"1/10","p.adjust":0.01,"count":1,"gene_id":["a"]}]}],"anno":{}}`
	d.EnrichOK = true
	body := generateReportHTML(d)
	if !strings.Contains(body, `id="me-data"`) || !strings.Contains(body, "KEGG_TEST_TERM") || !strings.Contains(body, `data-kind="KEGG"`) {
		t.Fatal("interactive enrichment data and KEGG tab are not reflected in report")
	}
	if strings.Contains(body, `<img src="../result/06_module_detail/pink/enrichment/`) {
		t.Fatal("retired enrichment images must not create broken image links")
	}
	if strings.Contains(body, `<img src="../result/module_overview.png"`) || !strings.Contains(body, "Figure not generated:") {
		t.Fatal("missing assets should show an explicit notice, not a broken image")
	}
	reportDir := t.TempDir()
	if err := writeReport(reportDir, d); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(reportDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `src="echarts.min.js"`) {
		t.Fatal("partial reports still require the offline chart runtime")
	}
	if strings.Contains(body, `<img src="../result/module_overview_ALL.png"`) {
		t.Fatal("missing overview asset emitted as a broken image")
	}
}

func TestReportPreviewAndSavedStatus(t *testing.T) {
	root, bundle := t.TempDir(), t.TempDir()
	id := "report-preview-test"
	if err := os.MkdirAll(filepath.Join(root, id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bundle, "report"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bundle, "result"), 0o700); err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string]string{"report/index.html": "<html>ready</html>", "report/design.css": "body{}", "result/test.tsv": "gene\nA", "secret.txt": "private"} {
		if err := os.WriteFile(filepath.Join(bundle, filepath.FromSlash(file)), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	st := reportStatus{State: "done", Dir: bundle, PreviewURL: "/api/jobs/" + id + "/report/view/report/index.html", Done: []string{"blue"}}
	b, _ := json.Marshal(st)
	if err := os.WriteFile(filepath.Join(root, id, "report-latest.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reportMu.Lock(); delete(reportStates, id); reportMu.Unlock() })
	h := NewHandler(job.NewManager(root, "unused", 1))
	mux := http.NewServeMux()
	h.Register(mux)
	for _, file := range []string{"report/index.html", "report/design.css", "result/test.tsv"} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest("GET", "/api/jobs/"+id+"/report/view/"+file, nil))
		if r.Code != 200 {
			t.Fatalf("asset %s: %d", file, r.Code)
		}
	}
	for _, file := range []string{"secret.txt", "report/%2e%2e/secret.txt", "report/%2e%2e%5csecret.txt"} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest("GET", "/api/jobs/"+id+"/report/view/"+file, nil))
		if r.Code == 200 {
			t.Fatalf("served forbidden path %s", file)
		}
	}
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("GET", "/api/jobs/"+id+"/report/status", nil))
	if r.Code != 200 || !strings.Contains(r.Body.String(), "preview_url") {
		t.Fatal("latest report did not survive status reload")
	}
	copy := h.savedReportStatus(id)
	copy.Done[0] = "changed"
	if h.savedReportStatus(id).Done[0] != "blue" {
		t.Fatal("status response aliases mutable state")
	}
}

func TestReportStreamingSampleMatchesInteractiveSample(t *testing.T) {
	// A small standalone graph with both edge types.
	dir := t.TempDir()
	for name, data := range map[string]string{"network_nodes.tsv": "gene\tmodule\tdegree\na\tblue\t2\nb\tblue\t2\nc\tpink\t2\nd\tpink\t2\n", "network_edges.tsv": "source\ttarget\tweight\na\tb\t0.8\nc\td\t0.7\na\tc\t0.6\nb\td\t0.6\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	full, err := buildNetworkIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, err := sampleNetwork(full, "combined", 2000, 20, 8, 20, 300, true)
	if err != nil {
		t.Fatal(err)
	}
	sn, se, err := reportNetworkSample(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nodes, sn) || !reflect.DeepEqual(edges, se) {
		t.Fatalf("streamed sample differs: %v %v vs %v %v", nodes, edges, sn, se)
	}
}

func TestReportProjectNames(t *testing.T) {
	for _, name := range []string{"..", "CON", "AUX.txt", "a/b", "a\\b", "bad.", "bad "} {
		if reportNameOK(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	if !reportNameOK("project-2026") {
		t.Fatal("rejected a normal project name")
	}
}
