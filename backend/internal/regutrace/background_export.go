package regutrace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func statusAllowed(mode, status string) bool {
	switch mode {
	case "", "exploratory":
		return true
	case "strict":
		return status == "strict_rbh"
	case "balanced":
		return status == "strict_rbh" || status == "dbd_calibrated"
	default:
		return true
	}
}

// ExportMetadata writes small, user-facing background tables. Large site and
// prior tables remain separate downloads; the manifest points at them.
func (m *Manager) ExportMetadata(id, kind, mode string) (string, string, error) {
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
	if mode == "" {
		mode = js.Params.MappingMode
	}
	if mode == "" {
		mode = "strict"
	}
	generation := js.generation
	dir := js.dir
	m.mu.RUnlock()
	if err := m.ensureExportCurrent(id, generation); err != nil {
		return "", "", err
	}
	switch kind {
	case "sites":
		path := js.sitesPath
		if !fileExists(path) {
			return "", "", fmt.Errorf("%w: site table missing", ErrNotFound)
		}
		return path, "regutrace-" + id + "-motif-sites.tsv", nil
	case "tf_catalogue":
		path := filepath.Join(dir, fmt.Sprintf("background.%d.tf_catalogue.tsv", generation))
		f, err := os.Create(path)
		if err != nil {
			return "", "", err
		}
		w := bufio.NewWriter(f)
		if _, err := w.WriteString("tf_id\ttf_symbol\tfamily\tidentification_status\tmapping_status\tmapping_method\treference_tf_id\tmotif_count\ttarget_count\ttestable_motifs\tunattainable_motifs\tdomain_architecture\tprotein_length\tdescription\tdiagnostic_reason\n"); err != nil {
			_ = f.Close()
			return "", "", err
		}
		if js.Summary != nil {
			for _, item := range js.Summary.TFCatalogue {
				fields := []string{
					item.ID, item.Name, item.Family, item.Status, item.MappingStatus, item.MappingMethod,
					item.ReferenceTFID, strconv.Itoa(item.MotifCount), strconv.Itoa(item.TargetCount),
					strconv.Itoa(item.TestableMotifs), strconv.Itoa(item.UnattainableMotifs),
					item.DomainArchitecture, strconv.Itoa(item.ProteinLength), item.Description, item.DiagnosticReason,
				}
				if _, err := w.WriteString(strings.Join(tsvClean(fields), "\t") + "\n"); err != nil {
					_ = f.Close()
					return "", "", err
				}
			}
		}
		if err := w.Flush(); err != nil {
			_ = f.Close()
			return "", "", err
		}
		if err := f.Close(); err != nil {
			return "", "", err
		}
		return path, "regutrace-" + id + "-tf-catalogue.tsv", nil
	case "tf_motif":
		path := filepath.Join(dir, fmt.Sprintf("background.%d.tf_motif.%s.tsv", generation, mode))
		f, err := os.Create(path)
		if err != nil {
			return "", "", err
		}
		w := bufio.NewWriter(f)
		if _, err := w.WriteString("tf_id\tmotif_id\tpwm_hash\tmapping_status\tmapping_method\tmapping_confidence\treference_tf_id\tdonor_species\tdbd_identity\tquery_dbd_coverage\tdonor_dbd_coverage\tspecificity_groups\talternative_donor_count\tdomain_mode\textraction_family\treference_pack_version\n"); err != nil {
			_ = f.Close()
			return "", "", err
		}
		if js.phase4 != nil {
			for _, tfID := range sortedMappingTFIDs(js.phase4.Mappings) {
				for _, row := range js.phase4.Mappings[tfID] {
					if !statusAllowed(mode, row.Status) {
						continue
					}
					if row.DomainMode == "single_domain_fallback" && !js.Params.IncludeSingleDomainFallback {
						continue
					}
					species := []string{}
					for _, entry := range js.registryEntries(row.MotifID) {
						if entry.Species != "" {
							species = append(species, entry.Species)
						}
					}
					fields := []string{
						row.TFID, row.MotifID, js.motifs[row.MotifID].PWMHash, row.Status, row.Method,
						row.MappingConfidence, row.ReferenceTFID, strings.Join(uniqueStrings(species...), ";"),
						row.DBDIdentity, row.QueryDBDCoverage, row.DonorDBDCoverage, row.SpecificityGroups,
						strconv.Itoa(row.AlternativeDonors), row.DomainMode, row.ExtractionFamily, row.ReferencePack,
					}
					if _, err := w.WriteString(strings.Join(tsvClean(fields), "\t") + "\n"); err != nil {
						_ = f.Close()
						return "", "", err
					}
				}
			}
		}
		if err := w.Flush(); err != nil {
			_ = f.Close()
			return "", "", err
		}
		if err := f.Close(); err != nil {
			return "", "", err
		}
		return path, "regutrace-" + id + "-tf-motif-" + mode + ".tsv", nil
	case "manifest":
		path := filepath.Join(dir, fmt.Sprintf("background.%d.manifest.json", generation))
		inputs := []ProvenanceInput{
			js.checksumFor("genome", js.genomePath),
			js.checksumFor("gff", js.gffPath),
		}
		for _, item := range []struct{ role, path string }{
			{"proteins", js.proteinsPath}, {"annotation", js.annotationPath},
			{"tf_genes", js.tfGenesPath}, {"tf_map", js.tfMapPath}, {"motifs", js.motifsPath},
		} {
			if item.path != "" && fileExists(item.path) {
				inputs = append(inputs, js.checksumFor(item.role, item.path))
			}
		}
		pack := ProvenancePack{Key: js.referencePackKey}
		if js.referencePackDir != "" {
			if manifest, err := loadPackManifest(js.referencePackDir); err == nil {
				pack.Version, pack.Species, pack.Assembly, pack.Checksums = manifest.PackVersion, manifest.Species, manifest.Assembly, manifest.Checksums
			}
		}
		var statistics any
		if js.Summary != nil {
			slim := *js.Summary
			slim.TFCatalogue = nil
			statistics = slim
		}
		manifest := map[string]any{
			"schema_version":    "regutrace-regulatory-background-v1",
			"job_id":            js.ID,
			"software":          "ReguTrace (rsWGCNA toolkit)",
			"mapping_mode":      mode,
			"gene_id_namespace": "GFF gene_id (target_id)",
			"reference_pack":    pack,
			"inputs":            inputs,
			"promoter_definition": map[string]any{
				"upstream_bp": js.Params.PromoterLen, "downstream_bp": js.Params.Downstream,
				"strand_mode": "both",
			},
			"scan": map[string]any{
				"pvalue": js.Params.PValue, "background_mode": js.Params.BackgroundMode,
				"reuse_exact_pwm": js.Params.ReuseExactPWM, "sites_only": js.Params.SitesOnly,
			},
			"mapping_methods": []string{"strict full-length RBH", "DBD-based candidate inference when present"},
			"tables": map[string]string{
				"tf_catalogue": "background.<generation>.tf_catalogue.tsv",
				"tf_motif":     "background.<generation>.tf_motif.<mode>.tsv",
				"sites":        filepath.Base(js.sitesPath),
				"prior":        "regutrace-<job>-prior-<policy>.db",
			},
			"statistics": statistics,
			"limitations": []string{
				"PWM promoter hits are candidate binding sites (evidence level D), not experimental targets.",
				"Exploratory DBD mappings are not calibrated and are not automatically promoted.",
				"Site-level evidence may remain a separate attachment; the prior is the rsWGCNA import contract.",
			},
		}
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return "", "", err
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return "", "", err
		}
		return path, "regutrace-" + id + "-background-manifest.json", nil
	default:
		return "", "", fmt.Errorf("%w: unknown metadata kind %q", ErrUnsupportedFormat, kind)
	}
}

func sortedMappingTFIDs(values map[string][]TFMappingRow) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func tsvClean(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		value = strings.ReplaceAll(value, "\t", " ")
		value = strings.ReplaceAll(value, "\n", " ")
		out[i] = value
	}
	return out
}
