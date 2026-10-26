// Package literature provides multi-source literature search (Europe PMC,
// PubMed, Semantic Scholar, optional Google Scholar) for the AI navigator.
package literature

import (
	"sort"
	"strings"
	"sync"
)

// Paper is a unified literature record from any source.
type Paper struct {
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Year     int      `json:"year"`
	Journal  string   `json:"journal"`
	PMID     string   `json:"pmid,omitempty"`
	DOI      string   `json:"doi,omitempty"`
	URL      string   `json:"url,omitempty"`
	Abstract string   `json:"abstract,omitempty"`
	Source   string   `json:"source"` // europepmc | pubmed | semantic | scholar
}

// Provider searches one literature source.
type Provider interface {
	Name() string
	Search(query string, limit int) ([]Paper, error)
}

// key returns a dedup key for a paper: PMID first, then DOI, then lower title.
func (p Paper) key() string {
	if p.PMID != "" {
		return "pmid:" + p.PMID
	}
	if p.DOI != "" {
		return "doi:" + strings.ToLower(p.DOI)
	}
	return "title:" + strings.ToLower(strings.TrimSpace(p.Title))
}

// MultiSearch queries all providers in parallel, merges and de-duplicates, and
// returns the top `limit` papers (richer records — with abstract — win ties).
func MultiSearch(providers []Provider, query string, limit int) []Paper {
	if len(providers) == 0 {
		return nil
	}
	type result struct {
		source string
		papers []Paper
		err    error
	}
	ch := make(chan result, len(providers))
	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			papers, err := p.Search(query, limit)
			ch <- result{p.Name(), papers, err}
		}(p)
	}
	wg.Wait()
	close(ch)

	merged := map[string]Paper{}
	var order []string
	for r := range ch {
		if r.err != nil || len(r.papers) == 0 {
			continue
		}
		for _, p := range r.papers {
			k := p.key()
			if existing, ok := merged[k]; ok {
				// keep the record with the most complete metadata
				if (existing.Abstract == "" && p.Abstract != "") ||
					(existing.Title == "" && p.Title != "") ||
					(existing.PMID == "" && p.PMID != "") {
					merged[k] = p
				}
				continue
			}
			merged[k] = p
			order = append(order, k)
		}
	}

	out := make([]Paper, 0, len(order))
	for _, k := range order {
		out = append(out, merged[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year > out[j].Year
		}
		return out[i].Title < out[j].Title
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
