package regutrace

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/subproc"
)

var backgroundExportMu sync.Mutex

type exportedPolicyRow struct {
	Policy          string
	PolicyVersion   string
	TFID            string
	MotifID         string
	SelectionRole   string
	SelectionSource string
	SelectionGroup  string
	Specificity     string
	ReferenceTFID   string
	Order           int
}

type exportedPolicyConfig struct {
	Policy           string
	PolicyVersion    string
	ParametersJSON   string
	SourceFile       string
	SourceSHA256     string
	CoordinateSource string
	CoordinateSHA256 string
	RowCount         int
}

func policySelectionGroup(row TFMappingRow) string {
	start, end := "", ""
	if row.QueryDBDStart > 0 {
		start = fmt.Sprintf("%.0f", row.QueryDBDStart)
	}
	if row.QueryDBDEnd > 0 {
		end = fmt.Sprintf("%.0f", row.QueryDBDEnd)
	}
	return strings.Join([]string{row.ExtractionFamily, row.DomainMode, row.DomainCount, start + "-" + end}, "|")
}

func (j *jobState) representativeMapping(tfID, motifID string) TFMappingRow {
	var chosen TFMappingRow
	found := false
	for _, row := range j.mappingRows(tfID) {
		if row.MotifID != motifID {
			continue
		}
		if !found || rankBetter(row, chosen) {
			chosen = row
			found = true
		}
	}
	return chosen
}

func (j *jobState) policyExportSelection() ([]exportedPolicyRow, []exportedPolicyConfig, string, error) {
	version := "phase5-v1"
	if j.phase4 == nil {
		return nil, nil, "", fmt.Errorf("this job has no TF–motif mapping metadata")
	}
	tfMapPath := j.tfMapPath
	if tfMapPath == "" {
		return nil, nil, "", fmt.Errorf("this job has no tf_map.tsv")
	}
	sourceHash, err := fileSHA256(tfMapPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("hash tf_map: %w", err)
	}
	baseParams := map[string]any{
		"overlap_threshold":          0.8,
		"ranking":                    []string{"full_architecture", "min(query_dbd_coverage,donor_dbd_coverage)", "dbd_identity", "dbd_bitscore", "motif_id"},
		"strict_calibrated_retained": true,
		"ambiguous_included":         false,
		"uncoordinated_exploratory":  "excluded from best/reps when any coordinate rows exist",
		"no_coordinate_fallback":     "single best exploratory motif per TF",
		"coordinate_layer":           "tf_motif_coordinate",
		"specificity_grouping":       "group_080 (0.80 DBD overlap); motifs with no 0.80 group are keyed by motif_id",
		"calibration_threshold":      "group_090, recorded per row in tf_motif.mapping_status",
		"selection_role_values":      []string{"fixed", "exploratory", "ambiguous"},
	}
	parameters := map[string]map[string]any{
		"all": {
			"candidate_set":                   "all base-map rows",
			"ambiguous_included":              true,
			"single_domain_fallback_included": true,
			"ranking":                         nil,
			"selection_role_values":           []string{"fixed", "exploratory", "ambiguous"},
		},
		"best": baseParams,
		"reps": {
			"overlap_threshold":          0.8,
			"ranking":                    baseParams["ranking"],
			"strict_calibrated_retained": true,
			"ambiguous_included":         false,
			"uncoordinated_exploratory":  "excluded from best/reps when any coordinate rows exist",
			"no_coordinate_fallback":     "single best exploratory motif per TF",
			"coordinate_layer":           "tf_motif_coordinate",
			"representative":             "one per specificity group; motif_id when absent",
			"specificity_grouping":       "group_080 (0.80 DBD overlap); motifs with no 0.80 group are keyed by motif_id",
			"calibration_threshold":      "group_090, recorded per row in tf_motif.mapping_status",
			"selection_role_values":      []string{"fixed", "exploratory", "ambiguous"},
		},
	}
	tfs := make([]string, 0, len(j.phase4.Mappings))
	for tf := range j.phase4.Mappings {
		tfs = append(tfs, tf)
	}
	sort.Strings(tfs)
	rows := make([]exportedPolicyRow, 0)
	configs := make([]exportedPolicyConfig, 0, 3)
	for _, policy := range []MotifPolicy{MotifPolicyAll, MotifPolicyBest, MotifPolicyReps} {
		count := 0
		for _, tf := range tfs {
			for order, motif := range j.policyMotifs(tf, policy) {
				row := j.representativeMapping(tf, motif)
				if row.MotifID == "" {
					continue
				}
				role := "exploratory"
				if row.Status == "strict_rbh" || row.Status == "dbd_calibrated" ||
					row.Method == "reciprocal_protein_similarity" ||
					row.Method == "reciprocal_protein_tied_candidate" {
					role = "fixed"
				} else if row.Status == "dbd_ambiguous" {
					// Ambiguity is a distinct third class: excluded from
					// best/reps but present in all. Collapsing it into
					// "exploratory" would erase that distinction downstream.
					role = "ambiguous"
				}
				source := "base_map"
				if rowHasCoordinates(row) {
					source = "coordinate_rerun"
				}
				rows = append(rows, exportedPolicyRow{
					Policy: string(policy), PolicyVersion: version, TFID: tf, MotifID: motif,
					SelectionRole: role, SelectionSource: source, SelectionGroup: policySelectionGroup(row),
					Specificity: strings.Join(j.specificityKeys(row), "|"), ReferenceTFID: row.ReferenceTFID,
					Order: order,
				})
				count++
			}
		}
		params, _ := json.Marshal(parameters[string(policy)])
		configs = append(configs, exportedPolicyConfig{
			Policy: string(policy), PolicyVersion: version, ParametersJSON: string(params),
			SourceFile: filepath.Base(tfMapPath), SourceSHA256: sourceHash,
			CoordinateSource: filepath.Base(tfMapPath), CoordinateSHA256: sourceHash, RowCount: count,
		})
	}
	return rows, configs, version, nil
}

func writePolicyTSV(dir string, rows []exportedPolicyRow, configs []exportedPolicyConfig) (string, string, error) {
	rowsPath := filepath.Join(dir, "policy-rows.tsv")
	rf, err := os.Create(rowsPath)
	if err != nil {
		return "", "", err
	}
	w := csv.NewWriter(rf)
	w.Comma = '\t'
	_ = w.Write([]string{"policy", "policy_version", "tf_id", "motif_id", "selection_role", "selection_source", "selection_group", "specificity_group", "reference_tf_id", "selection_order"})
	for _, row := range rows {
		_ = w.Write([]string{row.Policy, row.PolicyVersion, row.TFID, row.MotifID, row.SelectionRole, row.SelectionSource, row.SelectionGroup, row.Specificity, row.ReferenceTFID, fmt.Sprintf("%d", row.Order)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		rf.Close()
		return "", "", err
	}
	if err := rf.Close(); err != nil {
		return "", "", err
	}
	configPath := filepath.Join(dir, "policy-config.tsv")
	cf, err := os.Create(configPath)
	if err != nil {
		return "", "", err
	}
	cw := csv.NewWriter(cf)
	cw.Comma = '\t'
	_ = cw.Write([]string{"policy", "policy_version", "parameters_json", "source_file", "source_sha256", "coordinate_source", "coordinate_sha256", "row_count"})
	for _, row := range configs {
		_ = cw.Write([]string{row.Policy, row.PolicyVersion, row.ParametersJSON, row.SourceFile, row.SourceSHA256, row.CoordinateSource, row.CoordinateSHA256, fmt.Sprintf("%d", row.RowCount)})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		cf.Close()
		return "", "", err
	}
	if err := cf.Close(); err != nil {
		return "", "", err
	}
	return rowsPath, configPath, nil
}

func (m *Manager) ExportBackground(ctx context.Context, id string) (string, string, error) {
	if !backgroundExportMu.TryLock() {
		return "", "", fmt.Errorf("a regulatory background export is already running; wait for it to finish")
	}
	defer backgroundExportMu.Unlock()
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
	bin := m.bin
	dir := js.dir
	tfGenes := js.tfGenesPath
	tfMap := js.tfMapPath
	sites := js.sitesPath
	testability := js.testabilityPath
	promoterSummary := js.promoterSummaryPath
	scanManifest := js.scanManifestPath
	motifs := js.motifsPath
	referencePackDir := js.referencePackDir
	promoterLen := js.Params.PromoterLen
	downstream := js.Params.Downstream
	pvalue := js.Params.PValue
	mappingMode := strings.TrimSpace(js.Params.MappingMode)
	rows, configs, version, selectionErr := js.policyExportSelection()
	m.mu.RUnlock()
	if bin == "" {
		return "", "", fmt.Errorf("ReguTrace binary is not available")
	}
	for label, path := range map[string]string{
		"tf_genes": tfGenes, "tf_map": tfMap, "sites": sites, "testability": testability,
		"promoter_summary": promoterSummary, "scan_manifest": scanManifest, "motifs": motifs,
	} {
		if strings.TrimSpace(path) == "" {
			return "", "", fmt.Errorf("background export needs %s; rerun with Phase 3 artifacts enabled", label)
		}
		if _, err := os.Stat(path); err != nil {
			return "", "", fmt.Errorf("background export needs %s: %w", label, err)
		}
	}
	if selectionErr != nil {
		return "", "", selectionErr
	}
	tmpDir, err := os.MkdirTemp(dir, "background-policy-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tmpDir)
	rowsPath, configPath, err := writePolicyTSV(tmpDir, rows, configs)
	if err != nil {
		return "", "", err
	}
	// Keep the build inside the temporary directory so a failure cannot leave a
	// multi-gigabyte partial database behind, then publish it atomically.
	out := filepath.Join(tmpDir, "background-v3.db")
	finalName := "regutrace-" + id + "-background-v3.db"
	finalPath := filepath.Join(dir, finalName)
	// Replace any earlier export for this job; repeated clicks would otherwise
	// accumulate ~1.4 GB copies that nothing ever cleans up.
	if stale, err := filepath.Glob(filepath.Join(dir, "regutrace-"+id+"-background-v3*.db")); err == nil {
		for _, path := range stale {
			_ = os.Remove(path)
		}
	}
	args := []string{
		"background",
		"--tf-genes", tfGenes,
		"--tf-map", tfMap,
		"--sites", sites,
		"--testability", testability,
		"--promoter-summary", promoterSummary,
		"--scan-manifest", scanManifest,
		"--motifs", motifs,
		"--policy-rows", rowsPath,
		"--policy-config", configPath,
		"--policy-version", version,
		"--promoter-upstream", strconv.Itoa(promoterLen),
		"--promoter-downstream", strconv.Itoa(downstream),
		"--pvalue", strconv.FormatFloat(pvalue, 'g', -1, 64),
		"--out", out,
	}
	if mappingMode != "" {
		args = append(args, "--mapping-mode", mappingMode)
	}
	if referencePackDir != "" {
		candidate := firstExisting(packMetadataPath(referencePackDir, "specificity_groups"), filepath.Join(referencePackDir, "motif-specificity-groups.tsv"))
		if _, err := os.Stat(candidate); err == nil {
			args = append(args, "--motif-metadata", candidate)
		}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	subproc.HideWindow(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", "", fmt.Errorf("build regulatory background: %s", msg)
	}
	check := exec.CommandContext(ctx, bin, "validate-db", "--db", out)
	subproc.HideWindow(check)
	if output, err := check.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("validate regulatory background: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if err := os.Rename(out, finalPath); err != nil {
		return "", "", fmt.Errorf("publish regulatory background: %w", err)
	}
	return finalPath, finalName, nil
}
