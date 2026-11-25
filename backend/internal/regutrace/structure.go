package regutrace

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// GFFSegment is one interval on a GFF feature. Phase is the GFF3 phase for CDS
// segments and is zero for exon/UTR intervals.
type GFFSegment struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
	Phase int    `json:"phase,omitempty"`
}

// TranscriptModel is the representative gene structure for one mRNA/transcript.
// Coordinates are 1-based and closed, matching GFF3.
type TranscriptModel struct {
	ID          string       `json:"id"`
	Name        string       `json:"name,omitempty"`
	Chrom       string       `json:"chrom,omitempty"`
	Start       uint64       `json:"start"`
	End         uint64       `json:"end"`
	Strand      string       `json:"strand"`
	Biotype     string       `json:"biotype,omitempty"`
	ProteinID   string       `json:"protein_id,omitempty"`
	Exons       []GFFSegment `json:"exons"`
	CDS         []GFFSegment `json:"cds"`
	FiveUTR     []GFFSegment `json:"five_utr,omitempty"`
	ThreeUTR    []GFFSegment `json:"three_utr,omitempty"`
	Projected   []DomainInfo `json:"projected_domains,omitempty"`
	ProjectNote string       `json:"projection_note,omitempty"`
}

// GeneModel is the GFF structure for one gene.
type GeneModel struct {
	GeneID      string             `json:"gene_id"`
	Name        string             `json:"name,omitempty"`
	Chrom       string             `json:"chrom"`
	Start       uint64             `json:"start"`
	End         uint64             `json:"end"`
	Strand      string             `json:"strand"`
	Biotype     string             `json:"biotype,omitempty"`
	Transcripts []*TranscriptModel `json:"transcripts"`
}

// DomainInfo is one HMMER domain from tf_genes.tsv evidence_json, optionally
// projected onto a transcript's CDS coordinates.
type DomainInfo struct {
	ProteinID         string       `json:"protein_id,omitempty"`
	ModelName         string       `json:"model_name,omitempty"`
	ModelAccession    string       `json:"model_accession,omitempty"`
	Family            string       `json:"family,omitempty"`
	Subfamily         string       `json:"subfamily,omitempty"`
	ProteinStart      int          `json:"protein_start"`
	ProteinEnd        int          `json:"protein_end"`
	ModelStart        int          `json:"model_start,omitempty"`
	ModelEnd          int          `json:"model_end,omitempty"`
	ModelLength       int          `json:"model_length,omitempty"`
	FullEvalue        *float64     `json:"full_evalue,omitempty"`
	ConditionalEvalue *float64     `json:"conditional_evalue,omitempty"`
	IndependentEvalue *float64     `json:"independent_evalue,omitempty"`
	DomainScore       *float64     `json:"domain_score,omitempty"`
	Genomic           []GFFSegment `json:"genomic,omitempty"`
}

// TFProteinView groups HMMER domains belonging to one protein isoform.
type TFProteinView struct {
	ProteinID string       `json:"protein_id"`
	Length    int          `json:"length"`
	Sequence  string       `json:"sequence,omitempty"`
	Domains   []DomainInfo `json:"domains"`
}

// TFMotifMapping is one row of the TF-to-motif map used by the scan.
type TFMotifMapping struct {
	MotifID            string       `json:"motif_id"`
	MotifName          string       `json:"motif_name,omitempty"`
	ReferenceTFID      string       `json:"reference_tf_id,omitempty"`
	MappingMethod      string       `json:"mapping_method,omitempty"`
	MappingStatus      string       `json:"mapping_status"`
	MappingConfidence  string       `json:"mapping_confidence,omitempty"`
	DomainMode         string       `json:"domain_mode,omitempty"`
	ExtractionFamily   string       `json:"extraction_family,omitempty"`
	PWMHash            string       `json:"pwm_hash,omitempty"`
	MatrixLength       int          `json:"matrix_length,omitempty"`
	InformationContent float64      `json:"information_content,omitempty"`
	Matrix             [4][]float64 `json:"matrix,omitempty"`
	TestabilityStatus  string       `json:"testability_status"`
	MinPValue          *float64     `json:"min_pvalue,omitempty"`
	RequestedPValue    *float64     `json:"requested_pvalue,omitempty"`
	Attainable         *bool        `json:"attainable,omitempty"`
	SharedPWMTFCount   int          `json:"shared_pwm_tf_count,omitempty"`
}

// TargetGeneView is the GFF structure of the selected target gene. The target
// promoter motif track and the target gene body are deliberately separate:
// promoter coordinates are relative to the extracted window, while the gene
// body uses genomic coordinates.
type TargetGeneView struct {
	TargetID          string     `json:"target_id"`
	TargetName        string     `json:"target_name"`
	DefaultTranscript string     `json:"default_transcript,omitempty"`
	Gene              *GeneModel `json:"gene,omitempty"`
}

// TFView is the display payload behind the interactive TF gene-structure and
// domain-evidence figure.
type TFView struct {
	Available         bool             `json:"available"`
	Reason            string           `json:"reason,omitempty"`
	TFID              string           `json:"tf_id"`
	TFName            string           `json:"tf_name"`
	TFSymbol          string           `json:"tf_symbol,omitempty"`
	TFFamily          string           `json:"tf_family,omitempty"`
	TFStatus          string           `json:"tf_status,omitempty"`
	TFSource          string           `json:"tf_source,omitempty"`
	DefaultTranscript string           `json:"default_transcript,omitempty"`
	Gene              *GeneModel       `json:"gene,omitempty"`
	Proteins          []TFProteinView  `json:"proteins,omitempty"`
	MotifMappings     []TFMotifMapping `json:"motif_mappings,omitempty"`
	MappingStatus     string           `json:"mapping_status"`
	MappingMethod     string           `json:"mapping_method,omitempty"`
	MappingConfidence string           `json:"mapping_confidence,omitempty"`
	ReferenceTFID     string           `json:"reference_tf_id,omitempty"`
	TargetCount       int              `json:"target_count"`
	SharedPWMTFCount  int              `json:"shared_pwm_tf_count"`
	DiagnosticReason  string           `json:"diagnostic_reason,omitempty"`
	TargetGene        *TargetGeneView  `json:"target_gene,omitempty"`
}

type gffCatalog struct {
	path string

	mu            sync.Mutex
	symbols       map[string]string
	symbolsLoaded bool
	models        map[string]*GeneModel
	attempted     map[string]bool
	err           error
}

func newGFFCatalog(path string) *gffCatalog {
	return &gffCatalog{
		path:      path,
		symbols:   map[string]string{},
		models:    map[string]*GeneModel{},
		attempted: map[string]bool{},
	}
}

func (c *gffCatalog) symbol(id string) string {
	if c == nil || id == "" {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.symbolsLoaded && c.err == nil {
		_ = c.scanLocked(nil)
	}
	return c.symbols[id]
}

// load ensures every requested gene ID has been looked up at least once and
// returns the model for one ID. A GFF without that gene is not an error: the
// caller explains the missing model in the API payload.
func (c *gffCatalog) load(ids map[string]bool, want string) *GeneModel {
	if c == nil || c.path == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	need := false
	for id := range ids {
		if id != "" && !c.attempted[id] {
			need = true
			break
		}
	}
	if need {
		_ = c.scanLocked(ids)
	}
	return c.models[want]
}

func (c *gffCatalog) scanLocked(ids map[string]bool) error {
	reader, err := openMaybeGzip(c.path)
	if err != nil {
		c.err = err
		return err
	}
	defer reader.Close()

	models := c.models
	if models == nil {
		models = map[string]*GeneModel{}
		c.models = models
	}
	symbols := c.symbols
	if symbols == nil {
		symbols = map[string]string{}
		c.symbols = symbols
	}
	transcripts := map[string]*TranscriptModel{}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 128*1024), 8*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.HasPrefix(line, "##FASTA") {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 9 {
			continue
		}
		start, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil || end < start {
			continue
		}
		attrs := parseGFFAttrs(fields[8])
		kind := strings.ToLower(fields[2])
		chrom, strand := fields[0], fields[6]
		if strand != "-" {
			strand = "+"
		}
		switch kind {
		case "gene":
			id := firstAttr(attrs, "ID", "gene_id", "Name", "locus_tag")
			if id == "" {
				continue
			}
			if name := geneSymbol(attrs, id); name != "" {
				symbols[id] = name
			}
			if ids != nil && ids[id] && !c.attempted[id] {
				gm := models[id]
				if gm == nil {
					gm = &GeneModel{GeneID: id}
					models[id] = gm
				}
				gm.Chrom = chrom
				gm.Start = start
				gm.End = end
				gm.Strand = strand
				gm.Biotype = firstAttr(attrs, "gene_biotype", "biotype")
				gm.Name = geneSymbol(attrs, id)
			}
		case "mrna", "transcript", "rna":
			id := firstAttr(attrs, "ID", "transcript_id")
			if id == "" {
				continue
			}
			parent := firstParent(attrs["Parent"])
			if parent == "" {
				parent = attrs["gene_id"]
			}
			if parent == "" && ids != nil && ids[id] {
				// GFFs without gene features sometimes use the selected gene ID
				// directly as the transcript ID.
				parent = id
			}
			if ids == nil || !ids[parent] || c.attempted[parent] {
				continue
			}
			gm := models[parent]
			if gm == nil {
				gm = &GeneModel{
					GeneID: parent,
					Chrom:  chrom,
					Start:  start,
					End:    end,
					Strand: strand,
					Name:   symbols[parent],
				}
				models[parent] = gm
			}
			if gm.Start == 0 || start < gm.Start {
				gm.Start = start
			}
			if end > gm.End {
				gm.End = end
			}
			if gm.Chrom == "" {
				gm.Chrom = chrom
			}
			if gm.Strand == "" {
				gm.Strand = strand
			}
			tr := &TranscriptModel{
				ID:        id,
				Name:      firstAttr(attrs, "Name", "transcript_id"),
				Chrom:     chrom,
				Start:     start,
				End:       end,
				Strand:    strand,
				Biotype:   firstAttr(attrs, "transcript_biotype", "biotype", "gbkey"),
				ProteinID: firstAttr(attrs, "protein_id"),
			}
			if transcripts[id] != nil {
				continue
			}
			gm.Transcripts = append(gm.Transcripts, tr)
			transcripts[id] = tr
		case "exon", "cds", "five_prime_utr", "three_prime_utr", "utr":
			parents := splitParents(attrs["Parent"])
			if len(parents) == 0 {
				if p := attrs["gene_id"]; p != "" {
					parents = []string{p}
				}
			}
			if len(parents) == 0 {
				continue
			}
			seg := GFFSegment{Start: start, End: end}
			if kind == "cds" {
				seg.Phase = parsePhase(fields[7])
			}
			for _, parent := range parents {
				tr := transcripts[parent]
				if tr == nil && ids != nil && ids[parent] && !c.attempted[parent] {
					// CDS/exon directly under a gene: synthesize one transcript.
					tr = &TranscriptModel{
						ID:        parent,
						Chrom:     chrom,
						Start:     start,
						End:       end,
						Strand:    strand,
						ProteinID: firstAttr(attrs, "protein_id"),
					}
					if gm := models[parent]; gm != nil {
						gm.Transcripts = append(gm.Transcripts, tr)
					}
					transcripts[parent] = tr
				}
				if tr == nil {
					continue
				}
				if tr.Chrom == "" {
					tr.Chrom = chrom
				}
				if tr.Start == 0 || start < tr.Start {
					tr.Start = start
				}
				if end > tr.End {
					tr.End = end
				}
				if tr.ProteinID == "" {
					tr.ProteinID = firstAttr(attrs, "protein_id")
				}
				switch kind {
				case "exon":
					tr.Exons = append(tr.Exons, seg)
				case "cds":
					tr.CDS = append(tr.CDS, seg)
				case "five_prime_utr":
					tr.FiveUTR = append(tr.FiveUTR, seg)
				case "three_prime_utr":
					tr.ThreeUTR = append(tr.ThreeUTR, seg)
				case "utr":
					// Without a strand-aware direction, treat generic UTR as
					// five-prime by convention only when the transcript has no
					// explicit UTR; inferUTRs will fill the other side.
					if tr.Strand == "-" {
						tr.ThreeUTR = append(tr.ThreeUTR, seg)
					} else {
						tr.FiveUTR = append(tr.FiveUTR, seg)
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		c.err = fmt.Errorf("read GFF line %d: %w", lineNo, err)
		return c.err
	}
	c.symbolsLoaded = true
	if ids != nil {
		for id := range ids {
			c.attempted[id] = true
		}
	}
	for _, gm := range models {
		normalizeGeneModel(gm)
	}
	return nil
}

func normalizeGeneModel(gm *GeneModel) {
	kept := gm.Transcripts[:0]
	for _, tr := range gm.Transcripts {
		normalizeTranscript(tr)
		if len(tr.Exons) == 0 && len(tr.CDS) == 0 {
			continue
		}
		kept = append(kept, tr)
	}
	gm.Transcripts = kept
	sort.Slice(gm.Transcripts, func(i, j int) bool {
		if gm.Transcripts[i].Start != gm.Transcripts[j].Start {
			return gm.Transcripts[i].Start < gm.Transcripts[j].Start
		}
		return gm.Transcripts[i].ID < gm.Transcripts[j].ID
	})
	if len(gm.Transcripts) > 0 {
		if gm.Start == 0 || gm.Transcripts[0].Start < gm.Start {
			gm.Start = gm.Transcripts[0].Start
		}
		if gm.Transcripts[0].End > gm.End {
			gm.End = gm.Transcripts[0].End
		}
	}
}

func normalizeTranscript(tr *TranscriptModel) {
	tr.Exons = mergeSegments(tr.Exons)
	tr.CDS = mergeSegments(tr.CDS)
	tr.FiveUTR = mergeSegments(tr.FiveUTR)
	tr.ThreeUTR = mergeSegments(tr.ThreeUTR)
	if tr.ProteinID == "" {
		// Many plant GFFs omit protein_id on mRNA/CDS and use the transcript ID
		// as the protein accession (e.g. Fta_1G0000630.1).
		tr.ProteinID = tr.ID
	}
	if len(tr.Exons) == 0 && len(tr.CDS) > 0 {
		tr.Exons = append([]GFFSegment(nil), tr.CDS...)
	}
	if len(tr.CDS) == 0 {
		return
	}
	inferredFive, inferredThree := inferUTRs(tr.Exons, tr.CDS, tr.Strand)
	if len(tr.FiveUTR) == 0 {
		tr.FiveUTR = inferredFive
	}
	if len(tr.ThreeUTR) == 0 {
		tr.ThreeUTR = inferredThree
	}
}

func mergeSegments(in []GFFSegment) []GFFSegment {
	if len(in) == 0 {
		return nil
	}
	segs := append([]GFFSegment(nil), in...)
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].Start != segs[j].Start {
			return segs[i].Start < segs[j].Start
		}
		return segs[i].End < segs[j].End
	})
	out := make([]GFFSegment, 0, len(segs))
	for _, seg := range segs {
		if seg.Start == 0 || seg.End < seg.Start {
			continue
		}
		if len(out) == 0 || seg.Start > out[len(out)-1].End+1 {
			out = append(out, seg)
			continue
		}
		if seg.End > out[len(out)-1].End {
			out[len(out)-1].End = seg.End
		}
	}
	return out
}

// inferUTRs returns exon bases outside CDS. The result is split into 5' and 3'
// UTR according to transcript direction.
func inferUTRs(exons, cds []GFFSegment, strand string) ([]GFFSegment, []GFFSegment) {
	var outside []GFFSegment
	for _, exon := range exons {
		pieces := []GFFSegment{exon}
		for _, coding := range cds {
			var next []GFFSegment
			for _, piece := range pieces {
				next = append(next, subtractSegment(piece, coding)...)
			}
			pieces = next
			if len(pieces) == 0 {
				break
			}
		}
		outside = append(outside, pieces...)
	}
	outside = mergeSegments(outside)
	if len(outside) == 0 {
		return nil, nil
	}
	firstCDS := cds[0]
	lastCDS := cds[len(cds)-1]
	var five, three []GFFSegment
	for _, seg := range outside {
		if strand == "-" {
			switch {
			case seg.End < firstCDS.Start:
				three = append(three, seg)
			case seg.Start > lastCDS.End:
				five = append(five, seg)
			}
		} else {
			switch {
			case seg.End < firstCDS.Start:
				five = append(five, seg)
			case seg.Start > lastCDS.End:
				three = append(three, seg)
			}
		}
	}
	return mergeSegments(five), mergeSegments(three)
}

func subtractSegment(seg, cut GFFSegment) []GFFSegment {
	if cut.End < seg.Start || cut.Start > seg.End {
		return []GFFSegment{seg}
	}
	var out []GFFSegment
	if cut.Start > seg.Start {
		out = append(out, GFFSegment{Start: seg.Start, End: cut.Start - 1})
	}
	if cut.End < seg.End {
		out = append(out, GFFSegment{Start: cut.End + 1, End: seg.End})
	}
	return out
}

func projectProteinRange(tr *TranscriptModel, startAA, endAA int) []GFFSegment {
	if startAA < 1 {
		startAA = 1
	}
	if endAA < startAA || len(tr.CDS) == 0 {
		return nil
	}
	order := append([]GFFSegment(nil), tr.CDS...)
	sort.Slice(order, func(i, j int) bool { return order[i].Start < order[j].Start })
	if tr.Strand == "-" {
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	skip := 0
	if order[0].Phase > 0 {
		skip = order[0].Phase
	}
	ntStart := (startAA - 1) * 3
	ntEnd := endAA * 3
	cursor := 0
	var out []GFFSegment
	for i, seg := range order {
		length := int(seg.End-seg.Start) + 1
		segStart := 0
		if i == 0 {
			if skip >= length {
				continue
			}
			segStart = skip
			length -= skip
		}
		segNTOffset := cursor
		segNTCoding := length
		from := maxInt(ntStart, segNTOffset) - segNTOffset
		to := minInt(ntEnd, segNTOffset+segNTCoding) - segNTOffset
		if from < to && from >= 0 && to <= length {
			var a, b uint64
			if tr.Strand == "-" {
				a = seg.End - uint64(segStart+to) + 1
				b = seg.End - uint64(segStart+from)
			} else {
				a = seg.Start + uint64(segStart+from)
				b = seg.Start + uint64(segStart+to) - 1
			}
			if a > 0 && b >= a {
				out = append(out, GFFSegment{Start: a, End: b})
			}
		}
		cursor += length
		if cursor >= ntEnd {
			break
		}
	}
	return mergeSegments(out)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstAttr(attrs map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(attrs[key]); v != "" {
			return v
		}
	}
	return ""
}

func firstParent(value string) string {
	parts := splitParents(value)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func splitParents(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(cleanGFFID(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func geneSymbol(attrs map[string]string, id string) string {
	for _, key := range []string{"gene_name", "gene", "Name", "locus_tag", "gene_synonym"} {
		value := strings.TrimSpace(attrs[key])
		if value == "" || value == id {
			continue
		}
		// gene_synonym can be a comma-separated list; the first name is the
		// most useful display label.
		if key == "gene_synonym" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if value != "" && value != id {
			return value
		}
	}
	return ""
}

func parsePhase(value string) int {
	phase, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || phase < 0 || phase > 2 {
		return 0
	}
	return phase
}

// tfGeneRecord is one row of tf_genes.tsv.
type tfGeneRecord struct {
	GeneID       string
	TFSymbol     string
	Family       string
	Status       string
	Source       string
	EvidenceJSON string
}

func readTFCatalogue(path string) (map[string]tfGeneRecord, error) {
	out := map[string]tfGeneRecord{}
	if strings.TrimSpace(path) == "" {
		return out, nil
	}
	reader, err := openMaybeGzip(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read tf_genes.tsv: %w", err)
	}
	if len(records) == 0 {
		return out, nil
	}
	header := make([]string, len(records[0]))
	for i, value := range records[0] {
		header[i] = normalizeHeader(value)
	}
	col := func(names ...string) int {
		for _, want := range names {
			for i, value := range header {
				if value == want {
					return i
				}
			}
		}
		return -1
	}
	geneCol := col("gene_id", "id", "target_id")
	if geneCol < 0 {
		return nil, errors.New("tf_genes.tsv has no gene_id column")
	}
	symbolCol := col("tf_symbol", "symbol", "gene_name")
	familyCol := col("family", "tf_family")
	statusCol := col("identification_status", "status")
	sourceCol := col("source")
	evidenceCol := col("evidence_json", "evidence")
	cell := func(rec []string, idx int) string {
		if idx < 0 || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}
	for _, rec := range records[1:] {
		gene := cell(rec, geneCol)
		if gene == "" {
			continue
		}
		if _, exists := out[gene]; exists {
			continue
		}
		out[gene] = tfGeneRecord{
			GeneID:       gene,
			TFSymbol:     cell(rec, symbolCol),
			Family:       cell(rec, familyCol),
			Status:       cell(rec, statusCol),
			Source:       cell(rec, sourceCol),
			EvidenceJSON: cell(rec, evidenceCol),
		}
	}
	return out, nil
}

type tfEvidenceRecord struct {
	ProteinID string `json:"protein_id"`
	Families  []struct {
		Family    string `json:"family"`
		Subfamily string `json:"subfamily"`
	} `json:"families"`
	Domains []struct {
		ConditionalEvalue *float64 `json:"conditional_evalue"`
		DomainScore       *float64 `json:"domain_score"`
		FullEvalue        *float64 `json:"full_evalue"`
		IndependentEvalue *float64 `json:"independent_evalue"`
		ModelAccession    string   `json:"model_accession"`
		ModelEnd          int      `json:"model_end"`
		ModelLength       int      `json:"model_length"`
		ModelName         string   `json:"model_name"`
		ModelStart        int      `json:"model_start"`
		ProteinEnd        int      `json:"protein_end"`
		ProteinID         string   `json:"protein_id"`
		ProteinStart      int      `json:"protein_start"`
	} `json:"domains"`
}

func parseTFProteins(evidence string) []TFProteinView {
	evidence = strings.TrimSpace(evidence)
	if evidence == "" || evidence == "-" || evidence == "null" {
		return nil
	}
	var records []tfEvidenceRecord
	if err := json.Unmarshal([]byte(evidence), &records); err != nil {
		var one tfEvidenceRecord
		if err2 := json.Unmarshal([]byte(evidence), &one); err2 != nil {
			return nil
		}
		records = []tfEvidenceRecord{one}
	}
	byProtein := map[string]*TFProteinView{}
	var order []string
	for _, rec := range records {
		pid := strings.TrimSpace(rec.ProteinID)
		if pid == "" && len(rec.Domains) > 0 {
			pid = rec.Domains[0].ProteinID
		}
		if pid == "" {
			continue
		}
		p := byProtein[pid]
		if p == nil {
			p = &TFProteinView{ProteinID: pid}
			byProtein[pid] = p
			order = append(order, pid)
		}
		family := ""
		subfamily := ""
		for _, f := range rec.Families {
			if family == "" {
				family = f.Family
			}
			if subfamily == "" {
				subfamily = f.Subfamily
			}
		}
		for _, d := range rec.Domains {
			domain := DomainInfo{
				ProteinID:         pid,
				ModelName:         d.ModelName,
				ModelAccession:    d.ModelAccession,
				Family:            family,
				Subfamily:         subfamily,
				ProteinStart:      d.ProteinStart,
				ProteinEnd:        d.ProteinEnd,
				ModelStart:        d.ModelStart,
				ModelEnd:          d.ModelEnd,
				ModelLength:       d.ModelLength,
				FullEvalue:        d.FullEvalue,
				ConditionalEvalue: d.ConditionalEvalue,
				IndependentEvalue: d.IndependentEvalue,
				DomainScore:       d.DomainScore,
			}
			if domain.ProteinEnd > p.Length {
				p.Length = domain.ProteinEnd
			}
			p.Domains = append(p.Domains, domain)
		}
	}
	sort.Strings(order)
	out := make([]TFProteinView, 0, len(order))
	for _, pid := range order {
		p := byProtein[pid]
		sort.Slice(p.Domains, func(i, j int) bool {
			if p.Domains[i].ProteinStart != p.Domains[j].ProteinStart {
				return p.Domains[i].ProteinStart < p.Domains[j].ProteinStart
			}
			return p.Domains[i].ProteinEnd < p.Domains[j].ProteinEnd
		})
		out = append(out, *p)
	}
	return out
}

func (j *jobState) tfDisplayName(tfID, tfSymbol string) string {
	if tfID == "" {
		return tfSymbol
	}
	var gffName string
	if j.gff != nil {
		gffName = j.gff.symbol(tfID)
		if gffName != "" && gffName != tfID {
			return gffName
		}
	}
	if ann, ok := j.annotations[tfID]; ok && ann.Symbol != "" && ann.Symbol != tfID {
		return ann.Symbol
	}
	if tfSymbol != "" && tfSymbol != tfID {
		return tfSymbol
	}
	return tfID
}

func (j *jobState) tfMotifMappings(tfID string) []TFMotifMapping {
	if tfID == "" {
		return nil
	}
	rows := j.mappingRows(tfID)
	if len(rows) > 0 {
		out := make([]TFMotifMapping, 0, len(rows))
		seen := map[string]bool{}
		for _, row := range rows {
			if row.DomainMode == "single_domain_fallback" && !j.Params.IncludeSingleDomainFallback {
				continue
			}
			key := row.MotifID + "\x00" + row.ReferenceTFID
			if seen[key] {
				continue
			}
			seen[key] = true
			meta := j.motifs[row.MotifID]
			item := j.testability(row.MotifID)
			mapping := TFMotifMapping{
				MotifID: row.MotifID, MotifName: meta.Name, ReferenceTFID: row.ReferenceTFID,
				MappingMethod: row.Method, MappingStatus: row.Status, MappingConfidence: row.MappingConfidence,
				DomainMode:       row.DomainMode,
				ExtractionFamily: row.ExtractionFamily, PWMHash: meta.PWMHash,
				MatrixLength: meta.Length, InformationContent: meta.InformationContent,
				Matrix: meta.Matrix, TestabilityStatus: item.Status,
				SharedPWMTFCount: j.sharedPWMTFCount(row.MotifID),
			}
			if item.Status != "not_computed" {
				minP, requested := item.MinPValue, item.RequestedPValue
				attainable := item.Attainable
				mapping.MinPValue, mapping.RequestedPValue, mapping.Attainable = &minP, &requested, &attainable
			}
			out = append(out, mapping)
		}
		sort.Slice(out, func(i, k int) bool {
			if out[i].MotifID != out[k].MotifID {
				return out[i].MotifID < out[k].MotifID
			}
			return out[i].ReferenceTFID < out[k].ReferenceTFID
		})
		return out
	}
	if j.tfMapPath == "" {
		return nil
	}
	reader, err := openMaybeGzip(j.tfMapPath)
	if err != nil {
		return nil
	}
	defer reader.Close()
	r := csv.NewReader(reader)
	r.Comma = '	'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}
	header := make([]string, len(records[0]))
	for i, v := range records[0] {
		header[i] = normalizeHeader(v)
	}
	col := func(names ...string) int {
		for _, want := range names {
			for i, value := range header {
				if value == want {
					return i
				}
			}
		}
		return -1
	}
	idCol := col("tf_id", "gene_id")
	motifCol := col("motif_id")
	if idCol < 0 || motifCol < 0 {
		return nil
	}
	refCol := col("reference_tf_id")
	methodCol := col("mapping_method")
	statusCol := col("mapping_status")
	modeCol := col("domain_mode")
	familyCol := col("extraction_family")
	seen := map[string]bool{}
	var out []TFMotifMapping
	for _, rec := range records[1:] {
		cell := func(idx int) string {
			if idx < 0 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}
		if cell(idCol) != tfID {
			continue
		}
		motifID := cell(motifCol)
		if motifID == "" {
			continue
		}
		key := motifID + "\x00" + cell(refCol)
		if seen[key] {
			continue
		}
		seen[key] = true
		meta := j.motifs[motifID]
		item := j.testability(motifID)
		method := cell(methodCol)
		mapping := TFMotifMapping{
			MotifID: motifID, MotifName: meta.Name, ReferenceTFID: cell(refCol),
			MappingMethod: method, MappingStatus: normalizeMappingStatus(cell(statusCol), method),
			DomainMode: cell(modeCol), ExtractionFamily: cell(familyCol),
			PWMHash: meta.PWMHash, MatrixLength: meta.Length, InformationContent: meta.InformationContent,
			Matrix: meta.Matrix, TestabilityStatus: item.Status,
			SharedPWMTFCount: j.sharedPWMTFCount(motifID),
		}
		if item.Status != "not_computed" {
			minP, requested := item.MinPValue, item.RequestedPValue
			attainable := item.Attainable
			mapping.MinPValue, mapping.RequestedPValue, mapping.Attainable = &minP, &requested, &attainable
		}
		out = append(out, mapping)
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].MotifID != out[k].MotifID {
			return out[i].MotifID < out[k].MotifID
		}
		return out[i].ReferenceTFID < out[k].ReferenceTFID
	})
	return out
}

func longestTranscript(gm *GeneModel) *TranscriptModel {
	if gm == nil || len(gm.Transcripts) == 0 {
		return nil
	}
	best := gm.Transcripts[0]
	bestLen := codingLength(best)
	for _, tr := range gm.Transcripts[1:] {
		if n := codingLength(tr); n > bestLen {
			best, bestLen = tr, n
		}
	}
	return best
}

func codingLength(tr *TranscriptModel) int {
	n := 0
	for _, seg := range tr.CDS {
		if seg.End >= seg.Start {
			n += int(seg.End-seg.Start) + 1
		}
	}
	return n
}

func (j *jobState) proteinSequence(proteinID string) string {
	proteinID = strings.TrimSpace(proteinID)
	if proteinID == "" || j.proteinsPath == "" || j.sequenceIsCDS {
		return ""
	}
	j.proteinMu.Lock()
	defer j.proteinMu.Unlock()
	if j.proteinSequences == nil {
		j.proteinSequences = map[string]string{}
		reader, err := openMaybeGzip(j.proteinsPath)
		if err == nil {
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
			id := ""
			var seq strings.Builder
			flush := func() {
				if id != "" {
					j.proteinSequences[id] = seq.String()
				}
				seq.Reset()
			}
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, ">") {
					flush()
					fields := strings.Fields(strings.TrimPrefix(line, ">"))
					if len(fields) > 0 {
						id = fields[0]
					} else {
						id = ""
					}
					continue
				}
				if id != "" {
					seq.WriteString(line)
				}
			}
			flush()
			_ = reader.Close()
		}
	}
	return j.proteinSequences[proteinID]
}

func (j *jobState) tfView(tfID string) (*TFView, error) {
	return j.tfViewWithTarget(tfID, "")
}

func (j *jobState) tfViewWithTarget(tfID, targetID string) (*TFView, error) {
	tfID = strings.TrimSpace(tfID)
	targetID = strings.TrimSpace(targetID)
	if tfID == "" {
		return nil, errors.New("tf is required")
	}
	catalogue, err := readTFCatalogue(j.tfGenesPath)
	if err != nil {
		return nil, err
	}
	rec, ok := catalogue[tfID]
	ids := map[string]bool{tfID: true}
	for id := range catalogue {
		ids[id] = true
	}
	var gm *GeneModel
	if j.gff != nil {
		gm = j.gff.load(ids, tfID)
	}
	proteins := parseTFProteins(rec.EvidenceJSON)
	for i := range proteins {
		proteins[i].Sequence = j.proteinSequence(proteins[i].ProteinID)
	}
	mappings := j.tfMotifMappings(tfID)
	statuses := make([]string, 0, len(mappings))
	methods := make([]string, 0, len(mappings))
	reference := ""
	confidence := ""
	shared := 0
	for _, mapping := range mappings {
		statuses = append(statuses, mapping.MappingStatus)
		if mapping.MappingMethod != "" {
			methods = append(methods, mapping.MappingMethod)
		}
		if confidence == "" {
			confidence = mapping.MappingConfidence
		}
		if reference == "" {
			reference = mapping.ReferenceTFID
		}
		if mapping.SharedPWMTFCount > shared {
			shared = mapping.SharedPWMTFCount
		}
	}
	mappingStatus := bestStatus(statuses)
	for _, mapping := range mappings {
		if mapping.MappingStatus == mappingStatus && mapping.MappingConfidence != "" {
			confidence = mapping.MappingConfidence
			break
		}
	}
	if len(statuses) == 0 {
		if j.tfMapPath == "" {
			mappingStatus = "not_computed"
		} else {
			mappingStatus = "no_supported_motif"
		}
	}
	targetCount := 0
	if j.data != nil {
		targetCount = j.data.tfTargetCounts[tfID]
	}
	diagnostic := ""
	switch mappingStatus {
	case "not_computed":
		diagnostic = "No TF–motif map was supplied; run a reference pack or provide tf_map.tsv."
	case "no_supported_motif":
		diagnostic = "No supported motif assignment in tf_map.tsv for this TF."
	case "dbd_ambiguous":
		diagnostic = "The DBD mapping is ambiguous; inspect donor and motif rows before using it."
	}
	if diagnostic == "" && len(mappings) > 0 && targetCount == 0 {
		diagnostic = "Mapped and scanned, but no promoter passed the current threshold."
	}
	view := &TFView{
		TFID:              tfID,
		TFSymbol:          rec.TFSymbol,
		TFFamily:          rec.Family,
		TFStatus:          rec.Status,
		TFSource:          rec.Source,
		TFName:            j.tfDisplayName(tfID, rec.TFSymbol),
		Gene:              gm,
		Proteins:          proteins,
		MotifMappings:     mappings,
		MappingStatus:     mappingStatus,
		MappingMethod:     firstNonEmpty(methods...),
		MappingConfidence: confidence,
		ReferenceTFID:     reference,
		TargetCount:       targetCount,
		SharedPWMTFCount:  shared,
		DiagnosticReason:  diagnostic,
	}
	if gm != nil {
		if gm.Name != "" {
			view.TFName = gm.Name
		}
		for i := range proteins {
			for _, tr := range gm.Transcripts {
				if tr.ProteinID == proteins[i].ProteinID {
					if aa := codingLength(tr) / 3; aa > proteins[i].Length {
						proteins[i].Length = aa
					}
				}
			}
		}
		if len(gm.Transcripts) > 0 {
			defaultTR := longestTranscript(gm)
			// Prefer a transcript whose protein has HMMER evidence.
			for _, p := range proteins {
				for _, tr := range gm.Transcripts {
					if tr.ProteinID == p.ProteinID {
						defaultTR = tr
						break
					}
				}
				if defaultTR != nil && defaultTR.ProteinID == p.ProteinID {
					break
				}
			}
			if defaultTR != nil {
				view.DefaultTranscript = defaultTR.ID
			}
			byProtein := map[string]TFProteinView{}
			for _, p := range proteins {
				byProtein[p.ProteinID] = p
			}
			for _, tr := range gm.Transcripts {
				tr.Projected = nil
				tr.ProjectNote = ""
				p := byProtein[tr.ProteinID]
				if p.ProteinID == "" && len(proteins) == 1 {
					p = proteins[0]
				}
				if p.ProteinID == "" || len(p.Domains) == 0 {
					continue
				}
				for _, d := range p.Domains {
					copy := d
					copy.Genomic = projectProteinRange(tr, d.ProteinStart, d.ProteinEnd)
					tr.Projected = append(tr.Projected, copy)
				}
				if len(tr.CDS) > 0 && tr.CDS[0].Phase != 0 {
					tr.ProjectNote = "The first CDS has a nonzero GFF phase; projected domain coordinates are approximate for a partial model."
				}
			}
		}
	}
	if targetID != "" && j.gff != nil {
		var targetGene *GeneModel
		if targetID == tfID {
			targetGene = gm
		} else {
			targetGene = j.gff.load(map[string]bool{targetID: true}, targetID)
		}
		if targetGene != nil && len(targetGene.Transcripts) > 0 {
			name := targetGene.Name
			if name == "" {
				name = j.gff.symbol(targetID)
			}
			if name == "" {
				if ann, exists := j.annotations[targetID]; exists {
					name = ann.Symbol
				}
			}
			if name == "" {
				name = targetID
			}
			view.TargetGene = &TargetGeneView{
				TargetID:   targetID,
				TargetName: name,
				Gene:       targetGene,
			}
			if defaultTR := longestTranscript(targetGene); defaultTR != nil {
				view.TargetGene.DefaultTranscript = defaultTR.ID
			}
		}
	}
	if gm != nil && len(gm.Transcripts) > 0 {
		view.Available = true
	} else if ok {
		view.Reason = "No GFF transcript model for this TF ID. Check that tf_genes.tsv gene IDs match the GFF and that the GFF has mRNA/exon/CDS features."
	} else {
		view.Reason = "This TF ID is motif-derived or absent from tf_genes.tsv. Run the reference-pack pipeline (or supply proteins plus HMM) to identify TF genes and HMMER domains."
	}
	if view.TFName == "" {
		view.TFName = tfID
	}
	return view, nil
}

// enrichSummaryNames replaces gene-ID labels in the summary chart with gene
// names from the GFF or annotation table. It also makes those names searchable.
func (j *jobState) enrichSummaryNames(data *dataset) {
	if data == nil || data.summary == nil {
		return
	}
	for i := range data.summary.TopTFs {
		item := &data.summary.TopTFs[i]
		name := j.tfDisplayName(item.ID, item.Name)
		item.Name = name
	}
	for id, keys := range data.tfKeys {
		name := j.tfDisplayName(id, "")
		if name != "" && name != id {
			data.tfKeys[id] = uniqueStrings(append(keys, name)...)
		}
	}
}

// Reference methods kept in this file so the structure parser remains close to
// the domain-projection logic.
func (m *Manager) TFView(id, tf, target string) (*TFView, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return nil, ErrNotCompleted
	}
	return js.tfViewWithTarget(tf, target)
}
