package inputcheck

import (
	"github.com/xuri/excelize/v2"
	"strings"
	"testing"
)

func TestCleanupAndAlignedValues(t *testing.T) {
	expr, r := Parse([]byte("\ufeffgene\t a \tb\tc\n\t\t\t\n g1 \t1\t2\t3\n"), "expr", "count")
	if !r.Valid || r.WarningCount == 0 || expr.Samples()[0] != "a" {
		t.Fatalf("%+v", r)
	}
	trait, r := Parse([]byte("ID,t\nc,30\na,10\nb,20\n"), "trait", "")
	if !r.Valid {
		t.Fatal(r)
	}
	out, r := Align(trait, []string{"b", "a"}, []string{"c"}, "trait")
	if !r.Valid || string(out.Bytes()) != "ID\tt\nb\t20\na\t10\n" {
		t.Fatalf("%s %+v", out.Bytes(), r)
	}
	_, r = Align(trait, []string{"a", "missing"}, nil, "trait")
	if r.Valid || !strings.Contains(r.Error(), "missing") || !strings.Contains(r.Error(), "extra") {
		t.Fatalf("%+v", r)
	}
}
func TestRejectUnsafeValuesAndIDs(t *testing.T) {
	for _, body := range []string{
		"gene\ta\t a \tc\ng\t1\t2\t3\n",
		"gene\ta\tb\tc\ng\t1\t2\tNaN\n",
		"gene\ta\tb\tc\ng\t1\tInf\t3\n",
		"gene\ta\tb\tc\ng\t1\t\t3\n",
		"gene\ta\tb\tc\ng\t-1\t2\t3\n",
		"gene\ta\tb\tc\ng\t1.5\t2\t3\n",
		"gene\ta\tb\tc\ng\t1\t2\t3\t4\n",
		"gene\ta\tb\tc\ng\t1\t2\t3\n g \t2\t3\t4\n",
	} {
		if _, r := Parse([]byte(body), "expr", "count"); r.Valid {
			t.Errorf("accepted %q", body)
		}
	}
	if _, r := Parse([]byte("gene\ta\tb\tc\ng\t1.5\t2\t3\n"), "expr", "expected_count"); !r.Valid {
		t.Fatal(r)
	}
	if _, r := Parse([]byte("ID\tt\na\ttext\n"), "trait", ""); r.Valid {
		t.Fatal("accepted categorical trait")
	}
}
func TestExcelAndMappingMultiplicity(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	for cell, v := range map[string]interface{}{"A1": "gene", "B1": " a ", "C1": "b", "D1": "c", "A2": "g", "B2": 1, "C2": 2, "D2": 3} {
		if err := f.SetCellValue("Sheet1", cell, v); err != nil {
			t.Fatal(err)
		}
	}
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	table, r := Parse(b.Bytes(), "expr", "count")
	if !r.Valid || table.Header[1] != "a" {
		t.Fatal(r)
	}
	if _, r := Parse([]byte("gene\tGO\ng\tGO:1\ng\tGO:2\n"), "gene2go", ""); !r.Valid {
		t.Fatal(r)
	}
}
