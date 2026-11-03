package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (p *Project) Root() string                       { return p.root }
func EngineFingerprint(worker string) (string, error) { return digest(worker) }

// Result reads the recorded execution output, including an outdated result.
// Callers must display validity separately rather than silently hiding provenance.
func (p *Project) Result(step string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readResultLocked(step)
}

// readResultLocked reads the committed primary result for `step`; caller holds p.mu.
func (p *Project) readResultLocked(step string) ([]byte, error) {
	record, ok := p.state.Steps[step]
	if !ok {
		return nil, errors.New("step has no saved execution")
	}
	for _, a := range record.Outputs {
		if filepath.Base(a.Path) == outputNames[step] {
			path, err := p.Resolve(a)
			if err != nil {
				return nil, err
			}
			return os.ReadFile(path)
		}
	}
	return nil, errors.New("result not committed")
}

// CandidateReportEnvelope builds the hub_priority report envelope atomically under
// the project lock: re-checked validity + step record + upstream execution ids +
// input SHAs + the committed result, all from ONE consistent state read. Callers
// must not assemble this from separate Snapshot/Validity/Result calls.
func (p *Project) CandidateReportEnvelope(engine string) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if reason := p.validity(engine)["hub_priority"]; reason != "" {
		return nil, errors.New(reason)
	}
	step, ok := p.state.Steps["hub_priority"]
	if !ok {
		return nil, errors.New("hub_priority not run")
	}
	data, err := p.readResultLocked("hub_priority")
	if err != nil {
		return nil, err
	}
	envelope := map[string]any{
		"project_id":   p.state.ID,
		"step":         "hub_priority",
		"execution_id": step.ExecutionID,
		"engine":       step.Engine,
		"params":       step.Params,
		"inputs":       step.Inputs,
		"upstream":     step.Upstream,
		"validity":     "",
		"result":       json.RawMessage(data),
	}
	return json.Marshal(envelope)
}

// Output reads one recorded output file by basename (e.g. step1_filtered.tsv).
func (p *Project) Output(step, basename string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	record, ok := p.state.Steps[step]
	if !ok {
		return nil, errors.New("step has no saved execution")
	}
	for _, a := range record.Outputs {
		if filepath.Base(a.Path) == basename {
			path, err := p.Resolve(a)
			if err != nil {
				return nil, err
			}
			return os.ReadFile(path)
		}
	}
	return nil, errors.New("output not committed")
}

// Input reads an imported data file (e.g. sample_info) by role.
func (p *Project) Input(role string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.state.Inputs[role]
	if !ok {
		return nil, errors.New("input not imported")
	}
	path, err := p.Resolve(a)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// Progress returns the latest worker progress, or the zero value before a run.
func (p *Project) Progress() Progress { p.mu.Lock(); defer p.mu.Unlock(); return p.progress }

func (p *Project) setProgress(pr Progress) { p.mu.Lock(); p.progress = pr; p.mu.Unlock() }
