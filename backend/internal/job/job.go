// Package job manages WGCNA jobs: each job is a directory on disk holding the
// uploaded inputs, a params snapshot, and the per-step outputs written by the
// Rust worker (which doubles as the step-by-step cache). It supports running
// individual steps (step-by-step), running the whole pipeline (one-step),
// caching done steps, and cancelling a running step.
package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// StepOrder is the linear execution order of the WGCNA pipeline.
var StepOrder = []string{"filter", "soft_threshold", "network", "module_trait", "module_detail", "hub"}

// Progress mirrors the worker's <job-dir>/progress.json.
type Progress struct {
	Step     string  `json:"step"`
	Status   string  `json:"status"` // running | done | error
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

// Job is the server-side view of a job.
type Job struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"` // idle | running | done | error | cancelled
	CurrentStep string          `json:"current_step"`
	DoneSteps   []string        `json:"done_steps"`
	Progress    *Progress       `json:"progress,omitempty"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"` // step3_modules.json when done
	CreatedAt   time.Time       `json:"created_at"`
	Threads     int             `json:"-"`
}

// Manager owns the on-disk jobs directory and the in-memory job registry.
type Manager struct {
	dir       string
	workerBin string
	threads   int

	mu     sync.RWMutex
	jobs   map[string]*Job
	cancel map[string]context.CancelFunc
}

// NewManager creates a Manager that runs the Rust worker.
func NewManager(dir, workerBin string, threads int) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{
		dir:       dir,
		workerBin: workerBin,
		threads:   threads,
		jobs:      map[string]*Job{},
		cancel:    map[string]context.CancelFunc{},
	}
}

// Dir returns the on-disk jobs directory.
func (m *Manager) Dir() string { return m.dir }

// PrepareReportDetails runs against an isolated report snapshot. It must not
// change the interactive job's selected module, params, progress or DoneSteps.
func (m *Manager) PrepareReportDetails(ctx context.Context, id, snapshotDir string) error {
	return m.PrepareReportStep(ctx, id, snapshotDir, "module_detail_all")
}

func (m *Manager) PrepareReportStep(ctx context.Context, id, snapshotDir, step string) error {
	cmd := exec.CommandContext(ctx, m.workerBin, "--job-dir", snapshotDir, "--step", step, "--threads", fmt.Sprint(m.threadsFor(id)))
	subproc.HideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("prepare module details: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Create stores the uploaded expression matrix + params. It does not run anything.
func (m *Manager) Create(expr []byte, params json.RawMessage, threads int) (string, error) {
	id := fmt.Sprintf("%d%s", time.Now().UnixMilli(), randHex(6))
	jobDir := filepath.Join(m.dir, id)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(jobDir, "expr.tsv"), expr, 0o644); err != nil {
		return "", err
	}
	if len(params) > 0 {
		_ = os.WriteFile(filepath.Join(jobDir, "params.json"), params, 0o644)
	}

	j := &Job{ID: id, Status: "idle", CreatedAt: time.Now(), Threads: threads}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	return id, nil
}

// SetThreads updates the number of worker threads used for a job's steps.
func (m *Manager) SetThreads(id string, threads int) {
	m.mu.Lock()
	if j, ok := m.jobs[id]; ok && threads > 0 {
		j.Threads = threads
	}
	m.mu.Unlock()
}

func (m *Manager) threadsFor(id string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[id]; ok && j.Threads > 0 {
		return j.Threads
	}
	return m.threads
}

// UploadTrait stores the trait table (needed by the module_trait step onwards).
func (m *Manager) UploadTrait(id string, trait []byte) error {
	jobDir := filepath.Join(m.dir, id)
	return os.WriteFile(filepath.Join(jobDir, "trait.tsv"), trait, 0o644)
}

// StoreMeta writes optional per-job metadata files (sample_info, annotation,
// gene2go, gene2kegg). Empty values are skipped.
func (m *Manager) StoreMeta(id string, files map[string][]byte) error {
	jobDir := filepath.Join(m.dir, id)
	for name, data := range files {
		if len(data) == 0 {
			continue
		}
		if err := os.WriteFile(filepath.Join(jobDir, name+".tsv"), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// UpdateParams overwrites the job's params.json (e.g. to exclude outlier samples).
func (m *Manager) UpdateParams(id string, params json.RawMessage) error {
	return os.WriteFile(filepath.Join(m.dir, id, "params.json"), params, 0o644)
}

// RunStep runs a single step (skipping nothing). Callers typically check Done
// first via IsDone.
func (m *Manager) RunStep(id, step string) {
	jobDir := filepath.Join(m.dir, id)

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel[id] = cancel
	m.updateLocked(id, func(j *Job) {
		j.Status = "running"
		j.CurrentStep = step
		j.Error = ""
		// 脏级联：重跑某步时，该步及所有下游的 done 标记都失效。
		idx := stepIndex(step)
		var keep []string
		for _, s := range j.DoneSteps {
			if stepIndex(s) < idx {
				keep = append(keep, s)
			}
		}
		j.DoneSteps = keep
	})
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.cancel, id)
		m.mu.Unlock()
	}()

	cmd := exec.CommandContext(ctx, m.workerBin, "--job-dir", jobDir, "--step", step, "--threads", fmt.Sprint(m.threadsFor(id)))
	subproc.HideWindow(cmd)
	out, err := cmd.CombinedOutput()

	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			m.mu.Lock()
			m.updateLocked(id, func(j *Job) { j.Status = "cancelled" })
			m.mu.Unlock()
		}
		return
	default:
	}

	if err != nil {
		msg := string(out)
		if p := readProgress(jobDir); p != nil && p.Message != "" {
			msg = p.Message
		}
		m.mu.Lock()
		m.updateLocked(id, func(j *Job) { j.Status = "error"; j.Error = msg })
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	m.updateLocked(id, func(j *Job) {
		j.Status = "done"
		j.Progress = nil
		if !contains(j.DoneSteps, step) {
			j.DoneSteps = append(j.DoneSteps, step)
		}
		if step == "network" {
			if b, e := os.ReadFile(filepath.Join(jobDir, "step3_modules.json")); e == nil {
				j.Result = json.RawMessage(b)
			}
		}
	})
	m.mu.Unlock()
}

// RunAuxStep runs a worker step that is NOT part of the linear pipeline
// (overview_network, cluster). It does not touch DoneSteps (no dirty cascade).
func (m *Manager) RunAuxStep(id, step string) {
	jobDir := filepath.Join(m.dir, id)

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel[id] = cancel
	m.updateLocked(id, func(j *Job) {
		j.Status = "running"
		j.CurrentStep = step
		j.Error = ""
	})
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.cancel, id)
		m.mu.Unlock()
	}()

	cmd := exec.CommandContext(ctx, m.workerBin, "--job-dir", jobDir, "--step", step, "--threads", fmt.Sprint(m.threadsFor(id)))
	subproc.HideWindow(cmd)
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.Canceled {
		m.mu.Lock()
		m.updateLocked(id, func(j *Job) { j.Status = "cancelled" })
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	if err != nil {
		msg := string(out)
		if p := readProgress(jobDir); p != nil && p.Message != "" {
			msg = p.Message
		}
		m.updateLocked(id, func(j *Job) { j.Status = "error"; j.Error = msg })
	} else {
		m.updateLocked(id, func(j *Job) { j.Status = "done"; j.Progress = nil })
	}
	m.mu.Unlock()
}

// Cancel kills the currently-running step, if any.
func (m *Manager) Cancel(id string) {
	m.mu.RLock()
	cancel, ok := m.cancel[id]
	m.mu.RUnlock()
	if ok {
		cancel()
	}
}

// IsDone reports whether a step's output already exists (cache hit).
func (m *Manager) IsDone(id, step string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[id]; ok {
		return contains(j.DoneSteps, step)
	}
	return false
}

// Get returns the current job state.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	return j, ok
}

// stepFiles maps a step id to its JSON output file in the job directory.
var stepFiles = map[string]string{
	"filter":         "step1_filter.json",
	"soft_threshold": "step2_sft.json",
	"network":        "step3_modules.json",
	"module_trait":   "step4_mtrait.json",
	"module_detail":  "step5_detail.json",
	"hub":            "step6_hub.json",
}

// LiveProgress returns the worker's current progress (progress.json), if present.
func (m *Manager) LiveProgress(id string) *Progress {
	return readProgress(filepath.Join(m.dir, id))
}

// Result returns a step's JSON output, if present on disk.
func (m *Manager) Result(id, step string) (json.RawMessage, bool) {
	file, ok := stepFiles[step]
	if !ok {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(m.dir, id, file))
	if err != nil {
		return nil, false
	}
	return json.RawMessage(b), true
}

func (m *Manager) status(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[id]; ok {
		return j.Status, true
	}
	return "", false
}

func (m *Manager) updateLocked(id string, f func(*Job)) {
	if j, ok := m.jobs[id]; ok {
		f(j)
	}
}

func readProgress(jobDir string) *Progress {
	b, err := os.ReadFile(filepath.Join(jobDir, "progress.json"))
	if err != nil {
		return nil
	}
	var p Progress
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	return &p
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// stepIndex returns the position of step in StepOrder, or -1.
func stepIndex(step string) int {
	for i, s := range StepOrder {
		if s == step {
			return i
		}
	}
	return -1
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}
