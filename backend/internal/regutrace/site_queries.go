package regutrace

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (j *jobState) siteQuery(opts QueryOptions) QueryResult {
	if j.data == nil || j.data.mode != "sites" {
		return QueryResult{}
	}
	if opts.Direction != "target" {
		opts.Direction = "tf"
	}
	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	motifQuery := strings.ToLower(strings.TrimSpace(opts.Motif))
	q := strings.TrimSpace(opts.Query)
	total := 0
	rows := make([]QueryRow, 0)
	consider := func(row QueryRow, matchScore int) {
		if opts.MaxPValue > 0 && row.PValue != nil && *row.PValue > opts.MaxPValue {
			return
		}
		if motifQuery != "" {
			hay := strings.ToLower(row.MotifID + " " + row.MotifName)
			if !strings.Contains(hay, motifQuery) {
				return
			}
		}
		if matchScore >= 99 {
			return
		}
		if row.MappingStatus != "" && !j.modeAllows(row.MappingStatus) {
			return
		}
		total++
		rows = append(rows, row)
	}
	if opts.Direction == "tf" {
		tfID, ok := j.resolveTFID(q)
		if q != "" && ok {
			q = tfID
		}
		motifs := j.siteMotifIDs(q)
		if len(motifs) == 0 {
			return QueryResult{Limit: limit, Offset: offset, Rows: []QueryRow{}}
		}
		for _, motif := range motifs {
			_ = j.data.forEachSiteAt(j.data.motifOffsets[motif], func(site SiteRecord) error {
				if !containsString(site.MotifIDs, motif) {
					return nil
				}
				score := 0
				if q != "" {
					score = textMatchScore([]string{tfID, j.tfDisplayName(tfID, "")}, q)
				}
				rows := j.siteQueryRows(site, tfID, motif)
				for _, row := range rows {
					consider(row, score)
				}
				return nil
			})
		}
	} else {
		targetID, ok := j.resolveSiteTarget(q)
		if q != "" && ok {
			q = targetID
		}
		if !ok && q != "" {
			return QueryResult{Limit: limit, Offset: offset, Rows: []QueryRow{}}
		}
		_ = j.data.forEachSiteAt(j.data.targetOffsets[q], func(site SiteRecord) error {
			for _, motif := range site.MotifIDs {
				for _, tf := range j.phase4.MotifTFs[motif] {
					score := 0
					if q != "" {
						symbol, description, _ := j.annotationFor(site.TargetID, site.TargetID)
						score = textMatchScore([]string{site.TargetID, symbol, description}, q)
					}
					rows := j.siteQueryRows(site, tf, motif)
					for _, row := range rows {
						consider(row, score)
					}
				}
			}
			return nil
		})
	}
	sort.SliceStable(rows, func(i, k int) bool { return siteRowBetter(rows[i], rows[k]) })
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return QueryResult{Total: total, Limit: limit, Offset: offset, Rows: rows[offset:end]}
}

func siteRowBetter(a, b QueryRow) bool {
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

func (j *jobState) siteTFTargets(opts TFTargetOptions) (TFTargetResult, error) {
	if j.data == nil || j.data.mode != "sites" {
		return TFTargetResult{}, errors.New("dataset is not a site dataset")
	}
	tfID, ok := j.resolveTFID(opts.TFID)
	if !ok {
		return TFTargetResult{}, fmt.Errorf("%w: TF %q has no evidence in this run", ErrNotFound, opts.TFID)
	}
	policy, err := normalizeMotifPolicy(opts.Policy)
	if err != nil {
		return TFTargetResult{}, err
	}
	allowedMotifs, policyInfo := j.policyAllowed(tfID, policy)
	motifFilter := strings.TrimSpace(opts.Motif)
	limit, offset := opts.Limit, opts.Offset
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	cacheKey := fmt.Sprintf("%s|%s|%s|%g|%s|%s|%d", tfID, strings.ToLower(opts.Status), strings.ToLower(motifFilter), opts.MaxP, j.Params.MappingMode, policy, opts.MinSites)
	j.tfTargetMu.Lock()
	if cached, ok := j.tfTargetCache[cacheKey]; ok {
		rows := cached.Rows
		total := len(rows)
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		j.tfTargetMu.Unlock()
		return TFTargetResult{Total: total, Limit: limit, Offset: offset, TFID: tfID, Policy: cached.Policy, Rows: rows[offset:end]}, nil
	}
	j.tfTargetMu.Unlock()
	type aggregate struct {
		row    TFTargetRow
		motifs map[string]bool
		sites  map[string]bool
		// maxMotifSites is the largest per-motif site count seen for this target.
		maxMotifSites int
	}
	byTarget := map[string]*aggregate{}
	for _, motif := range j.siteMotifIDs(tfID) {
		if motifFilter != "" && !strings.EqualFold(motif, motifFilter) {
			continue
		}
		if allowedMotifs != nil && !allowedMotifs[motif] {
			continue
		}
		// Sites are walked one motif at a time, so counting per target inside
		// this loop gives the per-motif multiplicity without keeping a set per
		// (target, motif) pair.
		perMotif := map[string]int{}
		_ = j.data.forEachSiteAt(j.data.motifOffsets[motif], func(site SiteRecord) error {
			if opts.MaxP > 0 && site.PValue != nil && *site.PValue > opts.MaxP {
				return nil
			}
			status := j.mappingStatus(tfID, motif)
			if opts.Status != "" {
				if !strings.EqualFold(status, opts.Status) {
					return nil
				}
			} else if !j.modeAllows(status) {
				return nil
			}
			if !containsString(site.MotifIDs, motif) {
				return nil
			}
			agg := byTarget[site.TargetID]
			if agg == nil {
				symbol, description, _ := j.annotationFor(site.TargetID, site.TargetID)
				agg = &aggregate{
					row: TFTargetRow{TargetID: site.TargetID, TargetSymbol: symbol, TargetDescription: description,
						EvidenceLevel: "D", MappingStatuses: []string{}},
					motifs: map[string]bool{}, sites: map[string]bool{},
				}
				byTarget[site.TargetID] = agg
			}
			if agg.row.BestPValue == nil || (site.PValue != nil && *site.PValue < *agg.row.BestPValue) {
				agg.row.BestPValue, agg.row.BestScore = site.PValue, site.Score
				agg.row.PromoterStart, agg.row.PromoterEnd = site.PromoterStart, site.PromoterEnd
				agg.row.HitStart, agg.row.HitEnd, agg.row.Strand = site.HitStart, site.HitEnd, site.HitStrand
			}
			perMotif[site.TargetID]++
			agg.motifs[motif] = true
			agg.row.SharedMotifTFs = maxIntValue(agg.row.SharedMotifTFs, j.sharedPWMTFCount(motif))
			key := fmt.Sprintf("%s|%v|%v|%s", site.TargetID, site.HitStart, site.HitEnd, site.HitStrand)
			agg.sites[key] = true
			if !containsString(agg.row.MappingStatuses, status) {
				agg.row.MappingStatuses = append(agg.row.MappingStatuses, status)
			}
			return nil
		})
		for target, count := range perMotif {
			if agg := byTarget[target]; agg != nil && count > agg.maxMotifSites {
				agg.maxMotifSites = count
			}
		}
	}
	rows := make([]TFTargetRow, 0, len(byTarget))
	for _, agg := range byTarget {
		agg.row.SiteCount = len(agg.sites)
		agg.row.MaxMotifSites = agg.maxMotifSites
		if opts.MinSites > 1 && agg.row.MaxMotifSites < opts.MinSites {
			continue
		}
		for motif := range agg.motifs {
			agg.row.MotifIDs = append(agg.row.MotifIDs, motif)
		}
		sort.Strings(agg.row.MotifIDs)
		sort.Strings(agg.row.MappingStatuses)
		rows = append(rows, agg.row)
	}
	sort.Slice(rows, func(i, k int) bool {
		ip, kp := floatOrZero(rows[i].BestPValue), floatOrZero(rows[k].BestPValue)
		if ip != kp {
			return ip < kp
		}
		return rows[i].TargetID < rows[k].TargetID
	})
	j.tfTargetMu.Lock()
	if j.tfTargetCache == nil {
		j.tfTargetCache = map[string]TFTargetResult{}
	}
	if len(j.tfTargetOrder) >= 4 {
		oldest := j.tfTargetOrder[0]
		j.tfTargetOrder = j.tfTargetOrder[1:]
		delete(j.tfTargetCache, oldest)
	}
	j.tfTargetCache[cacheKey] = TFTargetResult{Total: len(rows), Policy: &policyInfo, Rows: rows}
	j.tfTargetOrder = append(j.tfTargetOrder, cacheKey)
	j.tfTargetMu.Unlock()
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return TFTargetResult{Total: total, Limit: limit, Offset: offset, TFID: tfID, Policy: &policyInfo, Rows: rows[offset:end]}, nil
}

func (j *jobState) siteTargetRegulators(target string, limit, offset int, motifFilter, statusFilter string, policy MotifPolicy, maxP float64) (RegulatorResult, error) {
	if j.data == nil || j.data.mode != "sites" {
		return RegulatorResult{}, errors.New("dataset is not a site dataset")
	}
	resolved, ok := j.resolveSiteTarget(target)
	if !ok {
		return RegulatorResult{}, fmt.Errorf("%w: target %q has no evidence in this run", ErrNotFound, target)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	type aggregate struct {
		row   RegulatorRow
		sites map[string]bool
	}
	byTF := map[string]*aggregate{}
	policyCache := map[string]map[string]bool{}
	_ = j.data.forEachSiteAt(j.data.targetOffsets[resolved], func(site SiteRecord) error {
		if maxP > 0 && site.PValue != nil && *site.PValue > maxP {
			return nil
		}
		for _, motif := range site.MotifIDs {
			if motifFilter != "" && !strings.EqualFold(motif, motifFilter) {
				continue
			}
			for _, tf := range j.phase4.MotifTFs[motif] {
				if policy != MotifPolicyAll {
					allowed, seen := policyCache[tf]
					if !seen {
						allowed, _ = j.policyAllowed(tf, policy)
						policyCache[tf] = allowed
					}
					if allowed != nil && !allowed[motif] {
						continue
					}
				}
				status := j.mappingStatus(tf, motif)
				if statusFilter != "" {
					if !strings.EqualFold(status, statusFilter) {
						continue
					}
				} else if !j.modeAllows(status) {
					continue
				}
				agg := byTF[tf]
				if agg == nil {
					agg = &aggregate{row: RegulatorRow{TFID: tf, TFSymbol: j.tfDisplayName(tf, ""), TFFamily: j.tfFamilyByID()[tf], EvidenceLevel: "D", MappingStatuses: []string{}}, sites: map[string]bool{}}
					byTF[tf] = agg
				}
				if agg.row.BestPValue == nil || (site.PValue != nil && *site.PValue < *agg.row.BestPValue) {
					agg.row.BestPValue, agg.row.BestScore = site.PValue, site.Score
				}
				if !containsString(agg.row.MotifIDs, motif) {
					agg.row.MotifIDs = append(agg.row.MotifIDs, motif)
				}
				agg.row.SharedMotifTFs = maxIntValue(agg.row.SharedMotifTFs, j.sharedPWMTFCount(motif))
				agg.sites[fmt.Sprintf("%v|%v|%s", site.HitStart, site.HitEnd, site.HitStrand)] = true
				if !containsString(agg.row.MappingStatuses, status) {
					agg.row.MappingStatuses = append(agg.row.MappingStatuses, status)
				}
			}
		}
		return nil
	})
	rows := make([]RegulatorRow, 0, len(byTF))
	for _, agg := range byTF {
		agg.row.SiteCount = len(agg.sites)
		sort.Strings(agg.row.MotifIDs)
		sort.Strings(agg.row.MappingStatuses)
		rows = append(rows, agg.row)
	}
	sort.Slice(rows, func(i, k int) bool {
		ip, kp := floatOrZero(rows[i].BestPValue), floatOrZero(rows[k].BestPValue)
		if ip != kp {
			return ip < kp
		}
		return rows[i].TFID < rows[k].TFID
	})
	total := len(rows)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return RegulatorResult{Total: total, Limit: limit, Offset: offset, TargetID: resolved, Policy: string(policy), Rows: rows[offset:end]}, nil
}

func maxIntValue(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (j *jobState) sitePromoterView(targetID string) (*PromoterView, error) {
	prom := j.promoters[targetID]
	if prom == nil {
		lower := strings.ToLower(targetID)
		for id, candidate := range j.promoters {
			if strings.ToLower(id) == lower {
				prom = candidate
				break
			}
		}
	}
	if prom == nil {
		return nil, fmt.Errorf("%w: target %q has no promoter in this run", ErrNotFound, targetID)
	}
	if prom.GeneID != targetID {
		targetID = prom.GeneID
	}
	hits, err := j.sitePromoterHits(targetID)
	if err != nil {
		return nil, err
	}
	for i := range hits {
		if hits[i].Start >= 1 && hits[i].End <= uint64(len(prom.Sequence)) {
			site := prom.Sequence[hits[i].Start-1 : hits[i].End]
			hits[i].Site = site
			if hits[i].Strand == "-" {
				hits[i].MatchedSite = reverseComplement(site)
			} else {
				hits[i].MatchedSite = site
			}
		}
	}
	symbol, desc, source := j.annotationFor(targetID, targetID)
	tss := len(prom.Sequence)
	if j.Params.PromoterLen > 0 && j.Params.Downstream > 0 {
		tss = j.Params.PromoterLen + 1
		if tss > len(prom.Sequence) {
			tss = len(prom.Sequence)
		}
		if tss < 1 {
			tss = 1
		}
	}
	return &PromoterView{
		TargetID: targetID, TargetSymbol: symbol, TargetDescription: desc, AnnotationSource: source,
		Chrom: prom.Chrom, Start: prom.Start, End: prom.End, Strand: prom.Strand,
		Length: len(prom.Sequence), Sequence: prom.Sequence, TSSPosition: tss, Hits: hits,
	}, nil
}

func (j *jobState) siteEvidenceFromSites(opts SiteEvidenceOptions) (SiteEvidenceResult, error) {
	if j.data == nil || j.data.mode != "sites" {
		return SiteEvidenceResult{}, errors.New("dataset is not a site dataset")
	}
	targetID, ok := j.resolveSiteTarget(opts.TargetID)
	if !ok {
		return SiteEvidenceResult{}, fmt.Errorf("%w: target %q has no evidence in this run", ErrNotFound, opts.TargetID)
	}
	rows := []QueryRow{}
	err := j.data.forEachSiteAt(j.data.targetOffsets[targetID], func(site SiteRecord) error {
		if !containsString(site.MotifIDs, opts.MotifID) {
			return nil
		}
		if opts.HitStart != nil && (site.HitStart == nil || *site.HitStart != *opts.HitStart) {
			return nil
		}
		if opts.HitEnd != nil && (site.HitEnd == nil || *site.HitEnd != *opts.HitEnd) {
			return nil
		}
		if opts.Strand != "" && !strings.EqualFold(site.HitStrand, opts.Strand) {
			return nil
		}
		for _, tf := range j.phase4.MotifTFs[opts.MotifID] {
			if opts.TFID != "" && !strings.EqualFold(tf, opts.TFID) {
				continue
			}
			rows = append(rows, j.siteQueryRows(site, tf, opts.MotifID)...)
		}
		return nil
	})
	if err != nil {
		return SiteEvidenceResult{}, err
	}
	return SiteEvidenceResult{Total: len(rows), Rows: rows}, nil
}
