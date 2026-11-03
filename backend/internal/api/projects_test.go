package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/project"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 4 && os.Args[1] == "--job-dir" {
		if os.Getenv("PROJECT_HTTP_WAIT") == "1" {
			time.Sleep(time.Minute)
			os.Exit(3)
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestProjectHTTPCancelAndShutdown(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			worker, _ := os.Executable()
			h, mux := projectHarness(t, worker, t.TempDir())
			t.Setenv("PROJECT_HTTP_WAIT", "1")
			state := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": t.TempDir(), "name": "cancel test"}, 201)
			id := state.State.ID
			path := "/api/projects/" + id
			state = projectUpload(t, mux, id, "expr", "gene\ts1\ng1\t1\n")
			state = projectJSON(t, mux, "PUT", path+"/draft", map[string]any{"revision": state.State.Revision, "draft_params": map[string]any{"filter": map[string]any{}}, "ui_state": map[string]any{}, "question_card": map[string]any{}}, 200)
			projectJSON(t, mux, "POST", path+"/steps/filter/run", map[string]any{"revision": state.State.Revision, "threads": 1}, 202)
			deadline := time.Now().Add(10 * time.Second)
			for {
				state = projectJSON(t, mux, "GET", path, nil, 200)
				if state.State.Steps["filter"].Status == "running" {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("worker did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if mode == "cancel" {
				projectJSON(t, mux, "POST", path+"/cancel", nil, 202)
				for {
					state = projectJSON(t, mux, "GET", path, nil, 200)
					if !state.Busy {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("cancel did not complete")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if state.Validity["filter"] == "" {
					t.Fatal("cancelled output accepted")
				}
				projectJSON(t, mux, "POST", path+"/close", nil, 200)
			} else {
				h.Shutdown()
			}
			p, err := project.Open(state.Path)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			if p.Snapshot().Steps["filter"].Status == "running" {
				t.Fatal("shutdown left running manifest")
			}
		})
	}
}

type projectReply struct {
	State    project.State     `json:"state"`
	Validity map[string]string `json:"validity"`
	Busy     bool              `json:"busy"`
	Error    string            `json:"error"`
	Path     string            `json:"path"`
}

func projectHarness(t *testing.T, worker, home string) (*ProjectHandler, *http.ServeMux) {
	t.Helper()
	h, err := NewProjectHandler(worker, home, filepath.Join(t.TempDir(), "staging"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Shutdown)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}
func projectJSON(t *testing.T, mux *http.ServeMux, method, path string, body any, status int) projectReply {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, &b)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	var result projectReply
	if status < 300 && strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(w.Body.Bytes(), &result)
	}
	return result
}
func TestProjectHTTPLifecycleAndConflicts(t *testing.T) {
	worker, _ := os.Executable()
	home := t.TempDir()
	h, mux := projectHarness(t, worker, home)
	parent := t.TempDir()
	created := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": parent, "name": "中文 project"}, 201)
	id := created.State.ID
	path := "/api/projects/" + id
	body := map[string]any{"revision": created.State.Revision, "draft_params": map[string]any{"filter": map[string]any{"n_keep": 80}}, "ui_state": map[string]any{"step": "soft_threshold"}, "question_card": map[string]string{"question": "为什么耐盐？"}}
	saved := projectJSON(t, mux, "PUT", path+"/draft", body, 200)
	projectJSON(t, mux, "PUT", path+"/draft", body, 409)
	same := projectJSON(t, mux, "POST", "/api/projects/open", map[string]string{"path": created.Path}, 200)
	if same.State.ID != id || same.State.Revision != saved.State.Revision {
		t.Fatal("same-path open lost state")
	}
	projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": parent, "name": "../outside"}, 400)
	projectJSON(t, mux, "POST", path+"/close", nil, 200)
	h.Shutdown()
	_, mux2 := projectHarness(t, worker, home)
	opened := projectJSON(t, mux2, "POST", "/api/projects/open", map[string]string{"path": created.Path}, 200)
	if opened.State.Question.Question != "为什么耐盐？" {
		t.Fatal("question not restored")
	}
	if string(opened.State.UI) != `{"step":"soft_threshold"}` {
		t.Fatalf("UI not restored: %s", opened.State.UI)
	}
	if b, err := os.ReadFile(filepath.Join(home, "recent-projects.json")); err != nil || !bytes.Contains(b, []byte(id)) {
		t.Fatal("local recents not persisted")
	}
	req := httptest.NewRequest("POST", "/api/projects", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://untrusted.example")
	rec := httptest.NewRecorder()
	mux2.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal("cross-origin filesystem request accepted")
	}
}

func projectUpload(t *testing.T, mux *http.ServeMux, id, role, data string) projectReply {
	t.Helper()
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	f, _ := writer.CreateFormFile("file", role+".tsv")
	_, _ = f.Write([]byte(data))
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/api/projects/"+id+"/inputs/"+role, &b)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("upload %s: %d %s", role, w.Code, w.Body.String())
	}
	var out projectReply
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestProjectHTTPRealWorkerRestart(t *testing.T) {
	worker := os.Getenv("WGCNA_PROJECT_TEST_WORKER")
	if worker == "" {
		t.Skip("native worker not supplied")
	}
	home := t.TempDir()
	h, mux := projectHarness(t, worker, home)
	created := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": t.TempDir(), "name": "resume project"}, 201)
	id := created.State.ID
	path := "/api/projects/" + id
	var matrix strings.Builder
	matrix.WriteString("gene")
	for sample := 0; sample < 24; sample++ {
		fmt.Fprintf(&matrix, "\ts%d", sample)
	}
	matrix.WriteString("\n")
	for gene := 0; gene < 100; gene++ {
		fmt.Fprintf(&matrix, "g%d", gene)
		for sample := 0; sample < 24; sample++ {
			fmt.Fprintf(&matrix, "\t%d", 1+(gene*17+sample*13+gene*sample*7)%101)
		}
		matrix.WriteString("\n")
	}
	uploaded := projectUpload(t, mux, id, "expr", matrix.String())
	params := map[string]any{"filter": map[string]any{"data_type": "normalized_count", "normalize_method": "raw", "n_keep": 80, "filter_method": "mad", "sample_percentage": 0, "cutoff": 0}, "soft_threshold": map[string]any{}, "network": map[string]any{"power": 6, "min_module_size": 10}}
	state := projectJSON(t, mux, "PUT", path+"/draft", map[string]any{"revision": uploaded.State.Revision, "draft_params": params, "ui_state": map[string]string{"step": "network"}, "question_card": map[string]string{"question": "test"}}, 200)
	for _, step := range []string{"filter", "soft_threshold", "network"} {
		projectJSON(t, mux, "POST", path+"/steps/"+step+"/run", map[string]any{"revision": state.State.Revision, "threads": 2}, 202)
		projectJSON(t, mux, "POST", path+"/close", nil, 409)
		deadline := time.Now().Add(20 * time.Second)
		for {
			state = projectJSON(t, mux, "GET", path, nil, 200)
			if !state.Busy {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("run timed out")
			}
			time.Sleep(20 * time.Millisecond)
		}
		if state.Error != "" || state.Validity[step] != "" {
			t.Fatalf("%s failed: %s %v", step, state.Error, state.Validity)
		}
	}
	// Leave draft beta=8 while preserving the beta=6 execution.
	params["network"].(map[string]any)["power"] = 8
	state = projectJSON(t, mux, "PUT", path+"/draft", map[string]any{"revision": state.State.Revision, "draft_params": params, "ui_state": map[string]string{"step": "network"}, "question_card": map[string]string{"question": "test"}}, 200)
	if state.Validity["network"] != "parameters changed" {
		t.Fatal("stale network not labelled")
	}
	h.Shutdown()
	_, mux2 := projectHarness(t, worker, home)
	state = projectJSON(t, mux2, "POST", "/api/projects/open", map[string]string{"path": created.Path}, 200)
	if state.Validity["soft_threshold"] != "" || state.Validity["network"] != "parameters changed" {
		t.Fatalf("invalid restored cache: %v", state.Validity)
	}
	if !bytes.Contains(state.State.Steps["network"].Params, []byte("6")) || !bytes.Contains(state.State.Draft["network"], []byte("8")) {
		t.Fatal("execution and draft mixed")
	}
	projectJSON(t, mux2, "GET", path+"/results/network", nil, 200)
}

func TestHubPriorityRunAndOutputWhitelist(t *testing.T) {
	worker, _ := os.Executable()
	_, mux := projectHarness(t, worker, t.TempDir())
	created := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": t.TempDir(), "name": "hp"}, 201)
	id := created.State.ID
	path := "/api/projects/" + id
	uploaded := projectUpload(t, mux, id, "expr", "gene\ts1\ng1\t1\n")
	state := projectJSON(t, mux, "PUT", path+"/draft", map[string]any{
		"revision": uploaded.State.Revision,
		"draft_params": map[string]any{
			"filter": map[string]any{}, "soft_threshold": map[string]any{}, "network": map[string]any{"power": 6},
			"hub_priority": map[string]any{"select_module": "blue", "design_confirmed": true},
		},
		"ui_state": map[string]any{}, "question_card": map[string]any{},
	}, 200)
	// Unknown step stays 400.
	projectJSON(t, mux, "POST", path+"/steps/bogus/run", map[string]any{"revision": state.State.Revision, "threads": 1}, 400)
	// hub_priority is a recognized step (202, not "unknown step"); it may fail async
	// because filter/network were not actually run here.
	projectJSON(t, mux, "POST", path+"/steps/hub_priority/run", map[string]any{"revision": state.State.Revision, "threads": 1}, 202)
	// Non-whitelisted output basename is rejected, never path-read.
	for _, basename := range []string{"project.json", "..%2Fproject.json"} {
		req := httptest.NewRequest("GET", path+"/outputs/hub_priority/"+basename, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 404 {
			t.Fatalf("non-whitelisted output %q accepted: %d", basename, rec.Code)
		}
	}
}

func TestCandidateReportNotRun(t *testing.T) {
	worker, _ := os.Executable()
	_, mux := projectHarness(t, worker, t.TempDir())
	created := projectJSON(t, mux, "POST", "/api/projects", map[string]string{"parent": t.TempDir(), "name": "report"}, 201)
	id := created.State.ID
	path := "/api/projects/" + id
	req := httptest.NewRequest("GET", path+"/report/hub-priority", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Fatalf("not-run report should be 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not run") {
		t.Fatalf("expected not-run reason, got %s", rec.Body.String())
	}
}
