// Package evidence builds an offline, code-rendered EvidenceBundle from a Project
// candidate-priority report envelope and validates read-only suggestion responses
// against it. It is independent of the legacy ai.Navigator (free-text chat + tool
// loop) and never issues remote calls or reads provider secrets.
package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// Fact is a single program-provided fact bound to a project/execution/field path.
// Gene is "" for module-level facts and the gene id for gene-level facts; Semantics
// names the metric (never parsed from the field path text).
type Fact struct {
	ID         string `json:"id"`
	Provenance string `json:"provenance"`
	FieldPath  string `json:"field_path"`
	Value      any    `json:"value"`
	Unit       string `json:"unit,omitempty"`
	Scope      string `json:"scope"`
	Quality    string `json:"quality,omitempty"`
	Source     string `json:"source"`
	Gene       string `json:"gene,omitempty"`
	Semantics  string `json:"semantics,omitempty"`
}

// Envelope mirrors project.CandidateReportEnvelope. The source contract requires
// params/inputs/upstream/validity to be present (never dropped) and validity to be
// an explicit string. A missing or null validity is rejected by BuildBundle.
type Envelope struct {
	ProjectID   string            `json:"project_id"`
	Step        string            `json:"step"`
	ExecutionID string            `json:"execution_id"`
	Engine      string            `json:"engine"`
	Params      json.RawMessage   `json:"params"`
	Inputs      map[string]string `json:"inputs"`
	Upstream    map[string]string `json:"upstream"`
	Validity    *string           `json:"validity"`
	Result      json.RawMessage   `json:"result"`
}

// InvalidReason is one invalid-replicate reason with its count.
type InvalidReason struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// PriorityResult is the hub_priority result fields the bundle needs.
type PriorityResult struct {
	Module                string          `json:"module"`
	MethodVersion         string          `json:"method_version"`
	Scope                 string          `json:"scope"`
	NBootstraps           int             `json:"n_bootstraps"`
	Attempted             int             `json:"attempted"`
	Valid                 int             `json:"valid"`
	Invalid               int             `json:"invalid"`
	InvalidReasons        []InvalidReason `json:"invalid_reasons"`
	Seed                  uint64          `json:"seed"`
	TopK                  int             `json:"top_k"`
	TieTol                float64         `json:"tie_tol"`
	Resampling            string          `json:"resampling"`
	StrataColumn          string          `json:"strata_column"`
	StratumCount          int             `json:"stratum_count"`
	StrataCounts          []int           `json:"strata_counts"`
	StrataFingerprint     string          `json:"strata_fingerprint"`
	GeneFingerprint       string          `json:"gene_fingerprint"`
	SampleFingerprint     string          `json:"sample_fingerprint"`
	Limited               bool            `json:"limited"`
	DesignConfirmed       bool            `json:"design_confirmed"`
	TraitSource           string          `json:"trait_source"`
	Power                 float64         `json:"power"`
	NetworkType           string          `json:"network_type"`
	GeneCount             int             `json:"gene_count"`
	SampleCount           int             `json:"sample_count"`
	DirectionUndetermined int             `json:"direction_undetermined"`
	ResourceNote          string          `json:"resource_note"`
	Genes                 []PriorityGene  `json:"genes"`
}

type PriorityGene struct {
	Gene       string   `json:"gene"`
	Kwithin    float64  `json:"kwithin"`
	KmeSigned  float64  `json:"kme_signed"`
	KmeAbs     float64  `json:"kme_abs"`
	RankLower  int      `json:"rank_lower"`
	RankUpper  int      `json:"rank_upper"`
	MidrankMed *float64 `json:"midrank_median"`
	MidrankP05 *float64 `json:"midrank_p05"`
	MidrankP95 *float64 `json:"midrank_p95"`
	MeanTopK   *float64 `json:"mean_top_k"`
}

// EvidenceBundle is the offline, deterministic fact set for one execution.
//
// The exported fields are a read-only snapshot for inspection/serialization.
// Validation and rendering never read them: they use the unexported sealed
// state created by BuildBundle, so a caller cannot forge a valid result by
// mutating exported fields or by constructing a zero-value bundle.
//
// A serialized bundle is not a trusted state: API/UI callers must rebuild it
// from the current atomic CandidateReportEnvelope and re-validate any persisted
// suggestion against that fresh provenance.
type EvidenceBundle struct {
	ProjectID    string   `json:"project_id"`
	ExecutionID  string   `json:"execution_id"`
	Engine       string   `json:"engine"`
	Provenance   string   `json:"provenance"`
	Valid        int      `json:"valid"`
	Invalid      int      `json:"invalid"`
	Facts        []Fact   `json:"facts"`
	Candidates   []string `json:"candidates"`
	FullCoverage bool     `json:"full_coverage"`
	GeneCount    int      `json:"gene_count"`
	Summary      string   `json:"summary"`
	Limitations  []string `json:"limitations"`
	sealed       *sealedBundle
}

// sealedBundle is the immutable source of truth used by ValidateSuggestion and
// renderSuggestion. It is never exposed or aliased by exported fields.
type sealedBundle struct {
	projectID    string
	executionID  string
	engine       string
	provenance   string
	valid        int
	invalid      int
	fullCoverage bool
	geneCount    int
	factIndex    map[string]Fact
	geneIndex    map[string]bool
	candidates   []string
	summary      string
	limitations  []string
}

const (
	// maxEnvelopeBytes and maxGeneRows make the input budget explicit. Rejecting
	// an over-budget input is deliberate: scientific rows are never silently
	// truncated to fit.
	maxEnvelopeBytes = 32 << 20
	maxGeneRows      = 100_000
)

const (
	ScopeNetworkCandidatePriority = "network_candidate_priority"

	LimitationNoTrait                   = "no_trait_evidence"
	LimitationInvalidReplicates         = "invalid_replicates"
	LimitationNoValid                   = "no_valid_replicates"
	LimitationDirectionUndetermined     = "direction_undetermined"
	LimitationBootstrapFrequencyNotProb = "bootstrap_frequency_not_probability"
)

var (
	allowedExplanations = map[string]bool{
		"network_candidate_priority": true,
		"ranking_stability":          true,
		"top_k_frequency":            true,
		"tie_indistinguishable":      true,
	}
	allowedLimitations = map[string]bool{
		LimitationNoTrait:                   true,
		LimitationInvalidReplicates:         true,
		LimitationNoValid:                   true,
		LimitationDirectionUndetermined:     true,
		LimitationBootstrapFrequencyNotProb: true,
	}
	allowedPlanPreview = map[string]bool{
		"view_candidates": true,
		"export_tsv":      true,
	}
)

// factID binds a fact to the full provenance fingerprint (project, execution,
// step, engine, params, inputs, upstream, and result digest), not just to the
// project/execution labels. Two executions with the same ids but different
// sources therefore cannot share facts.
func factID(provenance, fieldPath string) string {
	h := sha256.Sum256([]byte(provenance + "\x00" + fieldPath))
	return hex.EncodeToString(h[:])
}

// ContractError is the stable, user-safe error returned by the public evidence
// API. Error deliberately contains no source labels; Diagnostic is for local
// operator logs only.
type ContractError struct {
	Code    string
	Message string
	detail  error
}

func (e *ContractError) Error() string { return e.Message }

// Diagnostic returns a bounded local diagnostic. It may contain source labels;
// do not render it in UI or share it outside the operator.
func (e *ContractError) Diagnostic() string {
	if e.detail == nil {
		return ""
	}
	text := e.detail.Error()
	if len(text) > 256 {
		text = text[:256] + "..."
	}
	return text
}

// ErrorCode returns the stable code for a ContractError, or "" otherwise.
func ErrorCode(err error) string {
	var ce *ContractError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

func contractError(code, message string, detail error) error {
	return &ContractError{Code: code, Message: message, detail: detail}
}

// BuildBundle parses a report envelope and builds the deterministic fact bundle.
// It enforces the source contract (step identity, params/inputs/upstream present,
// validity empty), the full-result invariants, and provenance binding. Everything
// used by ValidateSuggestion/renderSuggestion is sealed in private state.
//
// The returned error is always a *ContractError: Error() is a fixed, safe
// message and ErrorCode() is stable; Diagnostic() holds local details.
func BuildBundle(envelopeJSON []byte) (*EvidenceBundle, error) {
	b, err := buildBundleFromEnvelope(envelopeJSON)
	if err != nil {
		var ce *ContractError
		if errors.As(err, &ce) {
			return nil, ce
		}
		return nil, contractError("invalid_envelope", "evidence envelope rejected", err)
	}
	return b, nil
}

func buildBundleFromEnvelope(envelopeJSON []byte) (*EvidenceBundle, error) {
	if len(envelopeJSON) > maxEnvelopeBytes {
		return nil, contractError("envelope_too_large", "evidence envelope exceeds size budget", nil)
	}
	var env Envelope
	if err := decodeStrict(envelopeJSON, &env); err != nil {
		return nil, contractError("invalid_envelope", "evidence envelope rejected",
			fmt.Errorf("invalid envelope: %w", err))
	}
	provenance, err := validateEnvelope(&env)
	if err != nil {
		return nil, contractError("invalid_envelope", "evidence envelope rejected", err)
	}

	var r PriorityResult
	if err := decodeStrict(env.Result, &r); err != nil {
		return nil, contractError("invalid_result", "evidence result rejected",
			fmt.Errorf("invalid result: %w", err))
	}
	if len(r.Genes) > maxGeneRows {
		return nil, contractError("result_too_large", "evidence result exceeds gene-row budget", nil)
	}
	if err := validatePriorityResult(&r); err != nil {
		return nil, contractError("invalid_result", "evidence result rejected", err)
	}

	sealed := &sealedBundle{
		projectID:    env.ProjectID,
		executionID:  env.ExecutionID,
		engine:       env.Engine,
		provenance:   provenance,
		valid:        r.Valid,
		invalid:      r.Invalid,
		fullCoverage: true,
		geneCount:    r.GeneCount,
		factIndex:    map[string]Fact{},
		geneIndex:    map[string]bool{},
	}
	var facts []Fact
	add := func(fieldPath, gene, semantics string, value any, unit, quality string) {
		f := Fact{
			ID:         factID(provenance, fieldPath),
			Provenance: provenance,
			FieldPath:  fieldPath,
			Value:      value,
			Unit:       unit,
			Scope:      ScopeNetworkCandidatePriority,
			Quality:    quality,
			Source:     "step8_hub_priority.json",
			Gene:       gene,
			Semantics:  semantics,
		}
		facts = append(facts, f)
		sealed.factIndex[f.ID] = f
	}

	quality := ""
	if r.Valid == 0 {
		quality = "no_valid"
	} else if r.Invalid > 0 {
		quality = "conditioned"
	}

	add("result.module", "", "module", r.Module, "", quality)
	add("result.method_version", "", "method", r.MethodVersion, "", quality)
	add("result.gene_count", "", "gene_count", r.GeneCount, "genes", quality)
	add("result.sample_count", "", "sample_count", r.SampleCount, "samples", quality)
	add("result.valid", "", "valid", r.Valid, "replicates", quality)
	add("result.attempted", "", "attempted", r.Attempted, "replicates", quality)
	add("result.invalid", "", "invalid", r.Invalid, "replicates", quality)
	add("result.n_bootstraps", "", "n_bootstraps", r.NBootstraps, "", quality)
	add("result.top_k", "", "top_k", r.TopK, "", quality)
	add("result.tie_tol", "", "tie_tol", r.TieTol, "", quality)
	add("result.seed", "", "seed", r.Seed, "", quality)
	add("result.resampling", "", "resampling", r.Resampling, "", quality)
	add("result.strata_column", "", "strata_column", r.StrataColumn, "", quality)
	add("result.stratum_count", "", "stratum_count", r.StratumCount, "strata", quality)
	for i, count := range r.StrataCounts {
		add(fmt.Sprintf("result.strata_counts.%d", i), "", "strata_count", count, "samples", quality)
	}
	add("result.strata_fingerprint", "", "strata_fingerprint", r.StrataFingerprint, "", quality)
	add("result.gene_fingerprint", "", "gene_fingerprint", r.GeneFingerprint, "", quality)
	add("result.sample_fingerprint", "", "sample_fingerprint", r.SampleFingerprint, "", quality)
	add("result.limited", "", "limited", r.Limited, "", quality)
	add("result.design_confirmed", "", "design_confirmed", r.DesignConfirmed, "", quality)
	add("result.trait_source", "", "trait_source", r.TraitSource, "", quality)
	add("result.power", "", "power", r.Power, "", quality)
	add("result.network_type", "", "network_type", r.NetworkType, "", quality)
	add("result.direction_undetermined", "", "direction_undetermined", r.DirectionUndetermined, "rounds", quality)
	add("result.resource_note", "", "resource_note", r.ResourceNote, "", quality)
	for _, ir := range r.InvalidReasons {
		add("result.invalid_reasons."+ir.Reason+".count", "", "invalid_reason", ir.Count, "", quality)
	}

	for _, g := range r.Genes {
		sealed.candidates = append(sealed.candidates, g.Gene)
		sealed.geneIndex[g.Gene] = true
		gq := quality
		if r.Valid == 0 {
			gq = "no_valid"
		}
		add("result.genes."+g.Gene+".kwithin", g.Gene, "kwithin", g.Kwithin, "", gq)
		add("result.genes."+g.Gene+".kme_signed", g.Gene, "kme_signed", g.KmeSigned, "", gq)
		add("result.genes."+g.Gene+".kme_abs", g.Gene, "kme_abs", g.KmeAbs, "", gq)
		add("result.genes."+g.Gene+".rank_lower", g.Gene, "rank_lower", g.RankLower, "", gq)
		add("result.genes."+g.Gene+".rank_upper", g.Gene, "rank_upper", g.RankUpper, "", gq)
		add("result.genes."+g.Gene+".tie", g.Gene, "tie", g.RankLower < g.RankUpper, "", gq)
		if g.MidrankMed != nil {
			add("result.genes."+g.Gene+".midrank_median", g.Gene, "midrank_median", *g.MidrankMed, "", gq)
		}
		if g.MidrankP05 != nil {
			add("result.genes."+g.Gene+".midrank_p05", g.Gene, "midrank_p05", *g.MidrankP05, "", gq)
		}
		if g.MidrankP95 != nil {
			add("result.genes."+g.Gene+".midrank_p95", g.Gene, "midrank_p95", *g.MidrankP95, "", gq)
		}
		if g.MeanTopK != nil {
			add("result.genes."+g.Gene+".mean_top_k", g.Gene, "mean_top_k", *g.MeanTopK, "", gq)
		}
	}
	sort.Strings(sealed.candidates)

	sealed.limitations = []string{LimitationNoTrait, LimitationBootstrapFrequencyNotProb}
	if r.Valid == 0 {
		sealed.limitations = append(sealed.limitations, LimitationNoValid)
	} else if r.Invalid > 0 {
		sealed.limitations = append(sealed.limitations, LimitationInvalidReplicates)
	}
	if r.DirectionUndetermined > 0 {
		sealed.limitations = append(sealed.limitations, LimitationDirectionUndetermined)
	}
	sealed.summary = fmt.Sprintf(
		"候选优先级（hub_priority）· 模块 %s · %d 基因（完整）· 有效 %d/%d · K %d · B %d · 无表型证据（网络候选优先级）",
		r.Module, r.GeneCount, r.Valid, r.Attempted, r.TopK, r.NBootstraps,
	)

	return &EvidenceBundle{
		ProjectID:    env.ProjectID,
		ExecutionID:  env.ExecutionID,
		Engine:       env.Engine,
		Provenance:   provenance,
		Valid:        r.Valid,
		Invalid:      r.Invalid,
		Facts:        append([]Fact(nil), facts...),
		Candidates:   append([]string(nil), sealed.candidates...),
		FullCoverage: true,
		GeneCount:    r.GeneCount,
		Summary:      sealed.summary,
		Limitations:  append([]string(nil), sealed.limitations...),
		sealed:       sealed,
	}, nil
}

// validateEnvelope enforces the source contract and returns the provenance
// fingerprint that binds facts and suggestions to this exact source.
func validateEnvelope(env *Envelope) (string, error) {
	if strings.TrimSpace(env.ProjectID) == "" || strings.TrimSpace(env.ExecutionID) == "" || strings.TrimSpace(env.Engine) == "" {
		return "", errors.New("envelope missing project/execution/engine identity")
	}
	if env.Step != "hub_priority" {
		return "", fmt.Errorf("unsupported step %q", env.Step)
	}
	if env.Validity == nil {
		return "", errors.New("envelope missing/null validity")
	}
	if *env.Validity != "" {
		return "", fmt.Errorf("stale/not-run evidence (validity=%q)", *env.Validity)
	}
	if len(env.Params) == 0 {
		return "", errors.New("envelope missing params")
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(env.Params, &params); err != nil || params == nil || len(params) == 0 {
		return "", errors.New("envelope params must be a non-empty JSON object")
	}
	if len(env.Inputs) == 0 || len(env.Upstream) == 0 {
		return "", errors.New("envelope missing inputs/upstream source")
	}
	for name, values := range map[string]map[string]string{"inputs": env.Inputs, "upstream": env.Upstream} {
		for key, value := range values {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("envelope %s contains an empty key/value", name)
			}
		}
	}
	if len(env.Result) == 0 {
		return "", errors.New("envelope missing result")
	}
	return provenanceFingerprint(env)
}

// provenanceSource is the canonical set of source fields covered by the
// provenance fingerprint. The result digest ties facts to the exact artifact.
type provenanceSource struct {
	ProjectID    string            `json:"project_id"`
	Step         string            `json:"step"`
	ExecutionID  string            `json:"execution_id"`
	Engine       string            `json:"engine"`
	Params       json.RawMessage   `json:"params"`
	Inputs       map[string]string `json:"inputs"`
	Upstream     map[string]string `json:"upstream"`
	ResultSHA256 string            `json:"result_sha256"`
}

func provenanceFingerprint(env *Envelope) (string, error) {
	params, err := canonicalJSON(env.Params)
	if err != nil {
		return "", fmt.Errorf("envelope params: %w", err)
	}
	result, err := canonicalJSON(env.Result)
	if err != nil {
		return "", fmt.Errorf("envelope result: %w", err)
	}
	resultDigest := sha256.Sum256(result)
	src := provenanceSource{
		ProjectID:    env.ProjectID,
		Step:         env.Step,
		ExecutionID:  env.ExecutionID,
		Engine:       env.Engine,
		Params:       params,
		Inputs:       env.Inputs,
		Upstream:     env.Upstream,
		ResultSHA256: hex.EncodeToString(resultDigest[:]),
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return "", fmt.Errorf("provenance fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// decodeStrict decodes exactly one JSON value and rejects unknown fields and
// trailing values. The evidence schema is versioned by method_version, so an
// unknown field is a contract change rather than silently ignored data.
func decodeStrict(data []byte, value any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	var extra any
	switch err := dec.Decode(&extra); {
	case err == io.EOF:
		return nil
	case err == nil:
		return errors.New("trailing JSON value")
	default:
		return err
	}
}

// canonicalJSON re-encodes a JSON value with object keys sorted and number
// literals preserved, so equivalent source documents hash identically.
func canonicalJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	switch err := dec.Decode(&extra); {
	case err == io.EOF:
	case err == nil:
		return nil, errors.New("trailing JSON value")
	default:
		return nil, err
	}
	return json.Marshal(value)
}

const (
	floatTolerance = 1e-12
	// Each bootstrap round's top-K membership weights sum to top_k, so the
	// per-gene mean_top_k values must sum to top_k across the full module. The
	// tolerance absorbs floating-point accumulation only.
	meanTopKSumTolerance = 1e-6
)

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// validIdentifier keeps source labels that are embedded in rendered output to a
// conservative ASCII set. Without this, a gene/module name containing a
// parenthesis or newline could inject arbitrary text past the explanation
// allow-list (e.g. "g1）\n· 结论：真实 hub 概率 100%（").
// validFingerprint accepts the worker's decimal FNV-1a u64 fingerprints.
func validFingerprint(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '.' || r == ':' || r == '-' || r == '|' || r == '+':
		default:
			return false
		}
	}
	return true
}

// validatePriorityResult seals the scientific and structural invariants that the
// worker's aggregate_priority/rank_round already enforce, so a forged or
// truncated result cannot be presented as a complete, valid bundle.
func validatePriorityResult(r *PriorityResult) error {
	if !validIdentifier(r.Module) {
		return fmt.Errorf("invalid module label %q", r.Module)
	}
	if r.MethodVersion != "hub_priority.v1" {
		return fmt.Errorf("unsupported method_version %q", r.MethodVersion)
	}
	if r.Scope != "fixed_full_module" {
		return fmt.Errorf("unsupported scope %q", r.Scope)
	}
	if !r.DesignConfirmed {
		return errors.New("unsupported design: design_confirmed must be true")
	}
	if r.Resampling != "iid" && r.Resampling != "stratified" {
		return fmt.Errorf("unsupported resampling %q", r.Resampling)
	}
	// The hub_priority MVP is trait-less; do not silently rewrite an unsupported
	// source value to "none".
	if r.TraitSource != "none" {
		return fmt.Errorf("unsupported trait_source %q (hub_priority MVP is trait-less)", r.TraitSource)
	}
	if !finite(r.TieTol) || r.TieTol < 0 {
		return fmt.Errorf("tie_tol must be finite and >= 0, got %v", r.TieTol)
	}
	if r.NBootstraps < 1 || r.NBootstraps > 1000 {
		return fmt.Errorf("n_bootstraps must be in [1,1000], got %d", r.NBootstraps)
	}
	if r.Attempted != r.NBootstraps {
		return fmt.Errorf("attempted(%d) != n_bootstraps(%d)", r.Attempted, r.NBootstraps)
	}
	if r.Valid < 0 || r.Invalid < 0 || int64(r.Valid)+int64(r.Invalid) != int64(r.Attempted) {
		return fmt.Errorf("attempted(%d) != valid(%d)+invalid(%d)", r.Attempted, r.Valid, r.Invalid)
	}
	if r.GeneCount <= 0 {
		return errors.New("gene_count must be positive")
	}
	if r.SampleCount <= 0 {
		return errors.New("sample_count must be positive")
	}
	if r.TopK < 1 || r.TopK > r.GeneCount {
		return fmt.Errorf("top_k must be in [1,%d], got %d", r.GeneCount, r.TopK)
	}
	if r.DirectionUndetermined < 0 || r.DirectionUndetermined > r.Valid {
		return fmt.Errorf("direction_undetermined must be in [0,valid=%d], got %d", r.Valid, r.DirectionUndetermined)
	}
	if r.Limited != (r.Invalid > 0) {
		return fmt.Errorf("limited must be %v when invalid=%d", r.Invalid > 0, r.Invalid)
	}
	if !finite(r.Power) || r.Power <= 0 {
		return fmt.Errorf("power must be finite and positive, got %v", r.Power)
	}
	if strings.TrimSpace(r.NetworkType) == "" {
		return errors.New("result missing network_type")
	}
	if !validFingerprint(r.StrataFingerprint) {
		return errors.New("invalid strata_fingerprint")
	}
	if !validFingerprint(r.GeneFingerprint) {
		return errors.New("invalid gene_fingerprint")
	}
	if !validFingerprint(r.SampleFingerprint) {
		return errors.New("invalid sample_fingerprint")
	}
	if strings.TrimSpace(r.ResourceNote) == "" {
		return errors.New("result missing resource_note")
	}
	if err := validateStrata(r); err != nil {
		return err
	}
	if err := validateInvalidReasons(r); err != nil {
		return err
	}
	return validateGenes(r)
}

func validateStrata(r *PriorityResult) error {
	switch r.Resampling {
	case "iid":
		if r.StrataColumn != "" || r.StratumCount != 0 || len(r.StrataCounts) != 0 {
			return errors.New("iid resampling must not carry strata fields")
		}
	case "stratified":
		if strings.TrimSpace(r.StrataColumn) == "" {
			return errors.New("stratified resampling requires strata_column")
		}
		if r.StratumCount < 1 {
			return fmt.Errorf("stratum_count must be positive, got %d", r.StratumCount)
		}
		if len(r.StrataCounts) != r.StratumCount {
			return fmt.Errorf("stratum_count=%d but strata_counts rows=%d", r.StratumCount, len(r.StrataCounts))
		}
		var sum int64
		for i, count := range r.StrataCounts {
			if count <= 0 {
				return fmt.Errorf("strata_counts[%d] must be positive, got %d", i, count)
			}
			// A stratum cannot be larger than the whole sample set; this also
			// prevents int overflow from being summed into a plausible total.
			if count > r.SampleCount {
				return fmt.Errorf("strata_counts[%d]=%d exceeds sample_count=%d", i, count, r.SampleCount)
			}
			// Check before adding so the sum can never wrap even for adversarial
			// values near the platform int maximum.
			if int64(count) > int64(r.SampleCount)-sum {
				return fmt.Errorf("strata_counts sum exceeds sample_count=%d", r.SampleCount)
			}
			sum += int64(count)
		}
		if sum != int64(r.SampleCount) {
			return fmt.Errorf("strata_counts sum=%d but sample_count=%d", sum, r.SampleCount)
		}
	}
	return nil
}

func validateInvalidReasons(r *PriorityResult) error {
	seen := map[string]bool{}
	var sum int64
	for _, reason := range r.InvalidReasons {
		name := strings.TrimSpace(reason.Reason)
		if name == "" {
			return errors.New("invalid_reasons contains an empty reason")
		}
		if seen[name] {
			return fmt.Errorf("duplicate invalid reason %q", name)
		}
		seen[name] = true
		if reason.Count <= 0 {
			return fmt.Errorf("invalid reason %q count must be positive, got %d", name, reason.Count)
		}
		if reason.Count > r.Invalid {
			return fmt.Errorf("invalid reason %q count=%d exceeds invalid=%d", name, reason.Count, r.Invalid)
		}
		sum += int64(reason.Count)
		if sum > int64(r.Invalid) {
			return fmt.Errorf("invalid_reasons sum exceeds invalid=%d", r.Invalid)
		}
	}
	if sum != int64(r.Invalid) {
		return fmt.Errorf("invalid_reasons sum=%d but invalid=%d", sum, r.Invalid)
	}
	return nil
}

func validateGenes(r *PriorityResult) error {
	// Full coverage must be proven by row count, not asserted by a boolean label.
	if len(r.Genes) != r.GeneCount {
		return fmt.Errorf("gene_count=%d but genes rows=%d (not full coverage)", r.GeneCount, len(r.Genes))
	}
	seen := map[string]bool{}
	var sumMeanTopK float64
	type groupKey struct{ lower, upper int }
	groups := map[groupKey]int{}
	for i, g := range r.Genes {
		// Validate the value that is actually stored and rendered. Trimming for
		// validation while keeping the original string would let leading/trailing
		// whitespace reach the fact field path and the rendered candidate list.
		gene := g.Gene
		if !validIdentifier(gene) {
			return fmt.Errorf("invalid gene id at index %d", i)
		}
		if seen[gene] {
			return fmt.Errorf("duplicate gene id %q", gene)
		}
		seen[gene] = true
		if !finite(g.Kwithin) || !finite(g.KmeSigned) || !finite(g.KmeAbs) {
			return fmt.Errorf("non-finite metric for gene %q", gene)
		}
		if g.Kwithin < 0 {
			return fmt.Errorf("negative kwithin for gene %q", gene)
		}
		if g.KmeAbs < 0 || g.KmeAbs > 1+floatTolerance {
			return fmt.Errorf("kme_abs out of [0,1] for gene %q", gene)
		}
		if g.KmeSigned < -1-floatTolerance || g.KmeSigned > 1+floatTolerance {
			return fmt.Errorf("kme_signed out of [-1,1] for gene %q", gene)
		}
		if math.Abs(g.KmeAbs-math.Abs(g.KmeSigned)) > floatTolerance {
			return fmt.Errorf("kme_abs does not match |kme_signed| for gene %q", gene)
		}
		if g.RankLower < 1 || g.RankUpper > r.GeneCount || g.RankLower > g.RankUpper {
			return fmt.Errorf("invalid rank bounds for gene %q", gene)
		}
		if g.MeanTopK != nil {
			sumMeanTopK += *g.MeanTopK
		}
		groups[groupKey{g.RankLower, g.RankUpper}]++
		if err := validateGeneStats(r, g); err != nil {
			return err
		}
	}
	if r.Valid > 0 && math.Abs(sumMeanTopK-float64(r.TopK)) > meanTopKSumTolerance {
		return fmt.Errorf("mean_top_k sum=%.12g does not match top_k=%d", sumMeanTopK, r.TopK)
	}
	// rank_round produces a partition of 1..gene_count: each tie group occupies
	// exactly its interval width and groups are contiguous, in order.
	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].lower != keys[j].lower {
			return keys[i].lower < keys[j].lower
		}
		return keys[i].upper < keys[j].upper
	})
	next := 1
	for _, key := range keys {
		if key.lower != next {
			return fmt.Errorf("tie groups do not partition ranks: expected lower=%d, got [%d,%d]", next, key.lower, key.upper)
		}
		want := key.upper - key.lower + 1
		if groups[key] != want {
			return fmt.Errorf("tie group [%d,%d] has %d genes, want %d", key.lower, key.upper, groups[key], want)
		}
		next = key.upper + 1
	}
	if next != r.GeneCount+1 {
		return fmt.Errorf("tie groups cover ranks 1..%d, not 1..%d", next-1, r.GeneCount)
	}
	if err := validateRankDerivation(r); err != nil {
		return err
	}
	if err := validateSingleRoundStatistics(r); err != nil {
		return err
	}
	return validateTwoRoundMedianSum(r)
}

// validateTwoRoundMedianSum checks the exact aggregate identity for two valid
// rounds: every round's midranks form a complete ranking partition, so each
// round sums to n(n+1)/2; each gene's median is the mean of its two round
// midranks, so the medians also sum to n(n+1)/2. This identity does not extend
// to valid>=3 (per-gene medians do not preserve each round's total).
func validateTwoRoundMedianSum(r *PriorityResult) error {
	if r.Valid != 2 {
		return nil
	}
	var sum float64
	for i := range r.Genes {
		sum += *r.Genes[i].MidrankMed
	}
	n := float64(r.GeneCount)
	required := n * (n + 1) / 2
	tol := 1e-9 * math.Max(1, math.Abs(required))
	if math.Abs(sum-required) > tol {
		return fmt.Errorf("two valid rounds: sum(midrank_median)=%.12g, want %.12g", sum, required)
	}
	return nil
}

// topKMembershipWeight mirrors the worker's membership_weight for one tie
// group [lower, upper] against top_k.
func topKMembershipWeight(lower, upper, topK int) float64 {
	if upper <= topK {
		return 1
	}
	if lower > topK {
		return 0
	}
	return float64(topK-lower+1) / float64(upper-lower+1)
}

// validateSingleRoundStatistics checks that a valid=1 result is realizable as
// one rank_round output: equal midranks form contiguous tie groups, each group's
// midrank is the exact group midpoint, and mean_top_k is that group's membership
// weight. With more than one valid round these per-round values are aggregated,
// so this check does not apply.
func validateSingleRoundStatistics(r *PriorityResult) error {
	if r.Valid != 1 {
		return nil
	}
	type group struct {
		mid     float64
		indexes []int
	}
	byMid := map[float64]*group{}
	for i := range r.Genes {
		g := r.Genes[i]
		grp := byMid[*g.MidrankMed]
		if grp == nil {
			grp = &group{mid: *g.MidrankMed}
			byMid[*g.MidrankMed] = grp
		}
		grp.indexes = append(grp.indexes, i)
	}
	groups := make([]*group, 0, len(byMid))
	for _, grp := range byMid {
		groups = append(groups, grp)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].mid < groups[j].mid })
	next := 1
	for _, grp := range groups {
		lower := next
		upper := lower + len(grp.indexes) - 1
		if upper > r.GeneCount {
			return errors.New("single valid round: tie group exceeds gene_count")
		}
		if grp.mid != float64(lower+upper)/2.0 {
			return fmt.Errorf("single valid round: midrank %.12g does not match group [%d,%d]", grp.mid, lower, upper)
		}
		want := topKMembershipWeight(lower, upper, r.TopK)
		for _, idx := range grp.indexes {
			if math.Abs(*r.Genes[idx].MeanTopK-want) > floatTolerance {
				return fmt.Errorf("single valid round: mean_top_k does not match membership for group [%d,%d]", lower, upper)
			}
		}
		next = upper + 1
	}
	if next != r.GeneCount+1 {
		return fmt.Errorf("single valid round: tie groups cover ranks 1..%d, not 1..%d", next-1, r.GeneCount)
	}
	return nil
}

// validateRankDerivation recomputes rank bounds from kWithin using the same
// stable descending sort and bounded-span tie rule as the worker's rank_round,
// then compares them to the reported bounds. This prevents a result from
// combining plausible-looking ranks with unrelated kWithin values.
func validateRankDerivation(r *PriorityResult) error {
	n := len(r.Genes)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return r.Genes[order[a]].Kwithin > r.Genes[order[b]].Kwithin
	})
	for i := 0; i < n; {
		j := i
		for j+1 < n && r.Genes[order[i]].Kwithin-r.Genes[order[j+1]].Kwithin <= r.TieTol {
			j++
		}
		lower, upper := i+1, j+1
		for _, idx := range order[i : j+1] {
			g := r.Genes[idx]
			if g.RankLower != lower || g.RankUpper != upper {
				return fmt.Errorf("rank bounds do not match kwithin order at gene index %d", idx)
			}
		}
		i = j + 1
	}
	return nil
}

func validateGeneStats(r *PriorityResult, g PriorityGene) error {
	if r.Valid == 0 {
		if g.MidrankMed != nil || g.MidrankP05 != nil || g.MidrankP95 != nil || g.MeanTopK != nil {
			return fmt.Errorf("gene %q: zero valid replicates require null resampling statistics", g.Gene)
		}
		return nil
	}
	if g.MidrankMed == nil || g.MidrankP05 == nil || g.MidrankP95 == nil || g.MeanTopK == nil {
		return fmt.Errorf("gene %q: missing resampling statistics with %d valid replicates", g.Gene, r.Valid)
	}
	for name, value := range map[string]float64{
		"midrank_median": *g.MidrankMed,
		"midrank_p05":    *g.MidrankP05,
		"midrank_p95":    *g.MidrankP95,
		"mean_top_k":     *g.MeanTopK,
	} {
		if !finite(value) {
			return fmt.Errorf("gene %q: non-finite %s", g.Gene, name)
		}
	}
	for name, value := range map[string]float64{
		"midrank_median": *g.MidrankMed,
		"midrank_p05":    *g.MidrankP05,
		"midrank_p95":    *g.MidrankP95,
	} {
		if value < 1 || value > float64(r.GeneCount) {
			return fmt.Errorf("gene %q: %s out of [1,%d]", g.Gene, name, r.GeneCount)
		}
	}
	if *g.MidrankP05 > *g.MidrankMed || *g.MidrankMed > *g.MidrankP95 {
		return fmt.Errorf("gene %q: require midrank_p05 <= median <= p95", g.Gene)
	}
	// With one valid round the worker computes all three quantiles from the same
	// single midrank, so they must be exactly equal.
	if r.Valid == 1 && (*g.MidrankP05 != *g.MidrankMed || *g.MidrankMed != *g.MidrankP95) {
		return fmt.Errorf("gene at ranks [%d,%d]: single valid replicate requires equal midrank quantiles", g.RankLower, g.RankUpper)
	}
	// With two valid rounds the worker interpolates P05/P95 between the same two
	// midranks whose average is the median: P05 + P95 = 2*median.
	if r.Valid == 2 {
		lhs := *g.MidrankP05 + *g.MidrankP95
		rhs := 2 * *g.MidrankMed
		tol := 1e-9 * math.Max(1, math.Abs(rhs))
		if math.Abs(lhs-rhs) > tol {
			return fmt.Errorf("gene at ranks [%d,%d]: two valid replicates require P05+P95=2*median", g.RankLower, g.RankUpper)
		}
		// Invert the worker's two-point interpolation: P05=a+0.05(b-a) and
		// P95=a+0.95(b-a), so the implied per-round midranks a,b must both
		// remain inside the valid rank range [1,gene_count].
		span := (*g.MidrankP95 - *g.MidrankP05) / 0.9
		lower := *g.MidrankP05 - 0.05*span
		upper := lower + span
		rankTol := 1e-9 * math.Max(1, float64(r.GeneCount))
		if lower < 1-rankTol || upper > float64(r.GeneCount)+rankTol {
			return fmt.Errorf("gene at ranks [%d,%d]: two-round quantiles imply midranks outside [1,%d]", g.RankLower, g.RankUpper, r.GeneCount)
		}
	}
	if *g.MeanTopK < 0 || *g.MeanTopK > 1 {
		return fmt.Errorf("gene %q: mean_top_k out of [0,1]", g.Gene)
	}
	// If a gene was never in the top K, every bootstrap midrank is above K; if
	// it was always in the top K, every midrank is at or below K. These are
	// robust cross-statistic constraints (they do not assume original ranks are
	// bootstrap ranks).
	if *g.MeanTopK == 0 && *g.MidrankP05 <= float64(r.TopK) {
		return fmt.Errorf("gene at ranks [%d,%d]: mean_top_k=0 contradicts midrank_p05<=top_k", g.RankLower, g.RankUpper)
	}
	if *g.MeanTopK == 1 && *g.MidrankP95 > float64(r.TopK) {
		return fmt.Errorf("gene at ranks [%d,%d]: mean_top_k=1 contradicts midrank_p95>top_k", g.RankLower, g.RankUpper)
	}
	return nil
}

// Suggestion is the read-only, structured response schema. ProjectID,
// ExecutionID and Provenance bind it to one exact bundle source.
type Suggestion struct {
	ProjectID    string         `json:"project_id"`
	ExecutionID  string         `json:"execution_id"`
	Provenance   string         `json:"provenance"`
	GeneIDs      []string       `json:"gene_ids"`
	FactIDs      []string       `json:"fact_ids"`
	Explanations []string       `json:"explanations"`
	Limitations  []string       `json:"limitations"`
	PlanPreview  []string       `json:"plan_preview"`
	Claims       map[string]any `json:"claims,omitempty"`
}

// ValidateSuggestion checks a suggestion against the sealed bundle: source
// provenance, every fact/gene reference, gene↔fact association, typed
// explanation conditions, enum allow-lists, and numeric claims (which must cite
// an already-referenced fact).
func (b *EvidenceBundle) ValidateSuggestion(s *Suggestion) error {
	err := b.validateSuggestion(s)
	if err == nil {
		return nil
	}
	var ce *ContractError
	if errors.As(err, &ce) {
		return ce
	}
	return contractError("suggestion_rejected", "suggestion rejected", err)
}

func (b *EvidenceBundle) validateSuggestion(s *Suggestion) error {
	if s == nil {
		return errors.New("empty suggestion")
	}
	st, err := b.sealedState()
	if err != nil {
		return err
	}
	if s.ProjectID != st.projectID || s.ExecutionID != st.executionID {
		return errors.New("suggestion bound to a different project/execution")
	}
	if s.Provenance == "" || s.Provenance != st.provenance {
		return errors.New("suggestion bound to a different source provenance")
	}
	if st.valid == 0 && len(s.GeneIDs) > 0 {
		return errors.New("no valid replicates: candidate recommendation not allowed")
	}
	referenced := map[string]bool{}
	for _, id := range s.FactIDs {
		if _, ok := st.factIndex[id]; !ok {
			return fmt.Errorf("fact %q does not exist in this execution", id)
		}
		if referenced[id] {
			return fmt.Errorf("duplicate fact %q", id)
		}
		referenced[id] = true
	}
	// Each candidate must have at least one fact bound to the SAME gene.
	seenGene := map[string]bool{}
	for _, g := range s.GeneIDs {
		if seenGene[g] {
			return fmt.Errorf("duplicate candidate %q", g)
		}
		seenGene[g] = true
		if !st.geneIndex[g] {
			return fmt.Errorf("gene %q not in the full candidate result", g)
		}
		has := false
		for _, id := range s.FactIDs {
			if st.factIndex[id].Gene == g {
				has = true
				break
			}
		}
		if !has {
			return fmt.Errorf("candidate %q has no associated fact", g)
		}
	}
	if err := validateExplanations(st, s); err != nil {
		return err
	}
	for _, l := range s.Limitations {
		if !allowedLimitations[l] {
			return fmt.Errorf("limitation %q not allowed", l)
		}
	}
	for _, p := range s.PlanPreview {
		if !allowedPlanPreview[p] {
			return fmt.Errorf("plan %q not allowed", p)
		}
	}
	for id, claimed := range s.Claims {
		if !referenced[id] {
			return fmt.Errorf("claim %q is not referenced in fact_ids", id)
		}
		f := st.factIndex[id]
		if !jsonEqual(f.Value, claimed) {
			return fmt.Errorf("claim for %q does not match the fact value", id)
		}
	}
	return nil
}

func (b *EvidenceBundle) sealedState() (*sealedBundle, error) {
	if b == nil || b.sealed == nil {
		return nil, errors.New("evidence bundle is not sealed; use BuildBundle")
	}
	return b.sealed, nil
}

// validateExplanations binds each explanation to facts for the SAME candidate
// gene. A fact for another gene (or an unrelated metric) can never justify it.
func validateExplanations(st *sealedBundle, s *Suggestion) error {
	midrank := map[string]bool{"midrank_median": true, "midrank_p05": true, "midrank_p95": true}
	meanTopK := map[string]bool{"mean_top_k": true}
	hasGeneFact := func(gene string, semantics map[string]bool) bool {
		for _, id := range s.FactIDs {
			f := st.factIndex[id]
			if f.Gene == gene && semantics[f.Semantics] {
				return true
			}
		}
		return false
	}
	for _, e := range s.Explanations {
		if !allowedExplanations[e] {
			return fmt.Errorf("explanation %q not allowed", e)
		}
		switch e {
		case "ranking_stability":
			if len(s.GeneIDs) == 0 {
				return errors.New("ranking_stability requires a candidate")
			}
			for _, g := range s.GeneIDs {
				if !hasGeneFact(g, midrank) {
					return fmt.Errorf("ranking_stability for %q requires a referenced midrank fact for the same gene", g)
				}
			}
		case "top_k_frequency":
			if len(s.GeneIDs) == 0 {
				return errors.New("top_k_frequency requires a candidate")
			}
			for _, g := range s.GeneIDs {
				if !hasGeneFact(g, meanTopK) {
					return fmt.Errorf("top_k_frequency for %q requires a referenced mean_top_k fact for the same gene", g)
				}
			}
		case "tie_indistinguishable":
			if len(s.GeneIDs) == 0 {
				return errors.New("tie_indistinguishable requires a candidate")
			}
			for _, g := range s.GeneIDs {
				tied := false
				for _, id := range s.FactIDs {
					f := st.factIndex[id]
					if f.Gene == g && f.Semantics == "tie" {
						if v, ok := f.Value.(bool); ok && v {
							tied = true
						}
					}
				}
				if !tied {
					return fmt.Errorf("tie_indistinguishable for %q requires a tied fact for the same gene", g)
				}
			}
		}
	}
	return nil
}

// DecodeSuggestion strictly decodes a suggestion: oversized input, non-JSON, a
// top-level null, unknown fields, and any trailing JSON/garbage are all rejected.
// Duplicate keys follow Go's last-key-wins policy (no sensitive fields exist).
func DecodeSuggestion(data []byte) (*Suggestion, error) {
	s, err := decodeSuggestion(data)
	if err != nil {
		return nil, contractError("invalid_suggestion_input", "suggestion input rejected", err)
	}
	return s, nil
}

func decodeSuggestion(data []byte) (*Suggestion, error) {
	if len(data) > 1<<20 {
		return nil, errors.New("suggestion too large")
	}
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" || len(trimmed) == 0 {
		return nil, errors.New("suggestion is null/empty")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Suggestion
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	var extra any
	switch err := dec.Decode(&extra); {
	case err == io.EOF:
		// exactly one object, nothing trailing
	case err == nil:
		return nil, errors.New("trailing JSON object after suggestion")
	default:
		return nil, fmt.Errorf("trailing garbage after suggestion: %w", err)
	}
	return &s, nil
}

// Evaluate is the single safe entry: Decode → Validate → Render. Callers cannot
// skip validation and render an unverified suggestion.
func (b *EvidenceBundle) Evaluate(raw []byte) (string, error) {
	s, err := DecodeSuggestion(raw)
	if err != nil {
		return "", err
	}
	if err := b.ValidateSuggestion(s); err != nil {
		return "", err
	}
	return b.renderSuggestion(s), nil
}

// OfflineSuggestion builds the deterministic, read-only fallback suggestion for
// this bundle. It never calls a model or reads external state. With no valid
// replicate it recommends no candidate; otherwise it selects the candidate with
// the lowest midrank median, breaking ties by larger mean_top_k then gene ID.
func (b *EvidenceBundle) OfflineSuggestion() (*Suggestion, error) {
	st, err := b.sealedState()
	if err != nil {
		return nil, contractError("offline_unavailable", "offline suggestion unavailable", err)
	}
	s := &Suggestion{
		ProjectID:    st.projectID,
		ExecutionID:  st.executionID,
		Provenance:   st.provenance,
		GeneIDs:      []string{},
		FactIDs:      []string{},
		Explanations: []string{"network_candidate_priority"},
		Limitations:  []string{},
		PlanPreview:  []string{"view_candidates", "export_tsv"},
	}
	if st.valid == 0 {
		return s, nil
	}

	// Index each gene's facts by semantics. Field paths are unique per gene, so
	// this map has no duplicate semantics for a valid bundle.
	byGene := map[string]map[string]string{}
	for _, f := range st.factIndex {
		if f.Gene == "" {
			continue
		}
		if byGene[f.Gene] == nil {
			byGene[f.Gene] = map[string]string{}
		}
		if _, exists := byGene[f.Gene][f.Semantics]; !exists {
			byGene[f.Gene][f.Semantics] = f.ID
		}
	}
	bestGene := ""
	var bestMid, bestMean float64
	for _, gene := range st.candidates {
		facts := byGene[gene]
		midID, ok := facts["midrank_median"]
		if !ok {
			continue
		}
		mid, ok := st.factIndex[midID].Value.(float64)
		if !ok {
			continue
		}
		mean := 0.0
		if id, ok := facts["mean_top_k"]; ok {
			mean, _ = st.factIndex[id].Value.(float64)
		}
		if bestGene == "" || mid < bestMid ||
			(mid == bestMid && (mean > bestMean || (mean == bestMean && gene < bestGene))) {
			bestGene, bestMid, bestMean = gene, mid, mean
		}
	}
	if bestGene == "" {
		return nil, contractError("offline_unavailable", "offline suggestion unavailable",
			errors.New("no candidate with usable midrank facts"))
	}
	s.GeneIDs = []string{bestGene}
	facts := byGene[bestGene]
	for _, semantics := range []string{
		"kwithin", "kme_signed", "kme_abs", "rank_lower", "rank_upper", "tie",
		"midrank_median", "midrank_p05", "midrank_p95", "mean_top_k",
	} {
		if id, ok := facts[semantics]; ok {
			s.FactIDs = append(s.FactIDs, id)
		}
	}
	if _, ok := facts["midrank_median"]; ok {
		s.Explanations = append(s.Explanations, "ranking_stability")
	}
	if _, ok := facts["mean_top_k"]; ok {
		s.Explanations = append(s.Explanations, "top_k_frequency")
	}
	if id, ok := facts["tie"]; ok {
		if tied, ok := st.factIndex[id].Value.(bool); ok && tied {
			s.Explanations = append(s.Explanations, "tie_indistinguishable")
		}
	}
	return s, nil
}

// renderSuggestion renders an already-validated suggestion with code-composed
// templates and ALWAYS appends the bundle's deterministic limitations. It is
// unexported: Evaluate is the only public entry (Decode -> Validate -> Render),
// so an unvalidated suggestion can never be rendered through the public API.
func (b *EvidenceBundle) renderSuggestion(s *Suggestion) string {
	st, err := b.sealedState()
	if err != nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(st.summary)
	sb.WriteString("\n")
	if len(s.GeneIDs) > 0 {
		sb.WriteString("候选（")
		if len(s.GeneIDs) < len(st.candidates) {
			sb.WriteString("子集 ")
		}
		sb.WriteString(strings.Join(s.GeneIDs, "、"))
		sb.WriteString("）")
	} else {
		sb.WriteString("未指定候选")
	}
	for _, e := range s.Explanations {
		switch e {
		case "network_candidate_priority":
			sb.WriteString("\n· 仅网络 kWithin 排名稳定性，无表型（GS）证据")
		case "ranking_stability":
			sb.WriteString("\n· midrank 是重采样下的并列平均排名（排名稳定性），非概率/显著性/置信区间")
		case "top_k_frequency":
			sb.WriteString("\n· mean_top_k 是前 K 名的分数入选频率（bootstrap 采样频率），非真实 hub 概率")
		case "tie_indistinguishable":
			sb.WriteString("\n· 该候选存在并列（排名区间重叠），视为不可区分，不强制唯一推荐")
		}
	}
	for _, p := range s.PlanPreview {
		switch p {
		case "view_candidates":
			sb.WriteString("\n· 可查看候选表")
		case "export_tsv":
			sb.WriteString("\n· 可导出候选 TSV")
		}
	}
	// Deterministic limitations are always rendered (never model-selected).
	for _, l := range st.limitations {
		switch l {
		case LimitationNoTrait:
			sb.WriteString("\n· 局限：无表型证据")
		case LimitationInvalidReplicates:
			sb.WriteString("\n· 局限：部分重复无效，条件于可计算重复")
		case LimitationNoValid:
			sb.WriteString("\n· 局限：全部重复无效，禁止推荐")
		case LimitationDirectionUndetermined:
			sb.WriteString("\n· 局限：部分轮 eigengene 方向无法确定")
		case LimitationBootstrapFrequencyNotProb:
			sb.WriteString("\n· 局限：采样频率不是真实 hub 概率或 p/FDR")
		}
	}
	return sb.String()
}

func jsonEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

// AdapterStatus is the deterministic offline fallback returned when no provider is
// configured or remote is disabled; it never silently switches providers.
type AdapterStatus struct {
	Configured bool   `json:"configured"`
	Mode       string `json:"mode"` // "deterministic_fallback"
	Message    string `json:"message"`
}

// OfflineStatus returns the deterministic fallback for an unconfigured adapter.
func OfflineStatus() AdapterStatus {
	return AdapterStatus{Configured: false, Mode: "deterministic_fallback", Message: "AI 未配置或远程关闭；仅提供确定性解释（无在线模型实测）"}
}
