package job

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

const guardExpr = "gene\ta\tb\tc\td\ng1\t1\t2\t3\t4\ng2\t4\t2\t3\t1\n"

func guardJob(t *testing.T) (*Manager, string) {
	t.Helper()
	m := NewManager(t.TempDir(), "unused", 1)
	id, err := m.Create([]byte(guardExpr), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	return m, id
}
func TestParameterDomainsAndNormalization(t *testing.T) {
	for _, raw := range []string{`null`, `{"filter":null}`, `{"filter":{"n_keep":""}}`, `{"filter":{"n_keep":2.5}}`, `{"filter":{"sample_percentage":50}}`, `{"network":{"deep_split":5}}`, `{"network":{"power":0}}`, `{"hub":{"kme_cutoff":1.2}}`, `{"cluster":{"kmax":1}}`, `{"filter":{"normalize_method":"typo"}}`} {
		if _, err := decodeParams([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	m, id := guardJob(t)
	dir := filepath.Join(m.dir, id)
	_ = os.WriteFile(filepath.Join(dir, "expr.tsv"), []byte("gene\ta\tb\tc\td\ng1\t0\t2\t3\t4\ng2\t4\t2\t3\t1\n"), 0644)
	for _, tc := range []struct {
		norm  string
		valid bool
	}{{"log2", false}, {"log", false}, {"log10", false}, {"log2p1", true}, {"raw", true}} {
		p, err := decodeParams([]byte(`{"filter":{"normalize_method":"` + tc.norm + `"}}`))
		if err != nil {
			t.Fatal(err)
		}
		err = m.validateRun(id, "filter", p)
		if (err == nil) != tc.valid {
			t.Errorf("%s: %v", tc.norm, err)
		}
	}
	p, _ := decodeParams([]byte(`{"filter":{"cutoff":100,"sample_percentage":0.5}}`))
	if m.validateRun(id, "filter", p) == nil {
		t.Fatal("accepted empty filter")
	}
}
func TestTraitSourceSelectionAndConstantTraits(t *testing.T) {
	m, id := guardJob(t)
	dir := filepath.Join(m.dir, id)
	_ = os.WriteFile(filepath.Join(dir, "step1_filtered.tsv"), []byte(guardExpr), 0644)
	_ = os.WriteFile(filepath.Join(dir, "step3_modules.json"), []byte(`{"colors":["blue","blue"]}`), 0644)
	p, _ := decodeParams([]byte(`{"module_trait":{"trait_source":"sample_info","trait_column":"group"}}`))
	for _, tc := range []struct {
		rows  string
		valid bool
	}{{"a\tA\nb\tB\nc\tA\nd\tB\n", true}, {"a\tA\nb\tA\nc\tA\nd\tA\n", false}, {"a\tA\nb\t\nc\tA\nd\tB\n", false}} {
		_ = os.WriteFile(filepath.Join(dir, "sample_info.tsv"), []byte("sample_id\tgroup\n"+tc.rows), 0644)
		if err := m.validateRun(id, "module_trait", p); (err == nil) != tc.valid {
			t.Fatalf("metadata: %v", err)
		}
	}
	p, _ = decodeParams(nil)
	if err := m.validateRun(id, "module_trait", p); err == nil {
		t.Fatal("accepted absent trait")
	}
	if err := m.UploadTrait(id, []byte("sample_id\tt\na\t1\nb\t1\nc\t1\nd\t1\n")); err != nil {
		t.Fatal(err)
	}
	if err := m.validateRun(id, "module_trait", p); err == nil {
		t.Fatal("accepted constant trait")
	}
}
func fakeWorker(t *testing.T, fail bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process fixture")
	}
	path := filepath.Join(t.TempDir(), "worker.sh")
	script := "#!/bin/sh\nsleep 0.15\n"
	if fail {
		script += "echo intentional-worker-failure >&2\nexit 1\n"
	} else {
		script += "cat > \"$2/step1_filter.json\" <<'JSON'\n{\"samples\":[\"a\",\"b\",\"c\",\"d\"],\"n_genes_final\":2}\nJSON\ncat > \"$2/step1_filtered.tsv\" <<'TSV'\n" + guardExpr + "TSV\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
func awaitStopped(t *testing.T, m *Manager, id string) *Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := m.Get(id)
		m.mu.RLock()
		busy := m.cancel[id] != nil
		m.mu.RUnlock()
		if !busy && j.Status != "running" {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("run did not stop")
	return nil
}
func TestAtomicStartCancellationAndSnapshots(t *testing.T) {
	m, id := guardJob(t)
	m.workerBin = fakeWorker(t, false)
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Start(id, "filter", nil, 1); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("%d workers", accepted)
	}
	snapshot, _ := m.Get(id)
	snapshot.Status = "modified"
	snapshot.DoneSteps = []string{"network"}
	actual, _ := m.Get(id)
	if actual.Status != "running" || len(actual.DoneSteps) != 0 {
		t.Fatal("Get leaks state")
	}
	if err := m.UploadTrait(id, []byte("ID\tt\na\t1\nb\t2\nc\t3\nd\t4\n")); err == nil {
		t.Fatal("changed running input")
	}
	m.Cancel(id)
	if j := awaitStopped(t, m, id); j.Status != "cancelled" {
		t.Fatalf("%+v", j)
	}
	if _, err := m.Start(id, "filter", nil, 1); err != nil {
		t.Fatal(err)
	}
	if j := awaitStopped(t, m, id); j.Status != "done" {
		t.Fatalf("%+v", j)
	}
	snapshot, _ = m.Get(id)
	snapshot.DoneSteps[0] = "corrupt"
	actual, _ = m.Get(id)
	if actual.DoneSteps[0] != "filter" {
		t.Fatal("done slice aliases state")
	}
}
func TestRejectDependenciesAndStaleParametersWithoutChangingResults(t *testing.T) {
	m, id := guardJob(t)
	m.workerBin = fakeWorker(t, false)
	if _, err := m.Start(id, "network", nil, 1); err == nil {
		t.Fatal("accepted missing dependency")
	}
	if _, err := m.Start(id, "filter", nil, 1); err != nil {
		t.Fatal(err)
	}
	awaitStopped(t, m, id)
	before, _ := os.ReadFile(filepath.Join(m.dir, id, "params.json"))
	_, err := m.Start(id, "soft_threshold", json.RawMessage(`{"filter":{"n_keep":3}}`), 1)
	if e, ok := err.(*GuardError); !ok || e.Code != "stale_parameters" {
		t.Fatalf("%v", err)
	}
	after, _ := os.ReadFile(filepath.Join(m.dir, id, "params.json"))
	if string(before) != string(after) {
		t.Fatal("rejection changed params")
	}
	if _, ok := m.Result(id, "filter"); !ok {
		t.Fatal("rejection discarded result")
	}
}
func TestFailedRerunCannotReturnOldResult(t *testing.T) {
	m, id := guardJob(t)
	m.workerBin = fakeWorker(t, false)
	if _, err := m.Start(id, "filter", nil, 1); err != nil {
		t.Fatal(err)
	}
	awaitStopped(t, m, id)
	m.workerBin = fakeWorker(t, true)
	run, err := m.Start(id, "filter", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Result(id, "filter"); ok {
		t.Fatal("running exposes old result")
	}
	if j := awaitStopped(t, m, id); j.Status != "error" {
		t.Fatalf("%+v", j)
	}
	if _, ok := m.Result(id, "filter"); ok {
		t.Fatal("failure exposes old result")
	}
	if _, err = os.Stat(filepath.Join(m.dir, id, "previous-runs", run, "step1_filter.json")); err != nil {
		t.Fatal("lost previous result", err)
	}
}
func TestReportReservationBlocksMutation(t *testing.T) {
	m, id := guardJob(t)
	m.mu.Lock()
	m.jobs[id].DoneSteps = []string{"network"}
	m.mu.Unlock()
	release, err := m.ReserveSnapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(id, "filter", nil, 1); err == nil {
		t.Fatal("started during report")
	}
	if err = m.UpdateParams(id, json.RawMessage(`{}`)); err == nil {
		t.Fatal("changed report source")
	}
	release()
	release()
	if err = m.UpdateParams(id, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
}
