package enrich

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed data/go-basic.obo.gz
var oboGz []byte

// GOTerm is a single GO term's core metadata.
type GOTerm struct {
	ID        string
	Name      string
	Namespace string
}

// GOParent is one outgoing edge child -> parent with a Wang weight.
type GOParent struct {
	ID     string
	Weight float64
}

// GOOntology is the parsed go-basic.obo graph (is_a + part_of).
type GOOntology struct {
	terms   map[string]GOTerm
	parents map[string][]GOParent
	DataVersion string

	svMu    sync.Mutex
	svCache map[string]map[string]float64
}

var (
	goMu    sync.Mutex
	goCache *GOOntology
	goErr   error
)

// LoadGO parses the embedded go-basic.obo once and returns the shared graph.
func LoadGO() (*GOOntology, error) {
	goMu.Lock()
	defer goMu.Unlock()
	if goCache != nil {
		return goCache, nil
	}
	goCache, goErr = loadGOData()
	if goErr != nil {
		return nil, goErr
	}
	return goCache, nil
}

// ReloadGO invalidates the cached ontology so the next LoadGO re-reads from
// disk (used after an update).
func ReloadGO() {
	goMu.Lock()
	goCache = nil
	goErr = nil
	goMu.Unlock()
}

// goDataPath is the user-writable location for an updated go-basic.obo.gz.
func goDataPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "wgcna-dashboard", "data", "go-basic.obo.gz")
}

func loadGOData() (*GOOntology, error) {
	if p := goDataPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if g, err := parseOBO(b); err == nil {
				return g, nil
			}
		}
	}
	return parseOBO(oboGz)
}

func parseOBO(data []byte) (*GOOntology, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	g := &GOOntology{
		terms:   map[string]GOTerm{},
		parents: map[string][]GOParent{},
		svCache: map[string]map[string]float64{},
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	inTerm := false
	obsolete := false
	cur := GOTerm{}
	flush := func() {
		if inTerm && cur.ID != "" && !obsolete {
			g.terms[cur.ID] = cur
		}
	}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "data-version: "):
			if g.DataVersion == "" {
				g.DataVersion = strings.TrimSpace(line[len("data-version: "):])
			}
		case line == "[Term]":
			flush()
			inTerm = true
			obsolete = false
			cur = GOTerm{}
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			flush()
			inTerm = false
		case inTerm && strings.HasPrefix(line, "id: "):
			cur.ID = strings.TrimSpace(line[4:])
		case inTerm && strings.HasPrefix(line, "name: "):
			cur.Name = strings.TrimSpace(line[6:])
		case inTerm && strings.HasPrefix(line, "namespace: "):
			cur.Namespace = strings.TrimSpace(line[11:])
		case inTerm && strings.HasPrefix(line, "is_obsolete: true"):
			obsolete = true
		case inTerm && strings.HasPrefix(line, "is_a: "):
			if p := parseOBORelation(line[6:]); p != "" {
				g.parents[cur.ID] = append(g.parents[cur.ID], GOParent{ID: p, Weight: 0.8})
			}
		case inTerm && strings.HasPrefix(line, "relationship: part_of "):
			if p := parseOBORelation(line[len("relationship: part_of "):]); p != "" {
				g.parents[cur.ID] = append(g.parents[cur.ID], GOParent{ID: p, Weight: 0.6})
			}
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

func parseOBORelation(s string) string {
	if i := strings.IndexAny(s, " \t!"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// TermName returns the GO term name, or the id when unknown.
func (g *GOOntology) TermName(id string) string {
	if t, ok := g.terms[id]; ok {
		return t.Name
	}
	return id
}

// TermNamespace returns the GO namespace (BP/MF/CC) for a term id.
func (g *GOOntology) TermNamespace(id string) string {
	if t, ok := g.terms[id]; ok {
		return t.Namespace
	}
	return ""
}

// sValues computes Wang S-values for every ancestor of id (including itself).
func (g *GOOntology) sValues(id string) map[string]float64 {
	g.svMu.Lock()
	if v, ok := g.svCache[id]; ok {
		g.svMu.Unlock()
		return v
	}
	g.svMu.Unlock()

	res := map[string]float64{id: 1.0}
	stack := []string{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, p := range g.parents[cur] {
			if sv := res[cur] * p.Weight; sv > res[p.ID] {
				res[p.ID] = sv
				stack = append(stack, p.ID)
			}
		}
	}

	g.svMu.Lock()
	g.svCache[id] = res
	g.svMu.Unlock()
	return res
}

// WangSimilarity returns the Wang semantic similarity in [0,1].
func (g *GOOntology) WangSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	sa := g.sValues(a)
	sb := g.sValues(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	var sumA, sumB, sumCommon float64
	for _, v := range sa {
		sumA += v
	}
	for _, v := range sb {
		sumB += v
	}
	for t, va := range sa {
		if vb, ok := sb[t]; ok {
			sumCommon += va + vb
		}
	}
	if sumA+sumB == 0 {
		return 0
	}
	return sumCommon / (sumA + sumB)
}

// TermScore pairs a term id with its significance (smaller = more significant).
type TermScore struct {
	ID string
	P  float64
}

// Simplify greedily removes terms whose Wang similarity to an already-kept
// (more significant) term exceeds cutoff. Terms are processed most-significant
// first. This mirrors clusterProfiler::simplify / rrvgo-style redundancy
// removal.
func (g *GOOntology) Simplify(terms []TermScore, cutoff float64) []TermScore {
	sorted := make([]TermScore, len(terms))
	copy(sorted, terms)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].P < sorted[j].P })

	kept := make([]TermScore, 0, len(sorted))
	for _, t := range sorted {
		redundant := false
		for _, k := range kept {
			if g.WangSimilarity(t.ID, k.ID) > cutoff {
				redundant = true
				break
			}
		}
		if !redundant {
			kept = append(kept, t)
		}
	}
	return kept
}
