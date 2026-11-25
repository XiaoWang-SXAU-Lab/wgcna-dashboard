package regutrace

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

type TFCatalogueItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Family      string   `json:"family"`
	Status      string   `json:"status"`
	Description string   `json:"description,omitempty"`
	Motifs      []string `json:"motifs"`

	MappingStatus      string `json:"mapping_status"`
	MappingMethod      string `json:"mapping_method,omitempty"`
	ReferenceTFID      string `json:"reference_tf_id,omitempty"`
	MotifCount         int    `json:"motif_count"`
	TargetCount        int    `json:"target_count"`
	TestableMotifs     int    `json:"testable_motifs,omitempty"`
	UnattainableMotifs int    `json:"unattainable_motifs,omitempty"`
	DomainArchitecture string `json:"domain_architecture,omitempty"`
	ProteinLength      int    `json:"protein_length,omitempty"`
	DiagnosticReason   string `json:"diagnostic_reason,omitempty"`
}

func readCatalogueMotifs(path string, includeFallback bool) (map[string][]string, error) {
	result := map[string][]string{}
	if path == "" {
		return result, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	tf := findColumn(head, "tf_id", "tf")
	motif := findColumn(head, "motif_id", "motif")
	mode := findColumn(head, "domain_mode")
	if tf < 0 || motif < 0 {
		return nil, errors.New("TF map needs tf_id and motif_id")
	}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if !includeFallback && strings.EqualFold(cell(row, mode), "single_domain_fallback") {
			continue
		}
		id := cell(row, tf)
		result[id] = uniqueStrings(append(result[id], cell(row, motif))...)
	}
	for _, ids := range result {
		sort.Strings(ids)
	}
	return result, nil
}

func (j *jobState) addTFCatalogue(summary *Summary, data *dataset) error {
	if j.tfGenesPath == "" {
		return nil
	}
	catalogue, err := readTFCatalogue(j.tfGenesPath)
	if err != nil {
		return err
	}
	var byTF map[string][]string
	if j.phase4 != nil && len(j.phase4.Mappings) > 0 {
		byTF = map[string][]string{}
		for tfID, rows := range j.phase4.Mappings {
			for _, row := range rows {
				if row.DomainMode == "single_domain_fallback" && !j.Params.IncludeSingleDomainFallback {
					continue
				}
				byTF[tfID] = append(byTF[tfID], row.MotifID)
			}
			byTF[tfID] = uniqueStrings(byTF[tfID]...)
		}
	} else {
		byTF, err = readCatalogueMotifs(j.tfMapPath, j.Params.IncludeSingleDomainFallback)
		if err != nil {
			return err
		}
	}
	identified, mapped, ambiguous, unmapped := 0, 0, 0, 0
	withTargets, withTestable := 0, 0
	statusCounts := map[string]int{}
	families := map[string]bool{}
	scanned := map[string]bool{}
	for _, rec := range catalogue {
		if rec.Status == "identified" {
			identified++
		}
		if rec.Status == "ambiguous" {
			ambiguous++
		}
		ids := byTF[rec.GeneID]
		statuses := []string{}
		methods := []string{}
		reference := ""
		for _, row := range j.mappingRows(rec.GeneID) {
			if row.DomainMode == "single_domain_fallback" && !j.Params.IncludeSingleDomainFallback {
				continue
			}
			statuses = append(statuses, row.Status)
			if row.Method != "" {
				methods = append(methods, row.Method)
			}
			if reference == "" {
				reference = row.ReferenceTFID
			}
		}
		status := bestStatus(statuses)
		if len(statuses) == 0 {
			if j.tfMapPath == "" {
				status = "not_computed"
			} else {
				status = "no_supported_motif"
			}
		}
		if len(ids) > 0 {
			mapped++
		}
		if rec.Status == "identified" && len(ids) == 0 {
			unmapped++
		}
		for _, motif := range ids {
			scanned[motif] = true
		}
		if rec.Family != "" {
			families[rec.Family] = true
		}
		testable, unattainable := 0, 0
		for _, motif := range ids {
			item := j.testability(motif)
			switch item.Status {
			case "significant_hits", "no_significant_hits":
				testable++
			case "threshold_unattainable":
				unattainable++
			}
		}
		if rec.Status == "identified" && data != nil && data.tfTargetCounts[rec.GeneID] > 0 {
			withTargets++
		}
		if rec.Status == "identified" && testable > 0 {
			withTestable++
		}
		domains := parseTFProteins(rec.EvidenceJSON)
		architecture := ""
		proteinLength := 0
		archSet := map[string]bool{}
		for _, protein := range domains {
			if protein.Length > proteinLength {
				proteinLength = protein.Length
			}
			for _, domain := range protein.Domains {
				label := domain.ModelName
				if label == "" {
					label = domain.Family
				}
				if label == "" {
					label = domain.ModelAccession
				}
				if label != "" && !archSet[label] {
					archSet[label] = true
					if architecture != "" {
						architecture += ";"
					}
					architecture += label
				}
			}
		}
		targetCount := 0
		if data != nil {
			targetCount = data.tfTargetCounts[rec.GeneID]
		}
		reason := ""
		switch status {
		case "not_computed":
			reason = "No TF–motif map was supplied; run a reference pack or provide tf_map.tsv."
		case "no_supported_motif":
			reason = "No supported motif assignment in tf_map.tsv for this TF."
		case "dbd_ambiguous":
			reason = "The DBD mapping is ambiguous; inspect the donor/motif rows before using it."
		}
		if reason == "" && len(ids) > 0 && targetCount == 0 {
			reason = "Mapped and scanned, but no promoter passed the current threshold."
		}
		if reason == "" && unattainable > 0 && testable == 0 {
			reason = "All mapped motifs are untestable at the requested p-value."
		}
		statusCounts[status]++
		summary.TFCatalogue = append(summary.TFCatalogue, TFCatalogueItem{
			ID: rec.GeneID, Name: j.tfDisplayName(rec.GeneID, rec.TFSymbol),
			Family: rec.Family, Status: rec.Status,
			Description: j.annotations[rec.GeneID].Description, Motifs: append([]string{}, ids...),
			MappingStatus: status, MappingMethod: firstNonEmpty(methods...), ReferenceTFID: reference,
			MotifCount: len(ids), TargetCount: targetCount,
			TestableMotifs: testable, UnattainableMotifs: unattainable,
			DomainArchitecture: architecture, ProteinLength: proteinLength, DiagnosticReason: reason,
		})
	}
	sort.Slice(summary.TFCatalogue, func(a, b int) bool {
		return summary.TFCatalogue[a].ID < summary.TFCatalogue[b].ID
	})
	summary.MappingStatusCounts = statusCounts
	summary.TFIdentified, summary.TFMapped = &identified, &mapped
	summary.TFAmbiguous = &ambiguous
	summary.UnmappedTFs = &unmapped
	summary.TFsWithTargets = &withTargets
	summary.TFsWithTestableMotifs = &withTestable
	if identified > 0 {
		coverage := float64(withTargets) / float64(identified)
		summary.ActualTargetCoverage = &coverage
	}
	familyCount := len(families)
	summary.TFFamilies = &familyCount
	motifCount := len(scanned)
	summary.MotifsScanned = &motifCount
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
