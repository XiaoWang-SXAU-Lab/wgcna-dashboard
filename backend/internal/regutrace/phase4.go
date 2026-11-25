package regutrace

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// TFMappingRow is one tf_map.tsv row with its provenance. Mapping status and
// TF–target evidence level are deliberately separate fields.
type TFMappingRow struct {
	TFID              string  `json:"tf_id"`
	MotifID           string  `json:"motif_id"`
	Status            string  `json:"mapping_status"`
	Method            string  `json:"mapping_method,omitempty"`
	ReferenceTFID     string  `json:"reference_tf_id,omitempty"`
	DomainMode        string  `json:"domain_mode,omitempty"`
	DomainCount       string  `json:"domain_count,omitempty"`
	ExtractionFamily  string  `json:"extraction_family,omitempty"`
	MappingConfidence string  `json:"mapping_confidence,omitempty"`
	SpecificityGroups string  `json:"specificity_groups,omitempty"`
	DBDIdentity       string  `json:"dbd_identity,omitempty"`
	QueryDBDCoverage  string  `json:"query_dbd_coverage,omitempty"`
	DonorDBDCoverage  string  `json:"donor_dbd_coverage,omitempty"`
	QueryDBDStart     float64 `json:"query_full_dbd_start,omitempty"`
	QueryDBDEnd       float64 `json:"query_full_dbd_end,omitempty"`
	DBDBitscore       float64 `json:"dbd_bitscore,omitempty"`
	AlternativeDonors int     `json:"alternative_donor_count,omitempty"`
	ReferencePack     string  `json:"reference_pack_version,omitempty"`
}

// MotifTestability is one row from motif-testability.tsv.
type MotifTestability struct {
	MotifID            string  `json:"motif_id"`
	PWMHash            string  `json:"pwm_hash,omitempty"`
	MatrixLength       int     `json:"matrix_length,omitempty"`
	InformationContent float64 `json:"information_content,omitempty"`
	MinPValue          float64 `json:"min_pvalue,omitempty"`
	RequestedPValue    float64 `json:"requested_pvalue,omitempty"`
	Attainable         bool    `json:"attainable"`
	Status             string  `json:"status"`
	Mapped             bool    `json:"mapped"`
	HitCount           int     `json:"motif_id_hit_count,omitempty"`
	TFCount            int     `json:"tf_count,omitempty"`
	BackgroundMode     string  `json:"background_mode,omitempty"`
}

// RegistryEntry is one row of a reference pack registry.tsv.
type RegistryEntry struct {
	MotifID       string `json:"motif_id"`
	DonorID       string `json:"donor_id,omitempty"`
	Family        string `json:"family,omitempty"`
	Source        string `json:"source,omitempty"`
	SourceVersion string `json:"source_version,omitempty"`
	Accession     string `json:"accession,omitempty"`
	Method        string `json:"method,omitempty"`
	Species       string `json:"species,omitempty"`
	TaxID         string `json:"tax_id,omitempty"`
	License       string `json:"license,omitempty"`
	QCStatus      string `json:"qc_status,omitempty"`
}

type scanManifestPhase4 struct {
	Background struct {
		Mode       string    `json:"mode"`
		ValuesACGT []float64 `json:"values_acgt"`
	} `json:"background"`
	Motifs struct {
		PWMGroups          int  `json:"pwm_groups"`
		UniquePWMs         int  `json:"unique_pwms"`
		DuplicatePWMGroups int  `json:"duplicate_pwm_groups"`
		ExactPWMReuse      bool `json:"exact_pwm_reuse"`
	} `json:"motifs"`
	ScanJobs struct {
		CacheHits   int `json:"cache_hits"`
		CacheMisses int `json:"cache_misses"`
		Threads     int `json:"threads"`
	} `json:"scan_jobs"`
	Sites struct {
		UniquePWMSites int `json:"unique_pwm_sites"`
		EvidenceRows   int `json:"evidence_rows"`
	} `json:"sites"`
	Testability struct {
		TestableMotifs     int `json:"testable_motifs"`
		UnattainableMotifs int `json:"unattainable_motifs"`
		MotifsWithHits     int `json:"motifs_with_hits"`
		NotScannedMotifs   int `json:"not_scanned_motifs"`
	} `json:"testability"`
	Runtime struct {
		Seconds float64 `json:"seconds"`
	} `json:"runtime"`
}

type phase4Metadata struct {
	Mappings          map[string][]TFMappingRow
	Testability       map[string]MotifTestability
	Registry          map[string][]RegistryEntry
	MotifTFs          map[string][]string
	SharedPWMTFs      map[string]int
	ScanManifest      *scanManifestPhase4
	PipelineReport    map[string]any
	SpecificityGroups map[string][]string
	Warnings          []string
}

// normalizeMappingStatus maps both explicit Phase 2.5 statuses and legacy
// mapping_method values to the five evidence-state values used by the UI.
func normalizeMappingStatus(status, method string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "strict_rbh", "dbd_calibrated", "dbd_ambiguous", "dbd_exploratory", "no_supported_motif":
		return status
	}
	method = strings.ToLower(strings.TrimSpace(method))
	switch {
	case strings.Contains(method, "dbd_transfer_calibrated"):
		return "dbd_calibrated"
	case strings.Contains(method, "dbd_transfer") || strings.Contains(method, "dbd_"):
		return "dbd_exploratory"
	default:
		return "strict_rbh"
	}
}

func statusRank(status string) int {
	switch status {
	case "strict_rbh":
		return 5
	case "dbd_calibrated":
		return 4
	case "dbd_ambiguous":
		return 3
	case "dbd_exploratory":
		return 2
	case "no_supported_motif":
		return 1
	default:
		return 0
	}
}

func bestStatus(statuses []string) string {
	best := "not_computed"
	for _, status := range statuses {
		if statusRank(status) > statusRank(best) {
			best = status
		}
	}
	return best
}

func readTFMappingRows(path string) (map[string][]TFMappingRow, error) {
	out := map[string][]TFMappingRow{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	reader, err := openMaybeGzip(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	normalized := make([]string, len(head))
	for i, value := range head {
		normalized[i] = normalizeHeader(value)
	}
	col := func(names ...string) int { return findColumn(normalized, names...) }
	tfCol := col("tf_id", "tf")
	motifCol := col("motif_id", "motif")
	if tfCol < 0 || motifCol < 0 {
		return nil, errors.New("TF map needs tf_id and motif_id")
	}
	statusCol := col("mapping_status")
	methodCol := col("mapping_method")
	refCol := col("reference_tf_id")
	modeCol := col("domain_mode")
	countCol := col("domain_count")
	familyCol := col("extraction_family", "family")
	confidenceCol := col("mapping_confidence")
	specificityCol := col("specificity_groups")
	identityCol := col("dbd_identity")
	queryCovCol := col("query_dbd_coverage")
	donorCovCol := col("donor_dbd_coverage")
	queryStartCol := col("query_full_dbd_start")
	queryEndCol := col("query_full_dbd_end")
	bitsCol := col("dbd_bitscore")
	altCol := col("alternative_donor_count")
	packCol := col("reference_pack_version")
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		tfID := cell(rec, tfCol)
		motifID := cell(rec, motifCol)
		if tfID == "" || motifID == "" {
			continue
		}
		method := cell(rec, methodCol)
		status := normalizeMappingStatus(cell(rec, statusCol), method)
		out[tfID] = append(out[tfID], TFMappingRow{
			TFID: tfID, MotifID: motifID, Status: status, Method: method,
			ReferenceTFID: cell(rec, refCol), DomainMode: cell(rec, modeCol),
			DomainCount:      cell(rec, countCol),
			ExtractionFamily: cell(rec, familyCol), MappingConfidence: cell(rec, confidenceCol),
			SpecificityGroups: cell(rec, specificityCol), DBDIdentity: cell(rec, identityCol),
			QueryDBDCoverage: cell(rec, queryCovCol), DonorDBDCoverage: cell(rec, donorCovCol),
			QueryDBDStart: floatCell(rec, queryStartCol), QueryDBDEnd: floatCell(rec, queryEndCol),
			DBDBitscore:       floatCell(rec, bitsCol),
			AlternativeDonors: intOrZero(rec, altCol), ReferencePack: cell(rec, packCol),
		})
	}
	for id, rows := range out {
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].MotifID != rows[j].MotifID {
				return rows[i].MotifID < rows[j].MotifID
			}
			return rows[i].ReferenceTFID < rows[j].ReferenceTFID
		})
		out[id] = rows
	}
	return out, nil
}

func readMotifTestability(path string) (map[string]MotifTestability, error) {
	out := map[string]MotifTestability{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	reader, err := openMaybeGzip(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	normalized := make([]string, len(head))
	for i, value := range head {
		normalized[i] = normalizeHeader(value)
	}
	col := func(names ...string) int { return findColumn(normalized, names...) }
	motifCol := col("motif_id", "motif")
	if motifCol < 0 {
		return nil, errors.New("testability table has no motif_id")
	}
	statusCol := col("status")
	pwmCol := col("pwm_hash")
	lenCol := col("matrix_length")
	icCol := col("information_content")
	minPCol := col("min_pvalue")
	reqPCol := col("requested_pvalue")
	attCol := col("attainable")
	mappedCol := col("mapped")
	hitCol := col("motif_id_hit_count")
	tfCol := col("tf_count")
	bgCol := col("background_mode")
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		id := cell(rec, motifCol)
		if id == "" {
			continue
		}
		item := MotifTestability{
			MotifID: id, PWMHash: cell(rec, pwmCol), Status: cell(rec, statusCol),
			BackgroundMode: cell(rec, bgCol), Mapped: strings.EqualFold(cell(rec, mappedCol), "true"),
			Attainable: strings.EqualFold(cell(rec, attCol), "true"),
		}
		item.MatrixLength, _ = strconv.Atoi(cell(rec, lenCol))
		item.InformationContent, _ = strconv.ParseFloat(cell(rec, icCol), 64)
		item.MinPValue, _ = strconv.ParseFloat(cell(rec, minPCol), 64)
		item.RequestedPValue, _ = strconv.ParseFloat(cell(rec, reqPCol), 64)
		item.HitCount, _ = strconv.Atoi(cell(rec, hitCol))
		item.TFCount, _ = strconv.Atoi(cell(rec, tfCol))
		if item.Status == "" {
			item.Status = "not_computed"
		}
		out[id] = item
	}
	return out, nil
}

func readRegistry(path string) (map[string][]RegistryEntry, error) {
	out := map[string][]RegistryEntry{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	reader, err := openMaybeGzip(path)
	if err != nil {
		return out, nil // registry is optional
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	head, err := r.Read()
	if err != nil {
		return out, nil
	}
	normalized := make([]string, len(head))
	for i, value := range head {
		normalized[i] = normalizeHeader(value)
	}
	col := func(names ...string) int { return findColumn(normalized, names...) }
	motifCol := col("motif_id")
	if motifCol < 0 {
		return out, nil
	}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out, nil
		}
		id := cell(rec, motifCol)
		if id == "" {
			continue
		}
		out[id] = append(out[id], RegistryEntry{
			MotifID: id, DonorID: cell(rec, col("donor_id")), Family: cell(rec, col("family")),
			Source: cell(rec, col("source")), SourceVersion: cell(rec, col("source_version")),
			Accession: cell(rec, col("accession")), Method: cell(rec, col("method")),
			Species: cell(rec, col("species")), TaxID: cell(rec, col("tax_id")),
			License: cell(rec, col("license")), QCStatus: cell(rec, col("qc_status")),
		})
	}
	return out, nil
}

func readSpecificityGroups(path string) (map[string][]string, error) {
	out := map[string][]string{}
	reader, err := openMaybeGzip(path)
	if err != nil {
		return out, err
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '	'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	head, err := r.Read()
	if err != nil {
		return out, err
	}
	normalized := make([]string, len(head))
	for i, value := range head {
		normalized[i] = normalizeHeader(value)
	}
	motifCol := findColumn(normalized, "motif_id")
	groupCol := findColumn(normalized, "specificity_group", "group_id", "group")
	if motifCol < 0 || groupCol < 0 {
		return out, errors.New("specificity group table needs motif_id and group")
	}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return out, err
		}
		motif, group := cell(rec, motifCol), cell(rec, groupCol)
		if motif != "" && group != "" {
			out[motif] = uniqueStrings(append(out[motif], group)...)
		}
	}
	return out, nil
}

func readScanManifest(path string) (*scanManifestPhase4, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest scanManifestPhase4
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func readPipelineReport(path string) (map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return report, nil
}

func (j *jobState) loadPhase4Metadata(report *Report) {
	meta := &phase4Metadata{
		Mappings: map[string][]TFMappingRow{}, Testability: map[string]MotifTestability{},
		Registry: map[string][]RegistryEntry{}, MotifTFs: map[string][]string{},
		SharedPWMTFs: map[string]int{}, SpecificityGroups: map[string][]string{},
	}
	paths := []struct {
		label string
		path  string
		load  func(string) error
	}{
		{"tf_map.tsv", j.tfMapPath, func(path string) error {
			rows, err := readTFMappingRows(path)
			if err != nil {
				return err
			}
			meta.Mappings = rows
			return nil
		}},
		{"motif-testability.tsv", firstExisting(j.testabilityPath, filepath.Join(j.dir, "testability.tsv")), func(path string) error {
			rows, err := readMotifTestability(path)
			if err != nil {
				return err
			}
			meta.Testability = rows
			return nil
		}},
	}
	for _, item := range paths {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		if _, err := os.Stat(item.path); err != nil {
			continue
		}
		if err := item.load(item.path); err != nil {
			meta.Warnings = append(meta.Warnings, fmt.Sprintf("%s: %v", item.label, err))
		}
	}
	registryPath := ""
	if j.referencePackDir != "" {
		registryPath = filepath.Join(j.referencePackDir, "registry.tsv")
	}
	if entries, err := readRegistry(registryPath); err == nil {
		meta.Registry = entries
	}
	if j.referencePackDir != "" {
		for _, name := range []string{"motif-specificity-groups.tsv", "specificity-groups.tsv"} {
			path := filepath.Join(j.referencePackDir, name)
			if groups, err := readSpecificityGroups(path); err == nil && len(groups) > 0 {
				meta.SpecificityGroups = groups
				break
			}
		}
	}
	if manifest, err := readScanManifest(j.scanManifestPath); err == nil {
		meta.ScanManifest = manifest
	} else if err != nil {
		meta.Warnings = append(meta.Warnings, fmt.Sprintf("scan-manifest.json: %v", err))
	}
	if reportDoc, err := readPipelineReport(j.pipelineReportPath); err == nil {
		meta.PipelineReport = reportDoc
	} else if err != nil {
		meta.Warnings = append(meta.Warnings, fmt.Sprintf("pipeline-report.json: %v", err))
	}
	// Motif -> mapped TFs and shared-PWM counts.
	for tfID, rows := range meta.Mappings {
		for _, row := range rows {
			if row.DomainMode == "single_domain_fallback" && !j.Params.IncludeSingleDomainFallback {
				continue
			}
			meta.MotifTFs[row.MotifID] = append(meta.MotifTFs[row.MotifID], tfID)
		}
	}
	for motifID, tfs := range meta.MotifTFs {
		meta.MotifTFs[motifID] = uniqueStrings(tfs...)
	}
	byHash := map[string]map[string]bool{}
	for motifID, tfs := range meta.MotifTFs {
		meta := j.motifs[motifID]
		if meta.PWMHash == "" {
			continue
		}
		if byHash[meta.PWMHash] == nil {
			byHash[meta.PWMHash] = map[string]bool{}
		}
		for _, tf := range tfs {
			byHash[meta.PWMHash][tf] = true
		}
	}
	for hash, tfs := range byHash {
		meta.SharedPWMTFs[hash] = len(tfs)
	}
	j.phase4 = meta
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if strings.TrimSpace(path) != "" {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func reportContextString(r *Report, code, key string) string {
	if r == nil {
		return ""
	}
	for _, d := range r.Diagnostics {
		if d.Code == code {
			return strings.TrimSpace(d.Context[key])
		}
	}
	return ""
}

func reportContextFloat(r *Report, code, key string) (float64, bool) {
	raw := reportContextString(r, code, key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (j *jobState) enrichPhase4Summary(report *Report) {
	summary := j.Summary
	if summary == nil {
		return
	}
	meta := j.phase4
	if meta == nil {
		return
	}
	summary.TestabilityAvailable = len(meta.Testability) > 0
	summary.ScanManifestAvailable = meta.ScanManifest != nil
	summary.SiteTableAvailable = j.sitesPath != "" && fileExists(j.sitesPath)
	summary.MetadataWarnings = meta.Warnings
	if meta.ScanManifest != nil {
		m := meta.ScanManifest
		summary.BackgroundMode = m.Background.Mode
		summary.BackgroundFrequencies = append([]float64{}, m.Background.ValuesACGT...)
		if m.Motifs.UniquePWMs > 0 {
			value := m.Motifs.UniquePWMs
			summary.UniquePWMs = &value
		}
		if m.Motifs.PWMGroups > 0 {
			value := m.Motifs.PWMGroups
			summary.PWMGroups = &value
		}
		if m.Sites.UniquePWMSites > 0 {
			value := m.Sites.UniquePWMSites
			summary.CandidateSites = &value
		}
		if m.Runtime.Seconds > 0 {
			value := m.Runtime.Seconds
			summary.ScanRuntimeSeconds = &value
		}
		hits, misses := m.ScanJobs.CacheHits, m.ScanJobs.CacheMisses
		summary.CacheHits, summary.CacheMisses = &hits, &misses
	}
	if summary.BackgroundMode == "" {
		summary.BackgroundMode = reportContextString(report, "SCAN_SUMMARY", "background_mode")
	}
	if len(summary.BackgroundFrequencies) == 0 {
		if raw := reportContextString(report, "SCAN_SUMMARY", "background_values"); raw != "" {
			parts := strings.Split(raw, ",")
			if len(parts) == 4 {
				values := make([]float64, 4)
				ok := true
				for i, part := range parts {
					value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
					if err != nil {
						ok = false
						break
					}
					values[i] = value
				}
				if ok {
					summary.BackgroundFrequencies = values
				}
			}
		}
	}
	if summary.BackgroundMode == "" {
		summary.BackgroundMode = j.Params.BackgroundMode
	}
	if summary.BackgroundMode == "" {
		summary.BackgroundMode = "uniform"
	}
	if len(summary.BackgroundFrequencies) == 0 {
		summary.BackgroundFrequencies = append([]float64{}, j.Params.Background[:]...)
	}
	if summary.CandidateSites == nil {
		if hits := reportContextInt(report, "SCAN_SUMMARY", "unique_site_hits"); hits > 0 {
			value := hits
			summary.CandidateSites = &value
		}
	}
	if summary.UniquePWMs == nil {
		hashes := map[string]bool{}
		for motifID := range meta.MotifTFs {
			if hash := j.motifs[motifID].PWMHash; hash != "" {
				hashes[hash] = true
			}
		}
		if len(hashes) > 0 {
			value := len(hashes)
			summary.UniquePWMs = &value
		}
	}
	if summary.TestableMotifs == nil && len(meta.Testability) > 0 {
		testable, unattainable, notScanned := 0, 0, 0
		for _, item := range meta.Testability {
			switch item.Status {
			case "significant_hits", "no_significant_hits":
				testable++
			case "threshold_unattainable":
				unattainable++
			case "not_scanned":
				notScanned++
			}
		}
		summary.TestableMotifs, summary.UnattainableMotifs, summary.NotScannedMotifs = &testable, &unattainable, &notScanned
	}
	if meta.ScanManifest != nil {
		if summary.TestableMotifs == nil {
			value := meta.ScanManifest.Testability.TestableMotifs
			summary.TestableMotifs = &value
		}
		if summary.UnattainableMotifs == nil {
			value := meta.ScanManifest.Testability.UnattainableMotifs
			summary.UnattainableMotifs = &value
		}
	}
	if summary.CacheHits == nil {
		if hits := reportContextInt(report, "SCAN_SUMMARY", "cache_hits"); hits > 0 || reportContextString(report, "SCAN_SUMMARY", "cache_hits") != "" {
			misses := reportContextInt(report, "SCAN_SUMMARY", "cache_misses")
			summary.CacheHits, summary.CacheMisses = &hits, &misses
		}
	}
	if summary.ScanRuntimeSeconds == nil {
		if value, ok := reportContextFloat(report, "SCAN_SUMMARY", "runtime_seconds"); ok {
			summary.ScanRuntimeSeconds = &value
		}
	}
	if summary.ReferencePack == "" {
		if meta.PipelineReport != nil {
			if pack, ok := meta.PipelineReport["reference_pack"].(map[string]any); ok {
				if version, ok := pack["version"].(string); ok {
					summary.ReferencePack = version
				}
			}
		}
	}
	if summary.ReferencePack == "" {
		summary.ReferencePack = j.referencePackKey
	}
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (j *jobState) modeAllows(status string) bool {
	switch j.Params.MappingMode {
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

func (j *jobState) mappingRows(tfID string) []TFMappingRow {
	if j.phase4 == nil {
		return nil
	}
	return j.phase4.Mappings[tfID]
}

func (j *jobState) mappingStatus(tfID, motifID string) string {
	for _, row := range j.mappingRows(tfID) {
		if row.MotifID == motifID {
			return row.Status
		}
	}
	if j.tfMapPath == "" {
		return "not_computed"
	}
	return "no_supported_motif"
}

func (j *jobState) testability(motifID string) MotifTestability {
	if j.phase4 == nil {
		return MotifTestability{MotifID: motifID, Status: "not_computed"}
	}
	if item, ok := j.phase4.Testability[motifID]; ok {
		return item
	}
	return MotifTestability{MotifID: motifID, Status: "not_computed"}
}

func (j *jobState) sharedPWMTFCount(motifID string) int {
	if j.phase4 == nil {
		return 0
	}
	meta := j.motifs[motifID]
	if meta.PWMHash == "" {
		return 0
	}
	if count := j.phase4.SharedPWMTFs[meta.PWMHash]; count > 0 {
		return count
	}
	return len(j.phase4.MotifTFs[motifID])
}

func (j *jobState) sharedMotifTFs(motifID string) []string {
	if j.phase4 == nil {
		return nil
	}
	return append([]string{}, j.phase4.MotifTFs[motifID]...)
}

func (j *jobState) registryEntries(motifID string) []RegistryEntry {
	if j.phase4 == nil {
		return nil
	}
	return j.phase4.Registry[motifID]
}

func registryFamily(entries []RegistryEntry) string {
	for _, entry := range entries {
		if entry.Family != "" {
			return entry.Family
		}
	}
	return ""
}

func registrySpecies(entries []RegistryEntry) []string {
	seen := map[string]bool{}
	var out []string
	for _, entry := range entries {
		if entry.Species != "" && !seen[entry.Species] {
			seen[entry.Species] = true
			out = append(out, entry.Species)
		}
	}
	sort.Strings(out)
	return out
}

func floatCell(rec []string, index int) float64 {
	if index < 0 || index >= len(rec) {
		return 0
	}
	value, _ := strconv.ParseFloat(strings.TrimSpace(rec[index]), 64)
	return value
}

func floatOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func finiteOr(value, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
}
