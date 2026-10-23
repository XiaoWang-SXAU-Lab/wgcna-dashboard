package enrich

import (
	"math"
	"testing"
)

func TestLoadGO(t *testing.T) {
	g, err := LoadGO()
	if err != nil {
		t.Fatalf("LoadGO: %v", err)
	}
	// go-basic.obo keeps obsolete stubs (~10k); the usable graph is ~38k terms.
	if len(g.terms) < 35000 {
		t.Fatalf("too few GO terms: %d", len(g.terms))
	}
	if g.TermName("GO:0008150") != "biological_process" {
		t.Fatalf("GO:0008150 name = %q", g.TermName("GO:0008150"))
	}
	if g.TermNamespace("GO:0008150") != "biological_process" {
		t.Fatalf("GO:0008150 namespace = %q", g.TermNamespace("GO:0008150"))
	}
	if len(g.parents) == 0 {
		t.Fatal("no is_a/part_of edges parsed")
	}
}

func TestWangSimilarity(t *testing.T) {
	g, err := LoadGO()
	if err != nil {
		t.Fatal(err)
	}
	if s := g.WangSimilarity("GO:0008150", "GO:0008150"); s != 1 {
		t.Fatalf("identity similarity = %v, want 1", s)
	}
	// Molecular function and biological process roots are disconnected.
	if s := g.WangSimilarity("GO:0003674", "GO:0008150"); s != 0 {
		t.Fatalf("cross-root similarity = %v, want 0", s)
	}
	// Symmetry.
	a, b := g.WangSimilarity("GO:0000009", "GO:0016757"), g.WangSimilarity("GO:0016757", "GO:0000009")
	if math.Abs(a-b) > 1e-12 {
		t.Fatalf("similarity not symmetric: %v vs %v", a, b)
	}
}

func TestSimplifyKeepsMostSignificant(t *testing.T) {
	g, err := LoadGO()
	if err != nil {
		t.Fatal(err)
	}
	terms := []TermScore{{ID: "GO:0008150", P: 0.01}, {ID: "GO:0008150", P: 0.02}}
	got := g.Simplify(terms, 0.7)
	if len(got) != 1 {
		t.Fatalf("simplify kept %d terms, want 1", len(got))
	}
	if got[0].P != 0.01 {
		t.Fatalf("kept term P = %v, want 0.01", got[0].P)
	}
}
