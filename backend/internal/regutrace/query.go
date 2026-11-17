package regutrace

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"
)

// QueryOptions controls the free-text TF/target query.
type QueryOptions struct {
	Direction string // tf | target
	Query     string
	Motif     string
	MaxPValue float64
	Limit     int
	Offset    int
}

// QueryRow is one TF-target evidence row enriched with annotation and motif
// display metadata.
type QueryRow struct {
	TFID              string   `json:"tf_id"`
	TFSymbol          string   `json:"tf_symbol"`
	TFName            string   `json:"tf_name,omitempty"`
	TFDescription     string   `json:"tf_description,omitempty"`
	TFFamily          string   `json:"tf_family,omitempty"`
	TargetID          string   `json:"target_id"`
	TargetSymbol      string   `json:"target_symbol"`
	TargetDescription string   `json:"target_description,omitempty"`
	AnnotationSource  string   `json:"annotation_source"`
	EvidenceType      string   `json:"evidence_type,omitempty"`
	EvidenceLevel     string   `json:"evidence_level,omitempty"`
	Method            string   `json:"method,omitempty"`
	Source            string   `json:"source,omitempty"`
	PMID              string   `json:"pmid,omitempty"`
	Condition         string   `json:"condition,omitempty"`
	PromoterStart     *uint64  `json:"promoter_start,omitempty"`
	PromoterEnd       *uint64  `json:"promoter_end,omitempty"`
	MotifID           string   `json:"motif_id,omitempty"`
	MotifName         string   `json:"motif_name,omitempty"`
	Consensus         string   `json:"consensus,omitempty"`
	HitStart          *uint64  `json:"hit_start,omitempty"`
	HitEnd            *uint64  `json:"hit_end,omitempty"`
	Strand            string   `json:"strand,omitempty"`
	Score             *float64 `json:"score,omitempty"`
	PValue            *float64 `json:"p_value,omitempty"`
	Notes             string   `json:"notes,omitempty"`
}

// QueryResult is a filtered page of evidence rows.
type QueryResult struct {
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
	Rows   []QueryRow `json:"rows"`
}

// DonorInfo is one reference-TF donor record from tf_motif_trace.tsv.
// ID, Symbol and Family are kept together; they must not be reconstructed by
// zipping separately sorted summary columns.
type DonorInfo struct {
	TFID     string `json:"tf_id"`
	TFSymbol string `json:"tf_symbol,omitempty"`
	TFFamily string `json:"tf_family,omitempty"`
}

func (d DonorInfo) key() string {
	return d.TFID + "\x00" + d.TFSymbol + "\x00" + d.TFFamily
}

// SiteEvidenceOptions is the exact full-key lookup for one grouped binding
// site. Promoter coordinates are part of the key, not optional decoration.
type SiteEvidenceOptions struct {
	TargetID      string
	MotifID       string
	PromoterStart *uint64
	PromoterEnd   *uint64
	HitStart      *uint64
	HitEnd        *uint64
	Strand        string
	TFID          string
}

// SiteEvidenceResult is the raw evidence rows for one exact site.
type SiteEvidenceResult struct {
	Total int        `json:"total"`
	Rows  []QueryRow `json:"rows"`
}

// TFInfo is the display payload for one TF at a binding site.
type TFInfo struct {
	ID     string      `json:"id"`
	Symbol string      `json:"symbol"`
	Name   string      `json:"name,omitempty"`
	Family string      `json:"family,omitempty"`
	Donors []DonorInfo `json:"donors,omitempty"`
}

// PromoterHit is one grouped binding site on a promoter. Multiple TF rows that
// share the same motif/coordinates are collapsed so the track is not overplotted.
type PromoterHit struct {
	MotifID       string   `json:"motif_id"`
	MotifName     string   `json:"motif_name,omitempty"`
	Consensus     string   `json:"consensus,omitempty"`
	Start         uint64   `json:"start"`
	End           uint64   `json:"end"`
	PromoterStart *uint64  `json:"promoter_start,omitempty"`
	PromoterEnd   *uint64  `json:"promoter_end,omitempty"`
	GenomicStart  uint64   `json:"genomic_start,omitempty"`
	GenomicEnd    uint64   `json:"genomic_end,omitempty"`
	Strand        string   `json:"strand"`
	Score         *float64 `json:"score,omitempty"`
	PValue        *float64 `json:"p_value,omitempty"`
	Site          string   `json:"site"`
	MatchedSite   string   `json:"matched_site"`
	TFs           []TFInfo `json:"tfs"`
}

// PromoterView is the sequence + binding-site payload used by the promoter
// visualization.
type PromoterView struct {
	TargetID          string        `json:"target_id"`
	TargetSymbol      string        `json:"target_symbol"`
	TargetDescription string        `json:"target_description,omitempty"`
	AnnotationSource  string        `json:"annotation_source"`
	Chrom             string        `json:"chrom,omitempty"`
	Start             uint64        `json:"start,omitempty"`
	End               uint64        `json:"end,omitempty"`
	Strand            string        `json:"strand,omitempty"`
	Length            int           `json:"length"`
	Sequence          string        `json:"sequence"`
	TSSPosition       int           `json:"tss_position"`
	Hits              []PromoterHit `json:"hits"`
}

func textMatchScore(values []string, q string) int {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return 0
	}
	best := 99
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		switch {
		case v == q:
			return 0
		case strings.HasPrefix(v, q):
			if best > 1 {
				best = 1
			}
		case strings.Contains(v, q):
			if best > 2 {
				best = 2
			}
		}
	}
	return best
}

func (j *jobState) annotationFor(targetID, fallbackSymbol string) (symbol, description, source string) {
	ann, ok := j.annotations[targetID]
	if ok {
		symbol = ann.Symbol
		description = ann.Description
		if symbol != "" || description != "" {
			source = j.annotationSource
		}
	}
	if symbol == "" {
		// EggNOG/CLI fallbacks often use the versioned transcript ID. When the
		// target itself is the gene-level ID, prefer that cleaner label.
		if base, ok := stripNumericVersion(fallbackSymbol); ok && base == targetID {
			symbol = targetID
		} else {
			symbol = fallbackSymbol
		}
	}
	if symbol == "" {
		symbol = targetID
	}
	if source == "" {
		if symbol != targetID {
			source = "GFF"
		} else {
			source = "gene ID"
		}
	}
	return
}

func (j *jobState) resolveSiteTarget(target string) (string, bool) {
	if _, ok := j.promoters[target]; ok {
		return target, true
	}
	lower := strings.ToLower(target)
	for id := range j.promoters {
		if strings.ToLower(id) == lower {
			return id, true
		}
	}
	if j.data != nil {
		return j.data.resolveTarget(target)
	}
	return "", false
}

// siteEvidence returns the original evidence rows for one exact full site key.
// Supplying promoter_start/end is required by the dashboard detail view; rows
// from a different promoter window never match.
func (j *jobState) siteEvidence(opts SiteEvidenceOptions) (SiteEvidenceResult, error) {
	if strings.TrimSpace(opts.TargetID) == "" || strings.TrimSpace(opts.MotifID) == "" {
		return SiteEvidenceResult{}, fmt.Errorf("target and motif are required")
	}
	targetID, ok := j.resolveSiteTarget(opts.TargetID)
	if !ok {
		return SiteEvidenceResult{}, fmt.Errorf("target %q has no evidence in this run", opts.TargetID)
	}
	var sourceEdges []Edge
	if j.data != nil {
		edges, err := j.data.readTargetEdges(targetID)
		if err != nil {
			return SiteEvidenceResult{}, err
		}
		sourceEdges = edges
	} else {
		sourceEdges = j.edges
	}
	match := func(value, wanted *uint64) bool {
		if wanted == nil {
			return true
		}
		return value != nil && *value == *wanted
	}
	rows := make([]QueryRow, 0)
	for _, e := range sourceEdges {
		if e.TargetID != targetID || e.MotifID != opts.MotifID {
			continue
		}
		if !match(e.PromoterStart, opts.PromoterStart) || !match(e.PromoterEnd, opts.PromoterEnd) {
			continue
		}
		if !match(e.HitStart, opts.HitStart) || !match(e.HitEnd, opts.HitEnd) {
			continue
		}
		if opts.Strand != "" && e.Strand != opts.Strand {
			continue
		}
		if opts.TFID != "" && e.TFID != opts.TFID {
			continue
		}
		rows = append(rows, j.queryRow(e))
	}
	sort.SliceStable(rows, func(i, k int) bool {
		if rows[i].TFID != rows[k].TFID {
			return rows[i].TFID < rows[k].TFID
		}
		if rows[i].EvidenceLevel != rows[k].EvidenceLevel {
			return rows[i].EvidenceLevel < rows[k].EvidenceLevel
		}
		return rows[i].Method < rows[k].Method
	})
	return SiteEvidenceResult{Total: len(rows), Rows: rows}, nil
}

func (j *jobState) query(opts QueryOptions) QueryResult {
	if j.data != nil {
		return j.queryDataset(opts)
	}
	return j.queryEdges(opts)
}

func (j *jobState) queryEdges(opts QueryOptions) QueryResult {
	if opts.Direction != "target" {
		opts.Direction = "tf"
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	motifQ := strings.ToLower(strings.TrimSpace(opts.Motif))

	type scored struct {
		edge  Edge
		score int
	}
	matches := make([]scored, 0, len(j.edges))
	for _, e := range j.edges {
		if opts.MaxPValue > 0 && e.PValue != nil && *e.PValue > opts.MaxPValue {
			continue
		}
		if motifQ != "" {
			meta := j.motifs[e.MotifID]
			hay := strings.ToLower(e.MotifID + " " + meta.Name)
			if !strings.Contains(hay, motifQ) {
				continue
			}
		}
		var score int
		if opts.Direction == "target" {
			symbol, desc, _ := j.annotationFor(e.TargetID, e.TargetSymbol)
			score = textMatchScore([]string{e.TargetID, e.TargetSymbol, symbol, desc}, opts.Query)
		} else {
			score = textMatchScore([]string{e.TFID, e.TFSymbol}, opts.Query)
		}
		if score >= 99 {
			continue
		}
		matches = append(matches, scored{edge: e, score: score})
	}
	sort.SliceStable(matches, func(i, k int) bool {
		return edgeBetter(matches[i].edge, matches[i].score, matches[k].edge, matches[k].score)
	})

	total := len(matches)
	rows := make([]QueryRow, 0, limit)
	for i := opts.Offset; i < len(matches) && len(rows) < limit; i++ {
		rows = append(rows, j.queryRow(matches[i].edge))
	}
	return QueryResult{Total: total, Limit: limit, Offset: opts.Offset, Rows: rows}
}

func edgeBetter(a Edge, aScore int, b Edge, bScore int) bool {
	if aScore != bScore {
		return aScore < bScore
	}
	ap, bp := 2.0, 2.0
	if a.PValue != nil {
		ap = *a.PValue
	}
	if b.PValue != nil {
		bp = *b.PValue
	}
	if ap != bp {
		return ap < bp
	}
	as, bs := 0.0, 0.0
	if a.Score != nil {
		as = *a.Score
	}
	if b.Score != nil {
		bs = *b.Score
	}
	if as != bs {
		return as > bs
	}
	if a.TFID != b.TFID {
		return a.TFID < b.TFID
	}
	return a.TargetID < b.TargetID
}

func (j *jobState) queryRow(e Edge) QueryRow {
	symbol, desc, source := j.annotationFor(e.TargetID, e.TargetSymbol)
	meta := j.motifs[e.MotifID]
	return QueryRow{
		TFID:              e.TFID,
		TFSymbol:          e.TFSymbol,
		TFName:            j.tfDisplayName(e.TFID, e.TFSymbol),
		TFDescription:     j.annotations[e.TFID].Description,
		TFFamily:          e.TFFamily,
		TargetID:          e.TargetID,
		TargetSymbol:      symbol,
		TargetDescription: desc,
		AnnotationSource:  source,
		EvidenceType:      e.EvidenceType,
		EvidenceLevel:     e.EvidenceLevel,
		Method:            e.Method,
		Source:            e.Source,
		PMID:              e.PMID,
		Condition:         e.Condition,
		PromoterStart:     e.PromoterStart,
		PromoterEnd:       e.PromoterEnd,
		MotifID:           e.MotifID,
		MotifName:         meta.Name,
		Consensus:         meta.Consensus,
		HitStart:          e.HitStart,
		HitEnd:            e.HitEnd,
		Strand:            e.Strand,
		Score:             e.Score,
		PValue:            e.PValue,
		Notes:             e.Notes,
	}
}

type datasetQueryItem struct {
	edge  Edge
	score int
}

type datasetQueryHeap []datasetQueryItem

func (h datasetQueryHeap) Len() int { return len(h) }
func (h datasetQueryHeap) Less(i, j int) bool {
	// The root is the worst retained item, so a bounded heap can evict it
	// when a better row arrives.
	return edgeBetter(h[j].edge, h[j].score, h[i].edge, h[i].score)
}
func (h datasetQueryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *datasetQueryHeap) Push(x any)   { *h = append(*h, x.(datasetQueryItem)) }
func (h *datasetQueryHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (j *jobState) queryDataset(opts QueryOptions) QueryResult {
	if opts.Direction != "target" {
		opts.Direction = "tf"
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	keep := opts.Offset + limit
	if keep < 1 {
		keep = 1
	}
	if keep > 5000 {
		keep = 5000
	}
	motifQ := strings.ToLower(strings.TrimSpace(opts.Motif))
	q := strings.TrimSpace(opts.Query)
	h := &datasetQueryHeap{}
	heap.Init(h)
	total := 0

	consider := func(e Edge) error {
		if opts.MaxPValue > 0 && e.PValue != nil && *e.PValue > opts.MaxPValue {
			return nil
		}
		if motifQ != "" {
			meta := j.motifs[e.MotifID]
			hay := strings.ToLower(e.MotifID + " " + meta.Name)
			if !strings.Contains(hay, motifQ) {
				return nil
			}
		}
		score := 0
		if opts.Direction == "target" {
			symbol, desc, _ := j.annotationFor(e.TargetID, e.TargetSymbol)
			score = textMatchScore([]string{e.TargetID, e.TargetSymbol, symbol, desc}, q)
		} else {
			score = textMatchScore([]string{e.TFID, e.TFSymbol, j.tfDisplayName(e.TFID, e.TFSymbol)}, q)
		}
		if score >= 99 {
			return nil
		}
		total++
		item := datasetQueryItem{edge: e, score: score}
		if h.Len() < keep {
			heap.Push(h, item)
			return nil
		}
		if edgeBetter(item.edge, item.score, (*h)[0].edge, (*h)[0].score) {
			heap.Pop(h)
			heap.Push(h, item)
		}
		return nil
	}

	if q == "" {
		if motifQ != "" {
			// A motif-only query can use the motif index instead of streaming
			// the entire evidence table.
			var offsets []int64
			for id := range j.data.motifOffsets {
				meta := j.motifs[id]
				hay := strings.ToLower(id + " " + meta.Name)
				if strings.Contains(hay, motifQ) {
					offsets = append(offsets, j.data.motifOffsets[id]...)
				}
			}
			if len(offsets) == 0 {
				return QueryResult{Limit: limit, Offset: opts.Offset, Rows: []QueryRow{}}
			}
			if err := j.data.forEachAt(offsets, consider); err != nil {
				return QueryResult{Limit: limit, Offset: opts.Offset, Rows: []QueryRow{}}
			}
		} else if err := j.data.forEach(consider); err != nil {
			return QueryResult{Limit: limit, Offset: opts.Offset, Rows: []QueryRow{}}
		}
	} else {
		var ids []string
		if opts.Direction == "target" {
			for id, keys := range j.data.targetKeys {
				if textMatchScore(keys, q) < 99 {
					ids = append(ids, id)
				}
			}
		} else {
			for id, keys := range j.data.tfKeys {
				if textMatchScore(keys, q) < 99 {
					ids = append(ids, id)
				}
			}
		}
		if len(ids) == 0 {
			return QueryResult{Limit: limit, Offset: opts.Offset, Rows: []QueryRow{}}
		}
		var offsets []int64
		for _, id := range ids {
			if opts.Direction == "target" {
				offsets = append(offsets, j.data.targetOffsets[id]...)
			} else {
				offsets = append(offsets, j.data.tfOffsets[id]...)
			}
		}
		if err := j.data.forEachAt(offsets, consider); err != nil {
			return QueryResult{Limit: limit, Offset: opts.Offset, Rows: []QueryRow{}}
		}
	}

	items := make([]datasetQueryItem, 0, h.Len())
	for h.Len() > 0 {
		items = append(items, heap.Pop(h).(datasetQueryItem))
	}
	sort.SliceStable(items, func(i, k int) bool {
		return edgeBetter(items[i].edge, items[i].score, items[k].edge, items[k].score)
	})
	start := opts.Offset
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	rows := make([]QueryRow, 0, end-start)
	for _, item := range items[start:end] {
		rows = append(rows, j.queryRow(item.edge))
	}
	return QueryResult{Total: total, Limit: limit, Offset: opts.Offset, Rows: rows}
}

func reverseComplement(seq string) string {
	out := make([]byte, len(seq))
	for i := len(seq) - 1; i >= 0; i-- {
		switch seq[i] {
		case 'A', 'a':
			out[len(seq)-1-i] = 'T'
		case 'C', 'c':
			out[len(seq)-1-i] = 'G'
		case 'G', 'g':
			out[len(seq)-1-i] = 'C'
		case 'T', 't':
			out[len(seq)-1-i] = 'A'
		default:
			out[len(seq)-1-i] = 'N'
		}
	}
	return string(out)
}

func edgePromoterStart(e Edge, prom *Promoter) *uint64 {
	if e.PromoterStart != nil {
		return e.PromoterStart
	}
	if prom.Start > 0 {
		v := prom.Start
		return &v
	}
	return nil
}

func edgePromoterEnd(e Edge, prom *Promoter) *uint64 {
	if e.PromoterEnd != nil {
		return e.PromoterEnd
	}
	if prom.End > 0 {
		v := prom.End
		return &v
	}
	return nil
}

func (j *jobState) promoterView(targetID string) (*PromoterView, error) {
	prom := j.promoters[targetID]
	if prom == nil {
		// Accept a symbol or a case-insensitive ID as a convenience.
		lower := strings.ToLower(targetID)
		for id, p := range j.promoters {
			if strings.ToLower(id) == lower {
				prom = p
				break
			}
		}
	}
	if prom == nil {
		return nil, fmt.Errorf("target %q has no promoter in this run", targetID)
	}
	if prom.GeneID != targetID {
		targetID = prom.GeneID
	}
	sourceEdges := j.edges
	if j.data != nil {
		edges, err := j.data.readTargetEdges(targetID)
		if err != nil {
			return nil, err
		}
		sourceEdges = edges
	}

	var targetSymbol string
	var annotationSource string
	for _, e := range sourceEdges {
		if e.TargetID == targetID {
			targetSymbol = e.TargetSymbol
			break
		}
	}
	symbol, desc, source := j.annotationFor(targetID, targetSymbol)
	annotationSource = source
	if symbol != "" {
		targetSymbol = symbol
	}

	type group struct {
		hit PromoterHit
		tfs map[string]TFInfo
	}
	groups := map[string]*group{}
	for _, e := range sourceEdges {
		if e.TargetID != targetID || e.MotifID == "" || e.HitStart == nil || e.HitEnd == nil {
			continue
		}
		// A target may have historical evidence from more than one promoter
		// window. This view shows the selected promoter only; never merge the
		// same relative coordinates across windows.
		if e.PromoterStart != nil && prom.Start > 0 && *e.PromoterStart != prom.Start {
			continue
		}
		if e.PromoterEnd != nil && prom.End > 0 && *e.PromoterEnd != prom.End {
			continue
		}
		key := hitKey(e)
		g := groups[key]
		if g == nil {
			meta := j.motifs[e.MotifID]
			g = &group{
				hit: PromoterHit{
					MotifID:       e.MotifID,
					MotifName:     meta.Name,
					Consensus:     meta.Consensus,
					Start:         *e.HitStart,
					End:           *e.HitEnd,
					PromoterStart: edgePromoterStart(e, prom),
					PromoterEnd:   edgePromoterEnd(e, prom),
					Strand:        e.Strand,
					Score:         e.Score,
					PValue:        e.PValue,
				},
				tfs: map[string]TFInfo{},
			}
			if prom.Start > 0 && prom.End >= prom.Start {
				if prom.Strand == "-" && *e.HitStart >= 1 && *e.HitEnd <= prom.End {
					g.hit.GenomicStart = prom.End - *e.HitEnd + 1
					g.hit.GenomicEnd = prom.End - *e.HitStart + 1
				} else if prom.Strand != "-" {
					g.hit.GenomicStart = prom.Start + *e.HitStart - 1
					g.hit.GenomicEnd = prom.Start + *e.HitEnd - 1
				}
				if g.hit.GenomicStart > g.hit.GenomicEnd {
					g.hit.GenomicStart, g.hit.GenomicEnd = g.hit.GenomicEnd, g.hit.GenomicStart
				}
			}
			if *e.HitStart >= 1 && *e.HitEnd <= uint64(len(prom.Sequence)) {
				site := prom.Sequence[*e.HitStart-1 : *e.HitEnd]
				g.hit.Site = site
				if e.Strand == "-" {
					g.hit.MatchedSite = reverseComplement(site)
				} else {
					g.hit.MatchedSite = site
				}
			}
			groups[key] = g
		}
		tf := TFInfo{ID: e.TFID, Symbol: e.TFSymbol, Name: j.tfDisplayName(e.TFID, e.TFSymbol), Family: e.TFFamily}
		if donors, ok := j.donors[donorKey{TFID: e.TFID, MotifID: e.MotifID}]; ok {
			tf.Donors = donors
		}
		g.tfs[e.TFID] = tf
	}

	hits := make([]PromoterHit, 0, len(groups))
	for _, g := range groups {
		for _, tf := range g.tfs {
			g.hit.TFs = append(g.hit.TFs, tf)
		}
		sort.Slice(g.hit.TFs, func(a, b int) bool {
			if g.hit.TFs[a].Symbol != g.hit.TFs[b].Symbol {
				return g.hit.TFs[a].Symbol < g.hit.TFs[b].Symbol
			}
			return g.hit.TFs[a].ID < g.hit.TFs[b].ID
		})
		hits = append(hits, g.hit)
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].Start != hits[b].Start {
			return hits[a].Start < hits[b].Start
		}
		if hits[a].End != hits[b].End {
			return hits[a].End < hits[b].End
		}
		if hits[a].Strand != hits[b].Strand {
			return hits[a].Strand < hits[b].Strand
		}
		return hits[a].MotifID < hits[b].MotifID
	})

	tss := len(prom.Sequence)
	if j.Params.PromoterLen > 0 && j.Params.Downstream > 0 {
		// With a downstream window, the TSS is the first downstream base, i.e.
		// position PromoterLen+1 in the oriented promoter.
		tss = j.Params.PromoterLen + 1
		if tss > len(prom.Sequence) {
			tss = len(prom.Sequence)
		}
		if tss < 1 {
			tss = 1
		}
	}
	return &PromoterView{
		TargetID:          targetID,
		TargetSymbol:      targetSymbol,
		TargetDescription: desc,
		AnnotationSource:  annotationSource,
		Chrom:             prom.Chrom,
		Start:             prom.Start,
		End:               prom.End,
		Strand:            prom.Strand,
		Length:            len(prom.Sequence),
		Sequence:          prom.Sequence,
		TSSPosition:       tss,
		Hits:              hits,
	}, nil
}
