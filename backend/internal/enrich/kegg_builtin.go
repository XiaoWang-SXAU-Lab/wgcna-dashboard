package enrich

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"sync"
)

//go:embed data/kegg_plants.tsv.gz
var keggPlantsGz []byte

var (
	keggOnce sync.Once
	keggBG   *KEGGBackground
	keggErr  error
)

// BuiltinKEGGBackground loads the embedded Plants kingdom KO -> pathway
// background (TBtools-format) once and returns the shared instance.
func BuiltinKEGGBackground() (*KEGGBackground, error) {
	keggOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(keggPlantsGz))
		if err != nil {
			keggErr = err
			return
		}
		defer zr.Close()
		data, err := io.ReadAll(zr)
		if err != nil {
			keggErr = err
			return
		}
		keggBG, keggErr = ParseTBtoolsKEGG(data)
	})
	if keggErr != nil {
		return nil, keggErr
	}
	return keggBG, nil
}
