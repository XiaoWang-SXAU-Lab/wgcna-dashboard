// Package demo bundles the rs-wgcna demo dataset (Arabidopsis HSFA2/HSFA3 heat
// stress memory, GEO GSE162434) so the dashboard can load it server-side or
// offer it as a downloadable zip.
package demo

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
)

//go:embed data/expr.tsv.gz
var exprGz []byte

//go:embed data/trait.tsv.gz
var traitGz []byte

//go:embed data/sample_info.tsv.gz
var sampleInfoGz []byte

//go:embed data/annotation.tsv.gz
var annotationGz []byte

//go:embed data/gene2go.tsv.gz
var gene2goGz []byte

//go:embed data/gene2kegg.tsv.gz
var gene2keggGz []byte

//go:embed data/README.md.gz
var readmeGz []byte

// Expr returns the raw expression matrix (gene x sample read counts).
func Expr() ([]byte, error) {
	return gunzip(exprGz)
}

// Trait returns the numeric trait table.
func Trait() ([]byte, error) {
	return gunzip(traitGz)
}

// Meta returns the optional metadata files keyed by their meta names
// (sample_info, annotation, gene2go, gene2kegg).
func Meta() (map[string][]byte, error) {
	meta := map[string][]byte{}
	for name, gz := range map[string][]byte{
		"sample_info": sampleInfoGz,
		"annotation":  annotationGz,
		"gene2go":     gene2goGz,
		"gene2kegg":   gene2keggGz,
	} {
		raw, err := gunzip(gz)
		if err != nil {
			return nil, err
		}
		meta[name] = raw
	}
	return meta, nil
}

// Zip returns a zip archive containing all demo files plus the README.
func Zip() ([]byte, error) {
	files := []struct {
		name string
		data []byte
	}{
		{"expr.tsv", exprGz},
		{"trait.tsv", traitGz},
		{"sample_info.tsv", sampleInfoGz},
		{"annotation.tsv", annotationGz},
		{"gene2go.tsv", gene2goGz},
		{"gene2kegg.tsv", gene2keggGz},
		{"README.md", readmeGz},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		raw, err := gunzip(f.data)
		if err != nil {
			return nil, err
		}
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(raw); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
