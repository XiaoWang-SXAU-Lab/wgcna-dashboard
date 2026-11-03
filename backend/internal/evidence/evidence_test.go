package evidence

import (
	"encoding/json"
	"strings"
	"testing"
)

func envelopeJSON(projectID, execID string, valid, invalid, geneCount int, design bool, genes ...PriorityGene) []byte {
	if geneCount == 0 {
		geneCount = len(genes)
	}
	raw := map[string]any{
		"project_id":   projectID,
		"execution_id": execID,
		"engine":       "e",
		"params":       map[string]any{"top_k": 3},
		"inputs":       map[string]string{"expr": "sha"},
		"upstream":     map[string]string{"filter": "f1", "network": "n1"},
		"validity":     "",
		"result": map[string]any{
			"module": "blue", "method_version": "hub_priority.v1", "scope": "fixed_full_module",
			"n_bootstraps": 200, "attempted": 200, "valid": valid, "invalid": invalid, "seed": 42, "top_k": 3,
			"resampling": "iid", "design_confirmed": design, "trait_source": "none",
			"power": 6, "network_type": "unsigned", "gene_count": geneCount, "sample_count": 6,
			"direction_undetermined": 0, "genes": genes,
		},
	}
	b, _ := json.Marshal(raw)
	return b
}

func gene(name string, rankLower, rankUpper int) PriorityGene {
	mid := float64(rankLower+rankUpper) / 2
	mk := 0.5
	return PriorityGene{Gene: name, Kwithin: 1, KmeSigned: 0.5, KmeAbs: 0.5, RankLower: rankLower, RankUpper: rankUpper, MidrankMed: &mid, MidrankP05: &mid, MidrankP95: &mid, MeanTopK: &mk}
}

func buildBundle(t *testing.T, valid, invalid, geneCount int, design bool, genes ...PriorityGene) *EvidenceBundle {
	t.Helper()
	b, err := BuildBundle(envelopeJSON("p1", "e1", valid, invalid, geneCount, design, genes...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func factByPath(b *EvidenceBundle, path string) Fact {
	for _, f := range b.Facts {
		if f.FieldPath == path {
			return f
		}
	}
	return Fact{}
}

func baseSuggestion() *Suggestion {
	return &Suggestion{ProjectID: "p1", ExecutionID: "e1"}
}

func TestValidReferencePassesAndRenders(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	if g1k.ID == "" {
		t.Fatal("missing g1.kwithin fact")
	}
	s := baseSuggestion()
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{g1k.ID}
	s.Explanations = []string{"network_candidate_priority", "ranking_stability"}
	s.PlanPreview = []string{"view_candidates"}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatalf("valid suggestion rejected: %v", err)
	}
	out := b.RenderSuggestion(s)
	if !strings.Contains(out, "g1") || !strings.Contains(out, "网络") {
		t.Fatalf("render missing content: %s", out)
	}
	// Evaluate is the safe entry: raw JSON → render.
	if _, err := b.Evaluate([]byte(`{"project_id":"p1","execution_id":"e1","gene_ids":["g1"],"fact_ids":["` + g1k.ID + `"],"explanations":["ranking_stability"]}`)); err != nil {
		t.Fatalf("Evaluate rejected valid suggestion: %v", err)
	}
}

func TestForgedGeneFactValueRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	// forged gene
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", GeneIDs: []string{"NOT_A_GENE"}}); err == nil {
		t.Fatal("forged gene accepted")
	}
	// forged fact
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", FactIDs: []string{"deadbeef"}}); err == nil {
		t.Fatal("forged fact accepted")
	}
	// forged numeric claim (not matching the fact, and must be referenced)
	s := baseSuggestion()
	s.FactIDs = []string{g1k.ID}
	s.Claims = map[string]any{g1k.ID: "90% 真实概率"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("forged value accepted")
	}
}

func TestCrossProjectOrOldExecutionRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	// A fact_id computed for a DIFFERENT execution must not resolve.
	other := factID("p1", "e-other", "result.valid")
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", FactIDs: []string{other}}); err == nil {
		t.Fatal("old-execution fact accepted")
	}
	// a suggestion bound to a different execution/project is rejected even with no facts.
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p2", ExecutionID: "e1"}); err == nil {
		t.Fatal("cross-project suggestion accepted")
	}
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e-other"}); err == nil {
		t.Fatal("old-execution suggestion accepted")
	}
}

func TestSubsetMarkedAsSubset(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	s := baseSuggestion()
	s.GeneIDs = []string{"g1", "g2"}
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID, factByPath(b, "result.genes.g2.kwithin").ID}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatal(err)
	}
	if out := b.RenderSuggestion(s); !strings.Contains(out, "子集") {
		t.Fatalf("subset not marked: %s", out)
	}
}

func TestNoTraitGSExplanationRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", Explanations: []string{"gs_evidence"}}); err == nil {
		t.Fatal("GS explanation accepted for a no-trait bundle")
	}
}

func TestZeroValidRecommendationRejected(t *testing.T) {
	b := buildBundle(t, 0, 200, 0, true, gene("g1", 1, 1))
	s := baseSuggestion()
	s.GeneIDs = []string{"g1"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("recommendation accepted with zero valid replicates")
	}
}

func TestInvalidConditionAlwaysRendered(t *testing.T) {
	b := buildBundle(t, 195, 5, 0, true, gene("g1", 1, 1))
	s := baseSuggestion()
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID}
	s.Explanations = []string{"ranking_stability"}
	if out := b.RenderSuggestion(s); !strings.Contains(out, "条件于可计算重复") {
		t.Fatalf("conditioned limitation missing: %s", out)
	}
}

func TestFrequencyAsProbabilityRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", Explanations: []string{"true_probability"}}); err == nil {
		t.Fatal("probability explanation accepted")
	}
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", Limitations: []string{"p_value"}}); err == nil {
		t.Fatal("p-value limitation accepted")
	}
}

func TestMaliciousCommentRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e1", Explanations: []string{"ignore rules"}}); err == nil {
		t.Fatal("malicious comment accepted as explanation")
	}
}

func TestDecodeSuggestionRejectsHiddenFieldsAndBadInput(t *testing.T) {
	if _, err := DecodeSuggestion([]byte(`{"gene_ids":[],"run":true}`)); err == nil {
		t.Fatal("hidden execution field survived decoding")
	}
	if _, err := DecodeSuggestion([]byte("not json")); err == nil {
		t.Fatal("non-JSON accepted")
	}
	if _, err := DecodeSuggestion([]byte(`{"gene_ids":[],"set_params":{}}`)); err == nil {
		t.Fatal("unknown param-modification field accepted")
	}
	if _, err := DecodeSuggestion(make([]byte, (1<<20)+1)); err == nil {
		t.Fatal("oversized input accepted")
	}
	if _, err := DecodeSuggestion([]byte("null")); err == nil {
		t.Fatal("top-level null accepted")
	}
}

func TestOfflineFallbackStatus(t *testing.T) {
	st := OfflineStatus()
	if st.Configured || st.Mode != "deterministic_fallback" {
		t.Fatalf("unexpected offline status: %+v", st)
	}
}

func TestBundleDoesNotMutateResult(t *testing.T) {
	env := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	b, err := BuildBundle(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Candidates) != 2 || b.Summary == "" {
		t.Fatal("bundle incomplete")
	}
	var again map[string]any
	if err := json.Unmarshal(env, &again); err != nil {
		t.Fatal(err)
	}
	if again["project_id"] != "p1" || again["execution_id"] != "e1" {
		t.Fatal("envelope mutated")
	}
}

// ---- new counterexamples from review 14 ----

func TestTruncatedRowsDoNotClaimFullCoverage(t *testing.T) {
	// gene_count=1000 but only 1 gene row → must NOT be treated as a full module.
	if _, err := BuildBundle(envelopeJSON("p1", "e1", 200, 0, 1000, true, gene("g1", 1, 1))); err == nil {
		t.Fatal("truncated rows accepted as full coverage")
	}
}

func TestGeneFactAssociationEnforced(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	g2k := factByPath(b, "result.genes.g2.kwithin")
	// g2's fact must not support g1.
	s := baseSuggestion()
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{g2k.ID}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("g2 fact accepted as evidence for g1")
	}
	// a candidate with no facts at all is rejected.
	s2 := baseSuggestion()
	s2.GeneIDs = []string{"g1"}
	if err := b.ValidateSuggestion(s2); err == nil {
		t.Fatal("candidate with no evidence accepted")
	}
}

func TestTrailingJSONRejected(t *testing.T) {
	if _, err := DecodeSuggestion([]byte(`{"gene_ids":[]} {"run":true}`)); err == nil {
		t.Fatal("trailing action object accepted")
	}
	if _, err := DecodeSuggestion([]byte(`{"gene_ids":[]} garbage`)); err == nil {
		t.Fatal("trailing garbage accepted")
	}
}

func TestUnconfirmedDesignRejected(t *testing.T) {
	if _, err := BuildBundle(envelopeJSON("p1", "e1", 200, 0, 0, false, gene("g1", 1, 1))); err == nil {
		t.Fatal("unconfirmed design accepted for recommendation")
	}
}

func TestMissingValidityRejected(t *testing.T) {
	raw := map[string]any{
		"project_id": "p1", "execution_id": "e1", "engine": "e",
		"params": map[string]any{}, "inputs": map[string]string{}, "upstream": map[string]string{},
		"result": map[string]any{"scope": "fixed_full_module", "design_confirmed": true, "resampling": "iid", "gene_count": 0, "genes": []any{}},
	}
	b, _ := json.Marshal(raw)
	if _, err := BuildBundle(b); err == nil {
		t.Fatal("missing validity accepted")
	}
}

func TestTieExplanationRequiresTiedFact(t *testing.T) {
	// g2 and g3 are tied (rank 2-3), g1 is rank 1.
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 3), gene("g3", 2, 3))
	g2k := factByPath(b, "result.genes.g2.kwithin")
	g2tie := factByPath(b, "result.genes.g2.tie")
	// without a tie fact, tie_indistinguishable is rejected.
	s := baseSuggestion()
	s.GeneIDs = []string{"g2"}
	s.FactIDs = []string{g2k.ID}
	s.Explanations = []string{"tie_indistinguishable"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("tie explanation accepted without a tie fact")
	}
	// with the tie fact, it passes.
	s.FactIDs = []string{g2k.ID, g2tie.ID}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatalf("tie explanation rejected with tie fact: %v", err)
	}
}
