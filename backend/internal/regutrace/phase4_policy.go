package regutrace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MotifPolicy selects which motif hypotheses contribute to a TF target set.
// It never changes the stored mapping or evidence; it is a query/export view.
type MotifPolicy string

const (
	MotifPolicyAll  MotifPolicy = "all"
	MotifPolicyBest MotifPolicy = "best"
	MotifPolicyReps MotifPolicy = "reps"
)

type PolicyInfo struct {
	Policy               string `json:"policy"`
	SelectedMotifs       int    `json:"selected_motifs"`
	FixedMappings        int    `json:"fixed_mappings"`
	ExploratorySelected  int    `json:"exploratory_selected"`
	CoordinateAvailable  bool   `json:"coordinate_available"`
	SpecificityAvailable bool   `json:"specificity_available"`
	UncoordinatedMotifs  int    `json:"uncoordinated_motifs,omitempty"`
	Fallback             bool   `json:"fallback"`
	Note                 string `json:"note,omitempty"`
}

func normalizeMotifPolicy(value string) (MotifPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(MotifPolicyAll):
		return MotifPolicyAll, nil
	case string(MotifPolicyBest):
		return MotifPolicyBest, nil
	case string(MotifPolicyReps):
		return MotifPolicyReps, nil
	default:
		return "", fmt.Errorf("motif_policy must be all, best or reps")
	}
}

func mappingFloat(value string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return v
}

func policyRank(row TFMappingRow) (float64, float64, float64, float64) {
	full := 0.0
	if row.DomainMode == "full_architecture" {
		full = 1
	}
	qcov := mappingFloat(row.QueryDBDCoverage)
	dcov := mappingFloat(row.DonorDBDCoverage)
	cov := qcov
	if dcov < cov {
		cov = dcov
	}
	return full, cov, mappingFloat(row.DBDIdentity), row.DBDBitscore
}

func rankBetter(a, b TFMappingRow) bool {
	af, ac, ai, ab := policyRank(a)
	bf, bc, bi, bb := policyRank(b)
	if af != bf {
		return af > bf
	}
	if ac != bc {
		return ac > bc
	}
	if ai != bi {
		return ai > bi
	}
	if ab != bb {
		return ab > bb
	}
	return a.MotifID < b.MotifID
}

func policyOverlap(a, b TFMappingRow) float64 {
	if a.QueryDBDStart <= 0 || a.QueryDBDEnd < a.QueryDBDStart || b.QueryDBDStart <= 0 || b.QueryDBDEnd < b.QueryDBDStart {
		return 0
	}
	start := a.QueryDBDStart
	if b.QueryDBDStart > start {
		start = b.QueryDBDStart
	}
	end := a.QueryDBDEnd
	if b.QueryDBDEnd < end {
		end = b.QueryDBDEnd
	}
	if end < start {
		return 0
	}
	inter := end - start + 1
	la := a.QueryDBDEnd - a.QueryDBDStart + 1
	lb := b.QueryDBDEnd - b.QueryDBDStart + 1
	if la < lb {
		return inter / la
	}
	return inter / lb
}

func policyGroups(rows []TFMappingRow) [][]TFMappingRow {
	byArch := map[string][]TFMappingRow{}
	for _, row := range rows {
		key := row.ExtractionFamily + "|" + row.DomainMode + "|" + row.DomainCount
		byArch[key] = append(byArch[key], row)
	}
	var groups [][]TFMappingRow
	for _, part := range byArch {
		if len(part) == 0 {
			continue
		}
		// RAV carries two DNA-binding domains that act as one composite
		// architecture; do not split it into independent motif assignments.
		composite := part[0].ExtractionFamily == "RAV"
		hasCoordinates := false
		for _, row := range part {
			if row.QueryDBDStart > 0 && row.QueryDBDEnd >= row.QueryDBDStart {
				hasCoordinates = true
				break
			}
		}
		parent := make([]int, len(part))
		for i := range parent {
			parent[i] = i
		}
		var find func(int) int
		find = func(i int) int {
			if parent[i] != i {
				parent[i] = find(parent[i])
			}
			return parent[i]
		}
		for i := range part {
			for k := i + 1; k < len(part); k++ {
				if !hasCoordinates || composite || policyOverlap(part[i], part[k]) >= 0.8 {
					ri, rk := find(i), find(k)
					if ri != rk {
						parent[rk] = ri
					}
				}
			}
		}
		byRoot := map[int][]TFMappingRow{}
		for i, row := range part {
			root := find(i)
			byRoot[root] = append(byRoot[root], row)
		}
		for _, group := range byRoot {
			groups = append(groups, group)
		}
	}
	sort.Slice(groups, func(i, k int) bool {
		if len(groups[i]) == 0 || len(groups[k]) == 0 {
			return len(groups[i]) > len(groups[k])
		}
		return groups[i][0].MotifID < groups[k][0].MotifID
	})
	return groups
}

func exactPWMKey(row TFMappingRow, motifs map[string]MotifInfo) string {
	if meta, ok := motifs[row.MotifID]; ok && meta.PWMHash != "" {
		return meta.PWMHash
	}
	return row.MotifID
}

func bestUnique(rows []TFMappingRow, motifs map[string]MotifInfo) []TFMappingRow {
	best := map[string]TFMappingRow{}
	for _, row := range rows {
		key := exactPWMKey(row, motifs)
		current, ok := best[key]
		if !ok || rankBetter(row, current) {
			best[key] = row
		}
	}
	out := make([]TFMappingRow, 0, len(best))
	for _, row := range best {
		out = append(out, row)
	}
	return out
}

func (j *jobState) specificityKeys(row TFMappingRow) []string {
	if j.phase4 != nil {
		if groups := j.phase4.SpecificityGroups[row.MotifID]; len(groups) > 0 {
			return groups
		}
	}
	raw := strings.TrimSpace(row.SpecificityGroups)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '|' || r == ';' || r == ',' })
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func (j *jobState) specificityAvailable(rows []TFMappingRow) bool {
	for _, row := range rows {
		if len(j.specificityKeys(row)) > 0 {
			return true
		}
	}
	return false
}

// representativeRows keeps the best row for each specificity group. Rows
// without a specificity group are keyed by motif ID, matching the Phase 5
// audit. When no specificity data exist at all, every exact PWM remains its
// own fallback group, so the result is the deduplicated exploratory set.
func (j *jobState) representativeRows(unique []TFMappingRow, useSpecificity bool) []TFMappingRow {
	if len(unique) == 0 || !useSpecificity {
		return unique
	}
	seen := map[string]bool{}
	out := make([]TFMappingRow, 0, len(unique))
	for _, row := range unique {
		keys := j.specificityKeys(row)
		if len(keys) == 0 {
			keys = []string{"__motif__:" + row.MotifID}
		}
		selected := false
		for _, key := range keys {
			if !seen[key] {
				selected = true
			}
		}
		if !selected {
			continue
		}
		out = append(out, row)
		for _, key := range keys {
			seen[key] = true
		}
	}
	return out
}

func rowHasCoordinates(row TFMappingRow) bool {
	return row.QueryDBDStart > 0 && row.QueryDBDEnd >= row.QueryDBDStart
}

func hasCoordinates(rows []TFMappingRow) bool {
	for _, row := range rows {
		if rowHasCoordinates(row) {
			return true
		}
	}
	return false
}

// coordinateExploratory drops exploratory rows absent from the coordinate
// rerun. They remain in the all-candidate view but are not eligible for
// coordinate-group best/reps selection. If the whole TF has no coordinates,
// the caller uses the documented opaque fallback instead.
func coordinateExploratory(rows []TFMappingRow) (with, without []TFMappingRow, motifsWithout int) {
	seen := map[string]bool{}
	for _, row := range rows {
		if rowHasCoordinates(row) {
			with = append(with, row)
		} else {
			without = append(without, row)
			if !seen[row.MotifID] {
				seen[row.MotifID] = true
				motifsWithout++
			}
		}
	}
	return with, without, motifsWithout
}

func splitPolicyRows(rows []TFMappingRow, includeSingleDomain bool) (fixed, exploratory []TFMappingRow) {
	for _, row := range rows {
		switch row.Status {
		case "strict_rbh", "dbd_calibrated":
			fixed = append(fixed, row)
		case "dbd_exploratory":
			if row.DomainMode == "single_domain_fallback" && !includeSingleDomain {
				continue
			}
			exploratory = append(exploratory, row)
		}
	}
	return fixed, exploratory
}

func (j *jobState) policyAllowed(tfID string, policy MotifPolicy) (map[string]bool, PolicyInfo) {
	info := j.policySelection(tfID, policy)
	if policy == MotifPolicyAll || len(j.mappingRows(tfID)) == 0 {
		return nil, info
	}
	motifs := j.policyMotifs(tfID, policy)
	allowed := make(map[string]bool, len(motifs))
	for _, motif := range motifs {
		allowed[motif] = true
	}
	return allowed, info
}

// policySelection computes the query/export view. Fixed strict/calibrated
// mappings are always retained; exploratory mappings are never silently
// promoted. Ambiguous mappings remain in "all" but are excluded from
// best/reps target sets.
func (j *jobState) policySelection(tfID string, policy MotifPolicy) PolicyInfo {
	rows := j.mappingRows(tfID)
	info := PolicyInfo{Policy: string(policy)}
	if len(rows) == 0 {
		info.Note = "no TF-motif mapping rows"
		return info
	}
	info.CoordinateAvailable = hasCoordinates(rows)
	info.SpecificityAvailable = j.specificityAvailable(rows)
	if policy == MotifPolicyAll {
		seen := map[string]bool{}
		for _, row := range rows {
			if !seen[row.MotifID] {
				seen[row.MotifID] = true
				info.SelectedMotifs++
			}
		}
		info.FixedMappings = countFixed(rows)
		return info
	}
	fixed, exploratory := splitPolicyRows(rows, j.Params.IncludeSingleDomainFallback)
	info.FixedMappings = len(fixed)
	selected := append([]TFMappingRow{}, fixed...)
	if len(exploratory) > 0 {
		if !info.CoordinateAvailable {
			unique := bestUnique(exploratory, j.motifs)
			sort.Slice(unique, func(i, k int) bool { return rankBetter(unique[i], unique[k]) })
			if len(unique) > 0 {
				selected = append(selected, unique[0])
				info.ExploratorySelected = 1
			}
			info.Fallback = true
			info.Note = "query DBD coordinates unavailable; one opaque group per TF"
			if policy == MotifPolicyReps {
				info.Note += "; representative policy selects the single best exploratory motif"
			}
		} else {
			eligible, _, skipped := coordinateExploratory(exploratory)
			exploratory = eligible
			info.UncoordinatedMotifs = skipped
			if skipped > 0 {
				info.Note = fmt.Sprintf("%d exploratory motif(s) lack query coordinates and are excluded from best/reps (still present in all)", skipped)
			}
			if policy == MotifPolicyReps && !info.SpecificityAvailable {
				info.Fallback = true
				if info.Note != "" {
					info.Note += "; "
				}
				info.Note += "specificity groups unavailable; representative policy retains every exact PWM per coordinate group"
			}
			for _, group := range policyGroups(exploratory) {
				unique := bestUnique(group, j.motifs)
				if len(unique) == 0 {
					continue
				}
				sort.Slice(unique, func(i, k int) bool { return rankBetter(unique[i], unique[k]) })
				if policy == MotifPolicyReps {
					for _, row := range j.representativeRows(unique, info.SpecificityAvailable) {
						selected = append(selected, row)
						info.ExploratorySelected++
					}
				} else {
					selected = append(selected, unique[0])
					info.ExploratorySelected++
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, row := range selected {
		if !seen[row.MotifID] {
			seen[row.MotifID] = true
			info.SelectedMotifs++
		}
	}
	return info
}

func countFixed(rows []TFMappingRow) int {
	n := 0
	for _, row := range rows {
		if row.Status == "strict_rbh" || row.Status == "dbd_calibrated" {
			n++
		}
	}
	return n
}

func (j *jobState) policyMotifs(tfID string, policy MotifPolicy) []string {
	rows := j.mappingRows(tfID)
	if len(rows) == 0 {
		return nil
	}
	if policy == MotifPolicyAll {
		seen := map[string]bool{}
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if !seen[row.MotifID] {
				seen[row.MotifID] = true
				out = append(out, row.MotifID)
			}
		}
		sort.Strings(out)
		return out
	}
	fixed, exploratory := splitPolicyRows(rows, j.Params.IncludeSingleDomainFallback)
	selected := append([]TFMappingRow{}, fixed...)
	if len(exploratory) > 0 {
		if !hasCoordinates(rows) {
			unique := bestUnique(exploratory, j.motifs)
			sort.Slice(unique, func(i, k int) bool { return rankBetter(unique[i], unique[k]) })
			if len(unique) > 0 {
				selected = append(selected, unique[0])
			}
		} else {
			exploratory, _, _ = coordinateExploratory(exploratory)
			useSpecificity := j.specificityAvailable(rows)
			for _, group := range policyGroups(exploratory) {
				unique := bestUnique(group, j.motifs)
				if len(unique) == 0 {
					continue
				}
				sort.Slice(unique, func(i, k int) bool { return rankBetter(unique[i], unique[k]) })
				if policy == MotifPolicyReps {
					selected = append(selected, j.representativeRows(unique, useSpecificity)...)
				} else {
					selected = append(selected, unique[0])
				}
			}
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(selected))
	for _, row := range selected {
		if !seen[row.MotifID] {
			seen[row.MotifID] = true
			out = append(out, row.MotifID)
		}
	}
	sort.Strings(out)
	return out
}
