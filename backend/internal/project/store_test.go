package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The test executable acts as a deterministic worker subprocess; these tests
// exercise real file commits and process boundaries without needing Rust/R.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--project-read" {
		p, err := Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		worker, _ := os.Executable()
		engine, _ := digest(worker)
		if p.Validity(engine)["soft_threshold"] != "" {
			os.Exit(3)
		}
		p.Close()
		os.Exit(0)
	}
	if len(os.Args) == 3 && os.Args[1] == "--project-abandon" {
		p, err := Open(os.Args[2])
		if err != nil {
			os.Exit(2)
		}
		s := p.Snapshot()
		s.Steps["filter"] = Step{ExecutionID: "abandoned", Status: "running", Params: json.RawMessage(`{}`)}
		if p.write(s) != nil {
			os.Exit(3)
		}
		os.Exit(0) // No Close: simulate process termination with a held OS lock.
	}
	if len(os.Args) > 2 && os.Args[1] == "--job-dir" {
		root, step := os.Args[2], os.Args[4]
		if os.Getenv("PROJECT_FIXTURE_FAIL") == "1" {
			os.Exit(7)
		}
		_ = os.WriteFile(filepath.Join(root, outputNames[step]), []byte(`{"fixture":true}`), 0o600)
		if step == "filter" {
			_ = os.WriteFile(filepath.Join(root, "step1_filtered.tsv"), []byte("gene\ts1\ng1\t1\n"), 0o600)
			_ = os.WriteFile(filepath.Join(root, "step1_genes.txt"), []byte("g1\n"), 0o600)
		}
		if step == "hub_priority" {
			_ = os.WriteFile(filepath.Join(root, "step8_hub_priority.tsv"), []byte("gene\tkwithin\ng1\t1\n"), 0o600)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func newFixture(t *testing.T) (*Project, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "中文 project")
	p, err := Create(root, "耐盐研究")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if _, err = p.Import("expr", strings.NewReader("gene\ts1\ng1\t1\n")); err != nil {
		t.Fatal(err)
	}
	s := p.Snapshot()
	draft := map[string]json.RawMessage{"filter": json.RawMessage(`{"n_keep":100}`), "soft_threshold": json.RawMessage(`{}`), "network": json.RawMessage(`{"power":6}`)}
	if _, err = p.Save(s.Revision, draft, json.RawMessage(`{"page":"/sft"}`), QuestionCard{Question: "为什么耐盐？"}); err != nil {
		t.Fatal(err)
	}
	return p, root
}

func runFixture(t *testing.T, p *Project, step string) string {
	t.Helper()
	worker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Run(context.Background(), step, worker, t.TempDir(), 1); err != nil {
		t.Fatal(err)
	}
	engine, err := digest(worker)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestReopenPreservesResultsDraftAndQuestion(t *testing.T) {
	p, root := newFixture(t)
	engine := runFixture(t, p, "filter")
	runFixture(t, p, "soft_threshold")
	if reason := p.Validity(engine)["soft_threshold"]; reason != "" {
		t.Fatal(reason)
	}
	s := p.Snapshot()
	s.Draft["network"] = json.RawMessage(`{"power":8}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	worker, _ := os.Executable()
	if out, err := exec.Command(worker, "--project-read", root).CombinedOutput(); err != nil {
		t.Fatalf("independent process reopen: %v %s", err, out)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if p2.Snapshot().Question.Question != "为什么耐盐？" {
		t.Fatal("question lost")
	}
	if reason := p2.Validity(engine)["soft_threshold"]; reason != "" {
		t.Fatal(reason)
	}
	if !strings.Contains(string(p2.Snapshot().Draft["network"]), "8") {
		t.Fatal("draft lost")
	}
	// Copying the whole project to another path keeps relative references valid.
	if err = p2.Close(); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved project")
	if err = os.CopyFS(moved, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	p3, err := Open(moved)
	if err != nil {
		t.Fatal(err)
	}
	defer p3.Close()
	if p3.Validity(engine)["soft_threshold"] != "" {
		t.Fatal("moved results invalid")
	}
}

func TestCacheInvalidation(t *testing.T) {
	for _, change := range []string{"params", "input", "output", "engine", "missing"} {
		t.Run(change, func(t *testing.T) {
			p, _ := newFixture(t)
			engine := runFixture(t, p, "filter")
			runFixture(t, p, "soft_threshold")
			s := p.Snapshot()
			switch change {
			case "params":
				s.Draft["filter"] = json.RawMessage(`{"n_keep":50}`)
				if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
					t.Fatal(err)
				}
			case "input":
				path, _ := p.Resolve(s.Inputs["expr"])
				if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "output", "missing":
				path, _ := p.Resolve(s.Steps["filter"].Outputs[0])
				var err error
				if change == "missing" {
					err = os.Remove(path)
				} else {
					err = os.WriteFile(path, []byte("changed"), 0o600)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "engine":
				engine = "different-engine"
			}
			reasons := p.Validity(engine)
			if reasons["filter"] == "" || reasons["soft_threshold"] == "" {
				t.Fatalf("accepted invalid cache: %v", reasons)
			}
		})
	}
}

func TestLockRevisionAndWriteFailure(t *testing.T) {
	p, root := newFixture(t)
	if other, err := Open(root); err == nil {
		other.Close()
		t.Fatal("concurrent writer accepted")
	}
	s := p.Snapshot()
	if _, err := p.Save(s.Revision-1, s.Draft, s.UI, s.Question); err != ErrConflict {
		t.Fatalf("expected revision conflict: %v", err)
	}
	// A failed backup write must leave both manifest and memory unchanged.
	if err := os.Remove(filepath.Join(root, "project.json.bak")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "project.json.bak"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Save(s.Revision, s.Draft, s.UI, QuestionCard{Question: "must not persist"}); err == nil {
		t.Fatal("write failure ignored")
	}
	if p.Snapshot().Revision != s.Revision {
		t.Fatal("memory advanced after failed write")
	}
	b, _ := os.ReadFile(filepath.Join(root, "project.json"))
	var disk State
	if err := json.Unmarshal(b, &disk); err != nil || disk.Revision != s.Revision {
		t.Fatal("manifest corrupted")
	}
}

func TestInterruptedAndCorruptManifest(t *testing.T) {
	p, root := newFixture(t)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	worker, _ := os.Executable()
	if out, err := exec.Command(worker, "--project-abandon", root).CombinedOutput(); err != nil {
		t.Fatalf("abandoned process: %v %s", err, out)
	}
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Snapshot().Steps["filter"].Status != "interrupted" {
		t.Fatal("running state not recovered")
	}
	p2.Close()
	b, _ := os.ReadFile(filepath.Join(root, "project.json.bak"))
	if !json.Valid(b) {
		t.Fatal("missing backup")
	}
	if err = os.WriteFile(filepath.Join(root, "project.json"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p3, err := Open(root); err == nil {
		p3.Close()
		t.Fatal("corrupt manifest accepted")
	}
}

func TestFailedWorkerAndParameterProvenance(t *testing.T) {
	p, _ := newFixture(t)
	engine := runFixture(t, p, "filter")
	runFixture(t, p, "soft_threshold")
	runFixture(t, p, "network")
	s := p.Snapshot()
	executed := string(s.Steps["network"].Params)
	s.Draft["network"] = json.RawMessage(`{"power":8}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if p.Validity(engine)["network"] != "parameters changed" {
		t.Fatal("edited params not stale")
	}
	if string(p.Snapshot().Steps["network"].Params) != executed {
		t.Fatal("execution provenance overwritten")
	}
	t.Setenv("PROJECT_FIXTURE_FAIL", "1")
	worker, _ := os.Executable()
	if err := p.Run(context.Background(), "network", worker, t.TempDir(), 1); err == nil {
		t.Fatal("failed worker accepted")
	}
	if p.Snapshot().Steps["network"].Status != "error" {
		t.Fatal("failure not recorded")
	}
	if p.Validity(engine)["filter"] != "" {
		t.Fatal("upstream lost after failure")
	}
}

func TestRunRejectsSamplePercentageOutsideUnitRange(t *testing.T) {
	p, _ := newFixture(t)
	s := p.Snapshot()
	s.Draft["filter"] = json.RawMessage(`{"n_keep":100,"sample_percentage":50}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	worker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	err = p.Run(context.Background(), "filter", worker, t.TempDir(), 1)
	if err == nil || !strings.Contains(err.Error(), "sample_percentage") {
		t.Fatalf("expected sample_percentage range error, got %v", err)
	}
}

func TestPortablePathValidation(t *testing.T) {
	p, _ := newFixture(t)
	hash := strings.Repeat("a", 64)
	for _, name := range []string{"../outside", "/tmp/out", `C:\outside`, "a/../b", "a//b"} {
		if _, err := p.Resolve(Artifact{Path: name, SHA256: hash}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	s := p.Snapshot()
	s.SchemaVersion = 999
	if err := validate(s); err == nil {
		t.Fatal("future schema accepted")
	}
}

func TestInvalidRunningManifestReleasesLock(t *testing.T) {
	p, root := newFixture(t)
	s := p.Snapshot()
	s.SchemaVersion = 999
	s.Steps["filter"] = Step{Status: "running", Params: json.RawMessage(`{}`)}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	if err := os.WriteFile(filepath.Join(root, "project.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if q, err := Open(root); err == nil {
		q.Close()
		t.Fatal("invalid schema accepted")
	}
	// A failed open must not retain ownership, even with a running record.
	q, err := acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	q.Close()
}

func TestRealWorkerFilterAndSFT(t *testing.T) {
	worker := os.Getenv("WGCNA_PROJECT_TEST_WORKER")
	if worker == "" {
		t.Skip("set WGCNA_PROJECT_TEST_WORKER for real Rust integration")
	}
	p, _ := newFixture(t)
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
	if _, err := p.Import("expr", strings.NewReader(matrix.String())); err != nil {
		t.Fatal(err)
	}
	s := p.Snapshot()
	s.Draft["filter"] = json.RawMessage(`{"data_type":"normalized_count","normalize_method":"raw","n_keep":80,"filter_method":"mad","sample_percentage":0,"cutoff":0}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"filter", "soft_threshold"} {
		if err := p.Run(context.Background(), step, worker, t.TempDir(), 2); err != nil {
			t.Fatal(err)
		}
	}
	engine, _ := digest(worker)
	if reason := p.Validity(engine)["soft_threshold"]; reason != "" {
		t.Fatal(reason)
	}
	root := p.root
	p.Close()
	p2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if p2.Validity(engine)["soft_threshold"] != "" {
		t.Fatal("real worker cache not restored")
	}
}

// TestRealWorkerHubPriority runs the REAL Rust worker through the Project path:
// filter → soft_threshold → network → hub_priority (no trait/hub), then verifies
// the JSON and TSV are committed. Guarded by WGCNA_PROJECT_TEST_WORKER.
func TestRealWorkerHubPriority(t *testing.T) {
	worker := os.Getenv("WGCNA_PROJECT_TEST_WORKER")
	if worker == "" {
		t.Skip("set WGCNA_PROJECT_TEST_WORKER for real Rust integration")
	}
	p, _ := newFixture(t)
	var matrix strings.Builder
	matrix.WriteString("gene")
	for sample := 0; sample < 24; sample++ {
		fmt.Fprintf(&matrix, "\ts%d", sample)
	}
	matrix.WriteString("\n")
	for gene := 0; gene < 120; gene++ {
		fmt.Fprintf(&matrix, "g%d", gene)
		block := gene / 20
		for sample := 0; sample < 24; sample++ {
			fmt.Fprintf(&matrix, "\t%d", 1+(block*7+sample*3+(gene*sample)%5)%40+gene%7)
		}
		matrix.WriteString("\n")
	}
	if _, err := p.Import("expr", strings.NewReader(matrix.String())); err != nil {
		t.Fatal(err)
	}
	s := p.Snapshot()
	s.Draft["filter"] = json.RawMessage(`{"data_type":"normalized_count","normalize_method":"raw","n_keep":100,"filter_method":"mad","sample_percentage":0,"cutoff":0}`)
	s.Draft["soft_threshold"] = json.RawMessage(`{"network_type":"unsigned","rsquared_cut":0.8}`)
	s.Draft["network"] = json.RawMessage(`{"power":6,"network_type":"unsigned","tom_type":"signed","tom_denom":"min","min_module_size":8,"merge_cut_height":0.25,"deep_split":2,"detect_cut_height":0.995}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"filter", "soft_threshold", "network"} {
		if err := p.Run(context.Background(), step, worker, t.TempDir(), 2); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	engine, _ := digest(worker)
	if reason := p.Validity(engine)["network"]; reason != "" {
		t.Fatalf("network invalid: %s", reason)
	}
	modulesRaw, err := p.Result("network")
	if err != nil {
		t.Fatal(err)
	}
	var mods struct {
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(modulesRaw, &mods); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range mods.Colors {
		counts[c]++
	}
	target := ""
	for c, n := range counts {
		if c == "grey" {
			continue
		}
		if target == "" || n > counts[target] {
			target = c
		}
	}
	if target == "" {
		t.Skip("no non-grey module detected in fixture")
	}
	s = p.Snapshot()
	s.Draft["hub_priority"] = json.RawMessage(fmt.Sprintf(`{"select_module":%q,"n_bootstraps":20,"seed":42,"top_k":3,"resampling":"iid","design_confirmed":true,"trait_source":"none"}`, target))
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if err := p.Run(context.Background(), "hub_priority", worker, t.TempDir(), 2); err != nil {
		t.Fatalf("hub_priority: %v", err)
	}
	if reason := p.Validity(engine)["hub_priority"]; reason != "" {
		t.Fatalf("hub_priority invalid: %s", reason)
	}
	data, err := p.Result("hub_priority")
	if err != nil || !json.Valid(data) {
		t.Fatalf("hub_priority result: %v", err)
	}
	if _, err := p.Output("hub_priority", "step8_hub_priority.tsv"); err != nil {
		t.Fatalf("hub_priority TSV: %v", err)
	}
}

func TestCandidateReportEnvelope(t *testing.T) {
	p, _ := newFixture(t)
	engine := runFixture(t, p, "filter")
	runFixture(t, p, "soft_threshold")
	runFixture(t, p, "network")
	// not run yet → rejected.
	if _, err := p.CandidateReportEnvelope(engine); err == nil {
		t.Fatal("not-run report should be rejected")
	}
	s := p.Snapshot()
	s.Draft["hub_priority"] = hubPriorityDraft()
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	runFixture(t, p, "hub_priority")
	env, err := p.CandidateReportEnvelope(engine)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(env, &m); err != nil {
		t.Fatal(err)
	}
	if m["project_id"] == "" || m["execution_id"] == "" || m["result"] == nil {
		t.Fatalf("envelope missing identity/result: %v", m)
	}
	if m["upstream"] == nil || m["params"] == nil || m["inputs"] == nil {
		t.Fatalf("envelope missing provenance: %v", m)
	}
	// stale params → rejected with a reason.
	s = p.Snapshot()
	s.Draft["hub_priority"] = json.RawMessage(`{"select_module":"blue","n_bootstraps":50,"seed":2,"top_k":2,"resampling":"iid","design_confirmed":true,"trait_source":"none"}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CandidateReportEnvelope(engine); err == nil {
		t.Fatal("stale params should be rejected")
	}
}

func hubPriorityDraft() json.RawMessage {
	return json.RawMessage(`{"select_module":"blue","n_bootstraps":10,"seed":1,"top_k":2,"resampling":"iid","design_confirmed":true,"trait_source":"none"}`)
}

// hub_priority depends only on filter+network; it must complete without
// module_trait/hub, and commit both JSON and TSV.
func TestHubPriorityLifecycleNoTraitOrHub(t *testing.T) {
	p, _ := newFixture(t)
	engine := runFixture(t, p, "filter")
	runFixture(t, p, "soft_threshold")
	runFixture(t, p, "network")
	s := p.Snapshot()
	s.Draft["hub_priority"] = hubPriorityDraft()
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	runFixture(t, p, "hub_priority")
	if reason := p.Validity(engine)["hub_priority"]; reason != "" {
		t.Fatalf("hub_priority invalid: %s", reason)
	}
	if _, ok := p.Snapshot().Steps["module_trait"]; ok {
		t.Fatal("module_trait must not be required or run for hub_priority")
	}
	if _, ok := p.Snapshot().Steps["hub"]; ok {
		t.Fatal("hub must not be required or run for hub_priority")
	}
	data, err := p.Result("hub_priority")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("hub_priority JSON not valid")
	}
	tsv, err := p.Output("hub_priority", "step8_hub_priority.tsv")
	if err != nil {
		t.Fatalf("hub_priority TSV not committed: %v", err)
	}
	if !strings.HasPrefix(string(tsv), "gene\t") {
		t.Fatal("hub_priority TSV content missing")
	}
}

func TestHubPriorityInvalidation(t *testing.T) {
	p, _ := newFixture(t)
	engine := runFixture(t, p, "filter")
	runFixture(t, p, "soft_threshold")
	runFixture(t, p, "network")
	s := p.Snapshot()
	s.Draft["hub_priority"] = hubPriorityDraft()
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	runFixture(t, p, "hub_priority")
	if reason := p.Validity(engine)["hub_priority"]; reason != "" {
		t.Fatalf("hub_priority should be valid: %s", reason)
	}
	// Upstream network param change invalidates hub_priority.
	s = p.Snapshot()
	s.Draft["network"] = json.RawMessage(`{"power":8}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if reason := p.Validity(engine)["hub_priority"]; !strings.HasPrefix(reason, "upstream invalid") {
		t.Fatalf("network change did not invalidate hub_priority: %q", reason)
	}
	// Restore network; change hub_priority B/seed → parameters changed.
	s = p.Snapshot()
	s.Draft["network"] = json.RawMessage(`{"power":6}`)
	s.Draft["hub_priority"] = json.RawMessage(`{"select_module":"blue","n_bootstraps":50,"seed":2,"top_k":2,"resampling":"iid","design_confirmed":true,"trait_source":"none"}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if reason := p.Validity(engine)["hub_priority"]; reason != "parameters changed" {
		t.Fatalf("B/seed change did not invalidate hub_priority: %q", reason)
	}
	// Display-only UI change must not alter validity.
	s = p.Snapshot()
	s.UI = json.RawMessage(`{"hub_tab":"evidence"}`)
	if _, err := p.Save(s.Revision, s.Draft, s.UI, s.Question); err != nil {
		t.Fatal(err)
	}
	if reason := p.Validity(engine)["hub_priority"]; reason != "parameters changed" {
		t.Fatalf("UI change altered hub_priority validity: %q", reason)
	}
}
