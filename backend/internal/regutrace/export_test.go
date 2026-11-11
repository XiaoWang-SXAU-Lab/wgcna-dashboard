package regutrace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExportFormatStatusErrors(t *testing.T) {
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence.1.tsv")
	if err := os.WriteFile(evidence, []byte("tf_id\ttarget_id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := &Manager{
		dir:  dir,
		jobs: map[string]*jobState{},
	}
	mgr.jobs["done"] = &jobState{
		Job:          Job{ID: "done", Status: "succeeded"},
		dir:          dir,
		generation:   1,
		evidencePath: evidence,
	}
	mgr.jobs["running"] = &jobState{
		Job:        Job{ID: "running", Status: "running"},
		dir:        dir,
		generation: 2,
	}

	if _, _, err := mgr.ExportFormat(context.Background(), "missing", "evidence", "csv"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing job error = %v", err)
	}
	if _, _, err := mgr.ExportFormat(context.Background(), "running", "evidence", "csv"); !errors.Is(err, ErrNotCompleted) {
		t.Fatalf("running job error = %v", err)
	}
	if _, _, err := mgr.ExportFormat(context.Background(), "done", "evidence", "parquet"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unknown format error = %v", err)
	}
	if _, _, err := mgr.ExportFormat(context.Background(), "done", "unknown", ""); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unknown kind error = %v", err)
	}
	path, _, err := mgr.ExportFormat(context.Background(), "done", "evidence", "tsv")
	if err != nil || path != evidence {
		t.Fatalf("tsv export = %q, %v", path, err)
	}
	if err := mgr.ensureExportCurrent("done", 1); err != nil {
		t.Fatalf("current generation rejected: %v", err)
	}
	if err := mgr.ensureExportCurrent("done", 2); !errors.Is(err, ErrNotCompleted) {
		t.Fatalf("stale generation error = %v", err)
	}
}
