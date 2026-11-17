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

func (j *jobState) addTFCatalogue(summary *Summary) error {
	if j.tfGenesPath == "" {
		return nil
	}
	catalogue, err := readTFCatalogue(j.tfGenesPath)
	if err != nil {
		return err
	}
	byTF, err := readCatalogueMotifs(j.tfMapPath, j.Params.IncludeSingleDomainFallback)
	if err != nil {
		return err
	}
	identified, mapped := 0, 0
	scanned := map[string]bool{}
	for _, rec := range catalogue {
		if rec.Status == "identified" {
			identified++
		}
		ids := byTF[rec.GeneID]
		if len(ids) > 0 {
			mapped++
		}
		for _, motif := range ids {
			scanned[motif] = true
		}
		summary.TFCatalogue = append(summary.TFCatalogue, TFCatalogueItem{
			ID: rec.GeneID, Name: j.tfDisplayName(rec.GeneID, rec.TFSymbol),
			Family: rec.Family, Status: rec.Status,
			Description: j.annotations[rec.GeneID].Description, Motifs: append([]string{}, ids...),
		})
	}
	sort.Slice(summary.TFCatalogue, func(a, b int) bool {
		return summary.TFCatalogue[a].ID < summary.TFCatalogue[b].ID
	})
	motifCount := len(scanned)
	summary.TFIdentified, summary.TFMapped, summary.MotifsScanned = &identified, &mapped, &motifCount
	return nil
}
