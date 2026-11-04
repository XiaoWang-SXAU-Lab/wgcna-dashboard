package evidence_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/evidence"
)

// validEnvelope builds a fully-valid envelope so a single removal/change is the
// only defect under test. It mirrors the fixtures in the internal test package.
func validEnvelope() []byte {
	gene := func(name string, rank int) map[string]any {
		mid := float64(rank)
		mk := 1.0 // both genes are inside top_k=2
		return map[string]any{
			"gene": name, "kwithin": float64(3 - rank), "kme_signed": 0.5, "kme_abs": 0.5,
			"rank_lower": rank, "rank_upper": rank,
			"midrank_median": mid, "midrank_p05": mid, "midrank_p95": mid, "mean_top_k": mk,
		}
	}
	raw := map[string]any{
		"project_id": "p1", "step": "hub_priority", "execution_id": "e1", "engine": "e",
		"params":   map[string]any{"select_module": "blue", "top_k": 2},
		"inputs":   map[string]string{"expr": "sha"},
		"upstream": map[string]string{"filter": "f1", "network": "n1"},
		"validity": "",
		"result": map[string]any{
			"module": "blue", "method_version": "hub_priority.v1", "scope": "fixed_full_module",
			"n_bootstraps": 200, "attempted": 200, "valid": 200, "invalid": 0,
			"invalid_reasons": []any{}, "seed": 42, "top_k": 2, "tie_tol": 1e-9,
			"resampling": "iid", "strata_column": "", "stratum_count": 0, "strata_counts": []int{},
			"strata_fingerprint": "10000000000000000001", "gene_fingerprint": "10000000000000000002",
			"sample_fingerprint": "10000000000000000003",
			"resource_note":      "engineering budget: g²×B ≤ 1e9 (test fixture)",
			"design_confirmed":   true, "trait_source": "none",
			"power": 6, "network_type": "unsigned", "gene_count": 2, "sample_count": 6,
			"limited": false, "direction_undetermined": 0,
			"genes": []map[string]any{gene("g1", 1), gene("g2", 2)},
		},
	}
	b, _ := json.Marshal(raw)
	return b
}

// TestPublicBuildBundleRejectsMissingOrNullValidity exercises the exported
// BuildBundle: an otherwise-valid envelope must fail when validity is absent,
// JSON null, or non-empty (stale/not-run), because the field must be an
// explicit empty string.
func TestPublicBuildBundleRejectsMissingOrNullValidity(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"missing": func(v map[string]any) { delete(v, "validity") },
		"null":    func(v map[string]any) { v["validity"] = nil },
		"stale":   func(v map[string]any) { v["validity"] = "parameters changed" },
	} {
		var v map[string]any
		if err := json.Unmarshal(validEnvelope(), &v); err != nil {
			t.Fatal(err)
		}
		mutate(v)
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := evidence.BuildBundle(b); err == nil {
			t.Fatalf("%s validity accepted", name)
		}
	}
}

// TestPublicEvaluateRejectsForgedGene proves the only exported render path
// (Evaluate) validates before rendering: a forged gene id cannot produce output.
func TestPublicEvaluateRejectsForgedGene(t *testing.T) {
	b, err := evidence.BuildBundle(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"project_id": "p1", "execution_id": "e1", "provenance": b.Provenance,
		"gene_ids": []string{"FORGED_GENE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.Evaluate(raw)
	if err == nil {
		t.Fatal("forged gene rendered through Evaluate")
	}
	if strings.Contains(out, "FORGED_GENE") {
		t.Fatalf("forged gene leaked into output: %q", out)
	}
}

// TestPublicSuggestionRequiresProvenance proves a suggestion must name the exact
// source provenance, not just project/execution labels.
func TestPublicSuggestionRequiresProvenance(t *testing.T) {
	b, err := evidence.BuildBundle(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{
		"missing": []byte(`{"project_id":"p1","execution_id":"e1"}`),
		"wrong":   []byte(`{"project_id":"p1","execution_id":"e1","provenance":"wrong"}`),
	} {
		if _, err := b.Evaluate(raw); err == nil {
			t.Fatalf("%s provenance accepted", name)
		}
	}
	ok, err := json.Marshal(map[string]any{
		"project_id": "p1", "execution_id": "e1", "provenance": b.Provenance,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Evaluate(ok); err != nil {
		t.Fatalf("correct provenance rejected: %v", err)
	}
}

// TestPublicSnapshotMutationCannotForgeValidation proves exported fields are a
// snapshot, not the state used by Evaluate.
func TestPublicSnapshotMutationCannotForgeValidation(t *testing.T) {
	b, err := evidence.BuildBundle(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	factID := ""
	for _, f := range b.Facts {
		if f.FieldPath == "result.genes.g1.kwithin" {
			factID = f.ID
		}
	}
	if factID == "" {
		t.Fatal("missing g1.kwithin fact")
	}
	b.Summary = "FORGED TRUSTED SUMMARY"
	b.Limitations = nil
	b.Valid = 0
	raw, err := json.Marshal(map[string]any{
		"project_id": "p1", "execution_id": "e1", "provenance": b.Provenance,
		"gene_ids": []string{"g1"}, "fact_ids": []string{factID},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := b.Evaluate(raw)
	if err != nil {
		t.Fatalf("sealed validation should ignore snapshot mutation: %v", err)
	}
	if strings.Contains(out, "FORGED") {
		t.Fatalf("forged summary rendered: %q", out)
	}
	if !strings.Contains(out, "局限") {
		t.Fatalf("deterministic limitations missing: %q", out)
	}
}
