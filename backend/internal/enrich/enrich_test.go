package enrich

import (
	"math"
	"os"
	"testing"
)

func TestHypergeometricPBounds(t *testing.T) {
	// P(X >= k) must decrease as k increases and stay in [0,1].
	prev := 2.0
	for k := 0; k <= 8; k++ {
		p := hypergeometricP(100, 20, 10, k)
		if p < 0 || p > 1 {
			t.Fatalf("p out of range: %v", p)
		}
		if p > prev+1e-12 {
			t.Fatalf("p not monotonic at k=%d: %v > %v", k, p, prev)
		}
		prev = p
	}
	if got := hypergeometricP(100, 20, 10, 0); got != 1 {
		t.Fatalf("P(X>=0) should be 1, got %v", got)
	}
}

func TestBHAdjust(t *testing.T) {
	got := bhAdjust([]float64{0.01, 0.02, 0.03, 0.04})
	for i, v := range got {
		if math.Abs(v-0.04) > 1e-12 {
			t.Fatalf("BH[%d] = %v, want 0.04", i, v)
		}
	}
}

func TestRunGO(t *testing.T) {
	bg := Background{
		Gene2Term: map[string][]string{
			"g1": {"GO:1"}, "g2": {"GO:1"}, "g3": {"GO:1"},
			"g4": {"GO:2"}, "g5": {"GO:2"}, "g6": {"GO:2"}, "g7": {"GO:2"},
		},
		TermName: map[string]string{"GO:1": "term one", "GO:2": "term two"},
		Universe: []string{"g1", "g2", "g3", "g4", "g5", "g6", "g7"},
	}
	res := Run([]string{"g1", "g2", "g3", "g4"}, bg, 1, 0)
	if len(res) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res))
	}
	if res[0].ID != "GO:1" {
		t.Fatalf("GO:1 should rank first, got %s", res[0].ID)
	}
	if res[0].Count != 3 || res[0].GeneRatio != "3/4" {
		t.Fatalf("unexpected GO:1 row: %+v", res[0])
	}
}

func TestParseTBtoolsKEGG(t *testing.T) {
	src := "K14400\tPCF11; pre-mRNA\t03015 mRNA surveillance pathway\tB  09122 Translation\tA09120 Genetic Information Processing\n" +
		"K14400\tPCF11\t03021 Transcription machinery\tB  09182 Protein families\tA09180 Brite Hierarchies\n" +
		"K10046\tGME\t00053 Ascorbate and aldarate metabolism\tB  09101 Carbohydrate metabolism\tA09100 Metabolism\n"
	bg, err := ParseTBtoolsKEGG([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(bg.KO2Pathway["K14400"]) != 2 {
		t.Fatalf("K14400 should have 2 pathways, got %d", len(bg.KO2Pathway["K14400"]))
	}
	if bg.KO2Pathway["K10046"][0].Name != "Ascorbate and aldarate metabolism" {
		t.Fatalf("bad pathway name: %+v", bg.KO2Pathway["K10046"][0])
	}
}

func TestKEGGEnrichment(t *testing.T) {
	bg, err := ParseTBtoolsKEGG([]byte(
		"K1\t\t0010 A\t\tA0\n" +
			"K2\t\t0010 A\t\tA0\n" +
			"K3\t\t0010 A\t\tA0\n" +
			"K4\t\t0020 B\t\tA0\n"))
	if err != nil {
		t.Fatal(err)
	}
	gene2ko := map[string][]string{
		"g1": {"K1"}, "g2": {"K2"}, "g3": {"K3"}, "g4": {"K4"}, "g5": {"K1"},
	}
	kb := bg.BuildBackground(gene2ko)
	res := Run([]string{"g1", "g2"}, kb, 1, 0)
	if len(res) == 0 {
		t.Fatal("expected at least one KEGG row")
	}
	if res[0].ID != "0010" {
		t.Fatalf("expected pathway 0010 first, got %s", res[0].ID)
	}
}

func TestParseTBtoolsKEGGPlantsFile(t *testing.T) {
	path := os.Getenv("KEGG_PLANTS")
	if path == "" {
		t.Skip("set KEGG_PLANTS to run against the real Plants background")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bg, err := ParseTBtoolsKEGG(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(bg.KO2Pathway) == 0 {
		t.Fatal("empty KO2Pathway")
	}
}
