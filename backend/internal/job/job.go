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
	"sync"
	"time"
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

// Create stores the uploaded expression matrix + params. It does not run anything.
func (m *Manager) Create(expr []byte, params json.RawMessage) (string, error) {
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

	j := &Job{ID: id, Status: "idle", CreatedAt: time.Now()}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()
	return id, nil
}

// UploadTrait stores the trait table (needed by the module_trait step onwards).
func (m *Manager) UploadTrait(id string, trait []byte) error {
	jobDir := filepath.Join(m.dir, id)
	return os.WriteFile(filepath.Join(jobDir, "trait.tsv"), trait, 0o644)
}

// RunStep runs a single step (skipping nothing). Callers typically check Done
// first via IsDone.
func (m *Manager) RunStep(id, step string) {
	jobDir := filepath.Join(m.dir, id)

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel[id] = cancel
	m.updateLocked(id, func(j *Job) { j.Status = "running"; j.CurrentStep = step; j.Error = "" })
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.cancel, id)
		m.mu.Unlock()
	}()

	cmd := exec.CommandContext(ctx, m.workerBin, "--job-dir", jobDir, "--step", step, "--threads", fmt.Sprint(m.threads))
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

// RunAll runs every step in order, skipping steps that are already done.
func (m *Manager) RunAll(id string) {
	for _, step := range StepOrder {
		if m.IsDone(id, step) {
			continue
		}
		m.RunStep(id, step)
		if st, _ := m.status(id); st != "done" {
			return // stop on error / cancel
		}
	}
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
	"soft_threshold": "step2_sft.json",
	"network":        "step3_modules.json",
	"module_trait":   "step4_mtrait.json",
	"module_detail":  "step5_detail.json",
	"hub":            "step6_hub.json",
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

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}
