package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/job"
)

func TestRunPreflightReturnsActionableErrorsWithoutStarting(t *testing.T) {
	m := job.NewManager(t.TempDir(), "must-not-execute", 1)
	id, err := m.Create([]byte("gene\ta\tb\tc\ng1\t0\t1\t2\ng2\t3\t2\t1\n"), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.Register(mux)
	for _, tc := range []struct {
		path, params, threads, code string
		status                      int
	}{
		{"network", "{}", "1", "dependencies", 422},
		{"filter", `{"filter":{"normalize_method":"log2"}}`, "1", "normalization", 422},
		{"filter", `{"filter":{"n_keep":""}}`, "1", "parameters", 422},
		{"filter", "{}", "1.5", "parameters", 422},
	} {
		body := url.Values{"params": {tc.params}, "threads": {tc.threads}}
		req := httptest.NewRequest("POST", "/api/jobs/"+id+"/steps/"+tc.path+"/run", strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		var result map[string]string
		_ = json.Unmarshal(out.Body.Bytes(), &result)
		if out.Code != tc.status || result["code"] != tc.code || result["message"] == "" {
			t.Fatalf("%s: %d %s", tc.path, out.Code, out.Body)
		}
		if j, _ := m.Get(id); j.Status != "idle" {
			t.Fatal("rejected preflight changed job state")
		}
	}
}
