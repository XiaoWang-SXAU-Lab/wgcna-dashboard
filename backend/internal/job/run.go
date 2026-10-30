package job

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// Start reserves a job synchronously before returning to HTTP. Validation and
// parameter persistence happen under the same lock, so two requests can never
// start workers or change the active worker's parameters concurrently.
func (m *Manager) Start(id, step string, patch json.RawMessage, threads int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return "", guard(404, "job_missing", "This job is no longer available. Load the demo or select your input files again.")
	}
	if _, busy := m.cancel[id]; busy || m.readers[id] > 0 {
		return "", guard(409, "busy", "This job is busy with an analysis or report. Wait for it to finish or cancel the active analysis before starting another.")
	}
	if stepIndex(step) < 0 && step != "cluster" && step != "cluster_elbow" && step != "overview_network" {
		return "", guard(400, "step", "Unknown analysis step.")
	}
	dir := filepath.Join(m.dir, id)
	old, err := os.ReadFile(filepath.Join(dir, "params.json"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	raw, err := mergeParams(old, patch)
	if err != nil {
		return "", err
	}
	p, err := decodeParams(raw)
	if err != nil {
		return "", err
	}
	if threads < 0 || threads > 1024 {
		return "", guard(422, "parameters", "Threads must be an integer between 1 and 1024.")
	}
	for _, dep := range prerequisites(step) {
		if !contains(j.DoneSteps, dep) {
			return "", guard(422, "dependencies", "Complete "+dep+" successfully before running "+step+".")
		}
		if applied := j.applied[dep]; len(applied) > 0 && !sameSection(applied, canonicalSection(p, dep)) {
			return "", guard(422, "stale_parameters", "Parameters for "+dep+" have changed. Rerun "+dep+" and its subsequent steps before running "+step+".")
		}
	}
	if err = m.validateRun(id, step, p); err != nil {
		return "", issueSummary(err)
	}
	if err = writeInput(filepath.Join(dir, "params.json"), raw); err != nil {
		return "", err
	}
	if threads > 0 {
		j.Threads = threads
	}
	runID := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), randHex(4))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel[id] = cancel
	j.Status = "running"
	j.CurrentStep = step
	j.RunID = runID
	j.Error = ""
	j.Progress = nil
	idx := stepIndex(step)
	if idx >= 0 {
		keep := []string{}
		for _, s := range j.DoneSteps {
			if stepIndex(s) < idx {
				keep = append(keep, s)
			}
		}
		j.DoneSteps = keep
		if idx <= stepIndex("network") {
			j.Result = nil
		}
	}
	// A previous progress.json belongs to an earlier run and must not be shown.
	_ = os.Remove(filepath.Join(dir, "progress.json"))
	go m.execute(ctx, id, step, runID, canonicalSection(p, step))
	return runID, nil
}

func (m *Manager) execute(ctx context.Context, id, step, runID string, applied []byte) {
	dir := filepath.Join(m.dir, id)
	defer func() {
		m.mu.Lock()
		if cancel := m.cancel[id]; cancel != nil {
			cancel()
		}
		delete(m.cancel, id)
		m.mu.Unlock()
	}()
	var out []byte
	var err error
	// Archive derived outputs, including downstream auxiliary files, so file-
	// based views cannot mistake the old result for a newly completed run.
	files := []string{}
	if idx := stepIndex(step); idx >= 0 {
		for _, s := range StepOrder[idx:] {
			files = append(files, stepFiles[s])
		}
		if idx <= stepIndex("network") {
			files = append(files, "network_edges.tsv", "network_nodes.tsv")
		}
		if idx == 0 {
			files = append(files, "overview_clusters.json", "overview_elbow.json")
		}
	} else {
		switch step {
		case "cluster":
			files = []string{"overview_clusters.json"}
		case "cluster_elbow":
			files = []string{"overview_elbow.json"}
		case "overview_network":
			files = []string{"network_edges.tsv", "network_nodes.tsv"}
		}
	}
	archive := filepath.Join(dir, "previous-runs", runID)
	for _, name := range files {
		source := filepath.Join(dir, name)
		if _, e := os.Stat(source); os.IsNotExist(e) {
			continue
		} else if e != nil {
			err = e
			break
		}
		if err = os.MkdirAll(archive, 0755); err != nil {
			break
		}
		if err = os.Rename(source, filepath.Join(archive, name)); err != nil {
			break
		}
	}
	if err == nil && step != "filter" {
		err = m.SyncSampleTables(id)
	}
	if err == nil {
		cmd := exec.CommandContext(ctx, m.workerBin, "--job-dir", dir, "--step", step, "--threads", fmt.Sprint(m.threadsFor(id)))
		subproc.HideWindow(cmd)
		out, err = cmd.CombinedOutput()
	}
	if ctx.Err() != nil {
		m.mu.Lock()
		m.updateLocked(id, func(j *Job) { j.Status = "cancelled"; j.Progress = nil })
		m.mu.Unlock()
		return
	}
	if err == nil && step == "filter" {
		err = m.SyncSampleTables(id)
	}
	if err == nil && stepIndex(step) >= 0 {
		// A process exiting 0 without its expected output is not a successful step.
		b, e := os.ReadFile(filepath.Join(dir, stepFiles[step]))
		if e != nil {
			err = fmt.Errorf("analysis completed without a result file; rerun %s: %w", step, e)
		} else if !json.Valid(b) {
			err = fmt.Errorf("analysis produced an invalid result; rerun %s", step)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil {
		m.updateLocked(id, func(j *Job) { j.Status = "cancelled"; j.Progress = nil })
		return
	}
	if err != nil {
		message := string(out)
		if message == "" {
			message = err.Error()
		}
		if progress := readProgress(dir); progress != nil && progress.Step == step && progress.Status == "error" && progress.Message != "" {
			message = progress.Message
		}
		m.updateLocked(id, func(j *Job) { j.Status = "error"; j.Progress = nil; j.Error = message })
		return
	}
	m.updateLocked(id, func(j *Job) {
		j.Status = "done"
		j.Progress = nil
		if stepIndex(step) >= 0 {
			if !contains(j.DoneSteps, step) {
				j.DoneSteps = append(j.DoneSteps, step)
			}
			if j.applied == nil {
				j.applied = map[string]json.RawMessage{}
			}
			j.applied[step] = append([]byte(nil), applied...)
		}
		if step == "network" {
			j.Result, _ = os.ReadFile(filepath.Join(dir, stepFiles[step]))
		}
	})
}
