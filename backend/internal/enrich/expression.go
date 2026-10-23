package enrich

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ExpressionResult is expression values for a set of genes across all samples.
type ExpressionResult struct {
	Samples []string        `json:"samples"`
	Rows    []ExpressionRow `json:"rows"`
}

// ExpressionRow is one gene's expression vector.
type ExpressionRow struct {
	Gene        string    `json:"gene"`
	Values      []float64 `json:"values"`
	Symbol      string    `json:"symbol"`
	GeneName    string    `json:"gene_name"`
	Description string    `json:"description"`
	Mean        float64   `json:"mean"`
}

// Expression reads expression values for the requested genes from the filtered
// matrix (step1_filtered.tsv), falling back to the raw expr.tsv.
func Expression(jobDir string, genes []string) (*ExpressionResult, error) {
	path := filepath.Join(jobDir, "step1_filtered.tsv")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(jobDir, "expr.tsv")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	want := make(map[string]bool, len(genes))
	for _, g := range genes {
		want[g] = true
	}

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) < 2 {
		return nil, io.ErrUnexpectedEOF
	}

	res := &ExpressionResult{Samples: header[1:]}
	n := len(res.Samples)
	rowMap := map[string][]float64{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 2 || !want[rec[0]] {
			continue
		}
		vals := make([]float64, n)
		for i := 1; i < len(rec) && i-1 < n; i++ {
			v, _ := strconv.ParseFloat(strings.TrimSpace(rec[i]), 64)
			vals[i-1] = v
		}
		rowMap[rec[0]] = vals
	}

	for _, g := range genes {
		vals := rowMap[g]
		if vals == nil {
			vals = make([]float64, n)
		}
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		if n > 0 {
			mean /= float64(n)
		}
		res.Rows = append(res.Rows, ExpressionRow{Gene: g, Values: vals, Mean: mean})
	}

	// Merge annotation (symbol / gene name / description) when available.
	if b, err := os.ReadFile(filepath.Join(jobDir, "annotation.tsv")); err == nil {
		if ann, perr := ParseAnnotation(b); perr == nil {
			for i := range res.Rows {
				if a, ok := ann[res.Rows[i].Gene]; ok {
					res.Rows[i].Symbol = a.Symbol
					res.Rows[i].GeneName = a.GeneName
					res.Rows[i].Description = a.Description
				}
			}
		}
	}
	return res, nil
}
