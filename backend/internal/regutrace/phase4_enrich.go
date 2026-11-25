package regutrace

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// EnrichmentRequest is the frontend exploratory motif enrichment request.
// The test is intentionally binary at the promoter level; it does not model
// multiple sites, promoter length, GC or motif similarity.
type EnrichmentRequest struct {
	GeneSet    []string `json:"gene_set"`
	Background []string `json:"background,omitempty"`
	MotifIDs   []string `json:"motif_ids"`
	MaxPValue  *float64 `json:"max_p_value,omitempty"`
}

type EnrichmentRow struct {
	MotifID                    string   `json:"motif_id"`
	MotifName                  string   `json:"motif_name,omitempty"`
	TestabilityStatus          string   `json:"testability_status"`
	GeneSetSize                int      `json:"gene_set_size"`
	BackgroundSize             int      `json:"background_size"`
	ObservedMotifGenes         int      `json:"observed_motif_genes"`
	BackgroundMotifGenes       int      `json:"background_motif_genes"`
	ExpectedOverlap            float64  `json:"expected_overlap"`
	EnrichmentRatio            float64  `json:"enrichment_ratio"`
	OddsRatio                  float64  `json:"odds_ratio"`
	PValue                     float64  `json:"p_value"`
	BHQValue                   float64  `json:"bh_q_value"`
	ContributingGenes          []string `json:"contributing_genes"`
	ContributingGenesTruncated bool     `json:"contributing_genes_truncated"`
}

type EnrichmentResult struct {
	GeneSetSize          int             `json:"gene_set_size"`
	BackgroundSize       int             `json:"background_size"`
	BackgroundSource     string          `json:"background_source"`
	MotifCount           int             `json:"motif_count"`
	MultipleTestingScope string          `json:"multiple_testing_scope"`
	Rows                 []EnrichmentRow `json:"rows"`
	Caveats              []string        `json:"caveats"`
}

func logChoose(n, k int) float64 {
	if k < 0 || k > n || n < 0 {
		return math.Inf(-1)
	}
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

// hypergeomSF returns P(X >= observed) for X ~ Hypergeometric(N, K, n).
func hypergeomSF(observed, draws, successes, population int) float64 {
	if observed <= 0 {
		return 1.0
	}
	if population <= 0 || draws <= 0 || successes <= 0 {
		return 1.0
	}
	minimum := draws - (population - successes)
	if minimum < 0 {
		minimum = 0
	}
	maximum := draws
	if successes < maximum {
		maximum = successes
	}
	if observed > maximum {
		return 0.0
	}
	start := observed
	if start < minimum {
		return 1.0
	}
	denominator := logChoose(population, draws)
	probability := math.Exp(logChoose(successes, start) + logChoose(population-successes, draws-start) - denominator)
	total := probability
	for x := start; x < maximum; x++ {
		numerator := float64(successes-x) * float64(draws-x)
		den := float64(x+1) * float64(population-successes-draws+x+1)
		if den <= 0 || math.IsNaN(numerator) || math.IsInf(numerator, 0) {
			break
		}
		probability *= numerator / den
		total += probability
		if probability == 0 {
			break
		}
	}
	return math.Max(0, math.Min(1, total))
}

func bhQValues(pvalues []float64) []float64 {
	m := len(pvalues)
	if m == 0 {
		return nil
	}
	order := make([]int, m)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return pvalues[order[i]] < pvalues[order[j]] })
	q := make([]float64, m)
	running := 1.0
	for rank := m - 1; rank >= 0; rank-- {
		index := order[rank]
		value := pvalues[index] * float64(m) / float64(rank+1)
		if value < running {
			running = value
		}
		q[index] = math.Max(0, math.Min(1, running))
	}
	return q
}

func (j *jobState) enrich(req EnrichmentRequest) (EnrichmentResult, error) {
	if j.data == nil {
		return EnrichmentResult{}, errors.New("this job has no disk-backed dataset")
	}
	if j.data.mode == "sites" {
		return j.siteEnrich(req)
	}
	set := map[string]bool{}
	for _, raw := range req.GeneSet {
		gene := strings.TrimSpace(raw)
		if gene == "" {
			continue
		}
		set[gene] = true
	}
	if len(set) == 0 {
		return EnrichmentResult{}, errors.New("gene_set must contain at least one gene")
	}
	if len(set) > 50000 {
		return EnrichmentResult{}, errors.New("gene_set is limited to 50,000 genes")
	}
	motifs := uniqueStrings(req.MotifIDs...)
	if len(motifs) == 0 {
		return EnrichmentResult{}, errors.New("select at least one motif_id")
	}
	if len(motifs) > 200 {
		return EnrichmentResult{}, errors.New("at most 200 motifs can be tested in one request")
	}
	if req.MaxPValue != nil && (*req.MaxPValue < 0 || *req.MaxPValue > 1) {
		return EnrichmentResult{}, errors.New("max_p_value must be in [0, 1]")
	}
	universe := map[string]bool{}
	backgroundSource := "all scanned promoters"
	for _, raw := range req.Background {
		gene := strings.TrimSpace(raw)
		if gene != "" {
			universe[gene] = true
		}
	}
	if len(universe) > 0 {
		backgroundSource = "user-supplied background intersected with scanned promoters"
	} else {
		for gene := range j.promoters {
			universe[gene] = true
		}
	}
	for gene := range universe {
		if _, ok := j.promoters[gene]; !ok {
			delete(universe, gene)
		}
	}
	for gene := range set {
		if !universe[gene] {
			delete(set, gene)
		}
	}
	if len(set) == 0 {
		return EnrichmentResult{}, errors.New("no gene_set member is in the scanned promoter universe")
	}
	backgroundSize := len(universe)
	rows := make([]EnrichmentRow, 0, len(motifs))
	pvalues := make([]float64, 0, len(motifs))
	for _, motifID := range motifs {
		meta := j.motifs[motifID]
		hitGenes := map[string]bool{}
		_ = j.data.forEachAt(j.data.motifOffsets[motifID], func(e Edge) error {
			if req.MaxPValue != nil && e.PValue != nil && *e.PValue > *req.MaxPValue {
				return nil
			}
			if e.TargetID != "" && universe[e.TargetID] {
				hitGenes[e.TargetID] = true
			}
			return nil
		})
		observed := 0
		contributing := make([]string, 0)
		for gene := range set {
			if hitGenes[gene] {
				observed++
				contributing = append(contributing, gene)
			}
		}
		sort.Strings(contributing)
		truncated := false
		if len(contributing) > 500 {
			contributing = contributing[:500]
			truncated = true
		}
		backgroundHits := len(hitGenes)
		expected := 0.0
		if backgroundSize > 0 {
			expected = float64(len(set)) * float64(backgroundHits) / float64(backgroundSize)
		}
		ratio := 0.0
		if expected > 0 {
			ratio = float64(observed) / expected
		}
		a, b := float64(observed), float64(len(set)-observed)
		c, d := float64(backgroundHits-observed), float64(backgroundSize-len(set)-backgroundHits+observed)
		odds := ((a + 0.5) * (d + 0.5)) / ((b + 0.5) * (c + 0.5))
		p := hypergeomSF(observed, len(set), backgroundHits, backgroundSize)
		rows = append(rows, EnrichmentRow{
			MotifID: motifID, MotifName: meta.Name,
			TestabilityStatus: j.testability(motifID).Status,
			GeneSetSize:       len(set), BackgroundSize: backgroundSize,
			ObservedMotifGenes: observed, BackgroundMotifGenes: backgroundHits,
			ExpectedOverlap: expected, EnrichmentRatio: ratio, OddsRatio: odds,
			PValue: p, ContributingGenes: contributing, ContributingGenesTruncated: truncated,
		})
		pvalues = append(pvalues, p)
	}
	qvalues := bhQValues(pvalues)
	for i := range rows {
		rows[i].BHQValue = qvalues[i]
	}
	sort.SliceStable(rows, func(i, k int) bool {
		if rows[i].PValue != rows[k].PValue {
			return rows[i].PValue < rows[k].PValue
		}
		return rows[i].MotifID < rows[k].MotifID
	})
	return EnrichmentResult{
		GeneSetSize: len(set), BackgroundSize: backgroundSize, BackgroundSource: backgroundSource,
		MotifCount: len(motifs), MultipleTestingScope: "BH across the requested motifs within this gene set",
		Rows: rows,
		Caveats: []string{
			"Binary promoter hit; multiple sites per promoter are collapsed.",
			"One-sided Fisher exact test; no promoter length or GC matching.",
			"BH is within this gene set; gene-set overlap and motif similarity are not modeled.",
			"An enriched motif is not an enriched individual TF when multiple TFs share the PWM.",
			"Exploratory only; not experimental validation of regulator activity.",
		},
	}, nil
}

func (m *Manager) Enrich(id string, req EnrichmentRequest) (EnrichmentResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	js, ok := m.jobs[id]
	if !ok {
		return EnrichmentResult{}, fmt.Errorf("%w: job not found", ErrNotFound)
	}
	if js.Status != "succeeded" {
		return EnrichmentResult{}, ErrNotCompleted
	}
	return js.enrich(req)
}

func (j *jobState) siteEnrich(req EnrichmentRequest) (EnrichmentResult, error) {
	set := map[string]bool{}
	for _, raw := range req.GeneSet {
		if gene := strings.TrimSpace(raw); gene != "" {
			set[gene] = true
		}
	}
	if len(set) == 0 {
		return EnrichmentResult{}, errors.New("gene_set must contain at least one gene")
	}
	motifs := uniqueStrings(req.MotifIDs...)
	if len(motifs) == 0 {
		return EnrichmentResult{}, errors.New("select at least one motif_id")
	}
	if len(motifs) > 200 {
		return EnrichmentResult{}, errors.New("at most 200 motifs can be tested in one request")
	}
	universe := map[string]bool{}
	backgroundSource := "all scanned promoters"
	for _, raw := range req.Background {
		if gene := strings.TrimSpace(raw); gene != "" {
			universe[gene] = true
		}
	}
	if len(universe) > 0 {
		backgroundSource = "user-supplied background intersected with scanned promoters"
	} else {
		for gene := range j.promoters {
			universe[gene] = true
		}
	}
	for gene := range universe {
		if _, ok := j.promoters[gene]; !ok {
			delete(universe, gene)
		}
	}
	for gene := range set {
		if !universe[gene] {
			delete(set, gene)
		}
	}
	if len(set) == 0 {
		return EnrichmentResult{}, errors.New("no gene_set member is in the scanned promoter universe")
	}
	rows := make([]EnrichmentRow, 0, len(motifs))
	pvalues := make([]float64, 0, len(motifs))
	for _, motifID := range motifs {
		hitGenes := map[string]bool{}
		_ = j.data.forEachSiteAt(j.data.motifOffsets[motifID], func(site SiteRecord) error {
			if req.MaxPValue != nil && site.PValue != nil && *site.PValue > *req.MaxPValue {
				return nil
			}
			if universe[site.TargetID] {
				hitGenes[site.TargetID] = true
			}
			return nil
		})
		observed := 0
		contributing := []string{}
		for gene := range set {
			if hitGenes[gene] {
				observed++
				contributing = append(contributing, gene)
			}
		}
		sort.Strings(contributing)
		truncated := false
		if len(contributing) > 500 {
			contributing = contributing[:500]
			truncated = true
		}
		backgroundHits := len(hitGenes)
		expected := 0.0
		if len(universe) > 0 {
			expected = float64(len(set)) * float64(backgroundHits) / float64(len(universe))
		}
		ratio := 0.0
		if expected > 0 {
			ratio = float64(observed) / expected
		}
		a, b := float64(observed), float64(len(set)-observed)
		c, d := float64(backgroundHits-observed), float64(len(universe)-len(set)-backgroundHits+observed)
		odds := ((a + 0.5) * (d + 0.5)) / ((b + 0.5) * (c + 0.5))
		p := hypergeomSF(observed, len(set), backgroundHits, len(universe))
		rows = append(rows, EnrichmentRow{
			MotifID: motifID, MotifName: j.motifs[motifID].Name,
			TestabilityStatus: j.testability(motifID).Status,
			GeneSetSize:       len(set), BackgroundSize: len(universe),
			ObservedMotifGenes: observed, BackgroundMotifGenes: backgroundHits,
			ExpectedOverlap: expected, EnrichmentRatio: ratio, OddsRatio: odds, PValue: p,
			ContributingGenes: contributing, ContributingGenesTruncated: truncated,
		})
		pvalues = append(pvalues, p)
	}
	qvalues := bhQValues(pvalues)
	for i := range rows {
		rows[i].BHQValue = qvalues[i]
	}
	sort.SliceStable(rows, func(i, k int) bool {
		if rows[i].PValue != rows[k].PValue {
			return rows[i].PValue < rows[k].PValue
		}
		return rows[i].MotifID < rows[k].MotifID
	})
	return EnrichmentResult{
		GeneSetSize: len(set), BackgroundSize: len(universe), BackgroundSource: backgroundSource,
		MotifCount: len(motifs), MultipleTestingScope: "BH across the requested motifs within this gene set",
		Rows: rows, Caveats: []string{
			"Binary promoter hit; multiple sites per promoter are collapsed.",
			"One-sided Fisher exact test; no promoter length or GC matching.",
			"BH is within this gene set; gene-set overlap and motif similarity are not modeled.",
			"An enriched motif is not an enriched individual TF when multiple TFs share the PWM.",
			"Exploratory only; not experimental validation of regulator activity.",
		},
	}, nil
}
