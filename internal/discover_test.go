package internal_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/josvazg/gobump/internal"
)

func TestFindModFiles(t *testing.T) {
	root := t.TempDir()

	// root/go.mod
	write(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.22.0\n")
	// root/sub/go.mod
	mkdir(t, filepath.Join(root, "sub"))
	write(t, filepath.Join(root, "sub", "go.mod"), "module example.com/sub\n\ngo 1.22.0\n")
	// root/sub/vendor — should be skipped
	mkdir(t, filepath.Join(root, "sub", "vendor", "pkg"))
	write(t, filepath.Join(root, "sub", "vendor", "pkg", "go.mod"), "module example.com/vendor\n\ngo 1.22.0\n")

	got, err := internal.FindModFiles(root)
	if err != nil {
		t.Fatalf("FindModFiles: %v", err)
	}
	sort.Strings(got)

	want := []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "sub", "go.mod"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFindModFiles_empty(t *testing.T) {
	got, err := internal.FindModFiles(t.TempDir())
	if err != nil {
		t.Fatalf("FindModFiles: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no results, got %v", got)
	}
}

func TestFindModFiles_defaultsToCurrentDir(t *testing.T) {
	// Passing "" should not error (falls back to ".").
	_, err := internal.FindModFiles("")
	if err != nil {
		t.Fatalf("FindModFiles with empty root: %v", err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
