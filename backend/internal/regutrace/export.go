package regutrace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// ErrNotFound marks a missing job or artifact.
var ErrNotFound = errors.New("regutrace job or artifact not found")

// ErrNotCompleted marks a job that has not finished a scan.
var ErrNotCompleted = errors.New("regutrace job has not completed a scan")

// ErrUnsupportedFormat marks a request for an unknown export format.
var ErrUnsupportedFormat = errors.New("unsupported export format")

type exportFormat struct {
	cliFormat string
	gzip      bool
	extension string
}

var exportFormats = map[string]exportFormat{
	"tsv":      {cliFormat: "tsv", extension: "tsv"},
	"csv":      {cliFormat: "csv", extension: "csv"},
	"tsv.gz":   {cliFormat: "tsv", gzip: true, extension: "tsv.gz"},
	"csv.gz":   {cliFormat: "csv", gzip: true, extension: "csv.gz"},
	"jsonl.gz": {cliFormat: "jsonl", gzip: true, extension: "jsonl.gz"},
	"sqlite":   {cliFormat: "sqlite", extension: "db"},
}

// ExportFormat returns the evidence table in the requested format. Text
// formats are converted from the generation-specific evidence table on demand
// and cached in the job directory. SQLite contains the full hit table plus an
// indexed `pairs` table used by project-level module queries.
func (m *Manager) ExportFormat(ctx context.Context, id, kind, format string) (string, string, error) {
	if kind == "" {
		kind = "evidence"
	}
	if kind != "evidence" {
		return m.ExportPath(id, kind)
	}
	if format == "" {
		format = "tsv"
	}
	spec, ok := exportFormats[format]
	if !ok {
		return "", "", fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
	}

	m.mu.RLock()
	js, ok := m.jobs[id]
	if !ok {
		m.mu.RUnlock()
		return "", "", fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		m.mu.RUnlock()
		return "", "", ErrNotCompleted
	}
	input := js.evidencePath
	if input == "" {
		input = filepath.Join(js.dir, "evidence.tsv")
	}
	dir := js.dir
	generation := js.generation
	bin := m.bin
	m.mu.RUnlock()

	if _, err := os.Stat(input); err != nil {
		return "", "", fmt.Errorf("%w: evidence table missing", ErrNotFound)
	}
	name := "regutrace-" + id + "-evidence." + spec.extension
	if format == "tsv" {
		return input, name, nil
	}

	output := filepath.Join(dir, fmt.Sprintf("evidence.%d.%s", generation, spec.extension))
	if _, err := os.Stat(output); err == nil {
		if err := m.ensureExportCurrent(id, generation); err != nil {
			return "", "", err
		}
		return output, name, nil
	}
	if bin == "" {
		return "", "", errors.New("ReguTrace binary is not available for format conversion")
	}

	m.exportMu.Lock()
	defer m.exportMu.Unlock()
	if _, err := os.Stat(output); err == nil {
		if err := m.ensureExportCurrent(id, generation); err != nil {
			return "", "", err
		}
		return output, name, nil
	}
	if err := m.ensureExportCurrent(id, generation); err != nil {
		return "", "", err
	}
	partial := filepath.Join(dir, fmt.Sprintf(
		".evidence.%d.%s.%d.partial", generation, spec.extension, time.Now().UnixNano(),
	))
	defer os.Remove(partial)
	args := []string{
		"convert",
		"--input", input,
		"--out", partial,
		"--format", spec.cliFormat,
	}
	if spec.gzip {
		args = append(args, "--gzip", "true")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	subproc.HideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", "", fmt.Errorf("convert evidence to %s: %s", format, msg)
	}
	if err := m.ensureExportCurrent(id, generation); err != nil {
		return "", "", err
	}
	if _, err := os.Stat(output); err == nil {
		return output, name, nil
	}
	if err := os.Rename(partial, output); err != nil {
		return "", "", fmt.Errorf("publish %s export: %w", format, err)
	}
	return output, name, nil
}

func (m *Manager) ensureExportCurrent(id string, generation uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return ErrNotCompleted
	}
	if js.generation != generation {
		return ErrNotCompleted
	}
	return nil
}
