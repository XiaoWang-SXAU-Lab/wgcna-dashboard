package job

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExcludedSamplesSynchronizeAndRestore(t *testing.T) {
	m := NewManager(t.TempDir(), "unused", 1)
	id, err := m.Create([]byte("gene\t a \tb\tc\td\ng\t1\t2\t3\t4\n"), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("ID\tt\nd\t40\nb\t20\na\t10\nc\t30\n")
	if err = m.UploadTrait(id, original); err != nil {
		t.Fatal(err)
	}
	if err = m.StoreMeta(id, map[string][]byte{"sample_info": []byte("sample_id\tgroup\na\tA\nb\tB\nc\tC\nd\tD\n"), "gene2go": []byte("gene\tGO\ng\tGO:1\ng\tGO:2\n")}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.dir, id)
	for _, samples := range [][]string{{"b", "a", "c"}, {"a", "c"}, {"a", "b", "c", "d"}} {
		b, _ := json.Marshal(map[string]any{"samples": samples})
		if err = os.WriteFile(filepath.Join(dir, "step1_filter.json"), b, 0644); err != nil {
			t.Fatal(err)
		}
		if err = m.SyncSampleTables(id); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"trait", "sample_info"} {
			raw, _ := os.ReadFile(filepath.Join(dir, kind+".tsv"))
			table, r := m.CheckInput(raw, kind, "")
			if !r.Valid || len(table.Rows) != len(samples) {
				t.Fatalf("%s %s %+v", kind, raw, r)
			}
			for i, s := range samples {
				if table.Rows[i][0] != s {
					t.Fatal("sample order")
				}
			}
		}
	}
	backup, _ := os.ReadFile(filepath.Join(dir, "trait.uploaded.tsv"))
	if string(backup) != string(original) {
		t.Fatal("original trait changed")
	}
	if err = m.UploadTrait(id, []byte("ID\tt\na\t1\nb\t2\nc\t3\n")); err == nil {
		t.Fatal("accepted missing original sample d")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "gene2go.tsv"))
	if string(raw) != "gene\tGO\ng\tGO:1\ng\tGO:2\n" {
		t.Fatal("gene annotations changed")
	}
}
func TestSyncRejectsMismatchesWithoutRewritingTables(t *testing.T) {
	m := NewManager(t.TempDir(), "unused", 1)
	id, err := m.Create([]byte("gene\ta\tb\tc\ng\t1\t2\t3\n"), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.UploadTrait(id, []byte("ID\tt\na\t1\nb\t2\nc\t3\n")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.dir, id)
	before, _ := os.ReadFile(filepath.Join(dir, "trait.tsv"))
	_ = os.WriteFile(filepath.Join(dir, "step1_filter.json"), []byte(`{"samples":["a","x"]}`), 0644)
	if m.SyncSampleTables(id) == nil {
		t.Fatal("accepted unknown filtered sample")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "trait.tsv"))
	if string(before) != string(after) {
		t.Fatal("changed table on validation failure")
	}
}

func TestFilterDefenseAndLegacyBackup(t *testing.T) {
	m := NewManager(t.TempDir(), "unused", 1)
	id, err := m.Create([]byte("gene\ta\tb\tc\td\ng\t1\t2\t3\t4\nh\t4\t3\t2\t1\n"), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.dir, id)
	for _, tc := range []struct {
		excluded string
		valid    bool
	}{{`["d"]`, true}, {`["c","d"]`, false}, {`["unknown"]`, false}} {
		if err = m.UpdateParams(id, json.RawMessage(`{"filter":{"exclude_samples":`+tc.excluded+`}}`)); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "params.json"))
		p, _ := decodeParams(raw)
		if err = m.validateRun(id, "filter", p); (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.excluded, err)
		}
	}
	original := []byte("ID\tt\nd\t4\nc\t3\nb\t2\na\t1\n")
	if err = os.WriteFile(filepath.Join(dir, "trait.tsv"), original, 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "step1_filter.json"), []byte(`{"samples":["a","b","c"]}`), 0644)
	if err = m.SyncSampleTables(id); err != nil {
		t.Fatal(err)
	}
	backup, _ := os.ReadFile(filepath.Join(dir, "trait.uploaded.tsv"))
	if string(backup) != string(original) {
		t.Fatal("legacy original not preserved")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "trait.tsv"))
	if string(raw) != "ID\tt\na\t1\nb\t2\nc\t3\n" {
		t.Fatalf("%s", raw)
	}
}
