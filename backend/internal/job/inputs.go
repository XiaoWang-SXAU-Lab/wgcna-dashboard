package job

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/inputcheck"
	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

// CheckInput is shared by preflight and the actual upload endpoints. The old
// XLS format uses the worker's existing calamine dependency; XLSX stays in Go.
func (m *Manager) CheckInput(raw []byte, kind, dataType string) (*inputcheck.Table, inputcheck.Report) {
	if bytes.HasPrefix(raw, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		dir, err := os.MkdirTemp("", "rswgcna-xls-")
		if err != nil {
			r := inputcheck.NewReport()
			r.Add("error", kind, err.Error(), 0, 0)
			return nil, r
		}
		defer os.RemoveAll(dir)
		input, output := filepath.Join(dir, "input.xls"), filepath.Join(dir, "output.tsv")
		err = os.WriteFile(input, raw, 0600)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, m.workerBin, "--convert-workbook", input, output)
			subproc.HideWindow(cmd)
			var log []byte
			log, err = cmd.CombinedOutput()
			if err != nil {
				err = fmt.Errorf("cannot convert XLS workbook: %s (%w)", log, err)
			}
		}
		if err == nil {
			raw, err = os.ReadFile(output)
		}
		if err != nil {
			r := inputcheck.NewReport()
			r.Add("error", kind, err.Error(), 0, 0)
			return nil, r
		}
	}
	return inputcheck.Parse(raw, kind, dataType)
}

func (m *Manager) expressionSamples(id string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(m.dir, id, "expr.tsv"))
	if err != nil {
		return nil, err
	}
	t, r := m.CheckInput(b, "expr", "normalized_count")
	if !r.Valid {
		return nil, r
	}
	return t.Samples(), nil
}

// SyncSampleTables regenerates analysis inputs from untouched upload copies.
// It is safe to repeat and restores rows if exclusions are reset. Gene-indexed
// annotations are deliberately independent of sample removal.
func (m *Manager) SyncSampleTables(id string) error {
	dir := filepath.Join(m.dir, id)
	raw, err := os.ReadFile(filepath.Join(dir, "step1_filter.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var result struct {
		Samples []string `json:"samples"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return err
	}
	if len(result.Samples) == 0 {
		return fmt.Errorf("filtered sample list is empty")
	}
	all, err := m.expressionSamples(id)
	if err != nil {
		return err
	}
	allSet := map[string]bool{}
	for _, s := range all {
		allSet[s] = true
	}
	seen := map[string]bool{}
	for _, s := range result.Samples {
		if !allSet[s] || seen[s] {
			return fmt.Errorf("invalid filtered sample ID %q", s)
		}
		seen[s] = true
	}
	excluded := []string{}
	for _, s := range all {
		if !seen[s] {
			excluded = append(excluded, s)
		}
	}
	outputs := map[string][]byte{}
	for _, kind := range []string{"trait", "sample_info"} {
		source := filepath.Join(dir, kind+".uploaded.tsv")
		b, e := os.ReadFile(source)
		if os.IsNotExist(e) {
			b, e = os.ReadFile(filepath.Join(dir, kind+".tsv"))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			// Preserve legacy job files before the first synchronization.
			if e = os.WriteFile(source, b, 0644); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		table, report := m.CheckInput(b, kind, "")
		if !report.Valid {
			return report
		}
		aligned, report := inputcheck.Align(table, result.Samples, excluded, kind)
		if !report.Valid {
			return report
		}
		outputs[filepath.Join(dir, kind+".tsv")] = aligned.Bytes()
	}
	for path, b := range outputs {
		if err = writeInput(path, b); err != nil {
			return err
		}
	}
	return nil
}

func writeInput(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".input-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Windows cannot rename over an existing file. Keep the previous bytes for
	// recovery when replacing fails, instead of leaving a truncated input.
	old, readErr := os.ReadFile(path)
	if err = os.Rename(name, path); err == nil {
		return nil
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if err = os.WriteFile(path, b, 0644); err != nil {
		if readErr == nil {
			_ = os.WriteFile(path, old, 0644)
		}
		return err
	}
	return nil
}
