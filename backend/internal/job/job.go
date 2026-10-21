// Package job manages WGCNA jobs: each job is a directory on disk holding the
// uploaded inputs, a params snapshot, and the per-step outputs written by the
// Rust worker (which doubles as the step-by-step cache).
package job

import (
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

// Progress mirrors the worker's <job-dir>/progress.json.
type Progress struct {
	Step     string  `json:"step"`
	Status   string  `json:"status"` // running | done | error
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

// Job is the server-side view of a running/finished job.
type Job struct {
	ID          string          `json:"id"`
	Status      string          `json:"status"` // running | done | error
	CurrentStep string          `json:"current_step"`
	Progress    *Progress       `json:"progress,omitempty"`
	Error       string          `json:"error,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Manager owns the on-disk jobs directory and the in-memory job registry.
type Manager struct {
	dir       string
	workerBin string
	threads   int
	steps     []string

	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewManager creates a Manager that runs the Rust worker for the given steps.
func NewManager(dir, workerBin string, threads int) *Manager {
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{
		dir:       dir,
		workerBin: workerBin,
		threads:   threads,
		steps:     []string{"filter", "soft_threshold", "network"},
		jobs:      map[string]*Job{},
	}
}

// Create stores the uploaded expression matrix + params and starts the pipeline.
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

	j := &Job{ID: id, Status: "running", CurrentStep: m.steps[0], CreatedAt: time.Now()}
	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go m.run(id, jobDir)
	return id, nil
}

// Get returns the current job state (and its result once done).
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	return j, ok
}

func (m *Manager) run(id, jobDir string) {
	for _, step := range m.steps {
		m.update(id, func(j *Job) { j.CurrentStep = step })

		cmd := exec.Command(m.workerBin, "--job-dir", jobDir, "--step", step, "--threads", fmt.Sprint(m.threads))
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if p := readProgress(jobDir); p != nil && p.Message != "" {
				msg = p.Message
			}
			m.update(id, func(j *Job) { j.Status = "error"; j.Error = msg })
			return
		}
		if p := readProgress(jobDir); p != nil {
			m.update(id, func(j *Job) { j.Progress = p })
		}
	}

	// On success, load the final modules result into the job.
	var result json.RawMessage
	if b, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json")); err == nil {
		result = json.RawMessage(b)
	}
	m.update(id, func(j *Job) { j.Status = "done"; j.Result = result; j.Progress = nil })
}

func (m *Manager) update(id string, f func(*Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}
