package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/inputcheck"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

func inputRequest(t *testing.T, path string, files map[string]string) *http.Request {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for kind, body := range files {
		p, err := w.CreateFormFile(kind, kind+".tsv")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write([]byte(body))
	}
	_ = w.WriteField("data_type", "count")
	_ = w.Close()
	r := httptest.NewRequest("POST", path, &b)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}
func TestInputPreflightAndAuthoritativeUpload(t *testing.T) {
	root := t.TempDir()
	m := job.NewManager(root, "unused", 1)
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.Register(mux)
	expr := "gene\t a \tb\tc\ng1\t1\t2\t3\n"
	for _, tc := range []struct {
		trait string
		valid bool
	}{{"ID\tt\nc\t3\na\t1\nb\t2\n", true}, {"ID\tt\na\t1\nb\t2\nx\t3\n", false}, {"ID\tt\na\t1\nb\tNA\nc\t3\n", false}} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, inputRequest(t, "/api/inputs/validate", map[string]string{"expr": expr, "trait": tc.trait}))
		var report inputcheck.Report
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &report) != nil || report.Valid != tc.valid {
			t.Fatalf("%d %s", res.Code, res.Body)
		}
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("preflight created a job")
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, inputRequest(t, "/api/jobs", map[string]string{"expr": "gene\ta\tb\tc\ng\t1\tNaN\t3\n"}))
	if res.Code != 400 {
		t.Fatalf("invalid upload: %d %s", res.Code, res.Body)
	}
	id, err := m.Create([]byte(expr), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	res = httptest.NewRecorder()
	mux.ServeHTTP(res, inputRequest(t, "/api/jobs/"+id+"/trait", map[string]string{"trait": "ID\tt\na\t1\nb\t2\nx\t3\n"}))
	if res.Code != 400 {
		t.Fatalf("mismatched direct upload: %d %s", res.Code, res.Body)
	}
}
func TestBundledDemoPassesInputChecks(t *testing.T) {
	h := NewHandler(job.NewManager(t.TempDir(), "unused", 1))
	res := httptest.NewRecorder()
	h.loadDemo(res, httptest.NewRequest("POST", "/api/demo/load", nil))
	if res.Code != 201 {
		t.Fatalf("demo validation: %d %s", res.Code, res.Body)
	}
}
