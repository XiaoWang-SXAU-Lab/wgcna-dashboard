package evidence

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func envelopeJSON(projectID, execID string, valid, invalid, geneCount int, design bool, genes ...PriorityGene) []byte {
	if geneCount == 0 {
		geneCount = len(genes)
	}
	topK := 3
	if geneCount < topK {
		topK = geneCount
	}
	// Keep the fixture internally consistent with the production ranking:
	// kWithin decreases with rank, and each gene's mean_top_k is its top-K
	// membership weight for the fixture's single rank round.
	for i := range genes {
		genes[i].Kwithin = float64(geneCount - genes[i].RankLower + 1)
		if genes[i].MeanTopK != nil {
			w := fixtureMeanTopK(genes[i].RankLower, genes[i].RankUpper, topK)
			genes[i].MeanTopK = &w
		}
	}
	invalidReasons := []map[string]any{}
	if invalid > 0 {
		invalidReasons = append(invalidReasons, map[string]any{"reason": "invalid", "count": invalid})
	}
	raw := map[string]any{
		"project_id":   projectID,
		"step":         "hub_priority",
		"execution_id": execID,
		"engine":       "e",
		"params":       map[string]any{"select_module": "blue", "top_k": topK},
		"inputs":       map[string]string{"expr": "sha"},
		"upstream":     map[string]string{"filter": "f1", "network": "n1"},
		"validity":     "",
		"result": map[string]any{
			"module": "blue", "method_version": "hub_priority.v1", "scope": "fixed_full_module",
			"n_bootstraps": 200, "attempted": 200, "valid": valid, "invalid": invalid,
			"invalid_reasons": invalidReasons, "seed": 42, "top_k": topK, "tie_tol": 1e-9,
			"resampling": "iid", "strata_column": "", "stratum_count": 0, "strata_counts": []int{},
			"strata_fingerprint": "10000000000000000001", "gene_fingerprint": "10000000000000000002",
			"sample_fingerprint": "10000000000000000003",
			"resource_note":      "engineering budget: g²×B ≤ 1e9 (test fixture)",
			"design_confirmed":   design, "trait_source": "none",
			"power": 6, "network_type": "unsigned", "gene_count": geneCount, "sample_count": 6,
			"limited": invalid > 0, "direction_undetermined": 0, "genes": genes,
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

// fixtureMeanTopK mirrors rank_round's membership weight for the fixture's
// single round: 1 entirely inside top_k, 0 entirely outside, otherwise the
// fraction of the tie group inside top_k.
func fixtureMeanTopK(rankLower, rankUpper, topK int) float64 {
	if rankUpper <= topK {
		return 1
	}
	if rankLower > topK {
		return 0
	}
	return float64(topK-rankLower+1) / float64(rankUpper-rankLower+1)
}

func geneNoValid(name string, rankLower, rankUpper int) PriorityGene {
	return PriorityGene{Gene: name, Kwithin: 1, KmeSigned: 0.5, KmeAbs: 0.5, RankLower: rankLower, RankUpper: rankUpper}
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

func baseSuggestion(b *EvidenceBundle) *Suggestion {
	return &Suggestion{ProjectID: b.ProjectID, ExecutionID: b.ExecutionID, Provenance: b.Provenance}
}

func suggestionJSON(t *testing.T, s *Suggestion) []byte {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mutateEnvelope(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustResult(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	result, ok := value["result"].(map[string]any)
	if !ok {
		t.Fatal("result is not an object")
	}
	return result
}

func mustGenes(t *testing.T, result map[string]any) []any {
	t.Helper()
	genes, ok := result["genes"].([]any)
	if !ok {
		t.Fatal("genes is not an array")
	}
	return genes
}

func TestValidReferencePassesAndRenders(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	g1mid := factByPath(b, "result.genes.g1.midrank_median")
	if g1k.ID == "" || g1mid.ID == "" {
		t.Fatal("missing g1 facts")
	}
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{g1k.ID, g1mid.ID}
	s.Explanations = []string{"network_candidate_priority", "ranking_stability"}
	s.PlanPreview = []string{"view_candidates"}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatalf("valid suggestion rejected: %v", err)
	}
	out := b.renderSuggestion(s)
	if !strings.Contains(out, "g1") || !strings.Contains(out, "网络") {
		t.Fatalf("render missing content: %s", out)
	}
	// Evaluate is the safe entry: raw JSON → render.
	if _, err := b.Evaluate(suggestionJSON(t, s)); err != nil {
		t.Fatalf("Evaluate rejected valid suggestion: %v", err)
	}
}

func TestForgedGeneFactValueRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	// forged gene
	s := baseSuggestion(b)
	s.GeneIDs = []string{"NOT_A_GENE"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("forged gene accepted")
	}
	// forged fact
	s = baseSuggestion(b)
	s.FactIDs = []string{"deadbeef"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("forged fact accepted")
	}
	// forged numeric claim (not matching the fact, and must be referenced)
	s = baseSuggestion(b)
	s.FactIDs = []string{g1k.ID}
	s.Claims = map[string]any{g1k.ID: "90% 真实概率"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("forged value accepted")
	}
}

func TestCrossProjectOrOldExecutionRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	// A fact_id computed for a DIFFERENT provenance must not resolve.
	other := factID("other-provenance", "result.valid")
	s := baseSuggestion(b)
	s.FactIDs = []string{other}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("old-provenance fact accepted")
	}
	// a suggestion bound to a different execution/project is rejected even with no facts.
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p2", ExecutionID: "e1", Provenance: b.Provenance}); err == nil {
		t.Fatal("cross-project suggestion accepted")
	}
	if err := b.ValidateSuggestion(&Suggestion{ProjectID: "p1", ExecutionID: "e-other", Provenance: b.Provenance}); err == nil {
		t.Fatal("old-execution suggestion accepted")
	}
	s = baseSuggestion(b)
	s.Provenance = "wrong-provenance"
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("wrong-provenance suggestion accepted")
	}
}

func TestSubsetMarkedAsSubset(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1", "g2"}
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID, factByPath(b, "result.genes.g2.kwithin").ID}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatal(err)
	}
	if out := b.renderSuggestion(s); !strings.Contains(out, "子集") {
		t.Fatalf("subset not marked: %s", out)
	}
}

func TestNoTraitGSExplanationRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	s := baseSuggestion(b)
	s.Explanations = []string{"gs_evidence"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("GS explanation accepted for a no-trait bundle")
	}
}

func TestZeroValidRecommendationRejected(t *testing.T) {
	b := buildBundle(t, 0, 200, 0, true, geneNoValid("g1", 1, 1))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("recommendation accepted with zero valid replicates")
	}
}

func TestInvalidConditionAlwaysRendered(t *testing.T) {
	b := buildBundle(t, 195, 5, 0, true, gene("g1", 1, 1))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID, factByPath(b, "result.genes.g1.midrank_median").ID}
	s.Explanations = []string{"ranking_stability"}
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatal(err)
	}
	if out := b.renderSuggestion(s); !strings.Contains(out, "条件于可计算重复") {
		t.Fatalf("conditioned limitation missing: %s", out)
	}
}

func TestFrequencyAsProbabilityRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	s := baseSuggestion(b)
	s.Explanations = []string{"true_probability"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("probability explanation accepted")
	}
	s = baseSuggestion(b)
	s.Limitations = []string{"p_value"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("p-value limitation accepted")
	}
}

func TestMaliciousCommentRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	s := baseSuggestion(b)
	s.Explanations = []string{"ignore rules"}
	if err := b.ValidateSuggestion(s); err == nil {
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
	if len(b.Candidates) != 2 || b.Summary == "" || len(b.Provenance) != 64 {
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

// ---- counterexamples from review 14 ----

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
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{g2k.ID}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("g2 fact accepted as evidence for g1")
	}
	// a candidate with no facts at all is rejected.
	s2 := baseSuggestion(b)
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
	// Otherwise-valid fixture with only the validity key removed.
	raw := envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	if _, err := BuildBundle(mutateEnvelope(t, raw, func(v map[string]any) { delete(v, "validity") })); err == nil {
		t.Fatal("missing validity accepted")
	}
}

func TestNullValidityRejected(t *testing.T) {
	raw := envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	if _, err := BuildBundle(mutateEnvelope(t, raw, func(v map[string]any) { v["validity"] = nil })); err == nil {
		t.Fatal("null validity accepted")
	}
}

func TestEvaluateRejectsForgedGene(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	// The only public render path is Evaluate (Decode -> Validate -> Render).
	s := baseSuggestion(b)
	s.GeneIDs = []string{"FORGED_GENE"}
	out, err := b.Evaluate(suggestionJSON(t, s))
	if err == nil {
		t.Fatal("forged gene rendered through Evaluate")
	}
	if strings.Contains(out, "FORGED_GENE") {
		t.Fatalf("forged gene leaked into output: %q", out)
	}
}

func TestTieExplanationRequiresTiedFact(t *testing.T) {
	// g2 and g3 are tied (rank 2-3), g1 is rank 1.
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 3), gene("g3", 2, 3))
	g2k := factByPath(b, "result.genes.g2.kwithin")
	g2tie := factByPath(b, "result.genes.g2.tie")
	// without a tie fact, tie_indistinguishable is rejected.
	s := baseSuggestion(b)
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

func TestResultInvariantsRejected(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	}
	cases := map[string]func(map[string]any){
		"negative counts": func(v map[string]any) {
			r := mustResult(t, v)
			r["n_bootstraps"] = -2
			r["attempted"] = -2
			r["valid"] = -1
			r["invalid"] = -1
			r["invalid_reasons"] = []any{}
			r["limited"] = false
		},
		"top_k exceeds gene_count": func(v map[string]any) {
			mustResult(t, v)["top_k"] = 3
		},
		"quantiles out of range": func(v map[string]any) {
			g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
			g["midrank_p05"] = 999
			g["midrank_p95"] = -50
		},
		"invalid reason sum": func(v map[string]any) {
			r := mustResult(t, v)
			r["valid"] = 195
			r["invalid"] = 5
			r["limited"] = true
			r["invalid_reasons"] = []any{map[string]any{"reason": "x", "count": -100}}
		},
		"zero valid with non-null statistics": func(v map[string]any) {
			r := mustResult(t, v)
			r["valid"] = 0
			r["invalid"] = 200
			r["limited"] = true
			r["invalid_reasons"] = []any{map[string]any{"reason": "invalid", "count": 200}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildBundle(mutateEnvelope(t, base(), mutate)); err == nil {
				t.Fatal("invalid result accepted")
			}
		})
	}
}

func TestBuildBundleInputBudget(t *testing.T) {
	valid := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	big := make([]byte, maxEnvelopeBytes+1)
	for i := range big {
		big[i] = ' '
	}
	copy(big, valid)
	if _, err := BuildBundle(big); err == nil {
		t.Fatal("envelope over the explicit size budget accepted")
	}
}

func TestProductionMetadataRequiredAndFactored(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	}
	cases := map[string]func(map[string]any){
		"missing strata fingerprint": func(v map[string]any) { delete(mustResult(t, v), "strata_fingerprint") },
		"missing gene fingerprint":   func(v map[string]any) { delete(mustResult(t, v), "gene_fingerprint") },
		"missing sample fingerprint": func(v map[string]any) { delete(mustResult(t, v), "sample_fingerprint") },
		"non-decimal fingerprint":    func(v map[string]any) { mustResult(t, v)["gene_fingerprint"] = "abc" },
		"blank resource note":        func(v map[string]any) { mustResult(t, v)["resource_note"] = " " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildBundle(mutateEnvelope(t, base(), mutate)); err == nil {
				t.Fatalf("%s accepted", name)
			}
		})
	}
	b, err := BuildBundle(base())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"result.strata_fingerprint",
		"result.gene_fingerprint",
		"result.sample_fingerprint",
		"result.resource_note",
	} {
		if factByPath(b, path).ID == "" {
			t.Errorf("missing fact %s", path)
		}
	}
}

func TestMeanTopKSumInvariant(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true,
			gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	}
	b, err := BuildBundle(base())
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, f := range b.Facts {
		if f.Semantics == "mean_top_k" {
			v, ok := f.Value.(float64)
			if !ok {
				t.Fatalf("mean_top_k fact is %T, want float64", f.Value)
			}
			sum += v
		}
	}
	if math.Abs(sum-3) > 1e-9 {
		t.Fatalf("fixture mean_top_k sum=%v, want 3", sum)
	}
	// The production invariant is sum(mean_top_k)=top_k: a single altered value
	// must not be accepted as a valid full module.
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
		g["mean_top_k"] = 0.0
	})); err == nil {
		t.Fatal("mean_top_k sum mismatch accepted")
	}
	// A tie group straddling top_k contributes its fractional membership:
	// 1 + 0.5 + 0.5 = 2 for top_k=2.
	tie := mutateEnvelope(t, envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 3), gene("g3", 2, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 2
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.5
		gs[2].(map[string]any)["mean_top_k"] = 0.5
	})
	if _, err := BuildBundle(tie); err != nil {
		t.Fatalf("valid fractional top-K fixture rejected: %v", err)
	}
}

func TestZeroValidNullStatisticsAccepted(t *testing.T) {
	b, err := BuildBundle(envelopeJSON("p1", "e1", 0, 200, 0, true, geneNoValid("g1", 1, 1)))
	if err != nil {
		t.Fatalf("valid zero-valid bundle rejected: %v", err)
	}
	if b.Valid != 0 {
		t.Fatalf("valid=%d, want 0", b.Valid)
	}
}

func TestStrataInvariants(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	}
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		mustResult(t, v)["strata_column"] = "batch"
	})); err == nil {
		t.Fatal("iid with strata fields accepted")
	}
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		mustResult(t, v)["resampling"] = "stratified"
	})); err == nil {
		t.Fatal("stratified without strata_column accepted")
	}
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		r := mustResult(t, v)
		r["resampling"] = "stratified"
		r["strata_column"] = "batch"
		r["stratum_count"] = 2
		r["strata_counts"] = []int{2, 2}
	})); err == nil {
		t.Fatal("strata counts not matching sample_count accepted")
	}
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		r := mustResult(t, v)
		r["resampling"] = "stratified"
		r["strata_column"] = "batch"
		r["stratum_count"] = 2
		r["strata_counts"] = []int{3, 3}
	})); err != nil {
		t.Fatalf("valid stratified result rejected: %v", err)
	}
}

func TestMethodDesignTieInvariantsRejected(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	}
	cases := map[string]func(map[string]any){
		"empty method_version":   func(v map[string]any) { mustResult(t, v)["method_version"] = "" },
		"unknown method_version": func(v map[string]any) { mustResult(t, v)["method_version"] = "hub_priority.v2" },
		"unknown trait_source":   func(v map[string]any) { mustResult(t, v)["trait_source"] = "uploaded" },
		"empty trait_source":     func(v map[string]any) { mustResult(t, v)["trait_source"] = "" },
		"direction exceeds valid": func(v map[string]any) {
			mustResult(t, v)["direction_undetermined"] = 201
		},
		"limited mismatch": func(v map[string]any) { mustResult(t, v)["limited"] = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildBundle(mutateEnvelope(t, base(), mutate)); err == nil {
				t.Fatal("invalid result accepted")
			}
		})
	}
	// A single gene cannot claim a two-rank tie group.
	badTie := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 2), gene("g2", 3, 3), gene("g3", 3, 3))
	if _, err := BuildBundle(badTie); err == nil {
		t.Fatal("incomplete tie group accepted")
	}
}

func TestProvenanceBindsFactIDsAndSuggestions(t *testing.T) {
	base := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	b1, err := BuildBundle(base)
	if err != nil {
		t.Fatal(err)
	}
	originalID := factByPath(b1, "result.valid").ID
	variants := map[string]func(map[string]any){
		"engine":   func(v map[string]any) { v["engine"] = "e2" },
		"params":   func(v map[string]any) { v["params"] = map[string]any{"select_module": "blue", "top_k": 2} },
		"inputs":   func(v map[string]any) { v["inputs"] = map[string]string{"expr": "other"} },
		"upstream": func(v map[string]any) { v["upstream"] = map[string]string{"filter": "f2", "network": "n1"} },
		"result":   func(v map[string]any) { mustResult(t, v)["module"] = "red" },
	}
	for name, mutate := range variants {
		t.Run(name, func(t *testing.T) {
			b2, err := BuildBundle(mutateEnvelope(t, base, mutate))
			if err != nil {
				t.Fatalf("%s variant rejected: %v", name, err)
			}
			if got := factByPath(b2, "result.valid").ID; got == originalID {
				t.Errorf("%s did not change fact ID", name)
			}
			s := baseSuggestion(b1)
			s.FactIDs = []string{originalID}
			if err := b2.ValidateSuggestion(s); err == nil {
				t.Errorf("%s: suggestion from another provenance accepted", name)
			}
		})
	}
}

func TestSuggestionProvenanceRequired(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1))
	s := baseSuggestion(b)
	s.Provenance = ""
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("suggestion without provenance accepted")
	}
	s.Provenance = "wrong"
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("suggestion with wrong provenance accepted")
	}
	s.Provenance = b.Provenance
	if err := b.ValidateSuggestion(s); err != nil {
		t.Fatalf("valid empty suggestion rejected: %v", err)
	}
}

func TestExplanationsRequireSameGeneFacts(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	g1mid := factByPath(b, "result.genes.g1.midrank_median")
	g1mean := factByPath(b, "result.genes.g1.mean_top_k")
	g2mid := factByPath(b, "result.genes.g2.midrank_median")

	ranking := func(ids ...string) *Suggestion {
		s := baseSuggestion(b)
		s.GeneIDs = []string{"g1"}
		s.FactIDs = ids
		s.Explanations = []string{"ranking_stability"}
		return s
	}
	if err := b.ValidateSuggestion(ranking(g1k.ID)); err == nil {
		t.Fatal("ranking_stability accepted without a midrank fact")
	}
	if err := b.ValidateSuggestion(ranking(g1k.ID, g2mid.ID)); err == nil {
		t.Fatal("ranking_stability accepted with another gene's midrank fact")
	}
	if err := b.ValidateSuggestion(ranking(g1k.ID, g1mid.ID)); err != nil {
		t.Fatalf("ranking_stability rejected with own midrank fact: %v", err)
	}

	frequency := func(ids ...string) *Suggestion {
		s := baseSuggestion(b)
		s.GeneIDs = []string{"g1"}
		s.FactIDs = ids
		s.Explanations = []string{"top_k_frequency"}
		return s
	}
	if err := b.ValidateSuggestion(frequency(g1k.ID)); err == nil {
		t.Fatal("top_k_frequency accepted without mean_top_k")
	}
	if err := b.ValidateSuggestion(frequency(g1k.ID, g1mean.ID)); err != nil {
		t.Fatalf("top_k_frequency rejected with own mean_top_k: %v", err)
	}

	tied := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 3), gene("g3", 2, 3))
	g2k := factByPath(tied, "result.genes.g2.kwithin")
	g2tie := factByPath(tied, "result.genes.g2.tie")
	g3tie := factByPath(tied, "result.genes.g3.tie")
	tieSuggestion := func(ids ...string) *Suggestion {
		s := baseSuggestion(tied)
		s.GeneIDs = []string{"g2"}
		s.FactIDs = ids
		s.Explanations = []string{"tie_indistinguishable"}
		return s
	}
	if err := tied.ValidateSuggestion(tieSuggestion(g2k.ID, g3tie.ID)); err == nil {
		t.Fatal("tie_indistinguishable accepted from another gene's tie fact")
	}
	if err := tied.ValidateSuggestion(tieSuggestion(g2k.ID, g2tie.ID)); err != nil {
		t.Fatalf("tie_indistinguishable rejected with own tie fact: %v", err)
	}
}

func TestExportedFieldsCannotForgeValidation(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	g1k := factByPath(b, "result.genes.g1.kwithin")
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	s.FactIDs = []string{g1k.ID}

	b.Summary = "FORGED TRUSTED SUMMARY"
	b.Limitations = nil
	b.Valid = 0
	b.Candidates = nil
	out, err := b.Evaluate(suggestionJSON(t, s))
	if err != nil {
		t.Fatalf("sealed validation should ignore snapshot mutation: %v", err)
	}
	if strings.Contains(out, "FORGED") {
		t.Fatalf("forged summary rendered: %s", out)
	}
	if !strings.Contains(out, "局限") {
		t.Fatalf("deterministic limitations missing: %s", out)
	}

	var zero EvidenceBundle
	if _, err := zero.Evaluate([]byte(`{"project_id":"p1","execution_id":"e1","provenance":"x"}`)); err == nil {
		t.Fatal("zero-value bundle accepted")
	}
}

func TestExportedValidCannotForgeZeroValidBundle(t *testing.T) {
	b := buildBundle(t, 0, 200, 0, true, geneNoValid("g1", 1, 1))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1"}
	b.Valid = 1
	b.Summary = "FORGED"
	if out, err := b.Evaluate(suggestionJSON(t, s)); err == nil {
		t.Fatalf("forged zero-valid bundle accepted: %s", out)
	}
}

func TestKwithinRankConsistency(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true,
			gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	}
	if _, err := BuildBundle(base()); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	// kWithin order no longer matches the reported ranks.
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		mustGenes(t, mustResult(t, v))[0].(map[string]any)["kwithin"] = 0.0
	})); err == nil {
		t.Fatal("rank bounds inconsistent with kWithin accepted")
	}
	// Equal kWithin values form a tie group; distinct ranks cannot claim them.
	if _, err := BuildBundle(mutateEnvelope(t, base(), func(v map[string]any) {
		gs := mustGenes(t, mustResult(t, v))
		gs[2].(map[string]any)["kwithin"] = gs[1].(map[string]any)["kwithin"]
	})); err == nil {
		t.Fatal("tie group not reflected in rank bounds accepted")
	}
}

func TestSingleValidQuantilesEqual(t *testing.T) {
	oneValid := mutateEnvelope(t, envelopeJSON("p1", "e1", 1, 199, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 1
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.0
	})
	if _, err := BuildBundle(oneValid); err != nil {
		t.Fatalf("valid single-round fixture rejected: %v", err)
	}
	// One valid round means P05, median and P95 are the same midrank.
	if _, err := BuildBundle(mutateEnvelope(t, oneValid, func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[1].(map[string]any)
		g["midrank_p05"] = 1.5
		g["midrank_median"] = 2.0
		g["midrank_p95"] = 2.0
	})); err == nil {
		t.Fatal("inconsistent quantiles with one valid round accepted")
	}
}

func TestSingleRoundRealizability(t *testing.T) {
	oneRound := mutateEnvelope(t, envelopeJSON("p1", "e1", 1, 199, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 1
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.0
		gs[2].(map[string]any)["mean_top_k"] = 0.0
	})
	if _, err := BuildBundle(oneRound); err != nil {
		t.Fatalf("valid single-round fixture rejected: %v", err)
	}
	// A legal single-round tie: midranks 1 and 2.5, membership 1 and 0.5.
	tie := mutateEnvelope(t, envelopeJSON("p1", "e1", 1, 199, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 3), gene("g3", 2, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 2
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.5
		gs[2].(map[string]any)["mean_top_k"] = 0.5
	})
	if _, err := BuildBundle(tie); err != nil {
		t.Fatalf("legal single-round tie rejected: %v", err)
	}
	cases := map[string]struct {
		mids  []float64
		means []float64
	}{
		"non-half midrank":     {[]float64{1.25, 1.75, 3}, []float64{0.5, 0.5, 0}},
		"impossible partition": {[]float64{1.5, 1.5, 2.5}, []float64{0.5, 0.5, 0}},
		"membership mismatch":  {[]float64{1, 2.5, 2.5}, []float64{0.5, 0.5, 0}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			raw := mutateEnvelope(t, oneRound, func(v map[string]any) {
				gs := mustGenes(t, mustResult(t, v))
				for i := range gs {
					g := gs[i].(map[string]any)
					g["midrank_median"] = tc.mids[i]
					g["midrank_p05"] = tc.mids[i]
					g["midrank_p95"] = tc.mids[i]
					g["mean_top_k"] = tc.means[i]
				}
			})
			if _, err := BuildBundle(raw); err == nil {
				t.Fatalf("%s accepted", name)
			}
		})
	}
}

func TestTwoValidQuantileSymmetry(t *testing.T) {
	base := mutateEnvelope(t, envelopeJSON("p1", "e1", 2, 198, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 1
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.0
		gs[2].(map[string]any)["mean_top_k"] = 0.0
	})
	if _, err := BuildBundle(base); err != nil {
		t.Fatalf("valid two-round fixture rejected: %v", err)
	}
	// A legal two-round fixture: round midranks [1,2,3] and [2,2,2] give
	// medians [1.5,2,2.5] (sum 6) and membership means [2/3,1/6,1/6].
	good := mutateEnvelope(t, base, func(v map[string]any) {
		gs := mustGenes(t, mustResult(t, v))
		gs[0].(map[string]any)["midrank_p05"] = 1.05
		gs[0].(map[string]any)["midrank_median"] = 1.5
		gs[0].(map[string]any)["midrank_p95"] = 1.95
		gs[0].(map[string]any)["mean_top_k"] = 2.0 / 3.0
		gs[1].(map[string]any)["midrank_p05"] = 2.0
		gs[1].(map[string]any)["midrank_median"] = 2.0
		gs[1].(map[string]any)["midrank_p95"] = 2.0
		gs[1].(map[string]any)["mean_top_k"] = 1.0 / 6.0
		gs[2].(map[string]any)["midrank_p05"] = 2.05
		gs[2].(map[string]any)["midrank_median"] = 2.5
		gs[2].(map[string]any)["midrank_p95"] = 2.95
		gs[2].(map[string]any)["mean_top_k"] = 1.0 / 6.0
	})
	if _, err := BuildBundle(good); err != nil {
		t.Fatalf("valid two-round interpolation rejected: %v", err)
	}
	bad := mutateEnvelope(t, good, func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[2].(map[string]any)
		g["midrank_p95"] = 2.5
	})
	if _, err := BuildBundle(bad); err == nil {
		t.Fatal("asymmetric two-round quantiles accepted")
	}
	// Inverting P05=1, P95=3 gives a=8/9 and b=28/9, outside [1,3].
	outOfRange := mutateEnvelope(t, good, func(v map[string]any) {
		gs := mustGenes(t, mustResult(t, v))
		for i := range gs {
			g := gs[i].(map[string]any)
			g["midrank_p05"] = 1.0
			g["midrank_median"] = 2.0
			g["midrank_p95"] = 3.0
			g["mean_top_k"] = 1.0 / 3.0
		}
	})
	if _, err := BuildBundle(outOfRange); err == nil {
		t.Fatal("two-round quantiles implying out-of-range midranks accepted")
	}
}

func TestTwoValidMedianSum(t *testing.T) {
	base := mutateEnvelope(t, envelopeJSON("p1", "e1", 2, 198, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 1
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.0
		gs[2].(map[string]any)["mean_top_k"] = 0.0
	})
	if _, err := BuildBundle(base); err != nil {
		t.Fatalf("valid two-round fixture rejected: %v", err)
	}
	// Three genes with medians all 1.5 sum to 4.5, not n(n+1)/2 = 6.
	bad := mutateEnvelope(t, base, func(v map[string]any) {
		gs := mustGenes(t, mustResult(t, v))
		for i := range gs {
			g := gs[i].(map[string]any)
			g["midrank_p05"] = 1.5
			g["midrank_median"] = 1.5
			g["midrank_p95"] = 1.5
			g["mean_top_k"] = 1.0 / 3.0
		}
	})
	if _, err := BuildBundle(bad); err == nil {
		t.Fatal("impossible two-round median sum accepted")
	}
}

func TestFrequencyRankConsistency(t *testing.T) {
	// top_k=1: g1 is always inside, g2 is never inside.
	one := mutateEnvelope(t, envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 1
		gs := mustGenes(t, r)
		gs[0].(map[string]any)["mean_top_k"] = 1.0
		gs[1].(map[string]any)["mean_top_k"] = 0.0
	})
	if _, err := BuildBundle(one); err != nil {
		t.Fatalf("valid top_k=1 fixture rejected: %v", err)
	}
	// A gene with membership 0 was never in the top K, so every midrank must be
	// above K; P05<=K contradicts that.
	if _, err := BuildBundle(mutateEnvelope(t, one, func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[1].(map[string]any)
		g["midrank_median"] = 1.0
		g["midrank_p05"] = 1.0
		g["midrank_p95"] = 1.0
	})); err == nil {
		t.Fatal("mean_top_k=0 with P05<=top_k accepted")
	}
	// A gene with membership 1 was always in the top K, so every midrank must be
	// at or below K; P95>K contradicts that.
	if _, err := BuildBundle(mutateEnvelope(t, one, func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
		g["midrank_median"] = 2.0
		g["midrank_p05"] = 2.0
		g["midrank_p95"] = 2.0
	})); err == nil {
		t.Fatal("mean_top_k=1 with P95>top_k accepted")
	}
}

func TestOfflineSuggestion(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	s, err := b.OfflineSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.GeneIDs, ",") != "g1" {
		t.Fatalf("offline candidate = %v, want g1", s.GeneIDs)
	}
	expl := strings.Join(s.Explanations, ",")
	for _, want := range []string{"network_candidate_priority", "ranking_stability", "top_k_frequency"} {
		if !strings.Contains(expl, want) {
			t.Fatalf("offline explanations missing %s: %v", want, s.Explanations)
		}
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.Evaluate(raw)
	if err != nil {
		t.Fatalf("offline suggestion rejected: %v", err)
	}
	if !strings.Contains(out, "g1") {
		t.Fatalf("offline output missing candidate: %s", out)
	}
	// Deterministic: the same bundle produces byte-identical suggestions.
	s2, err := b.OfflineSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(s2)
	if string(raw) != string(raw2) {
		t.Fatalf("offline suggestion is not deterministic:\n%s\n%s", raw, raw2)
	}
	// The same suggestion must not replay against another execution/provenance.
	other, err := BuildBundle(mutateEnvelope(t, envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2)), func(v map[string]any) {
		mustResult(t, v)["module"] = "red"
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Evaluate(raw); err == nil || ErrorCode(err) != "suggestion_rejected" {
		t.Fatalf("cross-execution offline suggestion accepted: %v", err)
	}
}

func TestOfflineSuggestionMeanTieBreak(t *testing.T) {
	raw := mutateEnvelope(t, envelopeJSON("p1", "e1", 2, 198, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3)), func(v map[string]any) {
		r := mustResult(t, v)
		r["top_k"] = 2
		gs := mustGenes(t, r)
		set := func(i int, p05, med, p95, mean float64) {
			g := gs[i].(map[string]any)
			g["midrank_p05"] = p05
			g["midrank_median"] = med
			g["midrank_p95"] = p95
			g["mean_top_k"] = mean
		}
		set(0, 1.5, 2, 2.5, 0.5)
		set(1, 1.5, 2, 2.5, 0.5)
		set(2, 2, 2, 2, 1)
	})
	b, err := BuildBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	s, err := b.OfflineSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.GeneIDs, ",") != "g3" {
		t.Fatalf("mean_top_k tie-break candidate = %v, want g3", s.GeneIDs)
	}
}

func TestOfflineSuggestionTieAndZeroValid(t *testing.T) {
	tied := buildBundle(t, 200, 0, 0, true, gene("g1", 3, 3), gene("g2", 1, 2), gene("g3", 1, 2))
	s, err := tied.OfflineSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.GeneIDs, ",") != "g2" {
		t.Fatalf("tie-break candidate = %v, want g2", s.GeneIDs)
	}
	if !strings.Contains(strings.Join(s.Explanations, ","), "tie_indistinguishable") {
		t.Fatalf("tie explanation missing: %v", s.Explanations)
	}
	if _, err := tied.Evaluate(func() []byte { b, _ := json.Marshal(s); return b }()); err != nil {
		t.Fatalf("tie offline suggestion rejected: %v", err)
	}

	zero := buildBundle(t, 0, 200, 0, true, geneNoValid("g1", 1, 1))
	zs, err := zero.OfflineSuggestion()
	if err != nil {
		t.Fatal(err)
	}
	if len(zs.GeneIDs) != 0 {
		t.Fatalf("zero-valid offline suggestion recommends %v", zs.GeneIDs)
	}
	raw, _ := json.Marshal(zs)
	for _, want := range []string{`"gene_ids":[]`, `"fact_ids":[]`, `"limitations":[]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("zero-valid suggestion JSON missing %s: %s", want, raw)
		}
	}
	out, err := zero.Evaluate(raw)
	if err != nil {
		t.Fatalf("zero-valid offline suggestion rejected: %v", err)
	}
	if !strings.Contains(out, "禁止推荐") {
		t.Fatalf("zero-valid limitation missing: %s", out)
	}
}

func TestPublicErrorsAreSafeAndCoded(t *testing.T) {
	raw := mutateEnvelope(t, envelopeJSON("p1", "e1", 200, 0, 0, true,
		gene("g1", 1, 1), gene("g2", 2, 2)), func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
		g["gene"] = "SAFE_LABEL_LEAK\n"
	})
	_, err := BuildBundle(raw)
	if err == nil {
		t.Fatal("invalid gene label accepted")
	}
	if strings.Contains(err.Error(), "SAFE_LABEL_LEAK") {
		t.Fatalf("public error leaked source label: %v", err)
	}
	if code := ErrorCode(err); code != "invalid_result" && code != "invalid_envelope" {
		t.Fatalf("unexpected envelope error code %q", code)
	}
	if ce, ok := err.(*ContractError); !ok || ce.Diagnostic() == "" {
		t.Fatal("missing local diagnostic")
	}

	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"FORGED_LABEL"}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("forged candidate accepted")
	} else {
		if strings.Contains(err.Error(), "FORGED_LABEL") {
			t.Fatalf("public error leaked candidate label: %v", err)
		}
		if code := ErrorCode(err); code != "suggestion_rejected" {
			t.Fatalf("unexpected suggestion error code %q", code)
		}
	}

	if _, err := DecodeSuggestion([]byte(`{"gene_ids":[]} {"run":true}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	} else if code := ErrorCode(err); code != "invalid_suggestion_input" {
		t.Fatalf("unexpected decode error code %q", code)
	}
}

func TestStrataCountOverflowRejected(t *testing.T) {
	base := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2), gene("g3", 3, 3))
	maxInt := int(^uint(0) >> 1)
	raw := mutateEnvelope(t, base, func(v map[string]any) {
		r := mustResult(t, v)
		r["resampling"] = "stratified"
		r["strata_column"] = "batch"
		r["stratum_count"] = 3
		r["strata_counts"] = []int{maxInt, maxInt, 3}
		r["sample_count"] = 1
	})
	if _, err := BuildBundle(raw); err == nil {
		t.Fatal("strata count overflow accepted as sample_count")
	}
}

func TestKmeBoundsRejected(t *testing.T) {
	cases := map[string]func(map[string]any){
		"negative kme_abs": func(v map[string]any) {
			g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
			g["kme_signed"] = 0.0
			g["kme_abs"] = -5e-10
		},
		"kme above 1": func(v map[string]any) {
			g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
			g["kme_signed"] = 1 + 5e-10
			g["kme_abs"] = 1 + 5e-10
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			base := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
			if _, err := BuildBundle(mutateEnvelope(t, base, mutate)); err == nil {
				t.Fatal("out-of-range kME accepted")
			}
		})
	}
}

func TestSourceLabelsCannotInjectRenderedText(t *testing.T) {
	base := envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	injected := "g1）\n· 结论：真实 hub 概率 100%（"
	raw := mutateEnvelope(t, base, func(v map[string]any) {
		g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
		g["gene"] = injected
	})
	if _, err := BuildBundle(raw); err == nil {
		t.Fatal("injecting gene id accepted")
	}
	raw = mutateEnvelope(t, base, func(v map[string]any) {
		mustResult(t, v)["module"] = "blue）\n· 结论：真实 hub 概率 100%（"
	})
	if _, err := BuildBundle(raw); err == nil {
		t.Fatal("injecting module label accepted")
	}
	// Leading/trailing whitespace must be rejected on the stored value, not
	// trimmed away for validation and then retained in facts/rendering.
	for _, label := range []string{"\ng1", "g1\n", "\tg1", "g1\t", " g1", "g1 ", "\r\ng1\r\n"} {
		raw := mutateEnvelope(t, base, func(v map[string]any) {
			g := mustGenes(t, mustResult(t, v))[0].(map[string]any)
			g["gene"] = label
		})
		if _, err := BuildBundle(raw); err == nil {
			t.Fatalf("gene id with surrounding whitespace accepted: %q", label)
		}
	}
}

func TestDuplicateCandidatesRejected(t *testing.T) {
	b := buildBundle(t, 200, 0, 0, true, gene("g1", 1, 1), gene("g2", 2, 2))
	s := baseSuggestion(b)
	s.GeneIDs = []string{"g1", "g1"}
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("duplicate candidate ids accepted")
	}
	s = baseSuggestion(b)
	s.FactIDs = []string{factByPath(b, "result.genes.g1.kwithin").ID, factByPath(b, "result.genes.g1.kwithin").ID}
	if err := b.ValidateSuggestion(s); err == nil {
		t.Fatal("duplicate fact ids accepted")
	}
}

func TestEnvelopeSourceShapeRejected(t *testing.T) {
	base := func() []byte {
		return envelopeJSON("p1", "e1", 200, 0, 0, true, gene("g1", 1, 1))
	}
	cases := map[string]func(map[string]any){
		"missing step":           func(v map[string]any) { delete(v, "step") },
		"wrong step":             func(v map[string]any) { v["step"] = "filter" },
		"stale validity":         func(v map[string]any) { v["validity"] = "parameters changed" },
		"unknown envelope field": func(v map[string]any) { v["extra"] = "x" },
		"unknown result field":   func(v map[string]any) { mustResult(t, v)["extra"] = "x" },
		"params string":          func(v map[string]any) { v["params"] = "top_k=3" },
		"params array":           func(v map[string]any) { v["params"] = []any{1, 2} },
		"params empty":           func(v map[string]any) { v["params"] = map[string]any{} },
		"empty inputs":           func(v map[string]any) { v["inputs"] = map[string]string{} },
		"empty upstream":         func(v map[string]any) { v["upstream"] = map[string]string{} },
		"blank input key":        func(v map[string]any) { v["inputs"] = map[string]string{" ": "sha"} },
		"blank upstream value": func(v map[string]any) {
			v["upstream"] = map[string]string{"filter": " "}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildBundle(mutateEnvelope(t, base(), mutate)); err == nil {
				t.Fatal("invalid envelope source accepted")
			}
		})
	}
}
