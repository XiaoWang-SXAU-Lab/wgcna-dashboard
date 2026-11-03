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
	ID        string `json:"id"`
	FieldPath string `json:"field_path"`
	Value     any    `json:"value"`
	Unit      string `json:"unit,omitempty"`
	Scope     string `json:"scope"`
	Quality   string `json:"quality,omitempty"`
	Source    string `json:"source"`
	Gene      string `json:"gene,omitempty"`
	Semantics string `json:"semantics,omitempty"`
}

// Envelope mirrors project.CandidateReportEnvelope. The source contract requires
// params/inputs/upstream/validity to be present (never dropped).
type Envelope struct {
	ProjectID   string            `json:"project_id"`
	ExecutionID string            `json:"execution_id"`
	Engine      string            `json:"engine"`
	Params      json.RawMessage   `json:"params"`
	Inputs      map[string]string `json:"inputs"`
	Upstream    map[string]string `json:"upstream"`
	Validity    string            `json:"validity"`
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
	Resampling            string          `json:"resampling"`
	StrataColumn          string          `json:"strata_column"`
	StratumCount          int             `json:"stratum_count"`
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
type EvidenceBundle struct {
	ProjectID    string   `json:"project_id"`
	ExecutionID  string   `json:"execution_id"`
	Engine       string   `json:"engine"`
	Valid        int      `json:"valid"`
	Invalid      int      `json:"invalid"`
	Facts        []Fact   `json:"facts"`
	Candidates   []string `json:"candidates"`
	FullCoverage bool     `json:"full_coverage"`
	GeneCount    int      `json:"gene_count"`
	Summary      string   `json:"summary"`
	Limitations  []string `json:"limitations"`
	factIndex    map[string]Fact
	geneIndex    map[string]bool
}

const (
	ScopeNetworkCandidatePriority = "network_candidate_priority"

	LimitationNoTrait                   = "no_trait_evidence"
	LimitationInvalidReplicates         = "invalid_replicates"
	LimitationNoValid                   = "no_valid_replicates"
	LimitationDirectionUndetermined     = "direction_undetermined"
	LimitationBootstrapFrequencyNotProb = "bootstrap_frequency_not_probability"
)

var (
	AllowedExplanations = map[string]bool{
		"network_candidate_priority": true,
		"ranking_stability":          true,
		"top_k_frequency":            true,
		"tie_indistinguishable":      true,
	}
	AllowedLimitations = map[string]bool{
		LimitationNoTrait:                   true,
		LimitationInvalidReplicates:         true,
		LimitationNoValid:                   true,
		LimitationDirectionUndetermined:     true,
		LimitationBootstrapFrequencyNotProb: true,
	}
	AllowedPlanPreview = map[string]bool{
		"view_candidates": true,
		"export_tsv":      true,
	}
)

func factID(projectID, executionID, fieldPath string) string {
	h := sha256.Sum256([]byte(projectID + "\x00" + executionID + "\x00" + fieldPath))
	return hex.EncodeToString(h[:])
}

// BuildBundle parses a report envelope and builds the deterministic fact bundle.
// It enforces the source contract (validity empty, params/inputs/upstream present)
// and the full-result invariants (gene count/rows, unique ids, supported design,
// finite values, attempted=valid+invalid).
func BuildBundle(envelopeJSON []byte) (*EvidenceBundle, error) {
	var env Envelope
	if err := json.Unmarshal(envelopeJSON, &env); err != nil {
		return nil, fmt.Errorf("invalid envelope: %w", err)
	}
	if env.ProjectID == "" || env.ExecutionID == "" || env.Engine == "" {
		return nil, errors.New("envelope missing project/execution/engine identity")
	}
	if env.Validity != "" {
		return nil, fmt.Errorf("stale/not-run evidence (validity=%q)", env.Validity)
	}
	if len(env.Params) == 0 || env.Inputs == nil || env.Upstream == nil {
		return nil, errors.New("envelope missing params/inputs/upstream source")
	}

	var r PriorityResult
	if err := json.Unmarshal(env.Result, &r); err != nil {
		return nil, fmt.Errorf("invalid result: %w", err)
	}
	if r.Scope != "fixed_full_module" {
		return nil, fmt.Errorf("unsupported scope %q", r.Scope)
	}
	if !r.DesignConfirmed {
		return nil, errors.New("unsupported design: design_confirmed must be true")
	}
	if r.Resampling != "iid" && r.Resampling != "stratified" {
		return nil, fmt.Errorf("unsupported resampling %q", r.Resampling)
	}
	if r.TraitSource == "" {
		r.TraitSource = "none"
	}
	if r.Attempted != r.Valid+r.Invalid {
		return nil, fmt.Errorf("attempted(%d) != valid(%d)+invalid(%d)", r.Attempted, r.Valid, r.Invalid)
	}
	if r.GeneCount <= 0 {
		return nil, errors.New("gene_count must be positive")
	}
	// Full coverage must be proven by row count, not asserted by a boolean label.
	if len(r.Genes) != r.GeneCount {
		return nil, fmt.Errorf("gene_count=%d but genes rows=%d (not full coverage)", r.GeneCount, len(r.Genes))
	}

	b := &EvidenceBundle{
		ProjectID:    env.ProjectID,
		ExecutionID:  env.ExecutionID,
		Engine:       env.Engine,
		Valid:        r.Valid,
		Invalid:      r.Invalid,
		FullCoverage: true,
		GeneCount:    r.GeneCount,
		factIndex:    map[string]Fact{},
		geneIndex:    map[string]bool{},
	}

	add := func(fieldPath, gene, semantics string, value any, unit, quality string) {
		f := Fact{
			ID:        factID(env.ProjectID, env.ExecutionID, fieldPath),
			FieldPath: fieldPath,
			Value:     value,
			Unit:      unit,
			Scope:     ScopeNetworkCandidatePriority,
			Quality:   quality,
			Source:    "step8_hub_priority.json",
			Gene:      gene,
			Semantics: semantics,
		}
		b.Facts = append(b.Facts, f)
		b.factIndex[f.ID] = f
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
	add("result.seed", "", "seed", r.Seed, "", quality)
	add("result.resampling", "", "resampling", r.Resampling, "", quality)
	add("result.strata_column", "", "strata_column", r.StrataColumn, "", quality)
	add("result.stratum_count", "", "stratum_count", r.StratumCount, "strata", quality)
	add("result.design_confirmed", "", "design_confirmed", r.DesignConfirmed, "", quality)
	add("result.trait_source", "", "trait_source", r.TraitSource, "", quality)
	add("result.power", "", "power", r.Power, "", quality)
	add("result.network_type", "", "network_type", r.NetworkType, "", quality)
	add("result.direction_undetermined", "", "direction_undetermined", r.DirectionUndetermined, "rounds", quality)
	for _, ir := range r.InvalidReasons {
		add("result.invalid_reasons."+ir.Reason+".count", "", "invalid_reason", ir.Count, "", quality)
	}

	seen := map[string]bool{}
	for _, g := range r.Genes {
		if strings.TrimSpace(g.Gene) == "" {
			return nil, errors.New("empty gene id in result")
		}
		if seen[g.Gene] {
			return nil, fmt.Errorf("duplicate gene id %q", g.Gene)
		}
		seen[g.Gene] = true
		if math.IsNaN(g.Kwithin) || math.IsInf(g.Kwithin, 0) || math.IsNaN(g.KmeAbs) || math.IsInf(g.KmeAbs, 0) {
			return nil, fmt.Errorf("non-finite metric for gene %q", g.Gene)
		}
		if g.RankLower < 1 || g.RankUpper > r.GeneCount || g.RankLower > g.RankUpper {
			return nil, fmt.Errorf("invalid rank bounds for gene %q", g.Gene)
		}
		if g.MidrankMed != nil && (*g.MidrankMed < 1 || *g.MidrankMed > float64(r.GeneCount)) {
			return nil, fmt.Errorf("midrank out of range for gene %q", g.Gene)
		}
		if g.MeanTopK != nil && (*g.MeanTopK < 0 || *g.MeanTopK > 1) {
			return nil, fmt.Errorf("mean_top_k out of [0,1] for gene %q", g.Gene)
		}
		b.Candidates = append(b.Candidates, g.Gene)
		b.geneIndex[g.Gene] = true
		gq := quality
		if g.MidrankMed == nil {
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
	sort.Strings(b.Candidates)

	b.Limitations = []string{LimitationNoTrait, LimitationBootstrapFrequencyNotProb}
	if r.Valid == 0 {
		b.Limitations = append(b.Limitations, LimitationNoValid)
	} else if r.Invalid > 0 {
		b.Limitations = append(b.Limitations, LimitationInvalidReplicates)
	}
	if r.DirectionUndetermined > 0 {
		b.Limitations = append(b.Limitations, LimitationDirectionUndetermined)
	}

	b.Summary = fmt.Sprintf(
		"候选优先级（hub_priority）· 模块 %s · %d 基因（完整）· 有效 %d/%d · K %d · B %d · 无表型证据（网络候选优先级）",
		r.Module, r.GeneCount, r.Valid, r.Attempted, r.TopK, r.NBootstraps,
	)
	return b, nil
}

// Suggestion is the read-only, structured response schema. ProjectID/ExecutionID
// bind it to a specific bundle execution.
type Suggestion struct {
	ProjectID    string         `json:"project_id"`
	ExecutionID  string         `json:"execution_id"`
	GeneIDs      []string       `json:"gene_ids"`
	FactIDs      []string       `json:"fact_ids"`
	Explanations []string       `json:"explanations"`
	Limitations  []string       `json:"limitations"`
	PlanPreview  []string       `json:"plan_preview"`
	Claims       map[string]any `json:"claims,omitempty"`
}

// ValidateSuggestion checks a suggestion against the bundle: identity, every
// fact/gene reference, gene↔fact association, explanation conditions, enum
// allow-lists, and numeric claims (which must cite an already-referenced fact).
func (b *EvidenceBundle) ValidateSuggestion(s *Suggestion) error {
	if s == nil {
		return errors.New("empty suggestion")
	}
	if s.ProjectID != b.ProjectID || s.ExecutionID != b.ExecutionID {
		return errors.New("suggestion bound to a different project/execution")
	}
	if b.Valid == 0 && len(s.GeneIDs) > 0 {
		return errors.New("no valid replicates: candidate recommendation not allowed")
	}
	referenced := map[string]bool{}
	for _, id := range s.FactIDs {
		if _, ok := b.factIndex[id]; !ok {
			return fmt.Errorf("fact %q does not exist in this execution", id)
		}
		referenced[id] = true
	}
	// Each candidate must have at least one fact bound to the SAME gene.
	for _, g := range s.GeneIDs {
		if !b.geneIndex[g] {
			return fmt.Errorf("gene %q not in the full candidate result", g)
		}
		has := false
		for _, id := range s.FactIDs {
			if f := b.factIndex[id]; f.Gene == g {
				has = true
				break
			}
		}
		if !has {
			return fmt.Errorf("candidate %q has no associated fact", g)
		}
	}
	for _, e := range s.Explanations {
		if !AllowedExplanations[e] {
			return fmt.Errorf("explanation %q not allowed", e)
		}
	}
	// tie_indistinguishable requires an actually-tied referenced gene fact.
	hasTie := false
	for _, id := range s.FactIDs {
		if f := b.factIndex[id]; f.Semantics == "tie" {
			if v, ok := f.Value.(bool); ok && v {
				hasTie = true
			}
		}
	}
	for _, e := range s.Explanations {
		if e == "tie_indistinguishable" && !hasTie {
			return errors.New("tie_indistinguishable requires a tied gene fact")
		}
	}
	for _, l := range s.Limitations {
		if !AllowedLimitations[l] {
			return fmt.Errorf("limitation %q not allowed", l)
		}
	}
	for _, p := range s.PlanPreview {
		if !AllowedPlanPreview[p] {
			return fmt.Errorf("plan %q not allowed", p)
		}
	}
	for id, claimed := range s.Claims {
		if !referenced[id] {
			return fmt.Errorf("claim %q is not referenced in fact_ids", id)
		}
		f := b.factIndex[id]
		if !jsonEqual(f.Value, claimed) {
			return fmt.Errorf("claim for %q does not match the fact value", id)
		}
	}
	return nil
}

// DecodeSuggestion strictly decodes a suggestion: oversized input, non-JSON, a
// top-level null, unknown fields, and any trailing JSON/garbage are all rejected.
// Duplicate keys follow Go's last-key-wins policy (no sensitive fields exist).
func DecodeSuggestion(data []byte) (*Suggestion, error) {
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
	return b.RenderSuggestion(s), nil
}

// RenderSuggestion renders an already-validated suggestion with code-composed
// templates and ALWAYS appends the bundle's deterministic limitations. It never
// emits raw model text as a verified conclusion.
func (b *EvidenceBundle) RenderSuggestion(s *Suggestion) string {
	var sb strings.Builder
	sb.WriteString(b.Summary)
	sb.WriteString("\n")
	if len(s.GeneIDs) > 0 {
		sb.WriteString("候选（")
		if len(s.GeneIDs) < len(b.Candidates) {
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
	for _, l := range b.Limitations {
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
