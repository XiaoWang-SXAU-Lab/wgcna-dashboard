package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

var dependencies = map[string][]string{
	"filter": {}, "soft_threshold": {"filter"}, "network": {"filter", "soft_threshold"},
	"module_trait": {"filter", "network"}, "module_detail": {"filter", "network", "module_trait"}, "hub": {"filter", "network", "module_trait"},
	"rean": {"filter", "network", "module_trait", "hub"},
	"hub_priority": {"filter", "network"},
}
var outputNames = map[string]string{
	"filter": "step1_filter.json", "soft_threshold": "step2_sft.json", "network": "step3_modules.json",
	"module_trait": "step4_mtrait.json", "module_detail": "step5_detail.json", "hub": "step6_hub.json",
	"rean": "step7_rean.json",
	"hub_priority": "step8_hub_priority.json",
}

// IsStep reports whether name is a recognized project execution step. The
// project path expresses real dependencies (not the legacy linear StepOrder).
func IsStep(name string) bool {
	_, ok := dependencies[name]
	return ok
}

func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	aa, _ := json.Marshal(x)
	bb, _ := json.Marshal(y)
	return bytes.Equal(aa, bb)
}

// Validity returns explanations instead of silently promoting old output files.
// Engine fingerprints are intentionally strict in this first format version.
func (p *Project) Validity(engine string) map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.validity(engine)
}

func (p *Project) validity(engine string) map[string]string {
	reasons := map[string]string{}
	for _, name := range []string{"filter", "soft_threshold", "network", "hub_priority", "module_trait", "module_detail", "hub", "rean"} {
		reasons[name] = p.checkStep(name, engine, reasons)
	}
	return reasons
}

func (p *Project) checkStep(name, engine string, reasons map[string]string) string {
	step, ok := p.state.Steps[name]
	if !ok {
		return "not run"
	}
	if step.Status != "done" {
		return step.Status
	}
	if step.Engine != engine {
		return "engine changed"
	}
	if !sameJSON(step.Params, p.state.Draft[name]) {
		return "parameters changed"
	}
	if len(step.Inputs) != len(p.state.Inputs) {
		return "inputs changed"
	}
	for role, a := range p.state.Inputs {
		if step.Inputs[role] != a.SHA256 {
			return "inputs changed"
		}
		if _, err := p.Resolve(a); err != nil {
			return err.Error()
		}
	}
	for _, dep := range dependencies[name] {
		if reasons[dep] != "" {
			return "upstream invalid: " + dep
		}
		if step.Upstream[dep] != p.state.Steps[dep].ExecutionID {
			return "upstream execution changed: " + dep
		}
	}
	if len(step.Outputs) == 0 {
		return "no committed results"
	}
	found := false
	for _, a := range step.Outputs {
		if filepath.Base(a.Path) == outputNames[name] {
			found = true
		}
		if _, err := p.Resolve(a); err != nil {
			return err.Error()
		}
	}
	if !found {
		return "missing primary result"
	}
	return ""
}

// Run executes the unchanged worker in machine-local staging. Even if the app
// dies and its worker survives, that orphan cannot write into the saved project.
// stagingRoot must be an explicitly selected machine-local directory.
func (p *Project) Run(ctx context.Context, name, worker, stagingRoot string, threads int) error {
	if _, ok := dependencies[name]; !ok {
		return errors.New("unknown step")
	}
	engine, err := digest(worker)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("project closed")
	}
	for _, step := range p.state.Steps {
		if step.Status == "running" {
			p.mu.Unlock()
			return errors.New("another step is running")
		}
	}
	if _, ok := p.state.Inputs["expr"]; !ok {
		p.mu.Unlock()
		return errors.New("expression input required")
	}
	if !json.Valid(p.state.Draft[name]) {
		p.mu.Unlock()
		return errors.New("step parameters required")
	}
	reasons := p.validity(engine)
	for _, dep := range dependencies[name] {
		if reasons[dep] != "" {
			p.mu.Unlock()
			return fmt.Errorf("%s requires valid %s: %s", name, dep, reasons[dep])
		}
	}
	stage, err := os.MkdirTemp(stagingRoot, "project-run-*")
	if err != nil {
		p.mu.Unlock()
		return err
	}
	defer os.RemoveAll(stage)
	// Preserve all validated upstream artifacts under worker-compatible names.
	artifacts := []Artifact{}
	for role, a := range p.state.Inputs {
		src, e := p.Resolve(a)
		if e != nil {
			p.mu.Unlock()
			return e
		}
		if e = copyFile(src, filepath.Join(stage, role+".tsv")); e != nil {
			p.mu.Unlock()
			return e
		}
	}
	for _, dep := range dependencies[name] {
		artifacts = append(artifacts, p.state.Steps[dep].Outputs...)
	}
	for _, a := range artifacts {
		src, e := p.Resolve(a)
		if e != nil {
			p.mu.Unlock()
			return e
		}
		if e = copyFile(src, filepath.Join(stage, filepath.Base(a.Path))); e != nil {
			p.mu.Unlock()
			return e
		}
	}
	raw, _ := json.Marshal(p.state.Draft)
	if err = os.WriteFile(filepath.Join(stage, "params.json"), raw, 0o600); err != nil {
		p.mu.Unlock()
		return err
	}
	record := Step{ExecutionID: identifier(), Status: "running", Params: append(json.RawMessage{}, p.state.Draft[name]...), Engine: engine, Inputs: map[string]string{}, Upstream: map[string]string{}, StartedAt: time.Now().UTC()}
	for role, a := range p.state.Inputs {
		record.Inputs[role] = a.SHA256
	}
	for _, dep := range dependencies[name] {
		record.Upstream[dep] = p.state.Steps[dep].ExecutionID
	}
	s := clone(p.state)
	s.Steps[name] = record
	if err = p.commit(s); err != nil {
		p.mu.Unlock()
		return err
	}
	p.active = true
	p.progress = Progress{Step: name, Status: "running", Progress: 0, Message: "starting..."}
	p.mu.Unlock()

	if threads < 1 {
		threads = 1
	}
	cmd := exec.CommandContext(ctx, worker, "--job-dir", stage, "--step", name, "--threads", fmt.Sprint(threads))
	subproc.HideWindow(cmd)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	runErr := cmd.Start()
	var stop chan struct{}
	if runErr == nil {
		stop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					if b, e := os.ReadFile(filepath.Join(stage, "progress.json")); e == nil {
						var pr Progress
						if json.Unmarshal(b, &pr) == nil {
							p.setProgress(pr)
						}
					}
				}
			}
		}()
		runErr = cmd.Wait()
		close(stop)
	}
	out := outBuf.Bytes()
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.active = false }()
	now := time.Now().UTC()
	record.FinishedAt = &now
	if runErr != nil {
		record.Status = "error"
		record.Error = fmt.Sprintf("%v: %s", runErr, strings.TrimSpace(string(out)))
	} else {
		record.Status = "done"
		prefix := strings.Split(outputNames[name], "_")[0] + "_"
		entries, e := os.ReadDir(stage)
		if e != nil {
			runErr = e
		}
		for _, entry := range entries {
			if runErr != nil {
				break
			}
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			rel := filepath.ToSlash(filepath.Join("analyses", "baseline", "executions", record.ExecutionID, entry.Name()))
			dest := filepath.Join(p.root, filepath.FromSlash(rel))
			if _, e = p.safeDirectory(filepath.ToSlash(filepath.Dir(rel))); e == nil {
				e = copyFile(filepath.Join(stage, entry.Name()), dest)
			}
			if e != nil {
				runErr = e
				break
			}
			hash, e := digest(dest)
			if e != nil {
				runErr = e
				break
			}
			record.Outputs = append(record.Outputs, Artifact{Path: rel, SHA256: hash})
		}
		// A successful process exit without the primary result is not completion.
		primary, e := os.ReadFile(filepath.Join(stage, outputNames[name]))
		if e != nil {
			runErr = e
		} else if !json.Valid(primary) {
			runErr = errors.New("worker primary result is invalid JSON")
		}
		if name == "filter" {
			for _, required := range []string{"step1_filtered.tsv", "step1_genes.txt"} {
				if _, e := os.Stat(filepath.Join(stage, required)); e != nil {
					runErr = e
				}
			}
		}
		if runErr != nil {
			record.Status = "error"
			record.Error = runErr.Error()
			record.Outputs = nil
		}
	}
	s = clone(p.state)
	s.Steps[name] = record
	if err = p.commit(s); err != nil {
		return fmt.Errorf("save execution status: %w", err)
	}
	return runErr
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if err == nil {
		err = dst.Sync()
	}
	closeErr := dst.Close()
	if err != nil {
		return err
	}
	return closeErr
}
