// Package enrich implements GO/KEGG over-representation analysis
// (one-tailed hypergeometric test + Benjamini-Hochberg FDR), producing
// clusterProfiler-style result rows. It has no external dependencies.
package enrich

import (
	"math"
	"sort"
	"strconv"
)

// Result is one enrichment row, shaped like clusterProfiler's data.frame.
type Result struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	GeneRatio   string   `json:"gene_ratio"` // "k/n"
	BgRatio     string   `json:"bg_ratio"`   // "K/N"
	Pvalue      float64  `json:"pvalue"`
	PAdjust     float64  `json:"p.adjust"`
	Qvalue      float64  `json:"qvalue"`
	GeneID      []string `json:"gene_id"`
	Count       int      `json:"count"`
}

// Background is a gene -> term mapping with a fixed universe (the set of all
// genes participating in the annotation).
type Background struct {
	Gene2Term map[string][]string
	TermName  map[string]string
	Universe  []string // sorted, unique
}

// Run performs one-tailed over-representation for every term and returns rows
// sorted by adjusted p-value ascending. padjCut keeps rows with Qvalue <= cut
// (use 1 to keep everything); top limits the row count (<=0 means no limit).
func Run(genes []string, bg Background, padjCut float64, top int) []Result {
	uni := make(map[string]bool, len(bg.Universe))
	for _, g := range bg.Universe {
		uni[g] = true
	}

	query := make([]string, 0, len(genes))
	seen := map[string]bool{}
	for _, g := range genes {
		if uni[g] && !seen[g] {
			seen[g] = true
			query = append(query, g)
		}
	}
	n := len(query)
	N := len(bg.Universe)
	if n == 0 || N == 0 {
		return nil
	}

	termGenes := make(map[string]map[string]bool)
	for g, terms := range bg.Gene2Term {
		if !uni[g] {
			continue
		}
		for _, t := range terms {
			m := termGenes[t]
			if m == nil {
				m = map[string]bool{}
				termGenes[t] = m
			}
			m[g] = true
		}
	}

	rows := make([]Result, 0)
	for term, tset := range termGenes {
		K := len(tset)
		k := 0
		for _, g := range query {
			if tset[g] {
				k++
			}
		}
		if k == 0 {
			continue
		}
		p := hypergeometricP(N, K, n, k)
		rows = append(rows, Result{
			ID:          term,
			Description: bg.TermName[term],
			GeneRatio:   ratio(k, n),
			BgRatio:     ratio(K, N),
			Pvalue:      p,
			GeneID:      intersect(query, tset),
			Count:       k,
		})
	}

	pvals := make([]float64, len(rows))
	for i := range rows {
		pvals[i] = rows[i].Pvalue
	}
	adj := bhAdjust(pvals)
	for i := range rows {
		rows[i].PAdjust = adj[i]
		rows[i].Qvalue = adj[i]
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].PAdjust != rows[j].PAdjust {
			return rows[i].PAdjust < rows[j].PAdjust
		}
		return rows[i].Count > rows[j].Count
	})

	if padjCut < 1 {
		out := rows[:0]
		for _, r := range rows {
			if r.Qvalue <= padjCut {
				out = append(out, r)
			}
		}
		rows = out
	}
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	return rows
}

func hypergeometricP(N, K, n, k int) float64 {
	if k > K || k > n || k < 0 {
		return 1
	}
	lo, hi := k, minInt(K, n)
	if lo > hi {
		return 1
	}
	logDen := logComb(N, n)
	var p float64
	for i := lo; i <= hi; i++ {
		if i < 0 || K-i < 0 || n-i < 0 || N-K-n+i < 0 {
			continue
		}
		p += math.Exp(logComb(K, i) + logComb(N-K, n-i) - logDen)
	}
	if p > 1 {
		p = 1
	}
	return p
}

func logComb(n, k int) float64 {
	if k < 0 || k > n {
		return math.Inf(-1)
	}
	if k == 0 || k == n {
		return 0
	}
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

func bhAdjust(p []float64) []float64 {
	n := len(p)
	if n == 0 {
		return nil
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return p[idx[a]] < p[idx[b]] })
	out := make([]float64, n)
	cumMin := 1.0
	for r := n; r >= 1; r-- {
		i := idx[r-1]
		v := p[i] * float64(n) / float64(r)
		if v < cumMin {
			cumMin = v
		}
		if cumMin > 1 {
			cumMin = 1
		}
		out[i] = cumMin
	}
	return out
}

func ratio(k, n int) string {
	return strconv.Itoa(k) + "/" + strconv.Itoa(n)
}

func intersect(query []string, set map[string]bool) []string {
	out := make([]string, 0)
	for _, g := range query {
		if set[g] {
			out = append(out, g)
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
