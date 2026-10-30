// Package inputcheck validates and canonicalizes uploaded scientific tables.
package inputcheck

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

type Issue struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Message  string `json:"message"`
	Row      int    `json:"row,omitempty"`
	Column   int    `json:"column,omitempty"`
}
type Report struct {
	Valid        bool    `json:"valid"`
	Issues       []Issue `json:"issues"`
	ErrorCount   int     `json:"error_count"`
	WarningCount int     `json:"warning_count"`
}

func NewReport() Report { return Report{Valid: true, Issues: []Issue{}} }
func (r *Report) Add(level, file, message string, row, col int) {
	if level == "error" {
		r.Valid = false
		r.ErrorCount++
	} else {
		r.WarningCount++
	}
	if len(r.Issues) < 100 {
		r.Issues = append(r.Issues, Issue{level, file, message, row, col})
	}
}
func (r *Report) Merge(other Report) {
	r.Valid = r.Valid && other.Valid
	r.ErrorCount += other.ErrorCount
	r.WarningCount += other.WarningCount
	for _, i := range other.Issues {
		if len(r.Issues) < 100 {
			r.Issues = append(r.Issues, i)
		}
	}
}
func (r Report) Error() string {
	var lines []string
	for _, i := range r.Issues {
		if i.Severity == "error" {
			lines = append(lines, fmt.Sprintf("%s: %s (row %d, column %d)", i.File, i.Message, i.Row, i.Column))
		}
	}
	return strings.Join(lines, "\n")
}

type Table struct {
	Header []string
	Rows   [][]string
}

func (t *Table) Bytes() []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = '\t'
	_ = w.Write(t.Header)
	_ = w.WriteAll(t.Rows)
	w.Flush()
	return b.Bytes()
}
func (t *Table) IDs() []string {
	out := make([]string, len(t.Rows))
	for i, row := range t.Rows {
		out[i] = row[0]
	}
	return out
}
func (t *Table) Samples() []string { return t.Header[1:] }

// Parse accepts TSV/CSV and the first XLSX sheet. Legacy XLS is converted by
// the worker's calamine reader before reaching this function.
func Parse(raw []byte, kind, dataType string) (*Table, Report) {
	report := NewReport()
	fail := func(message string) (*Table, Report) { report.Add("error", kind, message, 0, 0); return nil, report }
	var records [][]string
	var recordLines []int
	isExcel := bytes.HasPrefix(raw, []byte{'P', 'K'})
	if bytes.HasPrefix(raw, []byte{'P', 'K'}) {
		f, err := excelize.OpenReader(bytes.NewReader(raw))
		if err != nil {
			return fail("Cannot read Excel workbook: " + err.Error())
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return fail("Excel workbook has no sheets")
		}
		records, err = f.GetRows(sheets[0], excelize.Options{RawCellValue: true})
		if err != nil {
			return fail(err.Error())
		}
		if len(sheets) > 1 {
			report.Add("warning", kind, "Only the first Excel sheet is imported: "+sheets[0], 0, 0)
		}
	} else {
		if !utf8.Valid(raw) {
			return fail("File is not UTF-8 text. Export as UTF-8 TSV/CSV or Excel (.xlsx/.xls).")
		}
		text := strings.TrimPrefix(string(raw), "\ufeff")
		var first string
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) != "" {
				first = line
				break
			}
		}
		reader := csv.NewReader(strings.NewReader(text))
		reader.FieldsPerRecord = -1
		reader.Comma = '\t'
		if strings.Count(first, ",") > strings.Count(first, "\t") {
			reader.Comma = ','
		}
		for {
			row, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fail("Malformed delimited table: " + err.Error())
			}
			line, _ := reader.FieldPos(0)
			recordLines = append(recordLines, line)
			records = append(records, row)
		}
	}
	t := &Table{}
	var lines []int
	trimmed, blank := 0, 0
	headerLine := 1
	for i, row := range records {
		empty := true
		for j, v := range row {
			clean := strings.TrimSpace(strings.TrimPrefix(v, "\ufeff"))
			if clean != v {
				trimmed++
			}
			row[j] = clean
			if clean != "" {
				empty = false
			}
		}
		if empty {
			blank++
			continue
		}
		if t.Header == nil {
			t.Header = row
			headerLine = i + 1
			if !isExcel {
				headerLine = recordLines[i]
			}
			continue
		}
		if isExcel && len(row) < len(t.Header) {
			row = append(row, make([]string, len(t.Header)-len(row))...)
		}
		t.Rows = append(t.Rows, row)
		line := i + 1
		if !isExcel {
			line = recordLines[i]
		}
		lines = append(lines, line)
	}
	if len(t.Header) < 2 {
		return fail("Expected an ID column and at least one data column; use tabs or commas as separators.")
	}
	if len(t.Rows) == 0 {
		return fail("No data rows found.")
	}
	if trimmed > 0 {
		report.Add("warning", kind, fmt.Sprintf("Removed leading/trailing whitespace or BOM from %d cells. Internal spaces and letter case are preserved.", trimmed), 0, 0)
	}
	if blank > 0 {
		report.Add("warning", kind, fmt.Sprintf("Ignored %d empty rows.", blank), 0, 0)
	}
	headerSeen := map[string]bool{}
	for j, id := range t.Header[1:] {
		if id == "" || headerSeen[id] || strings.ContainsAny(id, "\r\n\t") {
			report.Add("error", kind, "Empty, duplicate or invalid column/sample ID: "+id, headerLine, j+2)
		}
		headerSeen[id] = true
	}
	rowSeen := map[string]bool{}
	constants := 0
	negative := false
	for i, row := range t.Rows {
		line := lines[i]
		if len(row) != len(t.Header) {
			report.Add("error", kind, fmt.Sprintf("Row has %d columns; header has %d. Check missing or extra cells.", len(row), len(t.Header)), line, 0)
			continue
		}
		if (kind == "expr" || kind == "trait" || kind == "sample_info") && rowSeen[row[0]] {
			report.Add("error", kind, "Duplicate row ID: "+row[0], line, 1)
		}
		rowSeen[row[0]] = true
		if row[0] == "" {
			report.Add("error", kind, "Empty row ID", line, 1)
		}
		if strings.ContainsAny(row[0], "\r\n\t") {
			report.Add("error", kind, "Row ID contains a tab or newline", line, 1)
		}
		if kind == "expr" || kind == "trait" {
			constant := true
			first := 0.0
			for j, cell := range row[1:] {
				n, err := strconv.ParseFloat(cell, 64)
				if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
					report.Add("error", kind, fmt.Sprintf("Invalid numeric value %q for ID %q, column %q. Missing values (NA/NaN), text and infinity are not accepted.", cell, row[0], t.Header[j+1]), line, j+2)
					continue
				}
				if j == 0 {
					first = n
				} else if n != first {
					constant = false
				}
				if kind == "expr" {
					if n < 0 {
						if dataType == "normalized_count" {
							negative = true
						} else {
							report.Add("error", kind, "Negative expression value for "+row[0]+" / "+t.Header[j+1], line, j+2)
						}
					}
					if dataType == "count" && n != math.Trunc(n) {
						report.Add("error", kind, "Raw count must be an integer. If these are estimated counts, select Expected count; for TPM/FPKM select Normalized count.", line, j+2)
					}
				}
			}
			if constant {
				constants++
			}
		}
	}
	if kind == "expr" && len(t.Header) < 4 {
		report.Add("error", kind, "WGCNA requires at least 3 sample columns (genes in rows, samples in columns).", 1, 0)
	}
	if constants > 0 && kind == "expr" {
		report.Add("warning", kind, fmt.Sprintf("%d genes have zero variance across samples; filtering may remove them.", constants), 0, 0)
	}
	if negative {
		report.Add("warning", kind, "Negative normalized expression detected. Use raw normalization for already transformed/standardized values; logarithmic transforms may be invalid.", 0, 0)
	}
	if kind == "trait" && report.Valid {
		for j := 1; j < len(t.Header); j++ {
			constant := true
			for _, row := range t.Rows[1:] {
				n, _ := strconv.ParseFloat(row[j], 64)
				first, _ := strconv.ParseFloat(t.Rows[0][j], 64)
				if n != first {
					constant = false
					break
				}
			}
			if constant {
				report.Add("warning", kind, "Trait column has zero variance; its correlations are undefined: "+t.Header[j], headerLine, j+1)
			}
		}
	}
	if !report.Valid {
		return nil, report
	}
	return t, report
}

// Align requires every retained sample. Only explicitly excluded IDs may be
// discarded; arbitrary mismatches never silently pass validation.
func Align(t *Table, samples, excluded []string, kind string) (*Table, Report) {
	report := NewReport()
	index := map[string][]string{}
	keep := map[string]bool{}
	allowed := map[string]bool{}
	for _, id := range excluded {
		allowed[id] = true
	}
	for _, id := range samples {
		keep[id] = true
	}
	for _, row := range t.Rows {
		index[row[0]] = row
	}
	missing, extra := []string{}, []string{}
	out := &Table{Header: t.Header}
	for _, id := range samples {
		if row, ok := index[id]; ok {
			out.Rows = append(out.Rows, row)
		} else {
			missing = append(missing, id)
		}
	}
	for _, row := range t.Rows {
		if !keep[row[0]] && !allowed[row[0]] {
			extra = append(extra, row[0])
		}
	}
	describe := func(ids []string) string {
		n := len(ids)
		if n > 20 {
			ids = ids[:20]
		}
		return fmt.Sprintf("%d [%s]%s", n, strings.Join(ids, ", "), map[bool]string{true: " …", false: ""}[n > 20])
	}
	if len(missing) > 0 {
		report.Add("error", kind, "Missing expression samples: "+describe(missing), 0, 0)
	}
	if len(extra) > 0 {
		report.Add("error", kind, "Unmatched extra samples: "+describe(extra), 0, 0)
	}
	if report.Valid {
		reordered := len(out.Rows) != len(t.Rows)
		if !reordered {
			for i, row := range out.Rows {
				if row[0] != t.Rows[i][0] {
					reordered = true
					break
				}
			}
		}
		if reordered {
			report.Add("warning", kind, "Rows aligned to expression sample order; explicitly excluded samples removed.", 0, 0)
		}
	}
	return out, report
}
