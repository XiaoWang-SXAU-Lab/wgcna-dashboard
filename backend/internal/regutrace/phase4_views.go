package regutrace

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MotifSummary is the registry-facing view of one motif. It is intentionally
// separate from TF-specific mappings: a family can have a curated motif even
// when no individual TF in the run has a supported assignment.
type MotifSummary struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Consensus          string   `json:"consensus"`
	Length             int      `json:"length"`
	PWMHash            string   `json:"pwm_hash,omitempty"`
	InformationContent float64  `json:"information_content"`
	Family             string   `json:"family,omitempty"`
	DonorSpecies       []string `json:"donor_species,omitempty"`
	Source             string   `json:"source,omitempty"`
	Method             string   `json:"method,omitempty"`
	License            string   `json:"license,omitempty"`
	QCStatus           string   `json:"qc_status,omitempty"`
	MappedTFs          []string `json:"mapped_tfs,omitempty"`
	MappedTFFamilies   []string `json:"mapped_tf_families,omitempty"`
	SharedPWMTFCount   int      `json:"shared_pwm_tf_count"`
	TestabilityStatus  string   `json:"testability_status"`
	MinPValue          *float64 `json:"min_pvalue,omitempty"`
	RequestedPValue    *float64 `json:"requested_pvalue,omitempty"`
	Attainable         *bool    `json:"attainable,omitempty"`
	EvidenceRows       int      `json:"evidence_rows"`
	FamilyResource     bool     `json:"family_resource"`
}

type MotifListResult struct {
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
	Rows   []MotifSummary `json:"rows"`
}

type MotifTFInfo struct {
	TFID          string `json:"tf_id"`
	TFSymbol      string `json:"tf_symbol,omitempty"`
	TFFamily      string `json:"tf_family,omitempty"`
	MappingStatus string `json:"mapping_status"`
	MappingMethod string `json:"mapping_method,omitempty"`
	ReferenceTFID string `json:"reference_tf_id,omitempty"`
	DomainMode    string `json:"domain_mode,omitempty"`
}

type MotifDetail struct {
	MotifSummary
	Matrix            [4][]float64    `json:"matrix,omitempty"`
	TFs               []MotifTFInfo   `json:"tfs"`
	Registry          []RegistryEntry `json:"registry,omitempty"`
	SpecificityGroups []string        `json:"specificity_groups,omitempty"`
	BackgroundMode    string          `json:"background_mode,omitempty"`
	Background        []float64       `json:"background,omitempty"`
	TargetCount       int             `json:"target_count"`
	TestabilityKnown  bool            `json:"testability_known"`
	Note              string          `json:"note,omitempty"`
}

type MotifListOptions struct {
	Query   string
	Family  string
	Species string
	Status  string
	Limit   int
	Offset  int
}

func (j *jobState) tfFamilyByID() map[string]string {
	out := map[string]string{}
	if j.Summary == nil {
		return out
	}
	for _, item := range j.Summary.TFCatalogue {
		if item.ID != "" {
			out[item.ID] = item.Family
		}
	}
	return out
}

func (j *jobState) motifSummary(motifID string, families map[string]string) MotifSummary {
	meta := j.motifs[motifID]
	entries := j.registryEntries(motifID)
	family := registryFamily(entries)
	tfs := j.sharedMotifTFs(motifID)
	tfFamilies := []string{}
	for _, tf := range tfs {
		if family := families[tf]; family != "" {
			tfFamilies = append(tfFamilies, family)
		}
	}
	if family == "" && len(tfFamilies) > 0 {
		family = tfFamilies[0]
	}
	item := j.testability(motifID)
	summary := MotifSummary{
		ID: meta.ID, Name: meta.Name, Consensus: meta.Consensus, Length: meta.Length,
		PWMHash: meta.PWMHash, InformationContent: meta.InformationContent,
		Family: family, DonorSpecies: registrySpecies(entries),
		MappedTFs: tfs, MappedTFFamilies: uniqueStrings(tfFamilies...),
		SharedPWMTFCount:  j.sharedPWMTFCount(motifID),
		TestabilityStatus: item.Status, EvidenceRows: 0,
		FamilyResource: len(tfs) == 0 && family != "",
	}
	if j.data != nil {
		summary.EvidenceRows = len(j.data.motifOffsets[motifID])
	}
	if item.Status != "not_computed" {
		minP, requested := item.MinPValue, item.RequestedPValue
		attainable := item.Attainable
		summary.MinPValue, summary.RequestedPValue, summary.Attainable = &minP, &requested, &attainable
	}
	if len(entries) > 0 {
		summary.Source = entries[0].Source
		summary.Method = entries[0].Method
		summary.License = entries[0].License
		summary.QCStatus = entries[0].QCStatus
	}
	return summary
}

func (j *jobState) motifList(opts MotifListOptions) MotifListResult {
	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	families := j.tfFamilyByID()
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	familyFilter := strings.ToLower(strings.TrimSpace(opts.Family))
	speciesFilter := strings.ToLower(strings.TrimSpace(opts.Species))
	statusFilter := strings.ToLower(strings.TrimSpace(opts.Status))
	ids := make([]string, 0, len(j.motifs))
	for id := range j.motifs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	matches := make([]MotifSummary, 0, len(ids))
	for _, id := range ids {
		item := j.motifSummary(id, families)
		if familyFilter != "" && !strings.EqualFold(item.Family, familyFilter) {
			continue
		}
		if speciesFilter != "" {
			found := false
			for _, species := range item.DonorSpecies {
				if strings.EqualFold(species, speciesFilter) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if statusFilter != "" && !strings.EqualFold(item.TestabilityStatus, statusFilter) {
			continue
		}
		if query != "" {
			hay := strings.ToLower(strings.Join([]string{
				item.ID, item.Name, item.Family, item.Consensus,
				strings.Join(item.DonorSpecies, " "), strings.Join(item.MappedTFs, " "),
			}, " "))
			if !strings.Contains(hay, query) {
				continue
			}
		}
		matches = append(matches, item)
	}
	total := len(matches)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return MotifListResult{Total: total, Limit: limit, Offset: offset, Rows: matches[offset:end]}
}

func (j *jobState) motifDetail(motifID string) (MotifDetail, error) {
	if _, ok := j.motifs[motifID]; !ok {
		return MotifDetail{}, fmt.Errorf("%w: motif %q is not in this library", ErrNotFound, motifID)
	}
	families := j.tfFamilyByID()
	summary := j.motifSummary(motifID, families)
	tfs := make([]MotifTFInfo, 0, len(summary.MappedTFs))
	for _, tfID := range summary.MappedTFs {
		status := "not_computed"
		method, reference, mode := "", "", ""
		for _, row := range j.mappingRows(tfID) {
			if row.MotifID != motifID {
				continue
			}
			status = row.Status
			method, reference, mode = row.Method, row.ReferenceTFID, row.DomainMode
			break
		}
		tfs = append(tfs, MotifTFInfo{
			TFID: tfID, TFSymbol: j.tfDisplayName(tfID, ""), TFFamily: families[tfID],
			MappingStatus: status, MappingMethod: method, ReferenceTFID: reference, DomainMode: mode,
		})
	}
	sort.Slice(tfs, func(i, k int) bool { return tfs[i].TFID < tfs[k].TFID })
	targetCount := 0
	if j.data != nil {
		targets := map[string]bool{}
		if j.data.mode == "sites" {
			_ = j.data.forEachSiteAt(j.data.motifOffsets[motifID], func(site SiteRecord) error {
				if site.TargetID != "" {
					targets[site.TargetID] = true
				}
				return nil
			})
		} else {
			_ = j.data.forEachAt(j.data.motifOffsets[motifID], func(e Edge) error {
				if e.TargetID != "" {
					targets[e.TargetID] = true
				}
				return nil
			})
		}
		targetCount = len(targets)
	}
	meta := j.motifs[motifID]
	detail := MotifDetail{
		MotifSummary: summary, Matrix: meta.Matrix, TFs: tfs,
		Registry:          j.registryEntries(motifID),
		SpecificityGroups: append([]string{}, j.phase4Specificity(motifID)...),
		BackgroundMode:    "", Background: []float64{}, TargetCount: targetCount,
		TestabilityKnown: summary.TestabilityStatus != "not_computed",
	}
	if j.Summary != nil {
		detail.BackgroundMode = j.Summary.BackgroundMode
		detail.Background = append([]float64{}, j.Summary.BackgroundFrequencies...)
	}
	if detail.FamilyResource {
		detail.Note = "This is a family-associated experimental motif resource, not an inferred TF-specific assignment."
	}
	return detail, nil
}

func (j *jobState) phase4Specificity(motifID string) []string {
	if j.phase4 == nil {
		return nil
	}
	return append([]string{}, j.phase4.SpecificityGroups[motifID]...)
}

// TFTargetRow is one aggregated target for a TF. It intentionally exposes the
// original motif and mapping status instead of hiding them behind a single
// candidate score.
type TFTargetRow struct {
	TargetID          string   `json:"target_id"`
	TargetSymbol      string   `json:"target_symbol"`
	TargetDescription string   `json:"target_description,omitempty"`
	BestPValue        *float64 `json:"best_p_value,omitempty"`
	BestScore         *float64 `json:"best_score,omitempty"`
	SiteCount         int      `json:"site_count"`
	MotifIDs          []string `json:"motif_ids"`
	MotifNames        []string `json:"motif_names,omitempty"`
	MappingStatuses   []string `json:"mapping_statuses"`
	EvidenceLevel     string   `json:"evidence_level"`
	SharedMotifTFs    int      `json:"shared_motif_tfs"`
	PromoterStart     *uint64  `json:"promoter_start,omitempty"`
	PromoterEnd       *uint64  `json:"promoter_end,omitempty"`
	HitStart          *uint64  `json:"hit_start,omitempty"`
	HitEnd            *uint64  `json:"hit_end,omitempty"`
	Strand            string   `json:"strand,omitempty"`
}

type TFTargetResult struct {
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	TFID   string        `json:"tf_id"`
	Rows   []TFTargetRow `json:"rows"`
}

type TFTargetOptions struct {
	TFID   string
	Limit  int
	Offset int
	Motif  string
	Status string
	MaxP   float64
}

func (j *jobState) resolveTFID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if j.data != nil {
		if _, ok := j.data.tfOffsets[value]; ok {
			return value, true
		}
		for id := range j.data.tfOffsets {
			if strings.EqualFold(id, value) {
				return id, true
			}
		}
	}
	if j.Summary != nil {
		for _, item := range j.Summary.TFCatalogue {
			if strings.EqualFold(item.ID, value) || strings.EqualFold(item.Name, value) {
				return item.ID, true
			}
		}
	}
	return "", false
}

func (j *jobState) tfTargets(opts TFTargetOptions) (TFTargetResult, error) {
	if j.data != nil && j.data.mode == "sites" {
		return j.siteTFTargets(opts)
	}
	if j.data == nil {
		return TFTargetResult{}, errors.New("this job has no disk-backed dataset")
	}
	tfID, ok := j.resolveTFID(opts.TFID)
	if !ok {
		return TFTargetResult{}, fmt.Errorf("%w: TF %q has no evidence in this run", ErrNotFound, opts.TFID)
	}
	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	motifFilter := strings.TrimSpace(opts.Motif)
	statusFilter := strings.TrimSpace(opts.Status)
	type aggregate struct {
		row    TFTargetRow
		motifs map[string]bool
		names  map[string]bool
		sites  map[string]bool
	}
	byTarget := map[string]*aggregate{}
	err := j.data.forEachAt(j.data.tfOffsets[tfID], func(e Edge) error {
		if opts.MaxP > 0 && e.PValue != nil && *e.PValue > opts.MaxP {
			return nil
		}
		if motifFilter != "" && !strings.EqualFold(e.MotifID, motifFilter) {
			return nil
		}
		status := j.mappingStatus(tfID, e.MotifID)
		if statusFilter != "" {
			if !strings.EqualFold(status, statusFilter) {
				return nil
			}
		} else if !j.modeAllows(status) {
			return nil
		}
		agg := byTarget[e.TargetID]
		if agg == nil {
			symbol, description, _ := j.annotationFor(e.TargetID, e.TargetSymbol)
			agg = &aggregate{
				row: TFTargetRow{TargetID: e.TargetID, TargetSymbol: symbol, TargetDescription: description,
					EvidenceLevel: e.EvidenceLevel, MappingStatuses: []string{}},
				motifs: map[string]bool{}, names: map[string]bool{}, sites: map[string]bool{},
			}
			byTarget[e.TargetID] = agg
		}
		if agg.row.BestPValue == nil || (e.PValue != nil && *e.PValue < *agg.row.BestPValue) {
			agg.row.BestPValue = e.PValue
			agg.row.BestScore = e.Score
			agg.row.PromoterStart = e.PromoterStart
			agg.row.PromoterEnd = e.PromoterEnd
			agg.row.HitStart = e.HitStart
			agg.row.HitEnd = e.HitEnd
			agg.row.Strand = e.Strand
		}
		if e.MotifID != "" {
			agg.motifs[e.MotifID] = true
			meta := j.motifs[e.MotifID]
			if meta.Name != "" {
				agg.names[meta.Name] = true
			}
			agg.row.SharedMotifTFs = maxInt(agg.row.SharedMotifTFs, j.sharedPWMTFCount(e.MotifID))
		}
		key := hitKey(e)
		if key == "" {
			key = fmt.Sprintf("%s|%s|%v|%v|%v", e.TargetID, e.MotifID, e.HitStart, e.HitEnd, e.Strand)
		}
		agg.sites[key] = true
		if !containsString(agg.row.MappingStatuses, status) {
			agg.row.MappingStatuses = append(agg.row.MappingStatuses, status)
		}
		return nil
	})
	if err != nil {
		return TFTargetResult{}, err
	}
	rows := make([]TFTargetRow, 0, len(byTarget))
	for _, agg := range byTarget {
		agg.row.SiteCount = len(agg.sites)
		for motif := range agg.motifs {
			agg.row.MotifIDs = append(agg.row.MotifIDs, motif)
		}
		for name := range agg.names {
			agg.row.MotifNames = append(agg.row.MotifNames, name)
		}
		sort.Strings(agg.row.MotifIDs)
		sort.Strings(agg.row.MotifNames)
		sort.Strings(agg.row.MappingStatuses)
		rows = append(rows, agg.row)
	}
	sort.Slice(rows, func(i, k int) bool {
		ip, kp := floatOrZero(rows[i].BestPValue), floatOrZero(rows[k].BestPValue)
		if ip != kp {
			return ip < kp
		}
		return rows[i].TargetID < rows[k].TargetID
	})
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return TFTargetResult{Total: total, Limit: limit, Offset: offset, TFID: tfID, Rows: rows[offset:end]}, nil
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// RegulatorRow is one aggregated TF for a target.
type RegulatorRow struct {
	TFID            string   `json:"tf_id"`
	TFSymbol        string   `json:"tf_symbol"`
	TFFamily        string   `json:"tf_family,omitempty"`
	BestPValue      *float64 `json:"best_p_value,omitempty"`
	BestScore       *float64 `json:"best_score,omitempty"`
	SiteCount       int      `json:"site_count"`
	MotifIDs        []string `json:"motif_ids"`
	MappingStatuses []string `json:"mapping_statuses"`
	EvidenceLevel   string   `json:"evidence_level"`
	SharedMotifTFs  int      `json:"shared_motif_tfs"`
}

type RegulatorResult struct {
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
	TargetID string         `json:"target_id"`
	Rows     []RegulatorRow `json:"rows"`
}

func (j *jobState) targetRegulators(targetID string, limit, offset int, motifFilter, statusFilter string, maxP float64) (RegulatorResult, error) {
	if j.data != nil && j.data.mode == "sites" {
		return j.siteTargetRegulators(targetID, limit, offset, motifFilter, statusFilter, maxP)
	}
	if j.data == nil {
		return RegulatorResult{}, errors.New("this job has no disk-backed dataset")
	}
	resolved, ok := j.resolveSiteTarget(targetID)
	if !ok {
		return RegulatorResult{}, fmt.Errorf("%w: target %q has no evidence in this run", ErrNotFound, targetID)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	type aggregate struct {
		row    RegulatorRow
		motifs map[string]bool
		sites  map[string]bool
	}
	byTF := map[string]*aggregate{}
	err := j.data.forEachAt(j.data.targetOffsets[resolved], func(e Edge) error {
		if maxP > 0 && e.PValue != nil && *e.PValue > maxP {
			return nil
		}
		if motifFilter != "" && !strings.EqualFold(e.MotifID, motifFilter) {
			return nil
		}
		status := j.mappingStatus(e.TFID, e.MotifID)
		if statusFilter != "" {
			if !strings.EqualFold(status, statusFilter) {
				return nil
			}
		} else if !j.modeAllows(status) {
			return nil
		}
		agg := byTF[e.TFID]
		if agg == nil {
			agg = &aggregate{
				row: RegulatorRow{TFID: e.TFID, TFSymbol: e.TFSymbol, TFFamily: e.TFFamily,
					EvidenceLevel: e.EvidenceLevel, MappingStatuses: []string{}},
				motifs: map[string]bool{}, sites: map[string]bool{},
			}
			byTF[e.TFID] = agg
		}
		if agg.row.BestPValue == nil || (e.PValue != nil && *e.PValue < *agg.row.BestPValue) {
			agg.row.BestPValue = e.PValue
			agg.row.BestScore = e.Score
		}
		if e.MotifID != "" {
			agg.motifs[e.MotifID] = true
			agg.row.SharedMotifTFs = maxInt(agg.row.SharedMotifTFs, j.sharedPWMTFCount(e.MotifID))
		}
		key := hitKey(e)
		if key == "" {
			key = fmt.Sprintf("%s|%s|%v|%v|%v", e.TargetID, e.MotifID, e.HitStart, e.HitEnd, e.Strand)
		}
		agg.sites[key] = true
		if !containsString(agg.row.MappingStatuses, status) {
			agg.row.MappingStatuses = append(agg.row.MappingStatuses, status)
		}
		return nil
	})
	if err != nil {
		return RegulatorResult{}, err
	}
	rows := make([]RegulatorRow, 0, len(byTF))
	for _, agg := range byTF {
		agg.row.SiteCount = len(agg.sites)
		for motif := range agg.motifs {
			agg.row.MotifIDs = append(agg.row.MotifIDs, motif)
		}
		sort.Strings(agg.row.MotifIDs)
		sort.Strings(agg.row.MappingStatuses)
		rows = append(rows, agg.row)
	}
	sort.Slice(rows, func(i, k int) bool {
		ip, kp := floatOrZero(rows[i].BestPValue), floatOrZero(rows[k].BestPValue)
		if ip != kp {
			return ip < kp
		}
		return rows[i].TFID < rows[k].TFID
	})
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return RegulatorResult{Total: total, Limit: limit, Offset: offset, TargetID: resolved, Rows: rows[offset:end]}, nil
}

// manager wrappers

func (m *Manager) MotifList(id string, opts MotifListOptions) (MotifListResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return MotifListResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return MotifListResult{}, ErrNotCompleted
	}
	return js.motifList(opts), nil
}

func (m *Manager) MotifDetail(id, motif string) (MotifDetail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return MotifDetail{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return MotifDetail{}, ErrNotCompleted
	}
	return js.motifDetail(motif)
}

func (m *Manager) TFTargets(id string, opts TFTargetOptions) (TFTargetResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return TFTargetResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return TFTargetResult{}, ErrNotCompleted
	}
	return js.tfTargets(opts)
}

func (m *Manager) TargetRegulators(id, target string, limit, offset int, motif, status string, maxP float64) (RegulatorResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return RegulatorResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return RegulatorResult{}, ErrNotCompleted
	}
	return js.targetRegulators(target, limit, offset, motif, status, maxP)
}
