package enrich

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ModulesFile mirrors the network step's step3_modules.json.
type ModulesFile struct {
	Genes  []string `json:"genes"`
	Colors []string `json:"colors"`
}

// ModuleGenes returns the gene ids assigned to the given module color.
func ModuleGenes(jobDir, module string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(jobDir, "step3_modules.json"))
	if err != nil {
		return nil, err
	}
	var mf ModulesFile
	if err := json.Unmarshal(b, &mf); err != nil {
		return nil, err
	}
	var genes []string
	for i, color := range mf.Colors {
		if color == module {
			genes = append(genes, mf.Genes[i])
		}
	}
	return genes, nil
}

// GOEnrich runs GO over-representation for the query genes using the job's
// uploaded gene2go.tsv.
func GOEnrich(jobDir string, genes []string, padjCut float64, top int) ([]Result, error) {
	data, err := os.ReadFile(filepath.Join(jobDir, "gene2go.tsv"))
	if err != nil {
		return nil, err
	}
	bg, err := ParseGene2GO(data)
	if err != nil {
		return nil, err
	}
	rows := Run(genes, bg, padjCut, top)
	// fallback: if the gene2go table had no ontology column, derive it from the obo
	if g, err := LoadGO(); err == nil {
		for i := range rows {
			if rows[i].Ontology == "" {
				rows[i].Ontology = shortOntology(g.TermNamespace(normalizeGOID(rows[i].ID)))
			}
		}
	}
	return rows, nil
}

// KEGGEnrich runs KEGG over-representation for the query genes using the job's
// uploaded gene2kegg.tsv (gene -> KO) and the built-in Plants KO -> pathway
// background.
func KEGGEnrich(jobDir string, genes []string, padjCut float64, top int) ([]Result, error) {
	data, err := os.ReadFile(filepath.Join(jobDir, "gene2kegg.tsv"))
	if err != nil {
		return nil, err
	}
	gene2ko, err := ParseGene2KEGG(data)
	if err != nil {
		return nil, err
	}
	kb, err := BuiltinKEGGBackground()
	if err != nil {
		return nil, err
	}
	bg := kb.BuildBackground(gene2ko)
	return Run(genes, bg, padjCut, top), nil
}

// AnnotationRow is one gene's annotation for the annotation table.
type AnnotationRow struct {
	GeneID   string `json:"gene_id"`
	GeneName string `json:"gene_name"`
	Symbol   string `json:"symbol"`
	Desc     string `json:"description"`
}

// Annotate returns annotation rows for the query genes (empty values when a
// gene has no annotation).
func Annotate(jobDir string, genes []string) ([]AnnotationRow, error) {
	all := map[string]GeneAnno{}
	if data, err := os.ReadFile(filepath.Join(jobDir, "annotation.tsv")); err == nil {
		if parsed, perr := ParseAnnotation(data); perr == nil {
			all = parsed
		}
	}
	rows := make([]AnnotationRow, 0, len(genes))
	for _, gene := range genes {
		a := all[gene]
		rows = append(rows, AnnotationRow{
			GeneID:   gene,
			GeneName: a.GeneName,
			Symbol:   a.Symbol,
			Desc:     a.Description,
		})
	}
	return rows, nil
}
