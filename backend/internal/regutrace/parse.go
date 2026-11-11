package regutrace

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"sort"
	"strconv"
	"strings"
)

// AnnotationEntry is one row of an optional gene annotation table.
type AnnotationEntry struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
}

// MotifInfo is the display metadata for one JASPAR PFM.
type MotifInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Consensus string `json:"consensus"`
	Length    int    `json:"length"`
}

// Promoter is one oriented promoter sequence extracted by ReguTrace.
type Promoter struct {
	GeneID   string `json:"gene_id"`
	Chrom    string `json:"chrom"`
	Strand   string `json:"strand"`
	Start    uint64 `json:"start"`
	End      uint64 `json:"end"`
	Sequence string `json:"-"`
}

// Edge is one ReguTrace evidence TSV row.
type Edge struct {
	TFID          string
	TFSymbol      string
	TFFamily      string
	TargetID      string
	TargetSymbol  string
	EvidenceType  string
	EvidenceLevel string
	Method        string
	Source        string
	PMID          string
	Condition     string
	PromoterStart *uint64
	PromoterEnd   *uint64
	MotifID       string
	HitStart      *uint64
	HitEnd        *uint64
	Strand        string
	Score         *float64
	PValue        *float64
	Notes         string
}

// Report is the stable diagnostics document emitted by `regutrace
// validate`/`scan --report-json`.
type Report struct {
	Tool        string            `json:"tool"`
	Version     string            `json:"version"`
	Command     string            `json:"command"`
	Inputs      map[string]string `json:"inputs"`
	Summary     ReportSummary     `json:"summary"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

type ReportSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

type Diagnostic struct {
	Severity   string            `json:"severity"`
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	File       string            `json:"file,omitempty"`
	Line       int               `json:"line,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
	Suggestion string            `json:"suggestion,omitempty"`
}

func normalizeHeader(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.TrimPrefix(s, "#")
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func findColumn(header []string, names ...string) int {
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

func sanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func readMaybeGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	magic, _ := br.Peek(2)
	var r io.Reader = br
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

// parseAnnotation auto-detects an eggNOG-mapper .annotations table or a
// generic gene annotation table. It returns the parsed map, a normalized
// gene_id/symbol TSV for the ReguTrace CLI, and a source label.
func parseAnnotation(data []byte) (map[string]AnnotationEntry, []byte, string, error) {
	data = []byte(strings.TrimPrefix(string(data), "\uFEFF"))
	r := csv.NewReader(strings.NewReader(string(data)))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, "", fmt.Errorf("parse annotation table: %w", err)
	}

	headerIdx := -1
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		first := normalizeHeader(rec[0])
		if first == "query" || first == "gene_id" || first == "gene" || first == "id" ||
			first == "target_id" || first == "locus" || first == "locus_tag" {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, nil, "", errors.New("annotation table needs a query/gene_id column")
	}
	header := make([]string, len(records[headerIdx]))
	for i, h := range records[headerIdx] {
		header[i] = normalizeHeader(h)
	}
	first := header[0]
	geneCol := findColumn(header, "query", "gene_id", "gene", "id", "target_id", "locus", "locus_tag")
	symbolCol := findColumn(header, "preferred_name", "symbol", "gene_name", "gene_symbol", "name", "target_name")
	descCol := findColumn(header, "description", "desc", "product", "function", "annotation", "target_description")
	if geneCol < 0 {
		return nil, nil, "", errors.New("annotation table needs a query/gene_id column")
	}
	isEggnog := first == "query"
	source := "annotation_table"
	if isEggnog {
		source = "eggnog"
	}
	if symbolCol < 0 && descCol < 0 {
		return nil, nil, "", errors.New("annotation table needs a symbol/preferred_name or description column")
	}

	ann := map[string]AnnotationEntry{}
	for _, rec := range records[headerIdx+1:] {
		gene := sanitizeCell(cell(rec, geneCol))
		if gene == "" {
			continue
		}
		symbol := sanitizeCell(cell(rec, symbolCol))
		desc := sanitizeCell(cell(rec, descCol))
		if _, exists := ann[gene]; exists {
			continue
		}
		ann[gene] = AnnotationEntry{Symbol: symbol, Description: desc}
	}
	if len(ann) == 0 {
		return nil, nil, "", errors.New("annotation table has no usable rows")
	}

	genes := make([]string, 0, len(ann))
	for gene := range ann {
		genes = append(genes, gene)
	}
	sort.Strings(genes)
	var out strings.Builder
	out.WriteString("gene_id\tsymbol\n")
	for _, gene := range genes {
		symbol := ann[gene].Symbol
		if symbol == "" {
			symbol = gene
		}
		out.WriteString(gene + "\t" + symbol + "\n")
	}
	return ann, []byte(out.String()), source, nil
}

// parseMotifMeta extracts motif ID, name, length and consensus from a JASPAR
// PFM text file. Malformed motifs are skipped; the scanner remains the source
// of truth for motif validity.
func parseMotifMeta(data []byte) (map[string]MotifInfo, error) {
	out := map[string]MotifInfo{}
	var id, name string
	var rows [4][]float64
	finalize := func() {
		if id == "" {
			return
		}
		n := 0
		for _, row := range rows {
			if len(row) > n {
				n = len(row)
			}
		}
		if n == 0 {
			id, name, rows = "", "", [4][]float64{}
			return
		}
		consensus := make([]byte, n)
		letters := []byte("ACGT")
		for i := 0; i < n; i++ {
			best, bestVal := byte('N'), -1.0
			for b := 0; b < 4; b++ {
				if i < len(rows[b]) && rows[b][i] > bestVal {
					bestVal = rows[b][i]
					best = letters[b]
				}
			}
			consensus[i] = best
		}
		out[id] = MotifInfo{ID: id, Name: name, Consensus: string(consensus), Length: n}
		id, name, rows = "", "", [4][]float64{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, ">") {
			finalize()
			fields := strings.Fields(strings.TrimPrefix(line, ">"))
			if len(fields) > 0 {
				id = fields[0]
				name = strings.Join(fields[1:], " ")
			}
			continue
		}
		if id == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var b int
		switch strings.ToUpper(fields[0]) {
		case "A":
			b = 0
		case "C":
			b = 1
		case "G":
			b = 2
		case "T":
			b = 3
		default:
			continue
		}
		vals := make([]float64, 0, len(fields)-1)
		for _, tok := range fields[1:] {
			tok = strings.Trim(tok, "[]")
			if tok == "" {
				continue
			}
			v, err := strconv.ParseFloat(tok, 64)
			if err != nil {
				vals = nil
				break
			}
			vals = append(vals, v)
		}
		if len(vals) > 0 {
			rows[b] = vals
		}
	}
	finalize()
	if len(out) == 0 {
		return nil, errors.New("motif file has no usable JASPAR motifs")
	}
	return out, nil
}

func parseUintPtr(raw string) (*uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer %q", raw)
	}
	return &v, nil
}

func parseFloatPtr(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", raw)
	}
	return &v, nil
}

func familyFromNotes(notes string) string {
	for _, part := range strings.Split(notes, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "family=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "family="))
		}
	}
	return ""
}

type evidenceSchema struct {
	header       []string
	tfIDCol      int
	tfSymCol     int
	targetIDCol  int
	targetSymCol int
	typeCol      int
	levelCol     int
	methodCol    int
	sourceCol    int
	pmidCol      int
	conditionCol int
	promStartCol int
	promEndCol   int
	motifCol     int
	hitStartCol  int
	hitEndCol    int
	strandCol    int
	scoreCol     int
	pCol         int
	notesCol     int
}

func newEvidenceSchema(rawHeader []string) (*evidenceSchema, error) {
	header := make([]string, len(rawHeader))
	for i, h := range rawHeader {
		header[i] = normalizeHeader(h)
	}
	s := &evidenceSchema{header: header}
	s.tfIDCol = findColumn(header, "tf_id", "tf")
	s.tfSymCol = findColumn(header, "tf_symbol", "symbol")
	s.targetIDCol = findColumn(header, "target_id", "target", "gene_id")
	s.targetSymCol = findColumn(header, "target_symbol", "target_name")
	if s.tfIDCol < 0 || s.targetIDCol < 0 {
		return nil, errors.New("evidence TSV needs tf_id/tf and target_id/target columns")
	}
	col := func(names ...string) int { return findColumn(header, names...) }
	s.typeCol = col("evidence_type")
	s.levelCol = col("evidence_level")
	s.methodCol = col("method")
	s.sourceCol = col("source")
	s.pmidCol = col("pmid")
	s.conditionCol = col("condition")
	s.promStartCol = col("promoter_start")
	s.promEndCol = col("promoter_end")
	s.motifCol = col("motif_id", "motif")
	s.hitStartCol = col("hit_start")
	s.hitEndCol = col("hit_end")
	s.strandCol = col("strand")
	s.scoreCol = col("score")
	s.pCol = col("p_value", "pvalue", "p")
	s.notesCol = col("notes")
	return s, nil
}

func (s *evidenceSchema) edge(rec []string, row int) (Edge, error) {
	tfID := strings.TrimSpace(cell(rec, s.tfIDCol))
	targetID := strings.TrimSpace(cell(rec, s.targetIDCol))
	if tfID == "" && targetID == "" {
		return Edge{}, nil
	}
	if tfID == "" || targetID == "" {
		return Edge{}, fmt.Errorf("evidence row %d: empty tf_id or target_id", row)
	}
	promStart, err := parseUintPtr(cell(rec, s.promStartCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d promoter_start: %w", row, err)
	}
	promEnd, err := parseUintPtr(cell(rec, s.promEndCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d promoter_end: %w", row, err)
	}
	hitStart, err := parseUintPtr(cell(rec, s.hitStartCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d hit_start: %w", row, err)
	}
	hitEnd, err := parseUintPtr(cell(rec, s.hitEndCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d hit_end: %w", row, err)
	}
	score, err := parseFloatPtr(cell(rec, s.scoreCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d score: %w", row, err)
	}
	pValue, err := parseFloatPtr(cell(rec, s.pCol))
	if err != nil {
		return Edge{}, fmt.Errorf("evidence row %d p_value: %w", row, err)
	}
	tfSymbol := strings.TrimSpace(cell(rec, s.tfSymCol))
	if tfSymbol == "" {
		tfSymbol = tfID
	}
	targetSymbol := strings.TrimSpace(cell(rec, s.targetSymCol))
	if targetSymbol == "" {
		targetSymbol = targetID
	}
	notes := strings.TrimSpace(cell(rec, s.notesCol))
	return Edge{
		TFID:          tfID,
		TFSymbol:      tfSymbol,
		TFFamily:      familyFromNotes(notes),
		TargetID:      targetID,
		TargetSymbol:  targetSymbol,
		EvidenceType:  strings.TrimSpace(cell(rec, s.typeCol)),
		EvidenceLevel: strings.TrimSpace(cell(rec, s.levelCol)),
		Method:        strings.TrimSpace(cell(rec, s.methodCol)),
		Source:        strings.TrimSpace(cell(rec, s.sourceCol)),
		PMID:          strings.TrimSpace(cell(rec, s.pmidCol)),
		Condition:     strings.TrimSpace(cell(rec, s.conditionCol)),
		PromoterStart: promStart,
		PromoterEnd:   promEnd,
		MotifID:       strings.TrimSpace(cell(rec, s.motifCol)),
		HitStart:      hitStart,
		HitEnd:        hitEnd,
		Strand:        strings.TrimSpace(cell(rec, s.strandCol)),
		Score:         score,
		PValue:        pValue,
		Notes:         notes,
	}, nil
}

// readEvidence parses ReguTrace's evidence TSV.
func readEvidence(path string) ([]Edge, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("parse evidence TSV header: %w", err)
	}
	schema, err := newEvidenceSchema(header)
	if err != nil {
		return nil, err
	}
	var edges []Edge
	for i := 0; ; i++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse evidence TSV row %d: %w", i+2, err)
		}
		e, err := schema.edge(rec, i+2)
		if err != nil {
			return nil, err
		}
		if e.TFID != "" {
			edges = append(edges, e)
		}
	}
	return edges, nil
}

func parsePromoterHeader(header string) (geneID, chrom, strand string, start, end uint64) {
	geneID = strings.TrimSpace(header)
	meta := ""
	if i := strings.LastIndex(geneID, "|"); i >= 0 {
		meta = geneID[i+1:]
		geneID = geneID[:i]
	}
	if meta == "" {
		return
	}
	open := strings.LastIndex(meta, "(")
	close := strings.LastIndex(meta, ")")
	if open < 0 || close != len(meta)-1 || open >= close {
		return
	}
	strand = meta[open+1 : close]
	coord := meta[:open]
	colon := strings.LastIndex(coord, ":")
	if colon < 0 {
		return
	}
	chrom = coord[:colon]
	dash := strings.LastIndex(coord[colon+1:], "-")
	if dash < 0 {
		return
	}
	dash += colon + 1
	s, err1 := strconv.ParseUint(coord[colon+1:dash], 10, 64)
	e, err2 := strconv.ParseUint(coord[dash+1:], 10, 64)
	if err1 == nil && err2 == nil {
		start, end = s, e
	}
	return
}

func readPromoters(path string) (map[string]*Promoter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	out := map[string]*Promoter{}
	var cur *Promoter
	var seq strings.Builder
	flush := func() {
		if cur == nil {
			return
		}
		cur.Sequence = strings.ToUpper(seq.String())
		out[cur.GeneID] = cur
		cur, seq = nil, strings.Builder{}
	}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			gene, chrom, strand, start, end := parsePromoterHeader(line[1:])
			if gene == "" {
				continue
			}
			cur = &Promoter{GeneID: gene, Chrom: chrom, Strand: strand, Start: start, End: end}
			continue
		}
		if cur != nil {
			seq.WriteString(line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	if len(out) == 0 {
		return nil, errors.New("promoter FASTA is empty")
	}
	return out, nil
}

func readReport(path string) (*Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(b, &report); err != nil {
		return nil, fmt.Errorf("parse diagnostics report: %w", err)
	}
	return &report, nil
}

// CountItem is one bar in the TF/motif summary chart.
type CountItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Targets int    `json:"targets"`
	Hits    int    `json:"hits"`
	Edges   int    `json:"edges"`
}

// Summary is the compact result overview returned to the dashboard.
type Summary struct {
	Genes            int         `json:"genes"`
	Promoters        int         `json:"promoters"`
	MotifLibrarySize int         `json:"motif_library_size"`
	MotifsWithHits   int         `json:"motifs_with_hits"`
	MotifHits        int         `json:"motif_hits"`
	TFTargetPairs    int         `json:"tf_target_pairs"`
	TFTargetEdges    int         `json:"tf_target_edges"`
	UniqueTFs        int         `json:"unique_tfs"`
	UniqueTargets    int         `json:"unique_targets"`
	UniqueMotifs     int         `json:"unique_motifs"`
	EvidenceLevelD   int         `json:"evidence_level_d"`
	Threads          int         `json:"threads,omitempty"`
	AnnotationSource string      `json:"annotation_source"`
	TopTFs           []CountItem `json:"top_tfs"`
	TopMotifs        []CountItem `json:"top_motifs"`
}

type hitSet map[string]bool

func hitKey(e Edge) string {
	if e.HitStart == nil || e.HitEnd == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%d|%d|%s", e.TargetID, e.MotifID, *e.HitStart, *e.HitEnd, e.Strand)
}

type targetBits []uint64

func (b *targetBits) set(idx uint32) {
	word := int(idx / 64)
	if word >= len(*b) {
		*b = append(*b, make([]uint64, word-len(*b)+1)...)
	}
	(*b)[word] |= uint64(1) << (idx % 64)
}

func (b targetBits) count() int {
	n := 0
	for _, word := range b {
		n += bits.OnesCount64(word)
	}
	return n
}

type summaryBuilder struct {
	trackHits    bool
	targetIndex  map[string]uint32
	tfTargets    map[string]*targetBits
	tfHits       map[string]hitSet
	tfEdges      map[string]int
	tfNames      map[string]string
	motifTargets map[string]*targetBits
	motifHits    map[string]hitSet
	motifEdges   map[string]int
	motifNames   map[string]string
	allHits      hitSet
	edges        int
	levelD       int
}

func newSummaryBuilder(trackHits bool) *summaryBuilder {
	return &summaryBuilder{
		trackHits:    trackHits,
		targetIndex:  map[string]uint32{},
		tfTargets:    map[string]*targetBits{},
		tfHits:       map[string]hitSet{},
		tfEdges:      map[string]int{},
		tfNames:      map[string]string{},
		motifTargets: map[string]*targetBits{},
		motifHits:    map[string]hitSet{},
		motifEdges:   map[string]int{},
		motifNames:   map[string]string{},
		allHits:      hitSet{},
	}
}

func bitsFor(m map[string]*targetBits, key string) *targetBits {
	b := m[key]
	if b == nil {
		b = &targetBits{}
		m[key] = b
	}
	return b
}

func (b *summaryBuilder) add(e Edge, motifs map[string]MotifInfo) {
	idx, ok := b.targetIndex[e.TargetID]
	if !ok {
		idx = uint32(len(b.targetIndex))
		b.targetIndex[e.TargetID] = idx
	}
	bitsFor(b.tfTargets, e.TFID).set(idx)
	b.tfEdges[e.TFID]++
	if _, ok := b.tfNames[e.TFID]; !ok {
		b.tfNames[e.TFID] = e.TFSymbol
	}
	b.edges++
	if strings.EqualFold(e.EvidenceLevel, "D") {
		b.levelD++
	}
	if b.trackHits {
		h := hitKey(e)
		if h != "" {
			b.allHits[h] = true
			b.tfHits[e.TFID] = ensureHits(b.tfHits[e.TFID])
			b.tfHits[e.TFID][h] = true
		}
	}
	if e.MotifID == "" {
		return
	}
	bitsFor(b.motifTargets, e.MotifID).set(idx)
	b.motifEdges[e.MotifID]++
	if b.trackHits {
		h := hitKey(e)
		if h != "" {
			b.motifHits[e.MotifID] = ensureHits(b.motifHits[e.MotifID])
			b.motifHits[e.MotifID][h] = true
		}
	}
	if _, ok := b.motifNames[e.MotifID]; !ok {
		if m, ok := motifs[e.MotifID]; ok {
			b.motifNames[e.MotifID] = m.Name
		}
	}
}

func (b *summaryBuilder) finish(promoterCount int, motifLibrarySize int, annotationSource string, motifHits int) *Summary {
	pairs := 0
	for _, targets := range b.tfTargets {
		pairs += targets.count()
	}
	if b.trackHits {
		motifHits = len(b.allHits)
	}
	if motifHits <= 0 && b.edges > 0 {
		// Older reports may not carry the exact motif-hit count. Falling back
		// to evidence rows is explicit and avoids reporting zero.
		motifHits = b.edges
	}
	s := &Summary{
		Genes:            promoterCount,
		Promoters:        promoterCount,
		MotifLibrarySize: motifLibrarySize,
		AnnotationSource: annotationSource,
		MotifHits:        motifHits,
		TFTargetPairs:    pairs,
		TFTargetEdges:    b.edges,
		UniqueTFs:        len(b.tfTargets),
		UniqueMotifs:     len(b.motifTargets),
		MotifsWithHits:   len(b.motifTargets),
		UniqueTargets:    len(b.targetIndex),
		EvidenceLevelD:   b.levelD,
	}
	for id, targets := range b.tfTargets {
		name := b.tfNames[id]
		if name == "" {
			name = id
		}
		hits := b.tfEdges[id]
		if b.trackHits {
			hits = len(b.tfHits[id])
		}
		s.TopTFs = append(s.TopTFs, CountItem{ID: id, Name: name, Targets: targets.count(), Hits: hits, Edges: b.tfEdges[id]})
	}
	for id, targets := range b.motifTargets {
		name := b.motifNames[id]
		if name == "" {
			name = id
		}
		hits := b.motifEdges[id]
		if b.trackHits {
			hits = len(b.motifHits[id])
		}
		s.TopMotifs = append(s.TopMotifs, CountItem{ID: id, Name: name, Targets: targets.count(), Hits: hits, Edges: b.motifEdges[id]})
	}
	sortCounts(s.TopTFs)
	sortCounts(s.TopMotifs)
	if len(s.TopTFs) > 20 {
		s.TopTFs = s.TopTFs[:20]
	}
	if len(s.TopMotifs) > 20 {
		s.TopMotifs = s.TopMotifs[:20]
	}
	return s
}

func buildSummary(promoters map[string]*Promoter, motifs map[string]MotifInfo, edges []Edge, annotationSource string) *Summary {
	b := newSummaryBuilder(true)
	for _, e := range edges {
		b.add(e, motifs)
	}
	return b.finish(len(promoters), len(motifs), annotationSource, 0)
}

func ensureHits(m hitSet) hitSet {
	if m == nil {
		return hitSet{}
	}
	return m
}

func sortCounts(items []CountItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Targets != items[j].Targets {
			return items[i].Targets > items[j].Targets
		}
		if items[i].Hits != items[j].Hits {
			return items[i].Hits > items[j].Hits
		}
		return items[i].Name < items[j].Name
	})
}
