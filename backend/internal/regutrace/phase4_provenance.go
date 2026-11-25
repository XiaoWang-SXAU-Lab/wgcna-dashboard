package regutrace

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

type ProvenanceInput struct {
	Role   string `json:"role"`
	Name   string `json:"name"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type ProvenancePack struct {
	Key       string            `json:"key,omitempty"`
	Version   string            `json:"version,omitempty"`
	Species   string            `json:"species,omitempty"`
	Assembly  string            `json:"assembly,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

type Provenance struct {
	JobID         string            `json:"job_id"`
	Tool          string            `json:"tool,omitempty"`
	ToolVersion   string            `json:"tool_version,omitempty"`
	Command       string            `json:"command,omitempty"`
	Inputs        []ProvenanceInput `json:"inputs"`
	ReferencePack ProvenancePack    `json:"reference_pack,omitempty"`
	Analysis      map[string]any    `json:"analysis"`
	ScanManifest  any               `json:"scan_manifest,omitempty"`
	Testability   map[string]int    `json:"testability,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	Limitations   []string          `json:"limitations"`
}

func (j *jobState) checksumFor(role, path string) ProvenanceInput {
	input := ProvenanceInput{Role: role, Name: filepath.Base(path), SHA256: ""}
	if strings.TrimSpace(path) == "" {
		return input
	}
	if info, err := os.Stat(path); err == nil {
		input.Size = info.Size()
	}
	j.checksumMu.Lock()
	defer j.checksumMu.Unlock()
	if j.checksums == nil {
		j.checksums = map[string]string{}
	}
	if cached, ok := j.checksums[path]; ok {
		input.SHA256 = cached
		return input
	}
	if hash, err := fileSHA256(path); err == nil {
		j.checksums[path] = hash
		input.SHA256 = hash
	}
	return input
}

func (j *jobState) provenance(report *Report) Provenance {
	inputs := []ProvenanceInput{
		j.checksumFor("genome", j.genomePath),
		j.checksumFor("gff", j.gffPath),
	}
	optional := []struct{ role, path string }{
		{"proteins", j.proteinsPath},
		{"annotation", j.annotationPath},
		{"motifs", j.motifsPath},
		{"tf_map", j.tfMapPath},
		{"tf_genes", j.tfGenesPath},
		{"motif_testability", j.testabilityPath},
		{"site_table", j.sitesPath},
		{"scan_manifest", j.scanManifestPath},
	}
	for _, item := range optional {
		if strings.TrimSpace(item.path) != "" && fileExists(item.path) {
			inputs = append(inputs, j.checksumFor(item.role, item.path))
		}
	}
	pack := ProvenancePack{}
	if j.referencePackDir != "" {
		pack.Key = j.referencePackKey
		if manifest, err := loadPackManifest(j.referencePackDir); err == nil {
			pack.Version = manifest.PackVersion
			pack.Species = manifest.Species
			pack.Assembly = manifest.Assembly
			pack.Checksums = manifest.Checksums
		}
	}
	analysis := map[string]any{}
	if j.Summary != nil {
		analysis = map[string]any{
			"genes": j.Summary.Genes, "promoters": j.Summary.Promoters,
			"identified_tfs": j.Summary.TFIdentified, "mapped_tfs": j.Summary.TFMapped,
			"ambiguous_tfs": j.Summary.TFAmbiguous, "unmapped_tfs": j.Summary.UnmappedTFs,
			"mapping_status_counts": j.Summary.MappingStatusCounts,
			"unique_motifs":         j.Summary.UniqueMotifs, "unique_pwms": j.Summary.UniquePWMs,
			"candidate_sites": j.Summary.CandidateSites, "motif_target_pairs": j.Summary.MotifTargetPairs,
			"tf_target_pairs": j.Summary.TFTargetPairs, "unique_targets": j.Summary.UniqueTargets,
			"testable_motifs": j.Summary.TestableMotifs, "unattainable_motifs": j.Summary.UnattainableMotifs,
			"background_mode": j.Summary.BackgroundMode, "background_frequencies": j.Summary.BackgroundFrequencies,
			"reference_pack": j.Summary.ReferencePack, "scan_runtime_seconds": j.Summary.ScanRuntimeSeconds,
			"cache_hits": j.Summary.CacheHits, "cache_misses": j.Summary.CacheMisses,
		}
	}
	var manifest any
	if data, err := os.ReadFile(j.scanManifestPath); err == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	testability := map[string]int{}
	if j.phase4 != nil {
		for _, item := range j.phase4.Testability {
			testability[item.Status]++
		}
	}
	warnings := []string{}
	if report != nil {
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Severity == "warning" || diagnostic.Severity == "error" {
				warnings = append(warnings, diagnostic.Code+": "+diagnostic.Message)
			}
		}
	}
	if j.phase4 != nil {
		warnings = append(warnings, j.phase4.Warnings...)
	}
	limitations := []string{
		"Promoter PWM hits are computational candidate binding sites (evidence level D), not validated targets.",
		"Site p-values are conditional on a zero-order background; no site/promoter FDR is reported.",
		"Empirical background is estimated from the same scanned promoters; parameter uncertainty is not propagated.",
		"Overlapping windows and shared PWMs are dependent; motif enrichment is exploratory.",
		"dbd_calibrated is a mapping confidence, not experimental validation.",
		"No external DAP-seq/ChIP-seq benchmark is included in this job.",
	}
	tool, version, command := "", "", ""
	if report != nil {
		tool, version, command = report.Tool, report.Version, report.Command
	}
	return Provenance{
		JobID: j.ID, Tool: tool, ToolVersion: version, Command: command,
		Inputs: inputs, ReferencePack: pack, Analysis: analysis, ScanManifest: manifest,
		Testability: testability, Warnings: warnings, Limitations: limitations,
	}
}

func (m *Manager) Provenance(id string) (Provenance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return Provenance{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return Provenance{}, ErrNotCompleted
	}
	return js.provenance(js.Diagnostics), nil
}

func (m *Manager) ExportPrior(ctx context.Context, id, policy, selected string, includeExploratory bool) (string, string, error) {
	policy = strings.ToLower(strings.TrimSpace(policy))
	if policy == "" {
		policy = "strict"
	}
	if policy != "strict" && policy != "strict_calibrated" && policy != "extended" && policy != "selected" {
		return "", "", fmt.Errorf("%w: prior policy must be strict, strict_calibrated or selected", ErrUnsupportedFormat)
	}
	selectedSet := map[string]bool{}
	for _, value := range strings.Split(selected, ",") {
		if key := strings.TrimSpace(value); key != "" {
			selectedSet[key] = true
		}
	}
	if policy == "selected" && len(selectedSet) == 0 {
		return "", "", errors.New("selected prior policy requires at least one tf_id")
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
	generation := js.generation
	dir := js.dir
	bin := m.bin
	phase4 := js.phase4
	m.mu.RUnlock()
	if js.Params.SitesOnly {
		return "", "", errors.New("site-only exploratory jobs do not materialize a TF–target prior; export the site table and mapping, or run without sites_only for a prior")
	}
	if _, err := os.Stat(input); err != nil {
		return "", "", fmt.Errorf("%w: evidence table missing", ErrNotFound)
	}
	statusByTF := map[string]string{}
	if phase4 != nil {
		for tfID, rows := range phase4.Mappings {
			statuses := make([]string, 0, len(rows))
			for _, row := range rows {
				statuses = append(statuses, row.Status)
			}
			statusByTF[tfID] = bestStatus(statuses)
		}
	}
	allowed := func(tfID string) bool {
		status, known := statusByTF[tfID]
		if !known {
			return false
		}
		switch policy {
		case "strict":
			return status == "strict_rbh"
		case "strict_calibrated":
			return status == "strict_rbh" || status == "dbd_calibrated"
		case "extended":
			return status == "strict_rbh" || status == "dbd_calibrated" || status == "dbd_exploratory"
		case "selected":
			if !selectedSet[tfID] {
				return false
			}
			switch status {
			case "strict_rbh", "dbd_calibrated":
				return true
			case "dbd_exploratory":
				return includeExploratory
			default:
				return false
			}
		default:
			return false
		}
	}
	motifTestable := func(motifID string) bool {
		if phase4 == nil || len(phase4.Testability) == 0 {
			return true
		}
		item, ok := phase4.Testability[motifID]
		if !ok {
			return true
		}
		return item.Attainable && item.Status != "threshold_unattainable" && item.Status != "not_scanned"
	}
	suffix := policy
	if policy == "selected" {
		suffix = "selected"
	}
	outputTSV := filepath.Join(dir, fmt.Sprintf("evidence.%d.prior-%s.tsv", generation, suffix))
	outputDB := filepath.Join(dir, fmt.Sprintf("evidence.%d.prior-%s.db", generation, suffix))
	if _, err := os.Stat(outputDB); err == nil {
		if err := m.ensureExportCurrent(id, generation); err != nil {
			return "", "", err
		}
		return outputDB, "regutrace-" + id + "-prior-" + suffix + ".db", nil
	}
	m.exportMu.Lock()
	defer m.exportMu.Unlock()
	if _, err := os.Stat(outputDB); err == nil {
		if err := m.ensureExportCurrent(id, generation); err != nil {
			return "", "", err
		}
		return outputDB, "regutrace-" + id + "-prior-" + suffix + ".db", nil
	}
	if err := m.ensureExportCurrent(id, generation); err != nil {
		return "", "", err
	}
	in, err := os.Open(input)
	if err != nil {
		return "", "", err
	}
	defer in.Close()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	out, err := os.Create(outputTSV)
	if err != nil {
		return "", "", err
	}
	writer := bufio.NewWriterSize(out, 1<<20)
	headerWritten := false
	tfCol, motifCol := -1, -1
	for scanner.Scan() {
		line := scanner.Text()
		if !headerWritten {
			headerWritten = true
			fields := strings.Split(line, "\t")
			for i, field := range fields {
				switch field {
				case "tf_id":
					tfCol = i
				case "motif_id":
					motifCol = i
				}
			}
			if tfCol < 0 || motifCol < 0 {
				_ = out.Close()
				_ = os.Remove(outputTSV)
				return "", "", errors.New("evidence table lacks tf_id/motif_id")
			}
			if _, err := writer.WriteString(line + "\n"); err != nil {
				_ = out.Close()
				return "", "", err
			}
			continue
		}
		fields := strings.Split(line, "\t")
		if tfCol >= len(fields) || motifCol >= len(fields) {
			continue
		}
		if !allowed(fields[tfCol]) || !motifTestable(fields[motifCol]) {
			continue
		}
		if _, err := writer.WriteString(line + "\n"); err != nil {
			_ = out.Close()
			return "", "", err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = out.Close()
		return "", "", err
	}
	if err := writer.Flush(); err != nil {
		_ = out.Close()
		return "", "", err
	}
	if err := out.Close(); err != nil {
		return "", "", err
	}
	if bin == "" {
		return "", "", errors.New("ReguTrace binary is not available for prior conversion")
	}
	partial := outputDB + ".partial"
	_ = os.Remove(partial)
	cmd := exec.CommandContext(ctx, bin, "convert", "--input", outputTSV, "--out", partial, "--format", "sqlite")
	subproc.HideWindow(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(partial)
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", "", fmt.Errorf("convert prior: %s", msg)
	}
	if err := os.Rename(partial, outputDB); err != nil {
		_ = os.Remove(partial)
		return "", "", err
	}
	return outputDB, "regutrace-" + id + "-prior-" + suffix + ".db", nil
}
