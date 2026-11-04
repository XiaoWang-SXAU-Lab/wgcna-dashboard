package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// testEvidenceEnvelope is a small, fully valid hub_priority envelope. It lets
// the API success path run in CI without a worker or a real project directory.
func testEvidenceEnvelope(t *testing.T) []byte {
	t.Helper()
	raw := map[string]any{
		"project_id": "p1", "step": "hub_priority", "execution_id": "e1", "engine": "e",
		"params":   map[string]any{"select_module": "blue", "top_k": 2},
		"inputs":   map[string]string{"expr": "sha"},
		"upstream": map[string]string{"filter": "f1"},
		"validity": "",
		"result": map[string]any{
			"module": "blue", "method_version": "hub_priority.v1", "scope": "fixed_full_module",
			"n_bootstraps": 10, "attempted": 10, "valid": 10, "invalid": 0,
			"invalid_reasons": []any{}, "seed": 42, "top_k": 2, "tie_tol": 1e-9,
			"resampling": "iid", "strata_column": "", "stratum_count": 0, "strata_counts": []any{},
			"strata_fingerprint": "14695981039346656037", "gene_fingerprint": "1719961226634372230",
			"sample_fingerprint": "15342682632623422293",
			"resource_note":      "engineering budget: test fixture",
			"design_confirmed":   true, "trait_source": "none",
			"power": 6, "network_type": "unsigned", "gene_count": 2, "sample_count": 4,
			"limited": false, "direction_undetermined": 0,
			"genes": []any{
				map[string]any{
					"gene": "g1", "kwithin": 2.0, "kme_signed": 0.5, "kme_abs": 0.5,
					"rank_lower": 1, "rank_upper": 1, "midrank_median": 1.0,
					"midrank_p05": 1.0, "midrank_p95": 1.0, "mean_top_k": 1.0,
				},
				map[string]any{
					"gene": "g2", "kwithin": 1.0, "kme_signed": 0.5, "kme_abs": 0.5,
					"rank_lower": 2, "rank_upper": 2, "midrank_median": 2.0,
					"midrank_p05": 2.0, "midrank_p95": 2.0, "mean_top_k": 1.0,
				},
			},
		},
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBuildHubPriorityEvidence(t *testing.T) {
	raw := testEvidenceEnvelope(t)
	resp, status, err := buildHubPriorityEvidence(raw, false)
	if err != nil || status != 200 {
		t.Fatalf("build evidence: status=%d err=%v", status, err)
	}
	if resp["offline"] != true || resp["candidate_count"] != 2 || resp["fact_count"] == 0 {
		t.Fatalf("unexpected evidence response: %+v", resp)
	}
	rendered, _ := resp["rendered"].(string)
	if !strings.Contains(rendered, "g1") || !strings.Contains(rendered, "局限") {
		t.Fatalf("rendered offline suggestion missing content: %q", rendered)
	}
	if _, ok := resp["facts"]; ok {
		t.Fatal("default response must not include the full fact list")
	}
	full, status, err := buildHubPriorityEvidence(raw, true)
	if err != nil || status != 200 {
		t.Fatalf("build full evidence: status=%d err=%v", status, err)
	}
	if _, ok := full["facts"]; !ok {
		t.Fatal("full response missing facts")
	}
	if _, ok := full["candidates"]; !ok {
		t.Fatal("full response missing candidates")
	}
}

// TestBuildHubPriorityEvidenceRealEnvelope uses the real worker e2e envelope
// when EVIDENCE_E2E_ENVELOPE is set; normal `go test` skips it.
func TestBuildHubPriorityEvidenceRealEnvelope(t *testing.T) {
	path := os.Getenv("EVIDENCE_E2E_ENVELOPE")
	if path == "" {
		t.Skip("EVIDENCE_E2E_ENVELOPE not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	resp, status, err := buildHubPriorityEvidence(raw, false)
	if err != nil || status != 200 {
		t.Fatalf("real envelope: status=%d err=%v", status, err)
	}
	rendered, _ := resp["rendered"].(string)
	if rendered == "" || !strings.Contains(rendered, "候选") {
		t.Fatalf("real envelope rendered output missing: %q", rendered)
	}
	if resp["offline"] != true {
		t.Fatalf("real evidence is not marked offline: %+v", resp)
	}
}
