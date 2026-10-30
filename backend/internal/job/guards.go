package job

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/XiaoWang-SXAU-Lab/wgcna-dashboard/internal/inputcheck"
)

type GuardError struct {
	Status        int
	Code, Message string
}

func (e *GuardError) Error() string                { return e.Message }
func guard(status int, code, message string) error { return &GuardError{status, code, message} }

// Defaults mirror the worker. Canonical sections make omitted defaults and
// explicit defaults equivalent when checking whether upstream results are stale.
var defaults = map[string]map[string]any{
	"filter":         {"data_type": "count", "normalize_method": "log10p1", "n_keep": 8000.0, "filter_method": "mad", "sample_percentage": 0.0, "cutoff": 1.0, "exclude_samples": []any{}},
	"soft_threshold": {"network_type": "unsigned", "rsquared_cut": .85},
	"network":        {"power": 6.0, "network_type": "unsigned", "tom_type": "signed", "tom_denom": "min", "min_module_size": 30.0, "merge_cut_height": .25, "deep_split": 2.0, "detect_cut_height": .995},
	"module_trait":   {"trait_source": "uploaded", "trait_column": ""},
	"module_detail":  {"select_module": "", "select_trait": ""},
	"hub":            {"select_module": "", "select_trait": "", "kme_cutoff": .8, "gs_cutoff": 0.0, "weight_threshold": .02, "power": 6.0, "top_n_neighbors": 20.0, "max_hubs": 30.0},
	"cluster":        {"method": "kmeans", "k": 0.0, "seed": 42.0, "kmax": 12.0},
	"overview":       {"edge_percentile": 10.0},
}

type paramSet map[string]map[string]any

func decodeParams(raw []byte) (paramSet, error) {
	var supplied map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &supplied); err != nil || supplied == nil {
			return nil, guard(422, "parameters", "Parameters must be a JSON object.")
		}
	}
	p := paramSet{}
	for section, fields := range defaults {
		out := map[string]any{}
		for k, v := range fields {
			out[k] = v
		}
		if b, ok := supplied[section]; ok {
			var user map[string]any
			if err := json.Unmarshal(b, &user); err != nil || user == nil {
				return nil, guard(422, "parameters", "Invalid parameter section: "+section)
			}
			for k, v := range user {
				if _, known := fields[k]; known {
					if v == nil {
						return nil, guard(422, "parameters", "Empty parameter: "+section+"."+k)
					}
					out[k] = v
				}
			}
		}
		p[section] = out
	}
	for section, fields := range p {
		for k, v := range fields {
			switch defaults[section][k].(type) {
			case float64:
				if n, ok := v.(float64); !ok || math.IsNaN(n) || math.IsInf(n, 0) {
					return nil, guard(422, "parameters", "Enter a finite number for "+section+"."+k)
				}
			case string:
				if _, ok := v.(string); !ok {
					return nil, guard(422, "parameters", "Invalid text value for "+section+"."+k)
				}
			case []any:
				a, ok := v.([]any)
				if !ok {
					return nil, guard(422, "parameters", "Excluded samples must be a list of IDs.")
				}
				for _, id := range a {
					if _, ok := id.(string); !ok {
						return nil, guard(422, "parameters", "Excluded sample IDs must be text.")
					}
				}
			}
		}
	}
	ranges := []struct {
		s, k    string
		lo, hi  float64
		integer bool
	}{
		{"filter", "n_keep", 2, 1e7, true}, {"filter", "sample_percentage", 0, 1, false}, {"filter", "cutoff", 0, math.MaxFloat64, false},
		{"soft_threshold", "rsquared_cut", 0, 1, false}, {"network", "power", .01, 100, false}, {"network", "min_module_size", 2, 1e7, true},
		{"network", "merge_cut_height", 0, 1, false}, {"network", "deep_split", 0, 4, true}, {"network", "detect_cut_height", 0, 1, false},
		{"hub", "kme_cutoff", 0, 1, false}, {"hub", "gs_cutoff", 0, 1, false}, {"hub", "weight_threshold", 0, 1, false}, {"hub", "power", .01, 100, false},
		{"hub", "top_n_neighbors", 1, 10000, true}, {"hub", "max_hubs", 1, 10000, true}, {"cluster", "k", 0, 10000, true},
		{"cluster", "kmax", 2, 1000, true}, {"cluster", "seed", 0, 9007199254740991, true}, {"overview", "edge_percentile", .001, 100, false},
	}
	for _, r := range ranges {
		v := p[r.s][r.k].(float64)
		if v < r.lo || v > r.hi || (r.integer && v != math.Trunc(v)) {
			return nil, guard(422, "parameters", fmt.Sprintf("%s.%s must be %sbetween %g and %g.", r.s, r.k, map[bool]string{true: "an integer ", false: ""}[r.integer], r.lo, r.hi))
		}
	}
	enums := map[string]map[string][]string{
		"filter":         {"data_type": {"count", "expected_count", "normalized_count", "peak_area", "protein_abundance"}, "normalize_method": {"raw", "vst", "log10p1", "log2p1", "log", "log10", "log2"}, "filter_method": {"mad", "var"}},
		"soft_threshold": {"network_type": {"signed", "unsigned", "signed hybrid"}}, "network": {"network_type": {"signed", "unsigned", "signed hybrid"}, "tom_type": {"signed", "unsigned"}, "tom_denom": {"min", "mean"}},
		"module_trait": {"trait_source": {"uploaded", "sample_info"}}, "cluster": {"method": {"kmeans", "cmeans"}},
	}
	for section, fields := range enums {
		for key, options := range fields {
			if !contains(options, p[section][key].(string)) {
				return nil, guard(422, "parameters", "Unsupported value for "+section+"."+key)
			}
		}
	}
	return p, nil
}
func canonicalSection(p paramSet, step string) []byte { b, _ := json.Marshal(p[step]); return b }
func mergeParams(old, patch []byte) ([]byte, error) {
	var values map[string]json.RawMessage
	if len(old) > 0 {
		if err := json.Unmarshal(old, &values); err != nil {
			return nil, guard(422, "parameters", "Saved parameters are damaged; select the inputs again to create a new job.")
		}
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	if len(patch) > 0 {
		var updates map[string]json.RawMessage
		if err := json.Unmarshal(patch, &updates); err != nil || updates == nil {
			return nil, guard(422, "parameters", "Parameters must be a JSON object.")
		}
		for k, v := range updates {
			values[k] = v
		}
	}
	return json.Marshal(values)
}

func prerequisites(step string) []string {
	switch step {
	case "filter":
		return nil
	case "soft_threshold":
		return []string{"filter"}
	case "network":
		return []string{"filter", "soft_threshold"}
	case "module_trait":
		return []string{"filter", "soft_threshold", "network"}
	case "module_detail", "hub":
		return []string{"filter", "soft_threshold", "network", "module_trait"}
	case "cluster", "cluster_elbow":
		return []string{"filter"}
	case "overview_network":
		return []string{"filter", "soft_threshold", "network"}
	}
	return nil
}

func readTable(path, kind string) (*inputcheck.Table, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, guard(422, "inputs", "Required input is missing: "+filepath.Base(path)+". Upload it or rerun the preceding step.")
	}
	table, r := inputcheck.Parse(b, kind, "normalized_count")
	if !r.Valid {
		return nil, guard(422, "inputs", r.Error())
	}
	return table, nil
}
func variableColumn(t *inputcheck.Table, col int) bool {
	first, _ := strconv.ParseFloat(t.Rows[0][col], 64)
	for _, row := range t.Rows[1:] {
		v, _ := strconv.ParseFloat(row[col], 64)
		if v != first {
			return true
		}
	}
	return false
}
func (m *Manager) validateRun(id, step string, p paramSet) error {
	dir := filepath.Join(m.dir, id)
	if step == "filter" {
		raw, err := os.ReadFile(filepath.Join(dir, "expr.tsv"))
		if err != nil {
			return err
		}
		t, r := m.CheckInput(raw, "expr", p["filter"]["data_type"].(string))
		if !r.Valid {
			return guard(422, "inputs", r.Error())
		}
		f := p["filter"]
		method := f["normalize_method"].(string)
		if method == "vst" && f["data_type"] != "count" && f["data_type"] != "expected_count" {
			return guard(422, "normalization", "VST requires count or expected_count data. Select a compatible normalization method.")
		}
		excluded := map[string]bool{}
		for _, v := range f["exclude_samples"].([]any) {
			excluded[v.(string)] = true
		}
		for id := range excluded {
			if !contains(t.Samples(), id) {
				return guard(422, "samples", "Excluded sample is not in the expression matrix: "+id)
			}
		}
		if len(t.Samples())-len(excluded) < 3 {
			return guard(422, "samples", "At least 3 samples must remain. Restore samples or select fewer outliers.")
		}
		usable := 0
		for _, row := range t.Rows {
			values := []float64{}
			below := 0
			minv, maxv := math.Inf(1), math.Inf(-1)
			for j, cell := range row[1:] {
				if excluded[t.Header[j+1]] {
					continue
				}
				v, _ := strconv.ParseFloat(cell, 64)
				if ((method == "log" || method == "log2" || method == "log10") && v <= 0) || ((method == "log2p1" || method == "log10p1") && v <= -1) {
					return guard(422, "normalization", fmt.Sprintf("%s is undefined for value %g at gene %s / sample %s. Choose raw, a valid log(x+1) transform, or correct the input; no pseudocount was added.", method, v, row[0], t.Header[j+1]))
				}
				if v < f["cutoff"].(float64) {
					below++
				}
				values = append(values, v)
				minv = math.Min(minv, v)
				maxv = math.Max(maxv, v)
			}
			if f["sample_percentage"].(float64) > 0 && float64(below) > f["sample_percentage"].(float64)*float64(len(values)) {
				continue
			}
			variable := maxv > minv || method == "vst"
			if method != "vst" && f["filter_method"] == "mad" {
				sort.Float64s(values)
				median := func(v []float64) float64 {
					n := len(v)
					if n%2 == 1 {
						return v[n/2]
					}
					return (v[n/2-1] + v[n/2]) / 2
				}
				center := median(values)
				for i, v := range values {
					values[i] = math.Abs(v - center)
				}
				sort.Float64s(values)
				variable = median(values) > 0
			}
			if variable {
				usable++
			}
		}
		if usable < 2 {
			return guard(422, "filter_empty", "Fewer than 2 variable genes survive these filter settings. Lower the cutoff, relax the sample percentage, choose VAR instead of MAD, or check the expression matrix.")
		}
		return nil
	}
	t, err := readTable(filepath.Join(dir, "step1_filtered.tsv"), "expr")
	if err != nil {
		return err
	}
	if len(t.Rows) < 2 {
		return guard(422, "filter_empty", "Too few filtered genes. Adjust filtering and rerun Data import.")
	}
	if step == "network" && p["network"]["network_type"] != p["soft_threshold"]["network_type"] {
		return guard(422, "stale_parameters", "Network type differs from the soft-threshold settings. Choose the same network type and rerun soft_threshold first.")
	}
	if step == "network" && p["network"]["min_module_size"].(float64) > float64(len(t.Rows)) {
		return guard(422, "parameters", fmt.Sprintf("min_module_size exceeds the %d retained genes. Reduce it or retain more genes.", len(t.Rows)))
	}
	if step == "cluster" || step == "cluster_elbow" {
		c := p["cluster"]
		if c["k"].(float64) == 1 || c["k"].(float64) > float64(len(t.Rows)) {
			return guard(422, "parameters", "Cluster k must be 0 (automatic) or between 2 and the retained gene count.")
		}
		if (step == "cluster_elbow" || c["k"].(float64) == 0) && c["kmax"].(float64) > float64(len(t.Rows)) {
			return guard(422, "parameters", fmt.Sprintf("kmax exceeds the %d retained genes; reduce kmax.", len(t.Rows)))
		}
	}
	if step == "module_trait" || step == "module_detail" || step == "hub" {
		var modules struct {
			Colors []string `json:"colors"`
		}
		b, e := os.ReadFile(filepath.Join(dir, "step3_modules.json"))
		if e != nil || json.Unmarshal(b, &modules) != nil {
			return guard(422, "dependencies", "Rerun Network & modules to prepare valid modules.")
		}
		nonGrey := false
		for _, c := range modules.Colors {
			if c != "grey" {
				nonGrey = true
			}
		}
		if !nonGrey {
			return guard(422, "no_modules", "All genes are grey/unassigned. Adjust module detection settings or retain more variable genes, then rerun Network & modules.")
		}
		traits := []string{}
		if p["module_trait"]["trait_source"] == "sample_info" {
			meta, e := readTable(filepath.Join(dir, "sample_info.tsv"), "sample_info")
			if e != nil {
				return e
			}
			meta, r := inputcheck.Align(meta, t.Samples(), nil, "sample_info")
			if !r.Valid {
				return guard(422, "samples", r.Error())
			}
			column := p["module_trait"]["trait_column"].(string)
			col := -1
			for j, h := range meta.Header {
				if j > 0 && h == column {
					col = j
				}
			}
			if col < 1 {
				return guard(422, "trait_source", "Select an available sample metadata column before generating traits.")
			}
			categories := map[string]bool{}
			for _, row := range meta.Rows {
				if row[col] == "" {
					return guard(422, "trait_source", "Missing metadata value for sample "+row[0]+" in "+column+". Fill it before generating traits.")
				}
				categories[row[col]] = true
			}
			if len(categories) < 2 {
				return guard(422, "constant_trait", "The chosen metadata column has only one category among retained samples. Select a variable column or restore samples.")
			}
			for c := range categories {
				traits = append(traits, c)
			}
		} else {
			// Use original rows when excluding outliers; do not mutate sample inputs
			// during a rejected run request.
			path := filepath.Join(dir, "trait.uploaded.tsv")
			if _, e := os.Stat(path); e != nil {
				path = filepath.Join(dir, "trait.tsv")
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return guard(422, "trait_source", "Upload a numeric trait table or choose a sample metadata column as the trait source.")
			}
			trait, r := m.CheckInput(b, "trait", "")
			if !r.Valid {
				return guard(422, "inputs", r.Error())
			}
			all, e := m.expressionSamples(id)
			if e != nil {
				return e
			}
			excluded := []string{}
			for _, s := range all {
				if !contains(t.Samples(), s) {
					excluded = append(excluded, s)
				}
			}
			trait, r = inputcheck.Align(trait, t.Samples(), excluded, "trait")
			if !r.Valid {
				return guard(422, "samples", r.Error())
			}
			for j, name := range trait.Header[1:] {
				if !variableColumn(trait, j+1) {
					return guard(422, "constant_trait", "Trait has zero variance among retained samples: "+name+". Remove that column, choose a variable trait source, or restore samples.")
				}
			}
			traits = trait.Header[1:]
		}
		if step == "module_detail" || step == "hub" {
			selectp := p[step]
			module := selectp["select_module"].(string)
			trait := selectp["select_trait"].(string)
			if module == "" || module == "grey" || !contains(modules.Colors, module) {
				return guard(422, "selection", "Select an available non-grey module before running this step.")
			}
			if trait == "" || !contains(traits, trait) {
				return guard(422, "selection", "Select an available trait before running this step.")
			}
		}
	}
	return nil
}

func sameSection(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}
func issueSummary(err error) error {
	if _, ok := err.(*GuardError); ok {
		return err
	}
	return guard(422, "inputs", strings.TrimSpace(err.Error()))
}

// ReserveSnapshot prevents a source job from being changed while a report is
// taking its immutable input snapshot. Historical report-only jobs are allowed.
func (m *Manager) ReserveSnapshot(id string) (func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel[id] != nil {
		return nil, guard(409, "busy", "Wait for the running analysis to finish before generating a report.")
	}
	if j, ok := m.jobs[id]; ok && !contains(j.DoneSteps, "network") {
		return nil, guard(422, "dependencies", "Complete network successfully before generating a report.")
	}
	m.readers[id]++
	var once sync.Once
	return func() { once.Do(func() { m.mu.Lock(); m.readers[id]--; m.mu.Unlock() }) }, nil
}
