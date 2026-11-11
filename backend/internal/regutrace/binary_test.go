package regutrace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSidecarPath(t *testing.T) {
	exe := filepath.Join("C:\\app", "rsWGCNA.exe")
	if runtime.GOOS != "windows" {
		exe = filepath.Join("/app", "rsWGCNA")
	}
	p := sidecarPath(exe)
	if filepath.Dir(p) != filepath.Dir(exe) {
		t.Fatalf("sidecar path %q not next to %q", p, exe)
	}
	if !strings.HasPrefix(filepath.Base(p), "regutrace") {
		t.Fatalf("unexpected sidecar name: %q", p)
	}
}

func sidecarData() []byte {
	if runtime.GOOS == "windows" {
		return []byte("MZ")
	}
	return []byte("regutrace")
}

func TestResolvePrefersEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), binaryName())
	if err := os.WriteFile(p, sidecarData(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGCNA_REGUTRACE", p)
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("got %q, want %q", got, p)
	}
}

func TestResolveRejectsMissingEnv(t *testing.T) {
	t.Setenv("WGCNA_REGUTRACE", filepath.Join(t.TempDir(), "missing-regutrace"))
	if _, err := Resolve(); err == nil {
		t.Fatal("expected an error for a missing WGCNA_REGUTRACE")
	}
}

func TestResolveReturnsAbsoluteEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, binaryName())
	if err := os.WriteFile(p, sidecarData(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("WGCNA_REGUTRACE", binaryName())
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("got relative path %q", got)
	}
	if got != p {
		t.Fatalf("got %q, want %q", got, p)
	}
}

func TestResolveRejectsInvalidWindowsEnv(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PE validation only")
	}
	p := filepath.Join(t.TempDir(), binaryName())
	if err := os.WriteFile(p, []byte("not a PE"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGCNA_REGUTRACE", p)
	if _, err := Resolve(); err == nil {
		t.Fatal("expected an error for a non-PE WGCNA_REGUTRACE")
	}
}

func TestResolveFindsSidecar(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := sidecarPath(exe)
	if _, err := os.Stat(p); err == nil {
		t.Skipf("sidecar already exists: %s", p)
	}
	if err := os.WriteFile(p, sidecarData(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(p) })
	t.Setenv("WGCNA_REGUTRACE", "")
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("got %q, want %q", got, p)
	}
}

func TestIsExecutable(t *testing.T) {
	p := filepath.Join(t.TempDir(), binaryName())
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(p, []byte("MZ"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !isExecutable(p, info) {
			t.Fatal("PE header rejected")
		}
		if err := os.WriteFile(p, []byte("not a PE"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err = os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if isExecutable(p, info) {
			t.Fatal("non-PE file accepted")
		}
		return
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if isExecutable(p, info) {
		t.Fatal("non-executable file accepted")
	}
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !isExecutable(p, info) {
		t.Fatal("executable file rejected")
	}
}
