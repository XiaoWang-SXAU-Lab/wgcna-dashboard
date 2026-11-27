package regutrace

import (
	"context"
	"strings"
	"testing"
)

func TestBackgroundRejectsConcurrentExport(t *testing.T) {
	backgroundExportMu.Lock()
	defer backgroundExportMu.Unlock()
	m := NewManager(t.TempDir(), "", "")
	_, _, err := m.ExportBackground(context.Background(), "anything")
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected actionable concurrent export error, got %v", err)
	}
}
