package regutrace

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type donorKey struct {
	TFID    string
	MotifID string
}

// loadDonors treats tf_map.tsv as the authority for which donor IDs belong to
// this run. An uploaded trace table only enriches matching
// (tf, motif, donor ID) records with symbol/family; stale donors, extra trace
// donors and trace-only TF–motif keys are ignored. This keeps an old trace
// upload from changing the donor ID shown for the current mapping.
func loadDonors(tracePath, tfMapPath string) (map[donorKey][]DonorInfo, error) {
	base := map[donorKey][]DonorInfo{}
	var err error
	if tfMapPath != "" {
		base, err = readTFMapDonors(tfMapPath)
		if err != nil {
			return nil, err
		}
	}
	if tracePath == "" || len(base) == 0 {
		return base, nil
	}
	trace, err := readTraceDonors(tracePath)
	if err != nil {
		return nil, err
	}
	return enrichDonors(base, trace), nil
}

// enrichDonors copies symbol/family from trace only onto donor IDs already
// present in the current tf-map record. Base order and base donor IDs are
// preserved.
func enrichDonors(base, trace map[donorKey][]DonorInfo) map[donorKey][]DonorInfo {
	out := make(map[donorKey][]DonorInfo, len(base))
	for key, baseDonors := range base {
		byID := make(map[string]DonorInfo, len(baseDonors))
		order := make([]string, 0, len(baseDonors))
		for _, donor := range baseDonors {
			if _, seen := byID[donor.TFID]; seen {
				continue
			}
			byID[donor.TFID] = donor
			order = append(order, donor.TFID)
		}
		for _, traceDonor := range trace[key] {
			donor, ok := byID[traceDonor.TFID]
			if !ok {
				continue
			}
			if traceDonor.TFSymbol != "" {
				donor.TFSymbol = traceDonor.TFSymbol
			}
			if traceDonor.TFFamily != "" {
				donor.TFFamily = traceDonor.TFFamily
			}
			byID[traceDonor.TFID] = donor
		}
		merged := make([]DonorInfo, 0, len(order))
		for _, id := range order {
			merged = append(merged, byID[id])
		}
		out[key] = merged
	}
	return out
}

func readTraceDonors(path string) (map[donorKey][]DonorInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open trace table: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read trace header: %w", err)
	}
	cols := map[string]int{}
	for i, h := range headers {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		cols[h] = i
	}
	tfCol, ok := cols["tf_id"]
	if !ok {
		return nil, fmt.Errorf("trace table missing tf_id column")
	}
	motifCol, ok := cols["motif_id"]
	if !ok {
		return nil, fmt.Errorf("trace table missing motif_id column")
	}
	jsonCol, hasJSON := cols["reference_donors"]
	refCol, hasRef := cols["reference_tf_id"]
	if !hasJSON && !hasRef {
		return nil, fmt.Errorf("trace table missing reference_donors and reference_tf_id columns")
	}

	out := map[donorKey][]DonorInfo{}
	for rowNum := 2; ; rowNum++ {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read trace row %d: %w", rowNum, err)
		}
		cell := func(idx int) string {
			if idx < 0 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}
		tfID := cell(tfCol)
		motifID := cell(motifCol)
		if tfID == "" || motifID == "" {
			continue
		}
		var donors []DonorInfo
		if hasJSON && cell(jsonCol) != "" {
			if err := json.Unmarshal([]byte(cell(jsonCol)), &donors); err != nil {
				return nil, fmt.Errorf("trace row %d: invalid reference_donors JSON: %w", rowNum, err)
			}
		} else if hasRef && cell(refCol) != "" {
			for _, id := range strings.Split(cell(refCol), ";") {
				id = strings.TrimSpace(id)
				if id != "" {
					donors = append(donors, DonorInfo{TFID: id})
				}
			}
		}
		donors = dedupDonors(donors)
		if len(donors) > 0 {
			key := donorKey{TFID: tfID, MotifID: motifID}
			out[key] = append(out[key], donors...)
		}
	}
	for key, donors := range out {
		out[key] = dedupDonors(donors)
	}
	return out, nil
}

func readTFMapDonors(path string) (map[donorKey][]DonorInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tf-map: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read tf-map header: %w", err)
	}
	cols := map[string]int{}
	for i, h := range headers {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		cols[h] = i
	}
	tfCol, ok := cols["tf_id"]
	if !ok {
		return map[donorKey][]DonorInfo{}, nil
	}
	motifCol, ok := cols["motif_id"]
	if !ok {
		return map[donorKey][]DonorInfo{}, nil
	}
	refCol, ok := cols["reference_tf_id"]
	if !ok {
		return map[donorKey][]DonorInfo{}, nil
	}
	out := map[donorKey][]DonorInfo{}
	for {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read tf-map row: %w", err)
		}
		cell := func(idx int) string {
			if idx < 0 || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}
		tfID, motifID := cell(tfCol), cell(motifCol)
		if tfID == "" || motifID == "" {
			continue
		}
		for _, id := range strings.Split(cell(refCol), ";") {
			id = strings.TrimSpace(id)
			if id != "" {
				key := donorKey{TFID: tfID, MotifID: motifID}
				out[key] = dedupDonors(append(out[key], DonorInfo{TFID: id}))
			}
		}
	}
	return out, nil
}

func dedupDonors(donors []DonorInfo) []DonorInfo {
	seen := map[string]bool{}
	out := make([]DonorInfo, 0, len(donors))
	for _, d := range donors {
		d.TFID = strings.TrimSpace(d.TFID)
		d.TFSymbol = strings.TrimSpace(d.TFSymbol)
		d.TFFamily = strings.TrimSpace(d.TFFamily)
		if d.TFID == "" {
			continue
		}
		key := d.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}
