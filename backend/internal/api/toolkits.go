package api

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/enrich"
)

// eggnogConvert parses an eggNOG-mapper .annotations TSV and converts it into
// the dashboard's annotation.tsv / gene2go.tsv / gene2kegg.tsv. The "kingdom"
// selects the KEGG background used to filter KO terms (only "plants" has a
// bundled background; other kingdoms keep all KO terms).
func (h *Handler) eggnogConvert(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "upload an eggNOG .annotations file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kingdom := strings.ToLower(strings.TrimSpace(r.FormValue("kingdom")))
	if kingdom == "" {
		kingdom = "plants"
	}

	header, rows, err := parseEggnog(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	idx := func(names ...string) int {
		for _, n := range names {
			for i, hd := range header {
				if hd == n {
					return i
				}
			}
		}
		return -1
	}
	q := idx("query")
	name := idx("preferred_name")
	desc := idx("description")
	goCol := idx("gos")
	koCol := idx("kegg_ko")
	if q < 0 {
		http.Error(w, "eggNOG file has no #query column", http.StatusBadRequest)
		return
	}

	var plantKO map[string]bool
	if kingdom == "plants" {
		if bg, e := enrich.BuiltinKEGGBackground(); e == nil {
			plantKO = map[string]bool{}
			for ko := range bg.KO2Pathway {
				plantKO[ko] = true
			}
		}
	}

	var ann, g2g, g2k strings.Builder
	ann.WriteString("gene_id\tgene_name\tsymbol\tdescription\n")
	g2g.WriteString("gene_id\tgo_id\n")
	g2k.WriteString("gene_id\tko_id\n")
	genes, goCount, koCount, koFiltered := 0, 0, 0, 0
	uniqGO := map[string]bool{}
	uniqKO := map[string]bool{}

	for _, rec := range rows {
		gene := strings.TrimSpace(rec[q])
		if gene == "" {
			continue
		}
		genes++
		nm := cellAt(rec, name)
		ds := cellAt(rec, desc)
		ann.WriteString(gene + "\t" + nm + "\t" + nm + "\t" + ds + "\n")
		if goCol >= 0 {
			for _, t := range strings.Split(cellAt(rec, goCol), ",") {
				t = strings.TrimSpace(t)
				if t == "" || t == "-" {
					continue
				}
				g2g.WriteString(gene + "\t" + t + "\n")
				goCount++
				uniqGO[t] = true
			}
		}
		if koCol >= 0 {
			for _, t := range strings.Split(cellAt(rec, koCol), ",") {
				ko := normalizeKO(t)
				if ko == "" {
					continue
				}
				if plantKO != nil && !plantKO[ko] {
					koFiltered++
					continue
				}
				g2k.WriteString(gene + "\t" + ko + "\n")
				koCount++
				uniqKO[ko] = true
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"kingdom": kingdom,
		"stats": map[string]int{
			"genes": genes, "go_count": goCount, "ko_count": koCount,
			"ko_filtered": koFiltered, "go_terms": len(uniqGO), "ko_terms": len(uniqKO),
		},
		"annotation": ann.String(),
		"gene2go":    g2g.String(),
		"gene2kegg":  g2k.String(),
	})
}

// parseEggnog parses an eggNOG TSV whose header starts with "#query". It
// lowercases headers and strips the leading "#" from the first column name.
func parseEggnog(data []byte) ([]string, [][]string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
	r := csv.NewReader(strings.NewReader(string(data)))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(recs) == 0 {
		return nil, nil, errors.New("empty file")
	}
	header := recs[0]
	for i := range header {
		header[i] = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(header[i], "#")))
	}
	var rows [][]string
	for _, rec := range recs[1:] {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		rows = append(rows, rec)
	}
	return header, rows, nil
}

func cellAt(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

// normalizeKO turns "ko:K00001" / "K00001" / "00001" into "K00001".
func normalizeKO(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"ko:", "KO:", "Ko:", "kO:"} {
		s = strings.TrimPrefix(s, p)
	}
	if s == "" || s == "-" {
		return ""
	}
	if !strings.HasPrefix(s, "K") {
		s = "K" + s
	}
	return s
}

// blastHit is the best alignment for one query sequence.
type blastHit struct {
	sseqid   string
	pident   float64
	evalue   float64
	bitscore float64
}

// blastMerge merges a BLAST outfmt-6 result and a reference annotation table
// (gene_id -> symbol / gene_name / description) into annotation.tsv. An optional
// base table (e.g. the eggNOG-converted annotation) is kept and overridden by the
// BLAST-derived rows.
func (h *Handler) blastMerge(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		http.Error(w, "multipart upload required", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	read := func(name string) ([]byte, error) {
		f, _, err := r.FormFile(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}

	blastData, err := read("blast")
	if err != nil {
		http.Error(w, "upload a BLAST result (outfmt 6)", http.StatusBadRequest)
		return
	}
	refData, err := read("reference")
	if err != nil {
		http.Error(w, "upload a reference annotation table", http.StatusBadRequest)
		return
	}
	baseData, _ := read("base")

	maxEvalue, _ := strconv.ParseFloat(r.FormValue("max_evalue"), 64)
	minPident, _ := strconv.ParseFloat(r.FormValue("min_pident"), 64)

	ref, err := enrich.ParseAnnotation(refData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	best, totalHits := parseBlast(blastData)

	base := map[string]enrich.GeneAnno{}
	if len(baseData) > 0 {
		if b, e := enrich.ParseAnnotation(baseData); e == nil {
			base = b
		}
	}

	merged := map[string]enrich.GeneAnno{}
	for g, a := range base {
		merged[g] = a
	}
	fromBlast := 0
	for q, hit := range best {
		if maxEvalue > 0 && hit.evalue > maxEvalue {
			continue
		}
		if minPident > 0 && hit.pident < minPident {
			continue
		}
		a, ok := ref[hit.sseqid]
		if !ok {
			continue
		}
		merged[q] = a
		fromBlast++
	}

	genes := make([]string, 0, len(merged))
	for g := range merged {
		genes = append(genes, g)
	}
	sort.Strings(genes)
	var out strings.Builder
	out.WriteString("gene_id\tgene_name\tsymbol\tdescription\n")
	for _, g := range genes {
		a := merged[g]
		out.WriteString(g + "\t" + a.GeneName + "\t" + a.Symbol + "\t" + a.Description + "\n")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stats": map[string]int{
			"genes": len(merged), "from_blast": fromBlast, "from_base": len(base), "blast_hits": totalHits,
		},
		"annotation": out.String(),
	})
}

// parseBlast keeps the best hit (lowest E-value, then highest bitscore) per query.
func parseBlast(data []byte) (map[string]blastHit, int) {
	best := map[string]blastHit{}
	total := 0
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if len(rec) < 12 {
			continue
		}
		total++
		q, s := rec[0], rec[1]
		pident, _ := strconv.ParseFloat(rec[2], 64)
		evalue, _ := strconv.ParseFloat(rec[10], 64)
		bitscore, _ := strconv.ParseFloat(rec[11], 64)
		cur, ok := best[q]
		if !ok || evalue < cur.evalue || (evalue == cur.evalue && bitscore > cur.bitscore) {
			best[q] = blastHit{sseqid: s, pident: pident, evalue: evalue, bitscore: bitscore}
		}
	}
	return best, total
}
