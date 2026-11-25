package regutrace

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type siteSchema struct {
	pwmHash, motifIDs, geneID, chrom, promoterStart, promoterEnd, promoterStrand int
	hitStart, hitEnd, hitStrand, genomicStart, genomicEnd, genomicStrand         int
	score, pValue, tfCount, scanKey                                              int
}

func newSiteSchema(header []string) (*siteSchema, error) {
	normalized := make([]string, len(header))
	for i, value := range header {
		normalized[i] = normalizeHeader(value)
	}
	col := func(names ...string) int { return findColumn(normalized, names...) }
	schema := &siteSchema{
		pwmHash: col("pwm_hash"), motifIDs: col("motif_ids", "motif_id"), geneID: col("gene_id", "target_id"),
		chrom: col("chrom"), promoterStart: col("promoter_start"), promoterEnd: col("promoter_end"),
		promoterStrand: col("promoter_strand"), hitStart: col("hit_start"), hitEnd: col("hit_end"),
		hitStrand: col("hit_strand"), genomicStart: col("genomic_hit_start"), genomicEnd: col("genomic_hit_end"),
		genomicStrand: col("genomic_hit_strand"), score: col("score"), pValue: col("p_value"),
		tfCount: col("tf_count"), scanKey: col("scan_key"),
	}
	if schema.geneID < 0 || schema.motifIDs < 0 || schema.hitStart < 0 || schema.hitEnd < 0 {
		return nil, errors.New("site table needs gene_id, motif_ids, hit_start and hit_end")
	}
	return schema, nil
}

type SiteRecord struct {
	PWMHash        string
	MotifIDs       []string
	TargetID       string
	Chrom          string
	PromoterStart  *uint64
	PromoterEnd    *uint64
	PromoterStrand string
	HitStart       *uint64
	HitEnd         *uint64
	HitStrand      string
	GenomicStart   *uint64
	GenomicEnd     *uint64
	GenomicStrand  string
	Score          *float64
	PValue         *float64
	TFCount        int
	ScanKey        string
}

func optionalUintCell(rec []string, index int) *uint64 {
	if index < 0 || index >= len(rec) {
		return nil
	}
	raw := strings.TrimSpace(rec[index])
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func optionalFloatCell(rec []string, index int) *float64 {
	if index < 0 || index >= len(rec) {
		return nil
	}
	raw := strings.TrimSpace(rec[index])
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func (s *siteSchema) record(rec []string) (SiteRecord, error) {
	target := cell(rec, s.geneID)
	if target == "" {
		return SiteRecord{}, errors.New("empty gene_id")
	}
	motifs := uniqueStrings(strings.Split(cell(rec, s.motifIDs), ";")...)
	if len(motifs) == 0 {
		return SiteRecord{}, errors.New("empty motif_ids")
	}
	return SiteRecord{
		PWMHash: cell(rec, s.pwmHash), MotifIDs: motifs, TargetID: target,
		Chrom: cell(rec, s.chrom), PromoterStart: optionalUintCell(rec, s.promoterStart),
		PromoterEnd: optionalUintCell(rec, s.promoterEnd), PromoterStrand: cell(rec, s.promoterStrand),
		HitStart: optionalUintCell(rec, s.hitStart), HitEnd: optionalUintCell(rec, s.hitEnd),
		HitStrand: cell(rec, s.hitStrand), GenomicStart: optionalUintCell(rec, s.genomicStart),
		GenomicEnd: optionalUintCell(rec, s.genomicEnd), GenomicStrand: cell(rec, s.genomicStrand),
		Score: optionalFloatCell(rec, s.score), PValue: optionalFloatCell(rec, s.pValue),
		TFCount: intOrZero(rec, s.tfCount), ScanKey: cell(rec, s.scanKey),
	}, nil
}

func intOrZero(rec []string, index int) int {
	if index < 0 || index >= len(rec) {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSpace(rec[index]))
	return value
}

// buildSiteDataset indexes the unique site table written by
// `regutrace scan --sites-only`. It never creates TF-target-site rows.
func buildSiteDataset(path string, motifs map[string]MotifInfo, annotations map[string]AnnotationEntry, promoterCount int, annotationSource string, mappings map[string][]TFMappingRow, includeFallback bool) (*dataset, error) {
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
		return nil, fmt.Errorf("parse site TSV header: %w", err)
	}
	schema, err := newSiteSchema(header)
	if err != nil {
		return nil, err
	}
	ds := &dataset{
		path: path, mode: "sites", siteSchema: schema,
		targetOffsets: map[string][]int64{}, motifOffsets: map[string][]int64{},
	}
	last := r.InputOffset()
	row := 1
	targetSymbols := map[string]string{}
	targetIndexes := map[string]int{}
	motifTargets := map[string][]byte{}
	for {
		start := last
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse site TSV row %d: %w", row+1, err)
		}
		last = r.InputOffset()
		row++
		site, err := schema.record(rec)
		if err != nil {
			return nil, fmt.Errorf("site row %d: %w", row, err)
		}
		ds.total++
		index, ok := targetIndexes[site.TargetID]
		if !ok {
			index = len(targetIndexes)
			targetIndexes[site.TargetID] = index
		}
		ds.targetOffsets[site.TargetID] = append(ds.targetOffsets[site.TargetID], start)
		for _, motif := range site.MotifIDs {
			ds.motifOffsets[motif] = append(ds.motifOffsets[motif], start)
			bits := motifTargets[motif]
			if len(bits) <= index/8 {
				bits = append(bits, make([]byte, index/8+1-len(bits))...)
			}
			bits[index/8] |= 1 << (index % 8)
			motifTargets[motif] = bits
		}
		if _, ok := targetSymbols[site.TargetID]; !ok {
			targetSymbols[site.TargetID] = site.TargetID
		}
	}
	total := ds.total
	uniqueMotifs := len(ds.motifOffsets)
	uniqueTargets := len(ds.targetOffsets)
	motifPairCount := 0
	for _, bits := range motifTargets {
		motifPairCount += countBits(bits)
	}
	ds.tfTargetCounts = map[string]int{}
	tfPairCount := 0
	for tfID, rows := range mappings {
		motifs := map[string]bool{}
		for _, row := range rows {
			if row.DomainMode == "single_domain_fallback" && !includeFallback {
				continue
			}
			motifs[row.MotifID] = true
		}
		var union []byte
		for motif := range motifs {
			union = orBits(union, motifTargets[motif])
		}
		count := countBits(union)
		ds.tfTargetCounts[tfID] = count
		tfPairCount += count
	}
	ds.summary = &Summary{
		Genes: promoterCount, Promoters: promoterCount,
		MotifLibrarySize: len(motifs), MotifHits: total,
		CandidateSites: &total, UniqueMotifs: uniqueMotifs, MotifsWithHits: uniqueMotifs,
		UniqueTargets: uniqueTargets, UniqueTFs: len(ds.tfTargetCounts),
		MotifTargetPairs: motifPairCount, TFTargetPairs: tfPairCount,
		AnnotationSource: annotationSource, SiteOnly: true, SiteTableAvailable: true,
	}
	ds.summary.TFTargetCounts = ds.tfTargetCounts
	ds.targetKeys = make(map[string][]string, len(targetSymbols))
	for id := range targetSymbols {
		ann := annotations[id]
		ds.targetKeys[id] = uniqueStrings(id, ann.Symbol, ann.Description)
	}
	return ds, nil
}

func orBits(target, source []byte) []byte {
	if len(target) < len(source) {
		target = append(target, make([]byte, len(source)-len(target))...)
	}
	for i, value := range source {
		target[i] |= value
	}
	return target
}

func countBits(bits []byte) int {
	count := 0
	for _, value := range bits {
		count += bitsOnes(value)
	}
	return count
}

func bitsOnes(value byte) int {
	count := 0
	for value != 0 {
		value &= value - 1
		count++
	}
	return count
}

func (ds *dataset) forEachSiteAt(offsets []int64, fn func(SiteRecord) error) error {
	if ds.siteSchema == nil {
		return errors.New("dataset is not a site dataset")
	}
	f, err := os.Open(ds.path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, offset := range offsets {
		if offset < 0 {
			continue
		}
		sr := io.NewSectionReader(f, offset, 2<<20)
		r := csv.NewReader(sr)
		r.Comma = '\t'
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read site at offset %d: %w", offset, err)
		}
		site, err := ds.siteSchema.record(rec)
		if err != nil {
			return err
		}
		if err := fn(site); err != nil {
			return err
		}
	}
	return nil
}

func (j *jobState) siteMotifIDs(tfID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, row := range j.mappingRows(tfID) {
		if row.DomainMode == "single_domain_fallback" && !j.Params.IncludeSingleDomainFallback {
			continue
		}
		if !seen[row.MotifID] {
			seen[row.MotifID] = true
			out = append(out, row.MotifID)
		}
	}
	sort.Strings(out)
	return out
}

func (j *jobState) siteQueryRows(site SiteRecord, tfID, motifID string) []QueryRow {
	symbol, description, source := j.annotationFor(site.TargetID, site.TargetID)
	meta := j.motifs[motifID]
	tfSymbol := j.tfDisplayName(tfID, "")
	return []QueryRow{{
		TFID: tfID, TFSymbol: tfSymbol, TFName: tfSymbol, TargetID: site.TargetID,
		TargetSymbol: symbol, TargetDescription: description, AnnotationSource: source,
		EvidenceType: "prediction_motif", EvidenceLevel: "D",
		Method: "jaspar_pwm", Source: "promoter_scan",
		PromoterStart: site.PromoterStart, PromoterEnd: site.PromoterEnd,
		MotifID: motifID, MotifName: meta.Name, Consensus: meta.Consensus,
		HitStart: site.HitStart, HitEnd: site.HitEnd, Strand: site.HitStrand,
		Score: site.Score, PValue: site.PValue,
		MappingStatus: j.mappingStatus(tfID, motifID), TestabilityStatus: j.testability(motifID).Status,
		SharedMotifTFs: j.sharedPWMTFCount(motifID), SharedMotifTFIDs: j.sharedMotifTFs(motifID),
	}}
}

func (j *jobState) sitePromoterHits(targetID string) ([]PromoterHit, error) {
	if j.data == nil || j.data.mode != "sites" {
		return nil, errors.New("dataset is not a site dataset")
	}
	type grouped struct {
		hit  PromoterHit
		seen map[string]bool
	}
	byKey := map[string]*grouped{}
	err := j.data.forEachSiteAt(j.data.targetOffsets[targetID], func(site SiteRecord) error {
		key := fmt.Sprintf("%s|%v|%v|%s", strings.Join(site.MotifIDs, ";"), site.HitStart, site.HitEnd, site.HitStrand)
		item := byKey[key]
		if item == nil {
			hit := PromoterHit{
				MotifID: strings.Join(site.MotifIDs, ";"), Start: valueOrZero(site.HitStart),
				End: valueOrZero(site.HitEnd), PromoterStart: site.PromoterStart, PromoterEnd: site.PromoterEnd,
				GenomicStart: valueOrZero(site.GenomicStart), GenomicEnd: valueOrZero(site.GenomicEnd),
				Strand: site.HitStrand, Score: site.Score, PValue: site.PValue,
			}
			if len(site.MotifIDs) > 0 {
				meta := j.motifs[site.MotifIDs[0]]
				hit.MotifName = meta.Name
				hit.Consensus = meta.Consensus
			}
			item = &grouped{hit: hit, seen: map[string]bool{}}
			byKey[key] = item
		}
		for _, motif := range site.MotifIDs {
			for _, tf := range j.phase4.MotifTFs[motif] {
				if item.seen[tf] {
					continue
				}
				item.seen[tf] = true
				item.hit.TFs = append(item.hit.TFs, TFInfo{ID: tf, Symbol: j.tfDisplayName(tf, ""), Family: j.tfFamilyByID()[tf]})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	hits := make([]PromoterHit, 0, len(byKey))
	for _, item := range byKey {
		sort.Slice(item.hit.TFs, func(i, k int) bool { return item.hit.TFs[i].ID < item.hit.TFs[k].ID })
		hits = append(hits, item.hit)
	}
	sort.Slice(hits, func(i, k int) bool { return hits[i].Start < hits[k].Start })
	return hits, nil
}

func valueOrZero(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}
