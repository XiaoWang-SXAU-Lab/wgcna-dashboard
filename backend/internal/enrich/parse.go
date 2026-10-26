package enrich

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
)

// readTable parses a delimiter-separated table. The delimiter is auto-detected
// (tab or comma). When hasHeader is true the first record is treated as the
// header row; otherwise all records are data.
func readTable(data []byte, hasHeader bool) (header []string, rows [][]string, err error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil, fmt.Errorf("empty table")
	}
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	comma := '\t'
	if bytes.Count(first, []byte{'\t'}) == 0 && bytes.Contains(first, []byte{','}) {
		comma = ','
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = comma
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(recs) == 0 {
		return nil, nil, fmt.Errorf("no rows")
	}
	if hasHeader {
		header = recs[0]
		for i := range header {
			header[i] = strings.TrimSpace(strings.ToLower(header[i]))
		}
		recs = recs[1:]
	}
	for _, rec := range recs {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		rows = append(rows, rec)
	}
	return header, rows, nil
}

func colIndex(header []string, names ...string) int {
	for i, h := range header {
		for _, n := range names {
			if h == n {
				return i
			}
		}
	}
	return -1
}

func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ParseGene2GO parses an organism-specific Gene2GO mapping into a GO
// Background. Recognized headers: gene_id + go_id, with optional go_name /
// ontology.
func ParseGene2GO(data []byte) (Background, error) {
	hdr, rows, err := readTable(data, true)
	if err != nil {
		return Background{}, err
	}
	gene := colIndex(hdr, "gene_id", "geneid", "gene", "id")
	goID := colIndex(hdr, "go_id", "goid", "go", "term")
	name := colIndex(hdr, "go_name", "goname", "description", "term_name")
	ont := colIndex(hdr, "ontology", "namespace", "category")
	if gene < 0 || goID < 0 {
		return Background{}, fmt.Errorf("gene2go needs gene_id and go_id columns")
	}
	bg := Background{Gene2Term: map[string][]string{}, TermName: map[string]string{}, TermOntology: map[string]string{}}
	uni := map[string]bool{}
	for _, r := range rows {
		g, t := cell(r, gene), cell(r, goID)
		if g == "" || t == "" {
			continue
		}
		bg.Gene2Term[g] = append(bg.Gene2Term[g], t)
		uni[g] = true
		if nm := cell(r, name); nm != "" {
			bg.TermName[t] = nm
		} else if bg.TermName[t] == "" {
			bg.TermName[t] = t
		}
		if o := shortOntology(cell(r, ont)); o != "" && bg.TermOntology[t] == "" {
			bg.TermOntology[t] = o
		}
	}
	bg.Universe = sortedKeys(uni)
	return bg, nil
}

// shortOntology maps a GO namespace string to its short code (BP/MF/CC).
func shortOntology(ns string) string {
	switch ns {
	case "biological_process":
		return "BP"
	case "molecular_function":
		return "MF"
	case "cellular_component":
		return "CC"
	case "BP", "MF", "CC":
		return ns
	default:
		return ""
	}
}

// normalizeGOID ensures a GO term id carries the "GO:" prefix so it can be
// looked up in the obo (which stores ids like "GO:0008150"). Handles inputs
// such as "GO:0008150", "0008150" and "GO0008150".
func normalizeGOID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "GO:") {
		return id
	}
	return "GO:" + strings.TrimPrefix(id, "GO")
}

// ParseGene2KEGG parses a gene -> KO mapping. Recognized headers: gene_id +
// ko_id (aliases ko / kegg_id / keg_id).
func ParseGene2KEGG(data []byte) (map[string][]string, error) {
	hdr, rows, err := readTable(data, true)
	if err != nil {
		return nil, err
	}
	gene := colIndex(hdr, "gene_id", "geneid", "gene", "id")
	ko := colIndex(hdr, "ko_id", "koid", "ko", "kegg_id", "keg_id")
	if gene < 0 || ko < 0 {
		return nil, fmt.Errorf("gene2kegg needs gene_id and ko_id columns")
	}
	out := map[string][]string{}
	for _, r := range rows {
		g, k := cell(r, gene), cell(r, ko)
		if g == "" || k == "" {
			continue
		}
		out[g] = append(out[g], k)
	}
	return out, nil
}

// GeneAnno holds one gene's human-readable annotation fields.
type GeneAnno struct {
	GeneName    string `json:"gene_name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
}

// ParseAnnotation parses GeneID / GeneName / Symbol / Description.
func ParseAnnotation(data []byte) (map[string]GeneAnno, error) {
	hdr, rows, err := readTable(data, true)
	if err != nil {
		return nil, err
	}
	gene := colIndex(hdr, "gene_id", "geneid", "gene", "id")
	name := colIndex(hdr, "gene_name", "genename", "name")
	sym := colIndex(hdr, "symbol")
	desc := colIndex(hdr, "description", "desc", "function")
	if gene < 0 {
		return nil, fmt.Errorf("annotation needs a gene_id column")
	}
	out := map[string]GeneAnno{}
	for _, r := range rows {
		g := cell(r, gene)
		if g == "" {
			continue
		}
		out[g] = GeneAnno{GeneName: cell(r, name), Symbol: cell(r, sym), Description: cell(r, desc)}
	}
	return out, nil
}

// SampleInfo holds one sample's free-form attributes (sample_id, sample_name,
// group, and any extra columns such as tissue / bio_rep / time).
type SampleInfo struct {
	SampleID string            `json:"sample_id"`
	Attrs    map[string]string `json:"attrs"`
}

// ParseSampleInfo parses sample metadata; sample_id is required.
func ParseSampleInfo(data []byte) ([]SampleInfo, error) {
	hdr, rows, err := readTable(data, true)
	if err != nil {
		return nil, err
	}
	id := colIndex(hdr, "sample_id", "sampleid", "sample", "id")
	if id < 0 {
		return nil, fmt.Errorf("sample info needs a sample_id column")
	}
	out := make([]SampleInfo, 0, len(rows))
	for _, r := range rows {
		sid := cell(r, id)
		if sid == "" {
			continue
		}
		attrs := map[string]string{}
		for i, h := range hdr {
			if v := cell(r, i); v != "" {
				attrs[h] = v
			}
		}
		out = append(out, SampleInfo{SampleID: sid, Attrs: attrs})
	}
	return out, nil
}
